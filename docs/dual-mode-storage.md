# 双模式存储架构

## 概述

项目支持两种部署模式，根据服务器规格和使用场景灵活选择：

| | Mode A: local (轻量) | Mode B: s3 (完整) |
|--|---|---|
| **适用场景** | 个人/小团队，单机部署 | 企业级，多节点可扩展 |
| **数据库** | SQLite (零运维) | MySQL 8.0 |
| **消息队列** | 进程内 goroutine channel | Kafka |
| **主存** | 本地磁盘 (直接写文件) | SeaweedFS (S3 API) |
| **冷存** | 阿里云 OSS (可选) | 阿里云 OSS |
| **下载方式** | Gateway 流式代理 (StreamFileContent RPC) | SeaweedFS/OSS 预签名 URL 直连 |
| **依赖服务** | Consul + Redis | Consul + Redis + MySQL + Kafka + SeaweedFS |

## 架构图

### Mode A: local

```
Client ──HTTP──▶ Gateway (:8080)
                    │ gRPC
        ┌───────────┴───────────┐
        ▼                       ▼
  User Service            File Service
  (SQLite)                (SQLite + 本地磁盘)
                               │
                          ┌────┴────┐
                          │  Redis  │
                          └─────────┘
                               │
                     goroutine MQ (进程内)
                               │
                          ┌────┴────┐
                          │阿里云 OSS│ (可选冷迁移)
                          └─────────┘

  ← ─ ─ Consul 服务发现 ─ ─ →

  下载: Gateway stream proxy (gRPC server-streaming)
```

### Mode B: s3

```
Client ──HTTP──▶ Gateway (:8080)
                    │ gRPC
        ┌───────────┴───────────┐
        ▼                       ▼
  User Service            File Service
  (MySQL)                 (MySQL + SeaweedFS)
                               │
                    ┌──────────┼──────────┐
                    ▼          ▼          ▼
                 MySQL      Redis    SeaweedFS
                                        │
                                  Kafka (cloud-migrate)
                                        │
                                   file-worker
                                        │
                                  ┌─────┴─────┐
                                  │ 阿里云 OSS │
                                  └───────────┘

  ← ─ ─ Consul 服务发现 ─ ─ →

  下载: SeaweedFS/OSS 预签名 URL (直连，不经 Gateway)
```

## 核心实现

### 1. 可插拔数据库 (MySQL / SQLite)

通过 `DB_DRIVER` 环境变量（或 config.yaml `data.database.driver`）选择数据库：

```go
// data/data.go - openDB helper
func openDB(c *conf.Data_Database, logger log.Logger) gorm.Dialector {
    driver := os.Getenv("DB_DRIVER")
    if driver == "" && c != nil {
        driver = c.Driver
    }
    if driver == "" {
        driver = "mysql"
    }
    switch driver {
    case "sqlite":
        dbPath := os.Getenv("SQLITE_PATH")
        if dbPath == "" {
            dbPath = "/app/data/cloud_disk.db"
        }
        return sqlite.Open(dbPath)
    default: // mysql
        dsn := buildMysqlDSN(c)
        return mysql.Open(dsn)
    }
}
```

GORM AutoMigrate 确保两种 DB 的表结构自动创建/迁移。

### 2. 可插拔消息队列 (Kafka / goroutine)

`NewMessageProducer` 工厂函数根据 `KAFKA_BROKERS` 是否配置自动选择：

```go
func NewMessageProducer(c *conf.Data, repo biz.FileRepo,
    objStore biz.ObjectStorage, cloudStore biz.CloudStorage,
    logger log.Logger) biz.MessageProducer {
    brokers := os.Getenv("KAFKA_BROKERS")
    if brokers != "" {
        return newKafkaProducer(brokers, logger)  // 真正的 Kafka
    }
    return newGoroutineMQ(repo, objStore, cloudStore, logger)  // 进程内 channel
}
```

**goroutine MQ** 使用带缓冲的 channel (容量 256) + 后台 goroutine 消费：
- `cloud-migrate` channel → `consumeCloudMigrate()` goroutine
- `file-thumbnail` channel → `consumeThumbnails()` goroutine

跟 Kafka 版 file-worker 逻辑一致：下载主存文件 → 上传 OSS → 更新 DB → 删除主存 → 递减 Redis 计数器。

### 3. 双模式上传

`FileUsecase.uploadMergedFile()` 根据 `StorageConfig.Mode` 分流：

```go
func (uc *FileUsecase) uploadMergedFile(ctx context.Context, key string, f *os.File) (string, error) {
    switch uc.storageCfg.Mode {
    case ModeLocal:
        // 硬链接/拷贝到 storeDir，StorageType = "local"
        dest := filepath.Join(uc.storeDir, key)
        os.MkdirAll(filepath.Dir(dest), 0o755)
        linkOrCopy(f.Name(), dest)
        return StorageLocal, nil

    default: // ModeS3
        // 上传 SeaweedFS，失败则兜底 OSS
        err := uc.objStore.Put(ctx, key, f)
        if err != nil {
            err = uc.cloudStore.Put(ctx, key, f)
            return "oss", err
        }
        return StorageSeaweedFS, nil
    }
}
```

### 4. 双模式下载

**Mode A (local)**: `GetDownloadURL` 返回 `local://path`，Gateway 将其重写为 `/api/v1/file/stream/{fileId}` 端点，由 `StreamFile` handler 调用 `StreamFileContent` server-streaming RPC 流式传输文件内容。

**Mode B (s3)**: `GetDownloadURL` 返回 SeaweedFS 或 OSS 预签名 URL，浏览器直接下载。

前端 `stores/file.ts` 的 `downloadFile` 自动检测 URL 格式：
- 以 `local://` 开头 → 调用 `fileApi.downloadBlob()` 经 Gateway 代理下载
- 正常 URL → `window.open(url)` 直接下载

### 5. 存储配置

`provideStorageConfig` 支持多级配置源，优先级从低到高：

1. 代码硬编码默认值 (Mode=local, 10GB, 80%, 90%)
2. config.yaml 中的 `storage` 配置
3. 环境变量 `STORAGE_MODE` / `PRIMARY_MAX_BYTES`

```go
func provideStorageConfig(c *conf.Storage) *biz.StorageConfig {
    cfg := &biz.StorageConfig{
        Mode:            biz.ModeLocal,
        PrimaryMaxBytes: 10 * 1024 * 1024 * 1024,
        ThresholdPct:    80,
        EvictTargetPct:  90,
    }
    // YAML config overrides
    if c != nil { ... }
    // Env overrides (highest priority)
    if m := os.Getenv("STORAGE_MODE"); m != "" { cfg.Mode = m }
    if v := os.Getenv("PRIMARY_MAX_BYTES"); v != "" { ... }
    return cfg
}
```

## Docker Compose 双模式部署

使用 Compose profiles 实现一份 compose 文件支持两种模式：

```bash
# Mode A: 轻量模式 (SQLite + 本地磁盘 + 进程内 MQ)
docker compose --env-file .env.local up -d

# Mode B: 完整模式 (MySQL + SeaweedFS + Kafka)
docker compose --env-file .env.s3 --profile s3 up -d
```

`profiles: [s3]` 标记的服务只在 `--profile s3` 时启动：
- mysql
- kafka
- seaweedfs
- file-worker

两种模式都会启动的服务：
- consul
- redis
- user-service
- file-service
- gateway
- frontend

服务的 DB/存储/MQ 配置通过 `.env` 文件中的环境变量控制。

### .env.local 示例

```env
DB_DRIVER=sqlite
SQLITE_PATH=/app/data/cloud_disk.db
STORAGE_MODE=local
PRIMARY_MAX_BYTES=10737418240
FILE_STORE_DIR=/app/store
```

### .env.s3 示例

```env
DB_DRIVER=mysql
STORAGE_MODE=s3
PRIMARY_MAX_BYTES=107374182400
KAFKA_BROKERS=kafka:9092
SEAWEEDFS_ENDPOINT=http://seaweedfs:8333
SEAWEEDFS_BUCKET=light-cloud-disk
```

## StreamFileContent RPC (本地模式下载)

Mode A 中文件存储在 File Service 本地磁盘，前端无法直连。通过 gRPC server-streaming RPC 流式传输：

```protobuf
// file.proto
rpc StreamFileContent(StreamFileContentRequest) returns (stream StreamFileContentReply);

message StreamFileContentRequest {
    int64 user_id = 1;
    int64 file_id = 2;
}

message StreamFileContentReply {
    bytes chunk = 1;          // 64KB 分片
    string file_name = 2;     // 仅首条消息
    int64 file_size = 3;      // 仅首条消息
    string content_type = 4;  // 仅首条消息
}
```

Gateway 的 `StreamFile` handler：
1. 调用 `StreamFileContent` RPC
2. 读取首条消息获取文件元数据，设置 HTTP headers
3. 循环 `Recv()` 并 `c.Writer.Write(chunk)` 流式写入 HTTP response

## 降级策略

| 场景 | 行为 |
|------|------|
| 未配置 SeaweedFS | `noopObjectStorage`，Mode B 上传全部直传 OSS |
| 未配置 OSS | `noopCloudStorage`，不执行冷迁移 |
| 未配置 Kafka | `goroutineMQ`，进程内 channel 替代 |
| DB_DRIVER 为空 | 默认 MySQL |
| STORAGE_MODE 为空 | 默认 local |
| 本地磁盘满 | `CheckUpload` 返回 `disk_full=true`，Gateway 503 |

## 面试要点

1. **为什么做双模式？** — 适配不同部署场景。个人用户不想装 MySQL+Kafka+SeaweedFS，单机 SQLite+本地磁盘即可跑；企业用户需要可扩展的对象存储和消息队列。
2. **goroutine MQ 和 Kafka 的差异？** — goroutine MQ 是进程内 channel，消息不持久化，重启丢失。Kafka 是分布式持久化 MQ。对于单机轻量部署来说，冷迁移消息丢失可接受（重启后自动触发淘汰检查）。
3. **SQLite 的并发问题怎么处理？** — GORM + SQLite WAL 模式支持单写多读。单机部署场景并发不高，够用。MySQL 模式下无此限制。
4. **本地文件下载为什么用 server-streaming？** — 文件可能很大，不能全部加载到内存。gRPC server-streaming 分片传输（64KB/片），Gateway 逐片写入 HTTP response，内存开销恒定。
5. **配置优先级？** — 代码默认值 < config.yaml < 环境变量。容器部署时 docker-compose 通过 env_file 注入环境变量覆盖一切。
