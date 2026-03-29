# 冷热分层存储

## 概述

采用**两层存储**——本地磁盘（热/温）+ 阿里云 OSS（冷），通过 LRU 淘汰把最久未访问的文件从本地磁盘异步搬到 OSS。大文件（>1 MB）在本地存储时还会经过纠删码编码（Reed-Solomon 4+2），提供冗余保护。

| 层 | 存储 | 用途 | 容量 | Redis Key |
|----|------|------|------|-----------|
| 主存 (热/温) | 本地磁盘 (`FILE_STORE_DIR`) | 合并后文件持久存放 + EC 分片 | `PRIMARY_MAX_BYTES` (默认 10GB) | `disk_usage:local` |
| 冷存 | 阿里云 OSS | LRU 淘汰后归档 (可选) | 无限 | — |

## 架构图

```
┌──────────────────────────────────────────────────────────────────┐
│                      File Service (gRPC :9002)                  │
│                                                                  │
│  MergeChunks():                                                  │
│    合并分块 → hardlink/copy 到 FILE_STORE_DIR                   │
│    IncrDiskUsage("local", fileSize)                              │
│    ▲ 磁盘满? → uploadToOSS() 直传 OSS                           │
│    ▲ 文件 ≥ 1MB? → encodeWithErasure() (Reed-Solomon 4+2)      │
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
    │  原始文件 + EC 分片       │  LRU  │   无限       │
    │  PRIMARY_MAX_BYTES       │ 淘汰  │              │
    └──────────────────────────┘       └──────────────┘
      disk_usage:local                     无限容量
      Redis 计数器
```

## 核心流程

### 1. 上传模式选择（CheckUpload）

```
CheckUpload(fileMD5, fileSize)
│
├── MD5 命中 file_store (completed) → 秒传 (upload_mode="direct")
│
├── MD5 命中 file_store (uploading) → 加入协作上传 (upload_mode="direct")
│
├── disk_usage:local + fileSize ≤ PRIMARY_MAX_BYTES
│   → diskFull=false, upload_mode="direct"
│
└── disk_usage:local + fileSize > PRIMARY_MAX_BYTES
    → diskFull=true, upload_mode="presigned" (降级直传 OSS)
```

### 2. 合并上传 (uploadMergedFile)

`MergeChunks` 合并分块后调用 `uploadMergedFile`：

```go
func (uc *FileUsecase) uploadMergedFile(ctx, mergedPath, objectKey, fileSize) (storageType, storePath) {
    // 主存空间够 → hardlink/copy 到 FILE_STORE_DIR, IncrDiskUsage("local", +fileSize)
    // 主存满 → 直传 OSS (兜底)
}
```

合并后若文件 ≥ `ERASURE_MIN_FILE_SIZE` (默认 1 MB) 且存储在本地，则调用 `encodeWithErasure()` 生成 4+2 纠删码分片，删除原始文件，`storage_type` 变为 `local_ec`。

### 3. LRU 淘汰 (maybeEvictToCloud)

`MergeChunks` 结束后调用：

```go
func (uc *FileUsecase) maybeEvictToCloud(ctx context.Context, currentUsed int64) {
    threshold := PrimaryMaxBytes * ThresholdPct / 100   // 默认 80%
    if currentUsed <= threshold { return }

    evictTarget := threshold * EvictTargetPct / 100     // 默认降到阈值的 90% (即总容量的 72%)
    toFree := currentUsed - evictTarget

    stores := repo.FindLRUStores(ctx, "local", 100)     // 按 last_accessed_at ASC

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

### 4. 异步迁移 (goroutine MQ)

通过进程内 buffered channel 异步执行：

```
consumeCloudMigrate:
  1. repo.FindStoreByMD5(md5) → 检查 storage_type (幂等)
  2. os.Open(本地文件)
  3. cloudStore.Put(key, data) → 上传 OSS
  4. repo.UpdateStorageLocation(md5, "oss", key) → 更新 DB
  5. os.Remove(本地文件)
  6. repo.IncrDiskUsage("local", -size) → 递减计数器
```

### 5. 下载（按 StorageType 路由）

下载时先刷新 `last_accessed_at`（LRU 时间戳），再根据 `storage_type` 字段选路径：

```go
func (uc *FileUsecase) GetDownloadURL(ctx, userID, fileID) (string, string, error) {
    store := repo.FindStoreByMD5(ctx, file.FileMD5)
    repo.UpdateLastAccessed(ctx, file.FileMD5)

    switch store.StorageType {
    case "local", "local_ec":
        return "local://" + store.StorePath  // Gateway 走 gRPC streaming 代理
    case "oss":
        return cloudStore.PresignGetURL(ctx, store.StorePath, 1h)
    }
}
```

- `local://` / `local_ec://` → Gateway 调 `StreamFileContent` server-streaming RPC，流式代理
  - `local_ec` 存储类型：自动从纠删码分片中重建原始文件再返回
- `oss` → 返回预签名 URL，客户端直连下载

## 数据模型

### file_store 关键字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `storage_type` | VARCHAR(32) | `"local"` / `"local_ec"` / `"oss"` |
| `upload_status` | VARCHAR(20) | `"uploading"` / `"completed"` |
| `store_path` | VARCHAR(255) | 存储 key，格式 `{md5}/{filename}` |
| `last_accessed_at` | DATETIME | 每次下载更新，LRU 淘汰排序依据 |

### erasure_shards 表

| 字段 | 类型 | 说明 |
|------|------|------|
| `file_store_id` | BIGINT | 关联 file_store |
| `shard_index` | INT | 分片序号 (0 ~ data+parity-1) |
| `shard_path` | VARCHAR(255) | 分片文件路径 |
| `shard_size` | BIGINT | 分片字节数 |
| `is_parity` | BOOLEAN | 是否为校验分片 |
| `checksum` | VARCHAR(64) | SHA-256 校验和 |

### Redis 用量计数器

| Key | 说明 | 增 | 减 |
|-----|------|-----|-----|
| `disk_usage:local` | 本地磁盘已用量 | `uploadMergedFile` 写入 storeDir 后 | goroutine MQ 迁移完成后 |

操作方式 `INCRBY`，支持正负 delta，原子性保证并发安全。

## 接口层

### biz 层接口

```go
// CloudStorage — 阿里云 OSS (冷存)
type CloudStorage interface {
    Put(ctx context.Context, key string, r io.Reader, size int64) error
    Delete(ctx context.Context, key string) error
    Get(ctx context.Context, key string) (io.ReadCloser, error)
    PresignGetURL(ctx context.Context, key string, expires time.Duration) (string, error)
    // 预签名 multipart upload 方法...
}

// ErasureEncoder — 纠删码编解码
type ErasureEncoder interface {
    Encode(data []byte, dataShards, parityShards int) ([][]byte, error)
    Reconstruct(shards [][]byte, dataShards, parityShards int) ([]byte, error)
    ShardChecksum(shard []byte) string
}
```

### gRPC (file.proto)

```protobuf
rpc GetDiskUsage(GetDiskUsageRequest) returns (GetDiskUsageReply);

message CheckUploadReply {
    bool can_fast_upload = 1;
    repeated int32 uploaded_chunks = 2;
    bool disk_full = 3;
    string upload_mode = 4;    // "direct" | "presigned"
    string upload_status = 5;  // "uploading" | "completed"
}

message GetDiskUsageReply {
    int64 primary_used_bytes = 1;
    int64 primary_max_bytes = 2;
    string primary_type = 3;   // "local"
}
```

## 配置

### StorageConfig (biz 层)

```go
type StorageConfig struct {
    PrimaryMaxBytes int64  // 主存上限 (10GB default)
    ThresholdPct    int32  // 淘汰触发阈值 (80%)
    EvictTargetPct  int32  // 淘汰目标 (阈值的 90%, 即总容量 72%)
}
```

### 环境变量

| 变量 | 说明 | 服务 | 默认值 |
|------|------|------|--------|
| `PRIMARY_MAX_BYTES` | 主存上限 (字节) | file | 10737418240 (10GB) |
| `FILE_TMP_DIR` | 分块临时目录 | file | /app/tmp |
| `FILE_STORE_DIR` | 本地合并文件目录 | file | /app/store |
| `ERASURE_DATA_SHARDS` | 纠删码数据分片 | file | 4 |
| `ERASURE_PARITY_SHARDS` | 纠删码校验分片 | file | 2 |
| `ERASURE_MIN_FILE_SIZE` | 纠删码最小文件 | file | 1048576 (1MB) |
| `OSS_ENDPOINT` | 阿里云 OSS 端点 | file | - |
| `OSS_REGION` | OSS Region | file | - |
| `OSS_BUCKET` | OSS Bucket | file | - |
| `OSS_ACCESS_KEY_ID` | OSS AK | file | - |
| `OSS_ACCESS_KEY_SECRET` | OSS SK | file | - |

## 降级策略

| 场景 | 行为 |
|------|------|
| OSS 未配置 | `noopCloudStorage`，不执行冷迁移，文件留在本地 |
| ErasureEncoder 为 nil | 跳过纠删码编码，文件按 `local` 原始存储 |
| 主存满 + 新文件上传 | `CheckUpload` 返回 `disk_full=true` + `upload_mode="presigned"`，客户端直传 OSS |

## 面试要点

1. **为什么分冷热两层？** — 本地磁盘容量有限但延迟低，OSS 容量无限但延迟高、按量付费。两层平衡了访问速度与存储成本
2. **为什么不直传 OSS？** — 热数据读取频繁，本地磁盘延迟低很多。OSS 按流量计费，频繁读写成本高
3. **纠删码和冷热分层的关系？** — 纠删码保障本地热数据的冗余性（容忍 2 个分片丢失），冷热分层管理存储成本（不常访问的文件迁移到 OSS）
4. **LRU 怎么实现？** — `file_store.last_accessed_at` 在每次下载时更新，`FindLRUStores()` 按此字段 ASC 取候选。淘汰比例有缓冲（降到 72%）避免反复触发
5. **用量怎么追踪？** — Redis `INCRBY disk_usage:local` 原子计数器。上传加、迁移减，不做 DB 聚合查询
6. **淘汰同步还是异步？** — 异步。`MergeChunks` 结束后调用 `maybeEvictToCloud`，发消息到 goroutine channel，后台执行搬迁
7. **主存满了怎么办？** — `CheckUpload` 检测到 `disk_usage + fileSize > PrimaryMaxBytes`，返回 `disk_full=true` + `upload_mode="presigned"`，前端切换为预签名直传 OSS
8. **Clean Architecture 怎么体现？** — biz 层定义 `CloudStorage`/`ErasureEncoder` 接口，data 层用 alibabacloud-oss-go-sdk-v2 和 klauspost/reedsolomon 实现，Wire 注入，测试全 Mock
