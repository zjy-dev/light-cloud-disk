# 分布式锁策略

## 概述

文件服务使用 Redis 实现两层分布式锁，确保多实例部署下的数据一致性。

## 锁类型

### 1. Chunk 级互斥锁 (SaveChunk)

```
key:  chunk_lock:{fileMD5}:{chunkIndex}
TTL:  30 秒
操作: SETNX + EXPIRE
```

**目的**: 防止同一分块被多个实例并行写入。

**流程**:
1. `SETNX chunk_lock:{md5}:{idx}` 获取锁
2. 写入分块数据到本地磁盘
3. `SADD uploaded_chunks:{md5} {idx}` 记录已上传分块
4. `DEL chunk_lock:{md5}:{idx}` 释放锁

**容错**: 写入失败时，通过 `defer` 确保锁被释放，不会阻塞其他实例重试。

### 2. CompleteUpload 级互斥锁

```
key:     merge_lock:{fileMD5}
TTL:     5 分钟
value:   随机 UUID (owner token)
操作:    SETNX + Lua 脚本原子释放
```

**目的**: 确保全局只有一个实例执行 `CompleteUpload`，避免重复创建同一个 FileStore。

**Lua 解锁脚本**:
```lua
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 0
end
```

只有锁的持有者（通过 owner token 验证）才能释放锁，防止 A 实例误删 B 实例的锁。

## 协作上传 (Cooperative Chunking)

当多个客户端上传同一文件（相同 MD5）时：

1. **CheckUpload** 检测到 `FindStoreByMD5` 返回 `uploading` 状态
2. 返回 `upload_status: "uploading"` 告知客户端加入协作
3. 客户端可以上传缺失的分块，加速整体上传
4. 任何客户端都可以在所有分块就位后触发 CompleteUpload

## Redis 数据结构

| Key 模式 | 类型 | 用途 |
|-----------|------|------|
| `chunk_lock:{md5}:{idx}` | String | 分块写入互斥锁 |
| `merge_lock:{md5}` | String | CompleteUpload 互斥锁 (值为 owner UUID) |
| `uploaded_chunks:{md5}` | Set | 已上传分块索引集合 |
| `disk_usage:local` | String | 本地磁盘已用量 (INCRBY 原子计数) |

## 跨实例故障转移

当一致性哈希重新平衡后，分块可能需要重新上传到新实例：

1. `CompleteUpload` 在获取锁后检查 `CountChunkRecords`
2. 如果本地分块数不足，返回错误提示客户端重新上传缺失分块
3. `CreateStoreWithStatus` 使用 MySQL UNIQUE 约束处理竞态

## 实现文件

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | 锁获取/释放调用，CompleteUpload 流程编排 |
| `app/file/internal/data/file.go` | Redis SETNX/Lua 脚本实现 |
