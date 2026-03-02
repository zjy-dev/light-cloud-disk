# 消息队列使用场景

## 概述

本项目预留了 Kafka 消息队列支持，用于未来实现异步任务和解耦服务。当前 docker-compose.yml 已包含 Kafka 服务，但应用代码中尚未集成 MQ 生产/消费逻辑。

## Topic 规划

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| `file-transfer` | File Service | TransferWorker | 文件异步转存 OSS |
| `file-thumbnail` | File Service | ThumbnailWorker | 生成文件缩略图 |

## 场景详解

### 1. 文件异步转存 OSS

**问题**: 大文件上传完成后同步转存 OSS 耗时长，影响用户体验

**方案**:
```
MergeChunks 完成 → 返回成功 → 发送 Kafka 消息 → 后台 Worker 异步转存
```

**消息格式**:
```json
{
  "file_md5": "abc123...",
  "cur_location": "/store/abc123/file.zip",
  "dest_location": "oss://bucket/abc123/file.zip"
}
```

### 2. 缩略图生成

**问题**: 图片/视频上传后需要生成预览图，同步处理慢

**方案**:
```
MergeChunks 完成 → 判断文件类型 → 发送消息 → Worker 生成缩略图 → 更新数据库
```

**消息格式**:
```json
{
  "file_id": 123,
  "file_path": "/store/abc123/image.jpg",
  "file_type": "image/jpeg"
}
```

## 集成位置

MQ 生产者应在 File Service 的 `MergeChunks` 方法中集成：

```go
// app/file/internal/biz/file.go
func (uc *FileUsecase) MergeChunks(...) error {
    // ... 合并逻辑
    // ... 更新存储用量

    // TODO: 发送转存消息到 Kafka
    // uc.mq.Produce("file-transfer", TransferMessage{...})

    // TODO: 如果是图片/视频，发送缩略图消息
    // if isMediaFile(fileName) {
    //     uc.mq.Produce("file-thumbnail", ThumbnailMessage{...})
    // }

    return nil
}
```

## 消息可靠性设计

1. **生产端**: 同步等待 Kafka 确认 (acks=all)
2. **消费端**: 处理成功后才提交 offset
3. **失败重试**: 消费失败写入死信队列
4. **幂等处理**: 使用 file_md5 作为唯一键，防止重复处理

## 面试要点

1. **为什么用 MQ？** — 异步解耦，削峰填谷，提高用户端响应速度
2. **为什么选 Kafka？** — 高吞吐，持久化，适合日志和事件流场景
3. **消息丢失怎么办？** — ACK 机制 + 死信队列 + 定时补偿扫描
4. **消息重复怎么办？** — 幂等设计，用 file_md5 去重
