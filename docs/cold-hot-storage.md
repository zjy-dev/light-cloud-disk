# 冷热分层存储

## 概述

两种部署模式都采用**两层存储**——主存（热/温）+ 阿里云 OSS（冷），通过 LRU 淘汰把最久未访问的文件从主存异步搬到 OSS。本地磁盘在 Mode B 里只做分块暂存，合并后立即上传 SeaweedFS 并删除本地文件，不算独立存储层。

详见 [dual-mode-storage.md](dual-mode-storage.md)。

| 模式 | 主存（热/温） | 冷存 | MQ |
|------|--------------|------|-----|
| Mode A (local) | 本地磁盘 (`/app/store/`) | 阿里云 OSS (可选) | goroutine channel |
| Mode B (s3) | SeaweedFS (S3 API) | 阿里云 OSS | Kafka → file-worker |

## 架构图 (Mode B: s3)

```
┌──────────────────────────────────────────────────────────────────┐
│                      File Service (gRPC :9002)                  │
│                                                                  │
│  SaveChunk():                                                    │
│    分块写入 FILE_TMP_DIR (临时缓冲)                              │
│                                                                  │
│  MergeChunks():                                                  │
│    合并分块 ──▶ 上传 SeaweedFS ──▶ 删除本地临时文件              │
│                  ▲ 满? → 直传 OSS 兜底                           │
│                                                                  │
│  maybeEvictToCloud():                                            │
│    SeaweedFS 用量 > 80% of PRIMARY_MAX_BYTES                    │
│    → FindLRUStores("seaweedfs", 100)                            │
│    → 发 Kafka cloud-migrate 消息                                 │
│                                                                  │
│  file-worker (独立进程):                                         │
│    下载 SeaweedFS → 上传 OSS → 更新 DB → 删 SeaweedFS → 递减计数│
└──────────────────────────────────────────────────────────────────┘

         本地磁盘              主存 (热/温)              冷存
    ┌──────────────┐    ┌──────────────────┐    ┌──────────────┐
    │  FILE_TMP_DIR│    │   SeaweedFS      │    │  阿里云 OSS  │
    │  (分块暂存)   │───▶│   (S3 兼容)      │───▶│  (冷归档)    │
    │  合并后即删   │    │  PRIMARY_MAX     │    │   无限       │
    └──────────────┘    └──────────────────┘    └──────────────┘
      临时缓冲区            disk_usage:seaweedfs     无限容量
      不计入用量             Redis 计数器
```

## 架构图 (Mode A: local)

```
┌──────────────────────────────────────────────────────────────────┐
│                      File Service (gRPC :9002)                  │
│                                                                  │
│  MergeChunks():                                                  │
│    合并分块 → hardlink/copy 到 FILE_STORE_DIR                   │
│    IncrDiskUsage("local", fileSize)                              │
│    ▲ 磁盘满? → uploadToOSS() 直传 OSS                           │
│                                                                  │
│  maybeEvictToCloud():                                            │
│    local 用量 > 80% of PRIMARY_MAX_BYTES                        │
│    → FindLRUStores("local", 100)                                │
│    → goroutine channel → 读本地 → 传 OSS → 改 DB → 删本地       │
└──────────────────────────────────────────────────────────────────┘

              主存 (热/温)                     冷存
    ┌──────────────────────────┐       ┌──────────────┐
    │     本地磁盘              │       │  阿里云 OSS  │
    │  FILE_STORE_DIR          │──────▶│  (冷归档)    │
    │  PRIMARY_MAX_BYTES       │       │   无限       │
    └──────────────────────────┘       └──────────────┘
      disk_usage:local                     无限容量
      Redis 计数器
```

## 存储层说明

### Mode B (s3) — 两层

| 层 | 存储 | 用途 | 容量 | Redis Key | 实现类 |
|----|------|------|------|-----------|--------|
| 主存 | SeaweedFS | 合并后文件持久存放 | `PRIMARY_MAX_BYTES` (默认 10GB) | `disk_usage:seaweedfs` | `seaweedfsClient` (aws-sdk-go-v2/s3) |
| 冷存 | 阿里云 OSS | LRU 淘汰后的归档 | 无限 | — | `ossClient` (alibabacloud-oss-go-sdk-v2) |

本地磁盘 (`FILE_TMP_DIR`) 只存分块暂存文件，合并上传 SeaweedFS 后立即 `os.Remove(mergedPath)`，不做持久存储。

### Mode A (local) — 两层

| 层 | 存储 | 用途 | 容量 | Redis Key | 实现类 |
|----|------|------|------|-----------|--------|
| 主存 | 本地磁盘 | 合并后文件持久存放 | `PRIMARY_MAX_BYTES` (默认 10GB) | `disk_usage:local` | 文件系统 (`FILE_STORE_DIR`) |
| 冷存 | 阿里云 OSS | LRU 淘汰后的归档 (可选) | 无限 | — | `ossClient` (alibabacloud-oss-go-sdk-v2) |

## 核心流程

### 1. 上传模式选择（CheckUpload）

```
CheckUpload(fileMD5, fileSize)
│
├── MD5 命中 file_store → 秒传 (upload_mode="direct")
│
├── Mode B (s3) → upload_mode="presigned"
│   （数据不过后端，直传 SeaweedFS/OSS）
│
└── Mode A (local)
    ├── disk_usage:local + fileSize ≤ PRIMARY_MAX_BYTES
    │   → diskFull=false, upload_mode="direct"
    └── disk_usage:local + fileSize > PRIMARY_MAX_BYTES
        → diskFull=true, upload_mode="presigned" (降级直传 OSS)
```

### 2. 合并上传 (uploadMergedFile)

`MergeChunks` 合并分块后调用 `uploadMergedFile`，根据模式和用量选择目标：

```go
func (uc *FileUsecase) uploadMergedFile(ctx, mergedPath, objectKey, fileSize) (storageType, storePath) {
    if Mode == "s3" {
        // 主存空间够 → 上传 SeaweedFS, IncrDiskUsage("seaweedfs", +fileSize)
        // 主存满 → 直传 OSS (兜底)
    } else { // Mode == "local"
        // 主存空间够 → hardlink/copy 到 FILE_STORE_DIR, IncrDiskUsage("local", +fileSize)
        // 主存满 → 直传 OSS (兜底)
    }
}
```

Mode B 中合并后还会删除本地临时文件：

```go
if storageType != StorageLocal {
    _ = os.Remove(mergedPath)
    _ = uc.repo.IncrDiskUsage(ctx, "local", -fileSize)
}
```

### 3. LRU 淘汰 (maybeEvictToCloud)

`MergeChunks` 结束后调用，两种模式逻辑一致——只是 `primaryDiskType()` 返回 `"local"` 或 `"seaweedfs"`：

```go
func (uc *FileUsecase) maybeEvictToCloud(ctx context.Context, currentUsed int64) {
    threshold := PrimaryMaxBytes * ThresholdPct / 100   // 默认 80%
    if currentUsed <= threshold { return }

    evictTarget := threshold * EvictTargetPct / 100     // 默认降到阈值的 90% (即总容量的 72%)
    toFree := currentUsed - evictTarget

    primaryType := primaryDiskType()                    // "local" or "seaweedfs"
    stores := repo.FindLRUStores(ctx, primaryType, 100) // 按 last_accessed_at ASC

    var freed int64
    for _, s := range stores {
        if freed >= toFree { break }
        mq.SendCloudMigrateMessage(ctx, &CloudMigrateMessage{
            FileMD5:   s.FileMD5,
            SourceKey: s.StorePath,
            FileSize:  s.Size,
        })
        freed += s.Size
    }
}
```

**淘汰参数**：

| 参数 | 默认值 | 含义 |
|------|--------|------|
| `ThresholdPct` | 80 | 主存用量超过 80% 触发淘汰 |
| `EvictTargetPct` | 90 | 淘汰到阈值的 90%，即总容量的 72%，减少反复触发 |

### 4. 异步迁移

两种模式逻辑相同，只是数据源不同：

**Mode A (goroutine MQ, 进程内 channel)**：
```
consumeCloudMigrate:
  1. os.Open(本地文件)
  2. cloudStore.Put(key, data) → 上传 OSS
  3. UPDATE file_store SET storage_type='oss'
  4. os.Remove(本地文件)
  5. IncrDiskUsage("local", -size)
```

**Mode B (Kafka → file-worker 独立进程)**：
```
handleCloudMigrateMessage:
  1. objStore.Get(key) → 从 SeaweedFS 下载
  2. cloudStore.Put(key, data) → 上传 OSS
  3. UPDATE file_store SET storage_type='oss'
  4. objStore.Delete(key) → 删除 SeaweedFS 数据
  5. IncrDiskUsage("seaweedfs", -size)
```

### 5. 下载（按 StorageType 路由）

下载时先刷新 `last_accessed_at`（LRU 时间戳），再根据 `storage_type` 字段选路径：

```go
func (uc *FileUsecase) GetDownloadURL(ctx, userID, fileID) (string, string, error) {
    store := repo.FindStoreByMD5(ctx, file.FileMD5)
    repo.UpdateLastAccessed(ctx, file.FileMD5)

    switch store.StorageType {
    case "local":
        return "local://" + store.StorePath  // Gateway 走 gRPC streaming 代理
    case "oss":
        return cloudStore.PresignGetURL(ctx, store.StorePath, 1h)
    default: // "seaweedfs"
        return objStore.PresignGetURL(ctx, store.StorePath, 1h)
    }
}
```

- `local://` → Gateway 调 `StreamFileContent` server-streaming RPC，流式代理给客户端
- `seaweedfs`/`oss` → 返回预签名 URL，客户端直连下载

## 数据模型

### file_store 关键字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `storage_type` | VARCHAR(32) | `"local"` / `"seaweedfs"` / `"oss"` |
| `store_path` | VARCHAR(255) | 存储 key，格式 `{md5}/{filename}` |
| `last_accessed_at` | DATETIME | 每次下载更新，LRU 淘汰排序依据 |

### Redis 用量计数器

| Key | 说明 | 增 | 减 |
|-----|------|-----|-----|
| `disk_usage:local` | 本地磁盘已用量 (Mode A) | `uploadMergedFile` 写入 storeDir 后 | goroutine MQ 迁移完成后 |
| `disk_usage:seaweedfs` | SeaweedFS 已用量 (Mode B) | `uploadMergedFile` 上传 SeaweedFS 后 | file-worker 迁移完成后 |

操作方式 `INCRBY`，支持正负 delta，原子性保证并发安全。

## 接口层

### biz 层接口

```go
// ObjectStorage — SeaweedFS (Mode B 主存)
type ObjectStorage interface {
    Put(ctx context.Context, key string, r io.Reader, size int64) error
    Delete(ctx context.Context, key string) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// CloudStorage — 阿里云 OSS (冷存)
type CloudStorage interface {
    Put(ctx context.Context, key string, r io.Reader, size int64) error
    Delete(ctx context.Context, key string) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}
```

### gRPC (file.proto)

```protobuf
rpc GetDiskUsage(GetDiskUsageRequest) returns (GetDiskUsageReply);

message CheckUploadReply {
    bool can_fast_upload = 1;
    repeated int32 uploaded_chunks = 2;
    bool disk_full = 3;
    string upload_mode = 4; // "direct" | "presigned"
}

message GetDiskUsageReply {
    int64 primary_used_bytes = 1;
    int64 primary_max_bytes = 2;
    string primary_type = 3; // "local" or "seaweedfs"
}
```

### HTTP

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/disk-usage | 返回主存用量、上限、类型 |

## 配置

### StorageConfig (biz 层)

```go
type StorageConfig struct {
    Mode            string // "local" | "s3"
    PrimaryMaxBytes int64  // 主存上限 (10GB default)
    ThresholdPct    int32  // 淘汰触发阈值 (80%)
    EvictTargetPct  int32  // 淘汰目标 (阈值的 90%, 即总容量 72%)
}
```

优先级：环境变量 > config.yaml > 代码默认值。

### 环境变量

| 变量 | 说明 | 服务 | 默认值 |
|------|------|------|--------|
| `STORAGE_MODE` | 存储模式 | file | local |
| `PRIMARY_MAX_BYTES` | 主存上限 (字节) | file | 10737418240 (10GB) |
| `FILE_TMP_DIR` | 分块临时目录 | file | /app/tmp |
| `FILE_STORE_DIR` | 本地合并文件目录 (Mode A) | file | /app/store |
| `SEAWEEDFS_ENDPOINT` | SeaweedFS S3 端点 | file, worker | - |
| `SEAWEEDFS_REGION` | S3 Region | file, worker | us-east-1 |
| `SEAWEEDFS_BUCKET` | S3 Bucket | file, worker | light-cloud-disk |
| `SEAWEEDFS_ACCESS_KEY` | S3 Access Key | file, worker | - |
| `SEAWEEDFS_SECRET_KEY` | S3 Secret Key | file, worker | - |
| `OSS_ENDPOINT` | 阿里云 OSS 端点 | file, worker | - |
| `OSS_REGION` | OSS Region | file, worker | - |
| `OSS_BUCKET` | OSS Bucket | file, worker | - |
| `OSS_ACCESS_KEY_ID` | OSS AK | file, worker | - |
| `OSS_ACCESS_KEY_SECRET` | OSS SK | file, worker | - |

## 降级策略

| 场景 | 行为 |
|------|------|
| SeaweedFS 未配置 | `noopObjectStorage`，Mode B 退化为直传 OSS |
| OSS 未配置 | `noopCloudStorage`，不执行冷迁移，文件留在主存 |
| Kafka 未配置 | goroutine MQ 自动接管，或 `noopProducer` |
| SeaweedFS 上传失败 | 兜底直传 OSS，`storage_type="oss"` |
| 主存满 + 新文件上传 | `CheckUpload` 返回 `disk_full=true` + `upload_mode="presigned"`，客户端直传 OSS |

## Docker Compose 集成

```yaml
seaweedfs:
  image: docker.io/chrislusf/seaweedfs:latest
  command: server -s3 -dir=/data
  ports: ["9333:9333", "8333:8333"]
  volumes: [seaweedfs_data:/data]
  healthcheck:
    test: ["CMD", "wget", "-qO-", "http://127.0.0.1:9333/cluster/status"]
```

file-service 和 file-worker 通过环境变量 `SEAWEEDFS_ENDPOINT=http://seaweedfs:8333` 连接。

## 面试要点

1. **为什么分冷热两层？** — 主存容量有限但延迟低（本地磁盘或内网 SeaweedFS），OSS 容量无限但延迟高、按量付费。两层平衡了访问速度与存储成本
2. **为什么不直传 OSS？** — 热数据读取频繁，走内网 SeaweedFS（Mode B）或本地磁盘（Mode A）延迟低很多。OSS 按流量计费，频繁读写成本高
3. **本地磁盘在 Mode B 算不算一层？** — 不算。本地磁盘只做分块的临时缓冲，`MergeChunks` 合并后上传 SeaweedFS 就立即 `os.Remove`，合并完成后本地不保留任何持久数据
4. **LRU 怎么实现？** — `file_store.last_accessed_at` 在每次下载时更新，`FindLRUStores()` 按此字段 ASC 取候选。淘汰比例有缓冲（降到 72%）避免反复触发
5. **用量怎么追踪？** — Redis `INCRBY disk_usage:{type}` 原子计数器。上传加、迁移减，不做 DB 聚合查询
6. **淘汰同步还是异步？** — 异步。`MergeChunks` 结束后调用 `maybeEvictToCloud`，发消息到 Kafka 或 goroutine channel，由 worker 执行实际搬迁
7. **主存满了怎么办？** — `CheckUpload` 检测到 `disk_usage + fileSize > PrimaryMaxBytes`，返回 `disk_full=true` + `upload_mode="presigned"`，前端切换为预签名直传 OSS
8. **Clean Architecture 怎么体现？** — biz 层定义 `ObjectStorage`/`CloudStorage` 接口，data 层分别用 aws-sdk-go-v2 (SeaweedFS) 和 alibabacloud-oss-go-sdk-v2 (OSS) 实现，Wire 注入，测试全 Mock
