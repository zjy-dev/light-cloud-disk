# 分块上传实现

## 概述

分块上传是处理大文件上传的核心功能，支持秒传、断点续传和双模式上传（直传 vs 预签名）。

系统根据部署模式和磁盘状态自动选择上传方式：

| 条件 | upload_mode | 上传路径 |
|------|-------------|----------|
| MD5 命中（秒传） | `direct` | 无需上传 |
| Mode A (local) + 磁盘未满 | `direct` | 浏览器 → Gateway → File Service → 本地磁盘 |
| Mode A (local) + 磁盘已满 | `presigned` | 浏览器 → OSS（直传） |
| Mode B (s3) | `presigned` | 浏览器 → SeaweedFS/OSS（直传） |

预签名上传的详细说明见 [presigned-upload.md](presigned-upload.md)。

## Direct 模式流程图

```
┌─────────────┐     ┌──────────────────────────────────────┐
│   客户端     │     │            API Gateway (:8080)       │
│             │────▶│  POST /api/v1/file/check-upload      │
└─────────────┘     └───────────────┬──────────────────────┘
                                    │ gRPC (MD5 一致性哈希路由)
                                    ▼
                    ┌──────────────────────────────────────┐
                    │         File Service (:9002)          │
                    │           CheckUpload                │
                    │  (秒传 + 模式选择 + 续传分块)         │
                    │  返回: upload_mode + uploaded_chunks  │
                    └───────────────┬──────────────────────┘
                                    │
                       ┌────────────┼────────────┐
                       │            │            │
                       ▼            ▼            ▼
                  秒传成功     upload_mode    upload_mode
               (store 已存在)   ="direct"     ="presigned"
                       │       检查 Redis       │
                       ▼       已上传分块        ▼
                  返回秒传       │           走预签名流程
                  完成          ▼           (InitPresigned
                           返回分块列表      Upload...)
                           (续传)
                                │
                                ▼
                ┌──────────────────────────────────────┐
                │            UploadChunk               │
                │  Gateway: POST /api/v1/file/upload-chunk │
                │  multipart/form-data 二进制分块上传   │
                │  分块写本地磁盘 + Redis INCRBY 计数    │
                └───────────────┬──────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────────────┐
                │            MergeChunks               │
                │  Gateway: POST /api/v1/file/merge-chunks │
                │  1. 合并所有分块为完整文件             │
                │  2. 计算 MD5 校验                     │
                │  3. 上传到 SeaweedFS (S3 API)         │
                │     ↳ 失败则兜底直接上传 OSS          │
                │  4. 写入数据库 (file_meta + file_store)│
                │  5. gRPC 调用 User Service            │
                │     更新用户存储用量                   │
                │  6. 清理 Redis 和本地临时文件          │
                │  7. 异步触发 LRU 淘汰检查             │
                └──────────────────────────────────────┘
```
                    │  6. 清理 Redis 和本地临时文件          │
                    │  7. 异步触发 LRU 淘汰检查             │
                    └──────────────────────────────────────┘
```

## 前端 MD5 计算（spark-md5 分块哈希）

### 问题背景

最初使用 `crypto.subtle.digest('SHA-256', file.arrayBuffer())` 方案：
- **阻塞浏览器**：`file.arrayBuffer()` 将整个文件加载到内存，大文件时 JS 主线程冻结
- **无进度反馈**：用户看到上传界面卡住，无法判断是否在工作

### 解决方案：spark-md5 分块哈希

```typescript
// frontend/src/composables/useUpload.ts
import SparkMD5 from 'spark-md5'

const HASH_CHUNK_SIZE = 2 * 1024 * 1024  // 2MB 每次读取

async function computeMd5(file: File, onProgress?: (pct: number) => void): Promise<string> {
  const spark = new SparkMD5.ArrayBuffer()
  const totalChunks = Math.ceil(file.size / HASH_CHUNK_SIZE)

  for (let i = 0; i < totalChunks; i++) {
    const start = i * HASH_CHUNK_SIZE
    const slice = file.slice(start, Math.min(start + HASH_CHUNK_SIZE, file.size))
    const buffer = await slice.arrayBuffer()  // 每次只读 2MB
    spark.append(buffer)
    onProgress?.((i + 1) / totalChunks)
    // Key point: yield to event loop so Vue can refresh UI
    await new Promise<void>((resolve) => setTimeout(resolve, 0))
  }

  return spark.end()  // 返回十六进制 MD5 字符串
}
```

### 进度映射策略

整个 `uploadFile` 进度分为三段：
- **0–15%**：MD5 哈希计算（`computeMd5` onProgress 回调）
- **15–90%**：分块上传（每个 chunk 均匀分配）
- **90–100%**：合并 + 完成

### 为什么用 MD5 不用 SHA-256？

- spark-md5 专为浏览器流式哈希设计，接口简单高效
- 文件去重场景下碰撞率可接受（MD5 128-bit，理论碰撞概率极低）
- SHA-256 通过 `crypto.subtle` 计算时不支持流式输入，必须一次性传入完整 buffer

---

## 秒传原理

1. 客户端计算文件完整 MD5
2. 通过 Gateway 调用 `CheckUpload` 接口
3. File Service 检查 `file_store` 表中 MD5 是否已存在
4. 如果存在，直接创建用户文件记录（引用同一个物理文件）
5. 文件存储表使用引用计数，删除时只减引用

## 断点续传原理

1. Redis 存储每个文件的上传状态 `upload:{md5}:chunks`
2. 状态包含已上传的分块索引列表
3. 客户端从 `CheckUpload` 获取已上传列表
4. 只上传缺失的分块
5. 分块信息 24 小时过期

## UploadChunk 协议

`/api/v1/file/upload-chunk` 使用 `multipart/form-data`，字段如下：

- `file_md5`: 文件 MD5
- `chunk_index`: 分块序号（从 0 开始）
- `chunk_size`: 分块字节数
- `chunk_file`: 分块二进制内容

这样做的好处是直接传二进制 chunk，避免 Base64 体积膨胀和编解码开销。

## 跨服务调用

文件合并完成后，File Service 通过 gRPC 调用 User Service 更新存储用量：

```go
// app/file/internal/biz/file.go
func (uc *FileUsecase) MergeChunks(ctx context.Context, ...) error {
    // ... merge local chunks

    // upload to SeaweedFS (fallback to OSS on failure)
    if err := uc.objStore.Put(ctx, key, file); err != nil {
        uc.cloudStore.Put(ctx, key, file)
    }

    // call User Service via gRPC (discovered by Consul)
    if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
        uc.log.Warnf("failed to update storage: %v", err)
        // fault tolerance: do not fail merge on this error
    }

    // async LRU eviction check
    go uc.maybeEvictToCloud(context.Background())
    return nil
}
```

## 关键代码

```go
// app/file/internal/biz/file.go
func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, bool, string, error) {
    // 1) Check instant upload by MD5 deduplication
    store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
    if err == nil && store != nil {
        return true, nil, false, "direct", nil // instant-upload hit
    }

    // 2) Determine upload mode
    // Mode B (s3): always presigned
    if uc.storageCfg.Mode == ModeS3 {
        return false, nil, false, "presigned", nil
    }

    // Mode A (local): check primary disk availability
    used, _ := uc.repo.GetDiskUsage(ctx, uc.primaryDiskType())
    if used+fileSize > uc.storageCfg.PrimaryMaxBytes {
        // Disk full → switch to presigned OSS upload
        return false, nil, true, "presigned", nil
    }

    // 3) Return uploaded chunks for resumable direct upload
    uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
    if err != nil {
        return false, nil, false, "direct", err
    }
    return false, uploadedChunks, false, "direct", nil
}
```

返回 5 个值：`canFastUpload, uploadedChunks, diskFull, uploadMode, error`。
`uploadMode` 决定前端走直传还是预签名流程。

## 面试要点

1. **为什么用 MD5？** — 快速判断文件是否相同，节省存储空间
2. **为什么用 Redis？** — 高性能读写，适合临时状态存储，自带过期机制；同时用 Redis 原子计数器追踪本地/SeaweedFS 磁盘用量
3. **分块大小选择？** — 5MB，平衡传输效率和失败重传成本
4. **并发上传？** — 支持多分块并行上传，提高速度
5. **存储用量更新失败？** — 容错处理，仅告警不阻塞，可通过定时任务修正
6. **引用计数？** — 多个用户秒传同一文件时共享存储，删除时减引用，引用为 0 才删物理文件
7. **磁盘满怎么办？** — `CheckUpload` 检测磁盘计数器，满则返回 `upload_mode="presigned"`，前端自动切换到预签名直传模式
8. **合并后文件去哪？** — 先上传 SeaweedFS，失败则兜底上传 OSS，同时更新 `file_store.storage_type`
9. **合并后为什么还要淘汰？** — 异步 `maybeEvictToCloud()` 检查 SeaweedFS 用量，超阈值则通过 Kafka 驱动 worker 将冷数据迁移到 OSS
10. **直传 vs 预签名？** — 见 [presigned-upload.md](presigned-upload.md)，Mode B 永远走预签名以减少后端带宽开销
