# 消息队列集成

## 概述

本项目使用消息队列实现文件上传后的异步处理，包括 **主存 → OSS 冷迁移**和缩略图生成。支持两种 MQ 实现：

| 实现 | 适用模式 | 触发条件 | 说明 |
|------|----------|----------|------|
| **Kafka** (`segmentio/kafka-go`) | Mode B (s3) | `KAFKA_BROKERS` 环境变量非空 | 分布式持久化 MQ，独立 file-worker 消费 |
| **goroutine channel** | Mode A (local) | 未配置 `KAFKA_BROKERS` | 进程内 buffered channel (容量 256)，零依赖 |

遵循 Clean Architecture，biz 层定义 `MessageProducer` 接口，data 层根据配置自动选择 Kafka 或 goroutine 实现。

## 架构

```
MergeChunks (File Service)
    │
    ├── maybeEvictToCloud() ──→ Kafka [cloud-migrate] ──→ file-worker (SeaweedFS→OSS 迁移)
    │   (SeaweedFS 用量超阈值时触发)
    │
    └── SendThumbnailMessage ─→ Kafka [file-thumbnail] ─→ ThumbnailWorker (缩略图)
         (仅媒体文件)
```

## Topic 规划

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| `cloud-migrate` | File Service (LRU 淘汰) | file-worker | SeaweedFS → 阿里云 OSS 冷迁移 |
| `file-thumbnail` | File Service | file-worker (ThumbnailWorker) | 生成文件缩略图 |

## 消息格式

### CloudMigrateMessage

当 SeaweedFS 用量超过阈值（默认 80%）时，`maybeEvictToCloud()` 选出 LRU 候选文件后发送。

```json
{
  "file_store_id": 42,
  "file_md5": "abc123...",
  "cur_location": "abc123/file.zip",
  "dest_location": "oss://abc123/file.zip",
  "size": 10485760
}
```

### ThumbnailMessage

仅当上传的是媒体文件（图片/视频）时发送。

```json
{
  "file_id": 123,
  "file_path": "abc123/image.jpg",
  "file_type": "image/jpeg"
}
```

支持的媒体扩展名：`.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.webp`, `.svg`, `.ico`, `.mp4`, `.avi`, `.mov`, `.mkv`, `.webm`, `.flv`, `.wmv`

## 代码结构

### 生产端 (File Service)

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | 定义 `MessageProducer` 接口、`CloudMigrateMessage`/`ThumbnailMessage` 结构体、`maybeEvictToCloud()` 淘汰逻辑 |
| `app/file/internal/data/kafka.go` | 实现 `kafkaProducer`（cloud-migrate + file-thumbnail 两个 Writer），以及无 Kafka 时的 `noopProducer` |
| `app/file/internal/conf/conf.proto` | `Data.Kafka` 配置（brokers, cloud_migrate_topic, thumbnail_topic） |

### 消费端 (Worker)

| 文件 | 职责 |
|------|------|
| `app/file/cmd/worker/main.go` | 独立进程，使用 kafka-go Reader（ConsumerGroup 模式）消费两个 topic。`handleCloudMigrateMessage` 执行 SeaweedFS→OSS 数据搬迁 + DB/Redis 状态更新 |

### 接口定义

```go
// biz/file.go
type MessageProducer interface {
    SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error
    SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
    Close() error
}
```

### 集成位置

在 `FileUsecase.MergeChunks()` 中，文件上传 SeaweedFS 成功后：

1. 异步调用 `maybeEvictToCloud()`：若 SeaweedFS 用量超阈值，查找 LRU 候选并发送 `CloudMigrateMessage`
2. 判断文件扩展名，若为媒体文件则发送 `ThumbnailMessage`
3. 发送失败仅 warn 日志，**不影响主流程**（fire-and-forget）

### file-worker 迁移流程

```
handleCloudMigrateMessage:
  1. SeaweedFS.Get(key) → 下载文件数据
  2. OSS.Put(destKey, data) → 上传到阿里云 OSS
  3. UPDATE file_store SET storage_type='oss', location=destLocation
  4. SeaweedFS.Delete(key) → 释放 SeaweedFS 空间
  5. Redis INCRBY disk_usage:seaweedfs -(size) → 递减用量计数器
```

## 配置

### conf.proto

```protobuf
message Data {
  message Kafka {
    repeated string brokers = 1;
    string cloud_migrate_topic = 2;
    string thumbnail_topic = 3;
  }
  Database database = 1;
  Redis redis = 2;
  Kafka kafka = 3;
}
```

### config.yaml

```yaml
data:
  kafka:
    brokers:
      - ${KAFKA_BROKERS:localhost:9092}
    cloud_migrate_topic: cloud-migrate
    thumbnail_topic: file-thumbnail
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `KAFKA_BROKERS` | Kafka broker 地址（逗号分隔） | 无（降级为 goroutine MQ） |
| `KAFKA_CLOUD_MIGRATE_TOPIC` | 云迁移 topic（Worker 端） | `cloud-migrate` |
| `KAFKA_THUMBNAIL_TOPIC` | 缩略图 topic（Worker 端） | `file-thumbnail` |
| `KAFKA_GROUP_ID` | 消费者组 ID | `file-worker-group` |

## goroutine MQ (轻量模式)

当 `KAFKA_BROKERS` 为空时，`NewMessageProducer` 自动创建 `goroutineMQ`：

```go
type goroutineMQ struct {
    cloudCh    chan *biz.CloudMigrateMessage   // 缓冲 256
    thumbCh    chan *biz.ThumbnailMessage      // 缓冲 256
    repo       biz.FileRepo
    objStore   biz.ObjectStorage
    cloudStore biz.CloudStorage
}
```

- 启动两个后台 goroutine 分别消费两个 channel
- `consumeCloudMigrate()`: 读主存文件 → 上传 OSS → 更新 DB → 删主存 → 递减 Redis 计数器
- 消息不持久化，进程重启丢失（单机场景可接受，重启后 `maybeEvictToCloud` 会重新触发）
- 实现 `biz.MessageProducer` 接口，业务层无感知

## 可靠性设计

1. **Kafka 生产端**: 同步 Write，`RequiredAcks = RequireAll`（等待所有副本确认）
2. **Kafka 消费端**: `FetchMessage` + 处理成功后 `CommitMessages`（手动 offset 提交）
3. **自动降级**: 未配置 `KAFKA_BROKERS` → goroutine MQ；MQ 发送失败仅 warn 日志，不阻塞主流程
4. **消息 Key**: CloudMigrateMessage 用 `file_md5`，ThumbnailMessage 用 `file_id`
5. **幂等处理**: Worker/goroutine 以 `file_store_id` + `storage_type` 判断是否已迁移，跳过重复消息

## 待实现

- [x] CloudMigrateWorker：SeaweedFS → OSS 异步迁移（完整实现）
- [ ] ThumbnailWorker：图片缩放 / 视频截帧（需引入图片处理库）
- [ ] 死信队列：消费失败 N 次后写入 `*-dlq` topic
- [ ] 定时补偿扫描：兜底检查未迁移的文件

## Docker 部署

```yaml
# Included in docker-compose.yml:
# - kafka (KRaft mode, apache/kafka:3.7.0)
# - seaweedfs (S3-compatible object storage)
# - file-worker (consumer, standalone container)
```

Worker 依赖 Kafka 健康检查通过后启动，与 file-service 独立部署，可独立扩缩容。

## 面试要点

1. **为什么用 MQ？** — 异步解耦，上传完即返回，冷迁移/缩略图后台处理，提高响应速度
2. **为什么选 Kafka？** — 高吞吐、持久化、消费者组实现水平扩展
3. **消息丢失怎么办？** — acks=all + 手动 commit + 死信队列 + 定时补偿
4. **消息重复怎么办？** — 幂等设计，Worker 检查 storage_type 是否已为 oss
5. **Clean Architecture 如何集成？** — biz 层定义 `MessageProducer` 接口，data 层实现，Wire 注入，业务逻辑不依赖具体 MQ 实现
6. **没有 Kafka 怎么办？** — `noopProducer` 降级，不影响核心上传流程
7. **为什么从 file-transfer 改为 cloud-migrate？** — 语义更清晰，消息不再是"本地→OSS"的简单转存，而是三级存储间的"冷迁移"
