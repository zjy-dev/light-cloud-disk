# 预签名分块上传 (Presigned Multipart Upload)

## 概述

预签名上传是一种"客户端直传"方案：后端签发临时授权 URL，浏览器直接将文件分块 PUT 到阿里云 OSS，绕过 Gateway 转发，大幅降低带宽和 CPU 开销。

与传统 "直传"（direct upload）模式相比：

| 维度 | Direct 模式 | Presigned 模式 |
|------|-------------|----------------|
| 数据路径 | 浏览器 → File Service HTTP 实例 (按 UploadPlan 直传) | 浏览器 → 阿里云 OSS（直传） |
| 适用场景 | 本地磁盘未满 | 本地磁盘已满（降级为 OSS） |
| 后端压力 | 低（Gateway 不转发字节流，file-service 只接收分配到自己的 chunk） | 低（仅签发 URL + 元数据管理） |
| 断点续传 | Redis chunk 状态 | upload_sessions + upload_parts 表 |
| 跨设备续传 | ❌（chunk 和 Redis 绑定同一节点） | ✅（session 持久化到 DB，任意设备可续） |

## 模式选择逻辑

```
CheckUpload()
│
├── MD5 命中 → 秒传 (upload_mode = "direct")
│
├── 本地磁盘未满 → upload_mode = "direct"
│   （走 UploadPlan + 直传 chunk + CompleteUpload）
│
└── 本地磁盘已满 → upload_mode = "presigned"
    （降级为阿里云 OSS 预签名上传）
```

## 完整流程

```
┌──────────┐        ┌───────────────┐        ┌──────────────┐        ┌──────────────┐
│  浏览器   │        │   Gateway     │        │ File Service │        │  阿里云 OSS  │
│          │        │   (:8080)     │        │  (:9002)     │        │              │
└────┬─────┘        └───────┬───────┘        └──────┬───────┘        └──────┬───────┘
     │  1. CheckUpload      │                       │                       │
     │─────────────────────▶│──── gRPC ────────────▶│                       │
     │  upload_mode=presigned│◀─────────────────────│                       │
     │◀─────────────────────│                       │                       │
     │                      │                       │                       │
     │  2. InitPresignedUpload                      │                       │
     │─────────────────────▶│──── gRPC ────────────▶│── InitMultipart ─────▶│
     │                      │                       │◀── uploadID ──────────│
     │                      │                       │── PresignUploadPart ─▶│
     │  {session_id, parts[{url, part_number}]}     │◀── presigned URLs ───│
     │◀─────────────────────│◀─────────────────────│                       │
     │                      │                       │                       │
     │  3. PUT 直传 (循环每个 part)                  │                       │
     │──────────────────────────────────────────────────── PUT part ───────▶│
     │◀─────────────────────────────────────────────────── ETag ───────────│
     │                      │                       │                       │
     │  4. ReportUploadedPart (每个 part 完成后)     │                       │
     │─────────────────────▶│──── gRPC ────────────▶│ (写 upload_parts 表)  │
     │◀─────────────────────│◀─────────────────────│                       │
     │                      │                       │                       │
     │  5. CompletePresignedUpload                  │                       │
     │─────────────────────▶│──── gRPC ────────────▶│── CompleteMPU ──────▶│
     │                      │                       │◀── OK ──────────────│
     │                      │                       │ (写 file_store + file)│
     │  {file}              │                       │                       │
     │◀─────────────────────│◀─────────────────────│                       │
```

## 数据模型

### upload_sessions 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(36) | UUID 主键 |
| user_id | BIGINT | 上传用户 |
| parent_id | BIGINT | 目标目录 |
| file_name | VARCHAR(255) | 原文件名 |
| file_md5 | VARCHAR(32) | 文件 MD5 (索引) |
| file_size | BIGINT | 文件总大小 |
| total_parts | INT | 总分块数 |
| part_size | BIGINT | 每块大小 (默认 5MB) |
| storage_target | VARCHAR(20) | "oss" |
| object_key | VARCHAR(255) | 对象存储 key |
| s3_upload_id | VARCHAR(255) | S3 multipart upload ID |
| status | VARCHAR(20) | "uploading" / "completed" / "aborted" |
| expires_at | DATETIME | 会话过期时间 (24h) |

### upload_parts 表

| 字段 | 类型 | 说明 |
|------|------|------|
| session_id | VARCHAR(36) | 关联 session |
| part_number | INT | 分块序号 (1-based) |
| etag | VARCHAR(255) | S3 返回的 ETag |
| size | BIGINT | 分块实际大小 |
| uploaded_at | DATETIME | 上传时间 |

## API 端点

### POST /api/v1/file/presigned-upload (InitPresignedUpload)

请求：
```json
{
  "file_name": "video.mp4",
  "file_md5": "d41d8cd98f00b204e9800998ecf8427e",
  "file_size": 104857600,
  "total_parts": 20,
  "parent_id": 0
}
```

响应（新上传）：
```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "storage_target": "oss",
  "part_size": 5242880,
  "pending_parts": [
    {"part_number": 1, "upload_url": "https://oss-cn-hangzhou.aliyuncs.com/bucket/key?...&partNumber=1"},
    {"part_number": 2, "upload_url": "https://oss-cn-hangzhou.aliyuncs.com/bucket/key?...&partNumber=2"}
  ]
}
```

响应（秒传命中）：
```json
{
  "can_fast_upload": true,
  "file": { ... }
}
```

### POST /api/v1/file/presigned-upload/part (ReportUploadedPart)

```json
{
  "session_id": "550e8400-...",
  "part_number": 1,
  "etag": "\"d41d8cd98f00b204e9800998ecf8427e\"",
  "size": 5242880
}
```

### POST /api/v1/file/presigned-upload/complete (CompletePresignedUpload)

```json
{ "session_id": "550e8400-..." }
```

### POST /api/v1/file/presigned-upload/abort (AbortPresignedUpload)

```json
{ "session_id": "550e8400-..." }
```

## 跨设备续传

传统 direct 模式下，分块状态保存在 Redis（`upload:{md5}:chunks`）中，与特定 File Service 实例绑定。用户换设备后，新请求可能路由到不同实例，Redis 和本地分块文件无法共享。

预签名模式完全消除了这个问题：

1. **会话持久化到 DB**：`upload_sessions` 和 `upload_parts` 表存在关系数据库中，任意节点均可读取
2. **数据直传 OSS**：分块直接上传到阿里云 OSS，不存在"本地分块"的概念
3. **按 MD5 查找会话**：`InitPresignedUpload` 通过 `file_md5 + user_id` 查找已有 session
4. **URL 可重新签发**：过期的 presigned URL 通过 resume 逻辑重新生成

续传流程：
```
设备 A: CheckUpload → presigned → InitPresignedUpload → 上传 3/10 个 part → 中断
                                                                     ↓
设备 B: CheckUpload → presigned → InitPresignedUpload ← 找到已有 session
                                   → 返回 completed_parts=[1,2,3]
                                   → 返回 pending_parts=[4..10] (新签名 URL)
                                   → 继续上传剩余 part → Complete
```

## MD5 一致性哈希路由

Gateway 对与上传相关的 4 个端点使用 MD5 一致性哈希路由，确保同一文件的所有请求路由到同一 File Service 实例：

```
Gateway hashRouter:
  FNV32a 哈希 + 150 虚拟节点/实例
  Consul 服务发现 (15s 刷新)
  per-instance gRPC 连接
```

路由的端点：
- `POST /file/check-upload` → `FileClientByKey(file_md5)`
- `POST /file/presigned-upload` → `FileClientByKey(file_md5)`
- `POST /file/upload-chunk` → `FileClientByKey(file_md5)`
- `POST /file/merge-chunks` → `FileClientByKey(file_md5)`

当 hashRouter 不可用时（单实例或 Consul 未配置），自动降级到默认的 File Service 连接。

## 前端实现

`useUpload.ts` composable 实现双模式上传：

```typescript
// 根据 CheckUpload 返回的 upload_mode 决定路径
const uploadMode = rawResult['upload_mode'] ?? rawResult['uploadMode']

if (uploadMode === 'presigned') {
  await presignedUpload(file, md5, totalChunks, ...)
} else {
  await directUpload(file, md5, totalChunks, uploadedChunks, ...)
}
```

presigned 上传流程：
1. 调用 `initPresignedUpload()` 获取 session 和 presigned URLs
2. 循环 `fetch(url, { method: 'PUT', body: chunkBlob })` 直传每个 part
3. 每个 part 完成后调用 `reportUploadedPart()` 上报 ETag
4. 全部完成后调用 `completePresignedUpload()` 合并

**注意**：Proto 生成的 JSON 使用 snake_case 字段名（如 `upload_mode`、`can_fast_upload`），而 TypeScript 类型定义用 camelCase。前端通过双键访问模式处理：`rawResult['upload_mode'] ?? rawResult['uploadMode']`。

## CloudStorage 接口

预签名操作由 `CloudStorage` 接口（阿里云 OSS）提供：

```go
type CloudStorage interface {
    Put(ctx, key string, data io.Reader, size int64) error
    PresignGetURL(ctx, key string, expires time.Duration) (string, error)
    InitMultipartUpload(ctx, key string) (uploadID string, err error)
    PresignUploadPart(ctx, key, uploadID string, partNumber int32, expires time.Duration) (string, error)
    CompleteMultipartUpload(ctx, key, uploadID string, parts []CompletedPart) error
    AbortMultipartUpload(ctx, key, uploadID string) error
}
```

## 测试

| 测试名 | 场景 | 覆盖点 |
|--------|------|--------|
| TestInitPresignedUpload_FastUpload | MD5 秒传 | 命中已有 store → 返回 file |
| TestInitPresignedUpload_NewSession | 新上传 | 创建 session + 签发 URL |
| TestInitPresignedUpload_ResumeSession | 跨设备续传 | 查找已有 session + 重新签发 |
| TestReportUploadedPart_Success | 正常上报 | 写入 upload_parts |
| TestReportUploadedPart_Unauthorized | 非 owner 上报 | 返回 ErrUnauthorized |
| TestReportUploadedPart_SessionNotFound | session 不存在 | 返回 ErrSessionNotFound |
| TestReportUploadedPart_SessionCompleted | session 已完成 | 返回 ErrSessionCompleted |
| TestCompletePresignedUpload_Success | 正常合并 | CompleteMPU + 建 store/file |
| TestCompletePresignedUpload_IncompleteParts | part 不全 | 返回 ErrIncompleteUpload |
| TestCompletePresignedUpload_Unauthorized | 非 owner 合并 | 返回 ErrUnauthorized |
| TestAbortPresignedUpload_Success | 正常取消 | AbortMPU + 更新状态 |
| TestAbortPresignedUpload_Unauthorized | 非 owner 取消 | 返回 ErrUnauthorized |
| TestAbortPresignedUpload_AlreadyCompleted | 已完成的取消 | 幂等：返回 nil |

## 面试要点

1. **为什么需要预签名？** — 大文件上传如果经过后端中转，Gateway 和 File Service 的 CPU/带宽成本极高。预签名让客户端直传对象存储，后端只负责签发 URL 和管理元数据。
2. **S3 multipart 协议** — InitMultipartUpload → PresignUploadPart × N → CompleteMPU。每个 part 返回 ETag，Complete 时需要提交所有 ETag 列表，S3 端做完整性校验。
3. **跨设备续传怎么实现？** — upload_sessions 和 upload_parts 持久化到 DB，任意节点可读。过期的 URL 在 resume 时重新签发。
4. **一致性哈希的作用？** — 同一文件 MD5 的所有请求路由到同一 File Service 实例，避免分布式 session 分裂，也利于缓存命中。
5. **presigned URL 安全性** — URL 自带签名和过期时间（2h），无法伪造。Session 24h 过期后自动清理。
6. **session 鉴权** — Report/Complete/Abort 三个操作均校验 `session.UserID == 请求 userID`，防止越权操作他人 session。
7. **totalParts 服务端计算** — 服务端根据 `fileSize/partSize` 独立计算分块数，不信任客户端传入的 totalParts，防止恶意构造。
8. **前端失败清理** — presignedUpload 失败时自动调用 `abortPresignedUpload` 清理 OSS 碎片，避免资源泄漏。
9. **事务原子性** — DeleteUploadSession 使用 GORM 事务原子删除 parts + session，避免中途失败产生孤儿记录。
10. **故障处理** — Session 过期后 abort S3 upload；presigned URL 过期但 session 未过期时重新签发；AbortPresignedUpload 是幂等操作。
