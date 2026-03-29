# 冷热分层存储

## 概述

当前冷热分层的核心不是“合并后整文件再迁移”，而是：

- 热层：本地 scattered chunk
- 冷层：OSS 对象
- 迁移触发：本地用量超过阈值后的 LRU 淘汰

除了冷迁移外，直传 chunk 的恢复分片也会写到 OSS，但那部分不计入冷层对象本身，它们用于实例故障时的 chunk 重建。

## 分层视图

| 层 | 介质 | 保存内容 | 说明 |
|----|------|----------|------|
| 热层 | 本地磁盘 | scattered chunk | 主下载路径，低延迟 |
| 恢复层 | OSS | 每块 recovery shards | 实例故障时重建 chunk |
| 冷层 | OSS | LRU 淘汰后的 chunk 或对象 | 节省本地容量 |

## 触发条件

`maybeEvictToCloud()` 基于 `disk_usage:local` 判断是否需要淘汰：

- `ThresholdPct`：默认 80
- `EvictTargetPct`：默认把用量降到阈值的 90%

淘汰时会同时扫描：

- `storage_type=local`
- `storage_type=scattered`

然后统一按 `last_accessed_at` 排序选出 LRU 候选。

## 迁移路径

### local

1. 读取本地对象
2. 上传 OSS
3. 更新 `file_stores.storage_type=oss`
4. 递减 `disk_usage:local`

### scattered

1. 查询该文件的全部 `chunk_records`
2. 逐块上传到 OSS
3. 把每个 chunk record 的 `storage_type` 更新为 `oss`
4. 递减对应的本地用量

因此迁移后的下载计划可以直接返回 chunk 级 OSS URL，而不是要求恢复到本地后再下载。

## 下载路径

- 热层 chunk：返回原始实例下载 URL
- 热层失败：前端切到 recovery URL，健康实例从恢复分片重建
- 冷层 chunk：`GetDownloadPlan` 直接返回该 chunk 的 OSS 预签名 URL
- 整对象 OSS：`GetDownloadURL` 返回单文件预签名 URL

## 用量计数

Redis 继续使用 `disk_usage:local` 原子计数器：

- chunk 落地时 `INCRBY +chunkSize`
- local/scattered 迁移完成时 `INCRBY -size`

这样不需要为每次上传做数据库聚合。

## 面试要点

1. 为什么恢复分片和冷层对象都在 OSS：两者职责不同，一个保可用性，一个保容量，但底层介质可以复用。
2. 为什么要迁移 scattered 而不是只迁移整文件：当前系统本来就不合并 chunk，迁移必须尊重 chunk 级元数据。
3. 为什么 LRU 候选要同时扫 local 和 scattered：系统里两种热层形态都可能占用本地磁盘，不能只看单一 storage_type。
