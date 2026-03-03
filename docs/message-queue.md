# 消息队列集成

## 概述

本项目使用 Kafka 消息队列实现文件上传后的异步处理，包括 OSS 转存和缩略图生成。采用 `segmentio/kafka-go` 作为 Go 客户端，遵循 Clean Architecture 在 biz 层定义 `MessageProducer` 接口，data 层实现 Kafka 生产者，独立 Worker 进程消费消息。

## 架构

```
MergeChunks (File Service)
    │
    ├── SendTransferMessage ──→ Kafka [file-transfer] ──→ TransferWorker (OSS 转存)
    │
    └── SendThumbnailMessage ─→ Kafka [file-thumbnail] ─→ ThumbnailWorker (缩略图)
         (仅媒体文件)
```

## Topic 规划

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| `file-transfer` | File Service | file-worker (TransferWorker) | 文件异步转存 OSS |
| `file-thumbnail` | File Service | file-worker (ThumbnailWorker) | 生成文件缩略图 |

## 消息格式

### TransferMessage

文件合并完成后始终发送，用于将本地文件异步转存至 OSS。

```json
{
  "file_md5": "abc123...",
  "cur_location": "/store/abc123/file.zip",
  "dest_location": "oss://bucket/abc123/file.zip"
}
```

### ThumbnailMessage

仅当上传的是媒体文件（图片/视频）时发送。

```json
{
  "file_id": 123,
  "file_path": "/store/abc123/image.jpg",
  "file_type": "image/jpeg"
}
```

支持的媒体扩展名：`.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.webp`, `.svg`, `.ico`, `.mp4`, `.avi`, `.mov`, `.mkv`, `.webm`, `.flv`, `.wmv`

## 代码结构

### 生产端 (File Service)

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | 定义 `MessageProducer` 接口、`TransferMessage`/`ThumbnailMessage` 结构体 |
| `app/file/internal/data/kafka.go` | 实现 `kafkaProducer`（使用 kafka-go Writer），以及无 Kafka 时的 `noopProducer` |
| `app/file/internal/conf/conf.proto` | `Data.Kafka` 配置（brokers, transfer_topic, thumbnail_topic） |

### 消费端 (Worker)

| 文件 | 职责 |
|------|------|
| `app/file/cmd/worker/main.go` | 独立进程，使用 kafka-go Reader（ConsumerGroup 模式）消费两个 topic |

### 接口定义

```go
// biz/file.go
type MessageProducer interface {
    SendTransferMessage(ctx context.Context, msg *TransferMessage) error
    SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
    Close() error
}
```

### 集成位置

在 `FileUsecase.MergeChunks()` 中，文件创建成功后：

1. **始终**发送 `TransferMessage`（本地路径 → OSS 目标路径）
2. 判断文件扩展名，若为媒体文件则发送 `ThumbnailMessage`
3. 发送失败仅 warn 日志，**不影响主流程**（fire-and-forget）

## 配置

### conf.proto

```protobuf
message Data {
  message Kafka {
    repeated string brokers = 1;
    string transfer_topic = 2;
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
    transfer_topic: file-transfer
    thumbnail_topic: file-thumbnail
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `KAFKA_BROKERS` | Kafka broker 地址（逗号分隔） | 无（降级为 noopProducer） |
| `KAFKA_TRANSFER_TOPIC` | 转存 topic（Worker 端） | `file-transfer` |
| `KAFKA_THUMBNAIL_TOPIC` | 缩略图 topic（Worker 端） | `file-thumbnail` |
| `KAFKA_GROUP_ID` | 消费者组 ID | `file-worker-group` |

## 可靠性设计

1. **生产端**: 同步 Write，`RequiredAcks = RequireAll`（等待所有副本确认）
2. **消费端**: `FetchMessage` + 处理成功后 `CommitMessages`（手动 offset 提交）
3. **降级策略**: 未配置 `KAFKA_BROKERS` 时自动使用 `noopProducer`，MQ 失败不阻塞主流程
4. **消息 Key**: TransferMessage 用 `file_md5`，ThumbnailMessage 用 `file_id`，保证同文件消息路由到同分区
5. **幂等处理**: 使用 `file_md5` / `file_id` 作为唯一键，消费端可据此去重

## 待实现

- [ ] TransferWorker：实际 OSS 上传（读本地文件 → 上传 OSS → 更新 file_store 表）
- [ ] ThumbnailWorker：图片缩放 / 视频截帧（需引入图片处理库）
- [ ] 死信队列：消费失败 N 次后写入 `*-dlq` topic
- [ ] 定时补偿扫描：兜底检查未转存的文件

## Docker 部署

```yaml
# docker-compose.yml 中已包含：
# - kafka (KRaft 模式, apache/kafka:3.6.2)
# - file-worker (消费端, 独立容器)
```

Worker 依赖 Kafka 健康检查通过后启动，与 file-service 独立部署，可独立扩缩容。

## 面试要点

1. **为什么用 MQ？** — 异步解耦，上传完即返回，转存/缩略图后台处理，提高响应速度
2. **为什么选 Kafka？** — 高吞吐、持久化、消费者组实现水平扩展
3. **消息丢失怎么办？** — acks=all + 手动 commit + 死信队列 + 定时补偿
4. **消息重复怎么办？** — 幂等设计，用 file_md5/file_id 去重
5. **Clean Architecture 如何集成？** — biz 层定义 `MessageProducer` 接口，data 层实现，Wire 注入，业务逻辑不依赖具体 MQ 实现
6. **没有 Kafka 怎么办？** — `noopProducer` 降级，不影响核心上传流程
