# 分块上传实现

## 概述

当前上传链路有两种模式：

| 条件 | upload_mode | 上传路径 |
|------|-------------|----------|
| MD5 命中已完成 FileStore | `direct` | 无需上传，直接秒传 |
| 本地主存充足 | `direct` | 浏览器 → 多个 File Service HTTP 实例 |
| 本地主存不足 | `presigned` | 浏览器 → OSS multipart |

`direct` 是主路径：Gateway 只负责生成 UploadPlan 和完成元数据写入，文件分块本身绕过网关，直接上传到 file-service HTTP 端口。

## Direct 模式流程

```
客户端
  │
  ├── POST /api/v1/file/check-upload ───────────▶ Gateway
  │                                                │
  │                                                ├── gRPC CheckUpload ─▶ File Service
  │                                                └── pickN 生成 UploadPlan
  │
  ├── 并发 PUT /api/v1/chunks/:md5/:index ───────▶ File Service HTTP :9003 ×N
  │      Authorization: Bearer JWT
  │      X-File-Size: <原始文件大小>
  │
  │      每个 chunk 会执行：
  │      1. SaveChunk 落本地 tmp_dir
  │      2. CreateChunkRecord 写 chunk_records
  │      3. PrepareChunkRecovery 生成恢复分片并写入 OSS
  │
  └── POST /api/v1/file/complete-upload ─────────▶ Gateway
                                                   │
                                                   └── gRPC CompleteUpload ─▶ File Service
                                                        1. 校验 chunk_records 完整性
                                                        2. 创建 FileStore(storage_type=scattered)
                                                        3. 创建逻辑 File
                                                        4. UpdateStorageUsed
                                                        5. maybeEvictToCloud
```

## UploadPlan

Gateway 基于一致性哈希环给每个分块分配目标实例：

- 哈希算法：FNV32a
- 虚拟节点：150
- 不健康实例冷却：15 秒
- 上传分配：`pickN(fileMD5, totalChunks)` 后按 round-robin 映射到 chunk index

前端只需要执行 UploadPlan 中的 `uploadUrl`，不需要感知服务发现细节。

## 断点续传与秒传

### 秒传

1. 前端使用 `spark-md5` 计算完整文件 MD5。
2. `CheckUpload` 查询 `file_stores` 中是否存在已完成的 `(file_md5, size)` 记录。
3. 命中则仅增加引用并创建新的逻辑文件记录。

### 断点续传

1. Redis 记录 `upload:{md5}:chunks`。
2. `CheckUpload` 返回已上传分块列表。
3. 前端跳过这些 chunk，仅上传缺失块。
4. CompleteUpload 以 `chunk_records` 数量作为最终完整性判定。

## 分块恢复链路

`direct` 模式下，每个分块上传成功后都会生成恢复分片：

1. File Service 对单个 chunk 执行 Reed-Solomon 编码。
2. 恢复分片写入 OSS，key 形如 `recovery/{md5}/{chunkIndex}/shard-{n}.rs`。
3. 下载计划里主下载地址仍是原始 chunk 所在实例。
4. Gateway 在 HTTP 响应里补充 `backupUrls`，指向 recovery 端点。
5. 当前端拉主地址失败时，自动改为请求 recovery URL，由健康实例从 OSS 分片重建该 chunk。

这样下载主链路仍是实例直连，只有失败时才经过 recovery 代理。

## Presigned 模式

当本地主存不足时，系统切换到 OSS multipart：

1. Gateway 调 `InitPresignedUpload`。
2. File Service 创建 upload session 并签发每个 part 的 URL。
3. 浏览器直接 PUT 到 OSS。
4. `CompletePresignedUpload` 完成 multipart 并创建 `storage_type=oss` 的 FileStore。

这条路径天然已经在 OSS，不依赖 scattered chunk recovery。

## 关键细节

- 直传请求必须携带 JWT，file-service HTTP 端点与 gateway 共享 `JWT_SECRET`。
- `X-File-Size` 头用于把 chunk_record 绑定到正确的 `(file_md5, file_size, chunk_index)`。
- `CreateChunkRecord` 使用 upsert，重复上传同一个 chunk 时保持幂等。
- `CompleteUpload` 不再合并 chunk；文件永久以 scattered 形式存在。

## 面试要点

1. 为什么直传不经过网关：避免网关占用上传带宽，只保留元数据控制面。
2. 为什么还要 chunk_records：需要显式记录 chunk 的实例地址、路径、大小和校验值，供下载计划与迁移使用。
3. 为什么把恢复分片写 OSS：实例宕机后仍能让其它健康实例无状态重建 chunk。
4. 为什么恢复 URL 放在备份链路：主链路继续走实例直连，只有故障时才引入 recovery 代理成本。
