# Kafka 消息队列

## 概述

light-cloud-disk 使用 Apache Kafka 作为异步消息队列，处理文件云迁移（本地 → OSS）和缩略图生成等后台任务。当环境变量 `KAFKA_BROKERS` 未设置时，自动降级为进程内 goroutine channel 实现，方便本地开发。

## 架构

```
┌──────────────────┐     produce     ┌──────────────────┐
│  File Service    │ ──────────────► │  Apache Kafka    │
│  (biz layer)     │                 │  (KRaft mode)    │
│  maybeEvictToCloud│                │                  │
└──────────────────┘                 │  Topics:         │
                                     │  - cloud-migrate │
                                     │  - file-thumbnail│
                                     └───────┬──────────┘
                                             │ consume
                                             ▼
                                     ┌──────────────────┐
                                     │  Kafka Consumer   │
                                     │  (file-service)   │
                                     │  - OSS upload     │
                                     │  - DB update      │
                                     └──────────────────┘
```

## Topic 定义

### cloud-migrate

本地磁盘文件迁移到阿里云 OSS 的异步任务。当本地主存用量超过阈值时触发 LRU 淘汰。

**消息格式**:
```json
{
  "file_store_id": 42,
  "file_md5": "abc123def456...",
  "source_key": "abc123/file.zip",
  "file_size": 10485760
}
```

**处理流程**:
1. 读取本地文件
2. 上传到 OSS
3. 更新 DB: storage_type → "oss"
4. 删除本地文件
5. 减少 Redis disk_usage:local 计数器

### file-thumbnail

缩略图生成任务（当前为 stub 实现）。

**消息格式**:
```json
{
  "file_id": 123,
  "file_path": "abc123/image.jpg",
  "file_type": "image/jpeg"
}
```

## 实现

### Producer

`kafkaProducer` 实现 `biz.MessageProducer` 接口：

```go
type MessageProducer interface {
    SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error
    SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
    Close() error
}
```

- 使用 `segmentio/kafka-go` 的 `kafka.Writer`
- 两个独立 Writer（cloud-migrate / file-thumbnail）
- `RequiredAcks: RequireAll` 确保消息持久化
- key 使用文件 MD5，保证同一文件的消息有序

### Consumer

`KafkaConsumer` 实现 Kratos `transport.Server` 接口：

```go
// Start begins consuming messages in background goroutines.
func (c *KafkaConsumer) Start(ctx context.Context) error

// Stop gracefully shuts down the consumer.
func (c *KafkaConsumer) Stop(ctx context.Context) error
```

- Consumer Group ID: `file-service`
- 每个 topic 独立 goroutine 消费
- `StartOffset: LastOffset` — 只消费新消息
- 显式 CommitMessages 确保 at-least-once 语义

### 降级策略

当 `KAFKA_BROKERS` 环境变量为空时，`NewMessageProducer` 自动创建 `goroutineMQ`：

```go
func NewMessageProducer(...) (biz.MessageProducer, func(), error) {
    if brokers := os.Getenv("KAFKA_BROKERS"); brokers != "" {
        return newKafkaProducer(...)
    }
    return newGoroutineMQ(...)
}
```

## 配置

### 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| KAFKA_BROKERS | Kafka broker 地址（逗号分隔） | localhost:9092 |

### config.yaml

```yaml
data:
  kafka:
    brokers:
      - ${KAFKA_BROKERS:localhost:9092}
    cloud_migrate_topic: cloud-migrate
    thumbnail_topic: file-thumbnail
```

### conf.proto

```protobuf
message Kafka {
  repeated string brokers = 1;
  string cloud_migrate_topic = 2;
  string thumbnail_topic = 3;
}
```

## Docker Compose

使用 Apache Kafka 3.9 KRaft 模式（无需 ZooKeeper）：

```yaml
kafka:
  image: apache/kafka:3.9
  ports:
    - "9092:9092"
  environment:
    KAFKA_NODE_ID: 1
    KAFKA_PROCESS_ROLES: broker,controller
    KAFKA_LISTENERS: PLAINTEXT://0.0.0.0:9092,CONTROLLER://0.0.0.0:9093
    KAFKA_ADVERTISED_LISTENERS: PLAINTEXT://kafka:9092
    KAFKA_CONTROLLER_QUORUM_VOTERS: 1@kafka:9093
    KAFKA_CONTROLLER_LISTENER_NAMES: CONTROLLER
    KAFKA_OFFSETS_TOPIC_REPLICATION_FACTOR: 1
```
