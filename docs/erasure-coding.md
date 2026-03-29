# 纠删码 (Erasure Coding)

## 概述

当前系统里有两条 Reed-Solomon 使用路径：

1. **主路径**：每个直传 chunk 上传成功后立即生成恢复分片，写入 OSS，实例故障时由健康实例重建该 chunk。
2. **兼容路径**：legacy `local_ec` 文件仍可通过 `erasure_shards` 表记录的分片信息重建整文件。

## 配置

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `ERASURE_DATA_SHARDS` | 4 | 数据分片数 |
| `ERASURE_PARITY_SHARDS` | 2 | 校验分片数 |
| `ERASURE_MIN_FILE_SIZE` | 1048576 (1MB) | 触发 EC 的最小文件大小 |

默认 4+2 配置下，6 个分片中任意丢失 2 个仍可恢复原始数据。当前 chunk recovery 和 legacy local_ec 共用同一套编码参数。

## Chunk Recovery 编码流程

```
UploadChunk 成功后:

  chunk 文件 (≥ MinFileSize)
       │
       ▼
  EncodeErasure(chunkPath, tempDir, md5.chunk.index, 4, 2)
       │
       ├── 读取 chunk → reedsolomon.Split → 4 个 data shard
       ├── reedsolomon.Encode → 生成 2 个 parity shard
       └── 逐个上传到 OSS:
           recovery/{md5}/{chunkIndex}/shard-0.rs
           ...
           recovery/{md5}/{chunkIndex}/shard-5.rs
       │
       ▼
  Gateway 在 DownloadPlan HTTP 响应里补 recovery URL
```

## Legacy local_ec 路径

旧的本地整文件编码能力仍然保留：

- `encodeWithErasure()` 会把整文件拆成 `{md5}.shard.{index}`
- `erasure_shards` 表保存这些分片的元数据
- `OpenLocalFile()` 在 `storage_type=local_ec` 时透明重建文件

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
Chunk recovery:

  失败的主 chunk
       │
       ▼
  Gateway recovery URL
       │
       ▼
  健康 File Service 实例
       │
       ├── 从 OSS 拉取 recovery/{md5}/{chunkIndex}/shard-*.rs
       ├── ReconstructErasure(paths, 4, 2)
       └── 返回重建后的 chunk 字节流

legacy local_ec:

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
| `app/file/internal/biz/file.go` | `ErasureEncoder` 接口、`PrepareChunkRecovery`/`RecoverChunk`/`reconstructFromShards` 方法 |
| `app/file/internal/data/erasure.go` | `rsEncoder` 实现、`EncodeErasure`/`ReconstructErasure` 函数 |
| `app/file/cmd/main.go` | `provideErasureConfig` 读取环境变量 |
