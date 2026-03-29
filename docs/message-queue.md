# 消息队列集成

## 概述

当前异步链路以 Kafka 为主，goroutineMQ 只保留为本地开发降级方案。消息队列主要承载两类任务：

1. `cloud-migrate`: 本地热数据超过阈值时，把 LRU 候选迁移到 OSS。
2. `file-thumbnail`: 预留给图片/视频缩略图任务，当前仍是 stub。

业务层只依赖 `MessageProducer` 接口，具体选择 Kafka 还是 goroutineMQ 由 data 层工厂决定。

## 整体流程

```
CompleteUpload / maybeEvictToCloud
        │
        └── SendCloudMigrateMessage
                 │
                 ├── Kafka producer (生产环境)
                 └── goroutineMQ (本地开发未配置 KAFKA_BROKERS)
                          │
                          ▼
                 handleCloudMigrate
                          │
                          ├── local / scattered 文件读取
                          ├── OSS Put
                          ├── 更新 file_stores / chunk_records
                          └── 递减 Redis disk_usage:local
```

## Topic 与消息结构

| Topic | 生产者 | 消费者 | 作用 |
|------|--------|--------|------|
| `cloud-migrate` | File Service | Kafka consumer 或 goroutineMQ | 本地热数据 → OSS 冷迁移 |
| `file-thumbnail` | File Service | Kafka consumer 或 goroutineMQ | 缩略图任务（当前 stub） |

`CloudMigrateMessage` 当前保留最小字段集：

```go
type CloudMigrateMessage struct {
    FileMD5   string
    SourceKey string
    FileSize  int64
}
```

## 代码结构

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | 定义 `MessageProducer`、消息结构和 `maybeEvictToCloud()` |
| `app/file/internal/data/mq_kafka.go` | Kafka producer + consumer |
| `app/file/internal/data/mq_goroutine.go` | 开发环境 goroutine 降级实现 |
| `app/file/internal/data/migration.go` | 冷迁移公共逻辑，支持 local 和 scattered |

## 冷迁移细节

迁移逻辑不是简单处理单个本地文件，而是会根据 `storage_type` 分支：

- `local`: 上传完整对象到 OSS，更新 `file_stores` 为 `oss`
- `scattered`: 遍历 `chunk_records`，把每个 chunk 单独上传到 OSS，并把 chunk record 的 `storage_type` 更新为 `oss`

这样 GetDownloadPlan 可以直接为已迁移的 chunk 签发 OSS URL，不需要再回源本地实例。

## 为什么保留 goroutineMQ

仓库仍保留 goroutineMQ，但用途已经收敛为“没有 Kafka 基础设施时的本地开发兜底”：

- 生产/容器编排路径默认使用 Kafka
- 单机开发如果没有 Kafka，可以通过不设置 `KAFKA_BROKERS` 启动最小可运行链路
- 两种实现共用同一套 `migration.go`，避免逻辑分叉

## 可靠性语义

- Kafka 路径提供至少一次消费语义，消费者失败可重试
- 冷迁移处理具备幂等性，已迁移到 OSS 的记录会被跳过
- 消息发送失败只记录告警，不阻塞上传完成路径
- goroutineMQ 不提供持久化，只适合作为开发环境降级

## 面试要点

1. 为什么 Kafka 只放在异步链路：上传和下载主路径必须尽量短，MQ 只承担冷迁移和后台任务。
2. 为什么还需要 migration.go：Kafka consumer 和 goroutineMQ 只是调度器，真正的迁移逻辑要复用，不能分叉两套实现。
3. 为什么 scattered 迁移要更新 chunk_records：下载计划依赖 chunk 级存储位置，迁移后必须让 GetDownloadPlan 知道 chunk 已经在 OSS。
