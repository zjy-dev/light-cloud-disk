# 消息队列使用场景

## 概述

本项目使用 Kafka 作为消息队列，用于处理异步任务和解耦服务。

## Topic 列表

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| `file-transfer` | FileService | TransferWorker | 文件异步转存OSS |
| `file-thumbnail` | FileService | ThumbnailWorker | 生成文件缩略图 |
| `trash-cleanup` | CronJob | CleanupWorker | 回收站过期清理 |
| `share-expire` | CronJob | ShareWorker | 分享链接过期处理 |
| `storage-update` | FileService | StorageWorker | 更新用户存储统计 |

## 场景详解

### 1. 文件异步转存OSS

**问题**: 大文件上传完成后同步转存OSS耗时长，影响用户体验

**方案**: 
```
上传完成 → 返回成功 → 发送Kafka消息 → 后台Worker异步转存
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
上传完成 → 判断文件类型 → 发送消息 → Worker生成缩略图 → 更新数据库
```

**支持格式**: jpg, png, gif, mp4, pdf

### 3. 回收站清理

**问题**: 回收站文件30天后自动删除，需要定时任务

**方案**:
```
CronJob(每天凌晨) → 查询过期文件 → 分批发送消息 → Worker删除文件和OSS对象
```

**好处**: 避免大批量删除阻塞数据库

### 4. 分享链接过期

**问题**: 分享链接过期后需要标记失效

**方案**:
```
CronJob(每小时) → 查询过期分享 → 发送消息 → Worker更新状态
```

## 消息可靠性

1. **生产端**: 同步等待Kafka确认
2. **消费端**: 处理成功后才提交offset
3. **失败重试**: 消费失败写入死信队列
4. **幂等处理**: 使用file_md5作为唯一键

## 面试要点

1. **为什么用MQ？** - 异步解耦，削峰填谷，提高响应速度
2. **为什么选Kafka？** - 高吞吐，持久化，适合日志和事件流
3. **消息丢失怎么办？** - ACK机制 + 死信队列 + 定时补偿
4. **消息重复怎么办？** - 幂等设计，用唯一键去重
