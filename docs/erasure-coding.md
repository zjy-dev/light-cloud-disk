# 纠删码 (Erasure Coding)

## 概述

文件服务对超过阈值大小的本地文件自动应用 Reed-Solomon 纠删码，将单一文件切分为数据分片 + 校验分片，提供磁盘级容错能力。

## 配置

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `ERASURE_DATA_SHARDS` | 4 | 数据分片数 |
| `ERASURE_PARITY_SHARDS` | 2 | 校验分片数 |
| `ERASURE_MIN_FILE_SIZE` | 1048576 (1MB) | 触发 EC 的最小文件大小 |

默认 4+2 配置下，6 个分片中任意丢失 2 个仍可完整恢复原文件。存储开销为 50% (6/4 = 1.5x)。

## 编码流程

```
MergeChunks 步骤 8.5:

  原始文件 (≥ MinFileSize)
       │
       ▼
  EncodeErasure(filePath, storeDir, md5, 4, 2)
       │
       ├── 读取文件 → reedsolomon.Split → 4 个 data shard
       ├── reedsolomon.Encode → 生成 2 个 parity shard
       └── 写入 6 个 shard 文件:
           {md5}.shard.0 (data)
           {md5}.shard.1 (data)
           {md5}.shard.2 (data)
           {md5}.shard.3 (data)
           {md5}.shard.4 (parity)
           {md5}.shard.5 (parity)
       │
       ▼
  CreateErasureShard × 6 (写入 DB)
       │
       ▼
  删除原始合并文件
       │
       ▼
  StorageType = "local_ec"
```

## 分片命名

```
{storeDir}/{fileMD5}.shard.{index}
```

- index 0 ~ (DataShards-1): 数据分片
- index DataShards ~ (DataShards+ParityShards-1): 校验分片

## 数据模型

```sql
CREATE TABLE erasure_shards (
    id            BIGINT AUTO_INCREMENT PRIMARY KEY,
    file_store_id BIGINT NOT NULL,     -- 关联 file_stores.id
    shard_index   INT NOT NULL,        -- 分片编号
    shard_path    VARCHAR(512) NOT NULL,-- 分片文件名
    shard_size    BIGINT DEFAULT 0,    -- 分片大小(字节)
    is_parity     BOOLEAN DEFAULT FALSE,-- 是否为校验分片
    checksum      VARCHAR(64),         -- 分片 MD5
    created_at    DATETIME,
    updated_at    DATETIME,
    UNIQUE(file_store_id, shard_index)
);
```

## 重建流程

```
OpenLocalFile (StorageType = "local_ec"):

  FindErasureShards(fileStoreID)
       │
       ▼
  构建 shardPaths[6] 数组
       │
       ▼
  ReconstructErasure(paths, 4, 2)
       │
       ├── 读取所有可用分片
       ├── 检测缺失分片 (读取失败视为缺失)
       ├── 缺失 ≤ 2: reedsolomon.Reconstruct 恢复
       ├── reedsolomon.Verify 校验完整性
       └── 拼接 data shard → 原始数据
       │
       ▼
  返回 io.ReadCloser (内存中重建)
```

## 接口抽象

```go
type ErasureEncoder interface {
    Encode(filePath, storeDir, fileMD5 string, dataShards, parityShards int) (shardPaths []string, err error)
    Reconstruct(shardPaths []string, dataShards, parityShards int) ([]byte, error)
    ShardChecksum(path string) (string, error)
}
```

Biz 层通过接口依赖注入，data 层 `rsEncoder` 提供基于 `klauspost/reedsolomon` 的实现。单元测试使用 `MockErasureEncoder`。

## 实现文件

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | `ErasureEncoder` 接口、`encodeWithErasure`/`reconstructFromShards` 方法 |
| `app/file/internal/data/erasure.go` | `rsEncoder` 实现、`EncodeErasure`/`ReconstructErasure` 函数 |
| `app/file/cmd/main.go` | `provideErasureConfig` 读取环境变量 |
