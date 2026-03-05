# 三级存储架构

## 概述

文件上传后的存储采用分层架构，通过 LRU 淘汰策略自动在各存储层之间迁移数据。设计思路：热数据就近访问、冷数据低成本归档。

项目支持两种部署模式（详见 [dual-mode-storage.md](dual-mode-storage.md)）：

| 模式 | 分层 | 说明 |
|------|------|------|
| Mode A (local) | 本地磁盘 → 阿里云 OSS | 单机轻量，无需 SeaweedFS |
| Mode B (s3) | 本地磁盘(暂存) → SeaweedFS → 阿里云 OSS | 完整三级存储 |

## 架构图 (Mode B: s3)

```
┌──────────────────────────────────────────────────────────────────┐
│                        File Service (gRPC :9002)                │
│                                                                  │
│  MergeChunks():                                                  │
│    本地磁盘 collect 分块 ──合并──▶ SeaweedFS (S3 API)            │
│    ▲ 磁盘满则返回 disk_full        ▲ 满?→ 直接上传 OSS          │
│    │                               │                              │
│    │    maybeEvictToCloud():       │                              │
│    │    primary 用量 > threshold%   │                              │
│    │    → 找 LRU 最近最少访问      │                              │
│    │    → 发 Kafka cloud-migrate   │                              │
│    │                               │                              │
│    │         file-worker:          │                              │
│    │    下载 SeaweedFS → 上传 OSS  │                              │
│    │    → 更新 DB storage_type     │                              │
│    │    → 删除 SeaweedFS 数据      │                              │
│    │    → 递减 Redis 计数器         │                              │
└──────────────────────────────────────────────────────────────────┘

         Tier 1                Tier 2                Tier 3
    ┌──────────────┐    ┌──────────────────┐    ┌──────────────┐
    │  本地磁盘     │    │   SeaweedFS      │    │  阿里云 OSS  │
    │  (分块暂存)   │───▶│   (S3 兼容)      │───▶│  (冷存储)    │
    │              │    │   primary_max    │    │   无限       │
    └──────────────┘    └──────────────────┘    └──────────────┘
      热数据 / 分块          温数据                  冷数据
      Redis 计数器          Redis 计数器            无限容量
```

## 架构图 (Mode A: local)

```
┌──────────────────────────────────────────────────────────────────┐
│                        File Service (gRPC :9002)                │
│                                                                  │
│  MergeChunks():                                                  │
│    本地磁盘 collect 分块 ──合并──▶ 本地 storeDir                │
│    ▲ 磁盘满则返回 disk_full                                      │
│    │                                                              │
│    │    maybeEvictToCloud():                                     │
│    │    local 用量 > threshold%                                  │
│    │    → 找 LRU 最近最少访问                                    │
│    │    → goroutine MQ (进程内 channel)                          │
│    │    → 读本地文件 → 上传 OSS → 更新 DB → 删本地 → 递减计数器  │
└──────────────────────────────────────────────────────────────────┘

               Tier 1                          Tier 2
    ┌──────────────────────────┐       ┌──────────────┐
    │     本地磁盘              │       │  阿里云 OSS  │
    │  (storeDir, 主存储)       │──────▶│  (冷存储)    │
    │  primary_max_bytes       │       │   无限       │
    └──────────────────────────┘       └──────────────┘
         热/温数据                         冷数据
         Redis 计数器                     无限容量
```

## 存储层说明

### Mode B (s3)

| 层级 | 存储 | 用途 | 容量配置 | 实现类 |
|------|------|------|----------|--------|
| Tier 1 | 本地磁盘 | 分块暂存、合并缓冲 | `LOCAL_MAX_BYTES` | 文件系统 |
| Tier 2 | SeaweedFS | 合并后文件主存 | `PRIMARY_MAX_BYTES` | `seaweedfsClient` (aws-sdk-go-v2/s3) |
| Tier 3 | 阿里云 OSS | 冷数据归档 | 无限 | `ossClient` (alibabacloud-oss-go-sdk-v2) |

### Mode A (local)

| 层级 | 存储 | 用途 | 容量配置 | 实现类 |
|------|------|------|----------|--------|
| Tier 1 | 本地磁盘 | 分块暂存 + 合并后主存 | `PRIMARY_MAX_BYTES` | 文件系统 |
| Tier 2 | 阿里云 OSS | 冷数据归档 (可选) | 无限 | `ossClient` (alibabacloud-oss-go-sdk-v2) |

## 核心流程

### 1. 分块上传（本地磁盘）

```
CheckUpload → SaveChunk → MergeChunks
```

- `CheckUpload`: 检查秒传 + 断点续传 + **本地磁盘是否已满**
  - 返回 `disk_full=true` 时，Gateway 回复 503
- `SaveChunk`: 分块写入本地 `FILE_TMP_DIR`，`IncrDiskUsage("local", chunkSize)`
- `MergeChunks`: 合并分块 → 上传 SeaweedFS → 清理本地文件

### 2. 合并上传（SeaweedFS / OSS 兜底）

```go
// Core MergeChunks logic
func (uc *FileUsecase) MergeChunks(...) {
    // 1. Merge local chunks into a complete file
    tmpPath := merge(chunks)

    // 2. Upload to SeaweedFS
    key := md5 + "/" + filename
    err := uc.objStore.Put(ctx, key, file)

    if err != nil {
        // SeaweedFS unavailable -> upload to OSS as fallback
        err = uc.cloudStore.Put(ctx, key, file)
        storageType = "oss"
    } else {
        storageType = "seaweedfs"
        uc.repo.IncrDiskUsage(ctx, "seaweedfs", fileSize)
    }

    // 3. Write metadata to DB
    store = &FileStore{StorageType: storageType, Location: key}

    // 4. Trigger eviction check
    go uc.maybeEvictToCloud(ctx)
}
```

### 3. LRU 淘汰（SeaweedFS → OSS）

当 SeaweedFS 用量超过阈值（默认 80%），异步触发淘汰：

```go
func (uc *FileUsecase) maybeEvictToCloud(ctx context.Context) {
    used := repo.GetDiskUsage(ctx, "seaweedfs")
    threshold := storageCfg.SeaweedFSMaxBytes * storageCfg.SeaweedFSThresholdPct / 100

    if used <= threshold {
        return // 未超阈值
    }

    excess := used - threshold
    candidates := repo.FindLRUStores(ctx, "seaweedfs", 50)

    var totalEvict int64
    for _, s := range candidates {
        producer.SendCloudMigrateMessage(ctx, &CloudMigrateMessage{
            FileStoreID:  s.ID,
            FileMD5:      s.FileMD5,
            CurLocation:  s.Location,
            DestLocation: "oss://" + s.Location,
            Size:         s.Size,
        })
        totalEvict += s.Size
        if totalEvict >= excess {
            break
        }
    }
}
```

### 4. 异步迁移（file-worker）

file-worker 消费 `cloud-migrate` topic：

```
handleCloudMigrateMessage:
  1. SeaweedFS.Get(key) → 下载文件数据
  2. OSS.Put(key, data) → 上传到 OSS
  3. UPDATE file_store SET storage_type='oss', location=destLocation
  4. SeaweedFS.Delete(key) → 释放 SeaweedFS 空间
  5. Redis INCRBY disk_usage:seaweedfs -(size)  → 递减用量计数器
```

### 5. 下载 URL（按 StorageType 路由）

```go
func (uc *FileUsecase) GetDownloadURL(ctx context.Context, fileID, userID int64) (string, error) {
    store := repo.FindStoreByFileID(ctx, fileID)
    repo.UpdateLastAccessed(ctx, store.ID) // 更新 LRU 时间戳

    switch store.StorageType {
    case "seaweedfs":
        return objStore.PresignGetURL(ctx, store.Location, 1*time.Hour)
    case "oss":
        return cloudStore.PresignGetURL(ctx, store.Location, 1*time.Hour)
    default:
        return "", errors.New("unknown storage type")
    }
}
```

## 数据模型

### file_store 表新增字段

| 字段 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| `storage_type` | VARCHAR(32) | `"seaweedfs"` | 存储层标识 |
| `last_accessed_at` | DATETIME | 自动更新 | LRU 排序依据 |

### Redis 用量计数器

| Key | 说明 | 操作 |
|-----|------|------|
| `disk_usage:local` | 本地磁盘已用量 | `SaveChunk` +，`MergeChunks` 清理 - |
| `disk_usage:seaweedfs` | SeaweedFS 已用量 | `MergeChunks` +，`worker` 迁移 - |

## 接口层

### biz 层接口

```go
// ObjectStorage — SeaweedFS (S3 API)
type ObjectStorage interface {
    Put(ctx context.Context, key string, r io.Reader) error
    Delete(ctx context.Context, key string) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}

// CloudStorage - Alibaba Cloud OSS
type CloudStorage interface {
    Put(ctx context.Context, key string, r io.Reader) error
    Delete(ctx context.Context, key string) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
}
```

### gRPC 新增

```protobuf
// file.proto
rpc GetDiskUsage(GetDiskUsageRequest) returns (GetDiskUsageReply);

message CheckUploadReply {
    bool can_fast_upload = 1;
    repeated int32 uploaded_chunks = 2;
    bool disk_full = 3;  // 新增：本地磁盘已满
}

message GetDiskUsageReply {
    int64 local_used_bytes = 1;
    int64 local_max_bytes = 2;
    int64 seaweedfs_used_bytes = 3;
    int64 seaweedfs_max_bytes = 4;
}
```

### HTTP 新增

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/disk-usage | 获取磁盘/SeaweedFS 用量 |

`CheckUpload` 返回 `disk_full=true` 时，Gateway 回复 HTTP 503。

## 配置

### conf.proto

```protobuf
message Storage {
    string mode = 1;                    // "local" | "s3"
    int64 primary_max_bytes = 2;        // 主存上限 (本地磁盘或 SeaweedFS)
    int64 threshold_percent = 3;        // 淘汰阈值 (默认 80%)
    int64 evict_target_percent = 4;     // 淘汰目标 (默认 90%)
    message SeaweedFS { ... }
    message OSS { ... }
    SeaweedFS seaweedfs = 5;
    OSS oss = 6;
}
```

### 环境变量

| 变量 | 说明 | 服务 | 默认值 |
|------|------|------|--------|
| `STORAGE_MODE` | 存储模式 | file | local |
| `PRIMARY_MAX_BYTES` | 主存上限 (字节) | file | 10737418240 (10GB) |
| `FILE_STORE_DIR` | 本地合并文件目录 | file | /app/store |
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
| SeaweedFS 未配置 | `noopObjectStorage`，文件不上传对象存储 |
| OSS 未配置 | `noopCloudStorage`，不执行云端迁移 |
| Kafka 未配置 | `noopProducer`，不发送迁移/缩略图消息 |
| SeaweedFS 上传失败 | 兜底直接上传 OSS |
| 本地磁盘满 | `CheckUpload` 返回 `disk_full=true`，Gateway 回复 503 |

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

1. **为什么三级存储？** — 成本与性能平衡。本地磁盘最快但容量有限，SeaweedFS 容量大且可水平扩展，OSS 成本最低适合冷数据
2. **为什么不直接传 OSS？** — OSS 访问延迟高，SeaweedFS 部署在内网延迟低，热数据读取更快
3. **LRU 淘汰怎么实现？** — `file_store.last_accessed_at` 在下载时更新，`FindLRUStores()` 按此字段 ASC 排序取候选
4. **用量怎么追踪？** — Redis 原子计数器 `disk_usage:{type}`，`INCRBY` 保证并发安全
5. **淘汰是同步还是异步？** — 异步。`MergeChunks` 结束后 `go maybeEvictToCloud()`，发 Kafka 消息由 worker 执行实际迁移
6. **SeaweedFS 挂了怎么办？** — 兜底上传 OSS，`storage_type` 标记为 `oss`，下载时按类型路由到不同后端
7. **磁盘满怎么处理？** — `CheckUpload` 检测本地磁盘计数器已满，返回 `disk_full=true`，Gateway 回复 503，客户端可稍后重试
8. **Clean Architecture 如何体现？** — biz 层定义 `ObjectStorage`/`CloudStorage` 接口，data 层分别用 aws-sdk-go-v2 和 alibabacloud-oss-go-sdk-v2 实现，Wire 注入，单元测试 Mock 接口
