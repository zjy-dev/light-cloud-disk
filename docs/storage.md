# 存储架构

## 概述

项目采用**本地磁盘 + 阿里云 OSS** 的两层存储架构，配合纠删码（Reed-Solomon 4+2）和 LRU 冷热淘汰。

| 层 | 存储 | 用途 | 特点 |
|----|------|------|------|
| 主存 (热/温) | 本地磁盘 (`FILE_STORE_DIR`) | 合并后文件 + EC 分片 | 低延迟，容量有限 |
| 冷存 | 阿里云 OSS | LRU 淘汰后归档 | 无限容量，按量付费 |

## 核心实现

### 1. 可插拔数据库 (MySQL / SQLite)

通过 `DB_DRIVER` 环境变量选择数据库：

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

### 2. 上传流程

`FileUsecase.uploadMergedFile()` 根据磁盘用量选择存储目标：

```go
func (uc *FileUsecase) uploadMergedFile(ctx, mergedPath, objectKey, fileSize) (storageType, storePath) {
    // 主存空间够 → hardlink/copy 到 FILE_STORE_DIR, IncrDiskUsage("local", +fileSize)
    // 主存满 → 直传 OSS (兜底), storage_type = "oss"
}
```

合并后若文件 ≥ `ERASURE_MIN_FILE_SIZE` (默认 1 MB) 且存储在本地，则调用 `encodeWithErasure()` 生成 4+2 纠删码分片，`storage_type` 变为 `local_ec`。

### 3. 下载流程

**本地文件 (`local` / `local_ec`)**: `GetDownloadURL` 返回 `local://path`，Gateway 的 `StreamFile` handler 调用 `StreamFileContent` server-streaming RPC 流式传输。`local_ec` 类型会先从纠删码分片重建原始文件。

**OSS 文件 (`oss`)**: `GetDownloadURL` 返回 OSS 预签名 URL，浏览器直接下载。

前端 `stores/file.ts` 的 `downloadFile` 自动检测 URL 格式：
- 以 `local://` 开头 → 调用 `fileApi.downloadBlob()` 经 Gateway 代理下载
- 正常 URL → `window.open(url)` 直接下载

### 4. 存储配置

`provideStorageConfig` 支持多级配置源，优先级从低到高：

1. 代码硬编码默认值 (10GB, 80%, 90%)
2. config.yaml 中的 `storage` 配置
3. 环境变量 `PRIMARY_MAX_BYTES`

## StreamFileContent RPC (本地文件下载)

文件存储在 File Service 本地磁盘，前端无法直连。通过 gRPC server-streaming RPC 流式传输：

```protobuf
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
| 未配置 OSS | `noopCloudStorage`，不执行冷迁移，文件留在本地 |
| ErasureEncoder 为 nil | 跳过纠删码编码，文件按 `local` 原始存储 |
| DB_DRIVER 为空 | 默认 MySQL |
| 本地磁盘满 | `CheckUpload` 返回 `disk_full=true` + `upload_mode="presigned"`，客户端直传 OSS |

## 面试要点

1. **SQLite 的并发问题怎么处理？** — GORM + SQLite WAL 模式支持单写多读。单机部署场景并发不高，够用。MySQL 模式下无此限制。
2. **本地文件下载为什么用 server-streaming？** — 文件可能很大，不能全部加载到内存。gRPC server-streaming 分片传输（64KB/片），Gateway 逐片写入 HTTP response，内存开销恒定。
3. **配置优先级？** — 代码默认值 < config.yaml < 环境变量。容器部署时 docker-compose 通过 env_file 注入环境变量覆盖一切。
4. **纠删码和存储的关系？** — 文件 ≥ 1 MB 在本地存储时进行 Reed-Solomon 4+2 编码，原始文件删除后只保留分片。下载时自动重建，容忍任意 2 个分片丢失。详见 [erasure-coding.md](erasure-coding.md)。
5. **冷热分层怎么做？** — 本地磁盘用量超 80% 时 LRU 淘汰到 OSS。详见 [cold-hot-storage.md](cold-hot-storage.md)。
