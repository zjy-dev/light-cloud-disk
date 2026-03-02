# 分块上传实现

## 概述

分块上传是处理大文件上传的核心功能，支持秒传和断点续传。实现在 File Service (`app/file/internal/biz/file.go`)。

## 流程图

```
┌─────────────┐     ┌──────────────────────────────────────┐
│   客户端     │     │            API Gateway (:8080)       │
│             │────▶│  POST /api/v1/file/check-upload      │
└─────────────┘     └───────────────┬──────────────────────┘
                                    │ gRPC
                                    ▼
                    ┌──────────────────────────────────────┐
                    │         File Service (:9002)          │
                    │           CheckUpload                │
                    └───────────────┬──────────────────────┘
                                    │
                             ┌──────┴──────┐
                             │             │
                             ▼             ▼
                       秒传成功       检查 Redis
                    (store 已存在)     已上传分块
                             │             │
                      ┌──────┴──────┐      │
                      │             │      │
                      ▼             ▼      ▼
                   返回秒传     返回分块    全新上传
                   完成         列表(续传)  (从头开始)
                                    │      │
                                    ▼      ▼
                    ┌──────────────────────────────────────┐
                    │            UploadChunk               │
                    │  Gateway: POST /api/v1/file/upload-chunk │
                    │  (循环上传每个分块，跳过已上传的)         │
                    └───────────────┬──────────────────────┘
                                    │
                                    ▼
                    ┌──────────────────────────────────────┐
                    │            MergeChunks               │
                    │  Gateway: POST /api/v1/file/merge-chunks │
                    │  1. 合并所有分块为完整文件             │
                    │  2. 计算 MD5 校验                     │
                    │  3. 写入数据库 (file_meta + file_store)│
                    │  4. gRPC 调用 User Service            │
                    │     更新用户存储用量                   │
                    │  5. 清理 Redis 和临时文件              │
                    └──────────────────────────────────────┘
```

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

## 跨服务调用

文件合并完成后，File Service 通过 gRPC 调用 User Service 更新存储用量：

```go
// app/file/internal/biz/file.go
func (uc *FileUsecase) MergeChunks(ctx context.Context, ...) error {
    // ... 合并逻辑

    // 通过 gRPC 调用 User Service (Consul 发现)
    if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
        uc.log.Warnf("failed to update storage: %v", err)
        // 容错: 不影响合并结果
    }
    return nil
}
```

## 关键代码

```go
// app/file/internal/biz/file.go
func (uc *FileUsecase) CheckUpload(ctx context.Context, fileMD5 string, fileSize int64, totalChunks int32) (bool, []int32, error) {
    // 1. 检查文件是否已存在（秒传）
    store, err := uc.repo.FindStoreByMD5(ctx, fileMD5)
    if err == nil && store != nil {
        return true, nil, nil  // 可以秒传
    }

    // 2. 获取已上传的分块（断点续传）
    uploadedChunks, err := uc.repo.GetUploadedChunks(ctx, fileMD5)
    if err != nil {
        return false, nil, err
    }

    return false, uploadedChunks, nil
}
```

## 面试要点

1. **为什么用 MD5？** — 快速判断文件是否相同，节省存储空间
2. **为什么用 Redis？** — 高性能读写，适合临时状态存储，自带过期机制
3. **分块大小选择？** — 5MB，平衡传输效率和失败重传成本
4. **并发上传？** — 支持多分块并行上传，提高速度
5. **存储用量更新失败？** — 容错处理，仅告警不阻塞，可通过定时任务修正
6. **引用计数？** — 多个用户秒传同一文件时共享存储，删除时减引用，引用为 0 才删物理文件
