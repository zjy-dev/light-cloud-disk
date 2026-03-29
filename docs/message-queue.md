# 消息队列集成

## 概述

本项目使用进程内 goroutine 消息队列实现文件上传后的异步处理，包括 **本地磁盘 → 阿里云 OSS 冷迁移**和缩略图生成。

遵循 Clean Architecture，biz 层定义 `MessageProducer` 接口，data 层通过 `goroutineMQ` 实现。业务逻辑与 MQ 实现完全解耦，未来可替换为 Kafka 等分布式 MQ 而不改动 biz 层。

## 架构

```
MergeChunks (File Service)
    │
    ├── maybeEvictToCloud() ──→ goroutine channel [cloud-migrate] ──→ 本地→OSS 冷迁移
    │   (本地磁盘用量超阈值时触发 LRU 淘汰)
    │
    └── SendThumbnailMessage ─→ goroutine channel [thumbnail] ─→ 缩略图 (stub)
         (仅媒体文件)
```

## 消息通道

| 通道 | 缓冲区 | 生产者 | 消费者 | 用途 |
|------|--------|--------|--------|------|
| `cloudMigrateCh` | 256 | File Service (LRU 淘汰) | goroutine | 本地磁盘 → 阿里云 OSS 冷迁移 |
| `thumbnailCh` | 256 | File Service | goroutine (stub) | 生成文件缩略图 |

## 消息格式

### CloudMigrateMessage

当本地磁盘用量超过阈值（`PRIMARY_MAX_BYTES`，默认 10 GB）时，`maybeEvictToCloud()` 选出 LRU 候选文件后发送。

```go
type CloudMigrateMessage struct {
    FileStoreID int64  // file_stores 表主键
    FileMD5     string // 文件 MD5
    SourceKey   string // 本地存储路径 key
    FileSize    int64  // 文件字节数
}
```

### ThumbnailMessage

仅当上传的是媒体文件（图片/视频）时发送。

```go
type ThumbnailMessage struct {
    FileID   int64  // 文件记录 ID
    FilePath string // 文件存储路径
    FileType string // MIME 类型
}
```

支持的媒体扩展名：`.jpg`, `.jpeg`, `.png`, `.gif`, `.bmp`, `.webp`, `.svg`, `.ico`, `.mp4`, `.avi`, `.mov`, `.mkv`, `.webm`, `.flv`, `.wmv`

## 代码结构

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | 定义 `MessageProducer` 接口、`CloudMigrateMessage`/`ThumbnailMessage` 结构体、`maybeEvictToCloud()` 淘汰逻辑 |
| `app/file/internal/data/mq_goroutine.go` | 实现 `goroutineMQ`：两个 buffered channel + 两个后台 goroutine 消费者 |

### 接口定义

```go
// biz/file.go
type MessageProducer interface {
    SendCloudMigrateMessage(ctx context.Context, msg *CloudMigrateMessage) error
    SendThumbnailMessage(ctx context.Context, msg *ThumbnailMessage) error
    Close() error
}
```

### goroutineMQ 实现

```go
type goroutineMQ struct {
    cloudMigrateCh chan *biz.CloudMigrateMessage // 缓冲 256
    thumbnailCh    chan *biz.ThumbnailMessage    // 缓冲 256
    repo           biz.FileRepo
    cloudStore     biz.CloudStorage
    storeDir       string
}
```

- `NewMessageProducer()` 创建 `goroutineMQ`，启动两个后台 goroutine
- `consumeCloudMigrate()`: 读本地文件 → 上传 OSS → 更新 DB (storage_type → oss) → 删本地文件 → 递减 Redis 计数器
- `consumeThumbnails()`: stub 实现，仅打印日志
- 优雅关闭：通过 `done` channel 通知两个 goroutine 停止，`wg.Wait()` 等待退出，关闭前 drain 残留消息

### 集成位置

在 `FileUsecase.MergeChunks()` 中，文件合并存储到本地磁盘后：

1. 异步调用 `maybeEvictToCloud()`：若本地磁盘用量超阈值，查找 LRU 候选并发送 `CloudMigrateMessage`
2. 判断文件扩展名，若为媒体文件则发送 `ThumbnailMessage`
3. 发送失败仅 warn 日志，**不影响主流程**（fire-and-forget）

### 冷迁移处理流程

```
handleCloudMigrate:
  1. repo.FindStoreByMD5(md5) → 查询文件状态
  2. 检查 storage_type 是否已为 oss (幂等跳过)
  3. os.Open(localPath) → 读取本地文件
  4. cloudStore.Put(key, data, size) → 上传到阿里云 OSS
  5. repo.UpdateStorageLocation(md5, "oss", key) → 更新 DB
  6. os.Remove(localPath) → 删除本地文件
  7. repo.IncrDiskUsage("local", -size) → 递减主存计数器
```

## 可靠性设计

1. **非阻塞发送**: `select` + `ctx.Done()` + `mq.done`，避免 channel 满时阻塞主流程
2. **幂等消费**: `handleCloudMigrate` 先检查 `storage_type != "local"` 则跳过，防止重复迁移
3. **fire-and-forget**: MQ 发送失败仅 warn 日志，不阻塞上传主流程
4. **优雅关闭**: `close(done)` → drain channel 残留消息 → `wg.Wait()`
5. **自动补偿**: 消息不持久化，进程重启丢失。重启后 `maybeEvictToCloud()` 会在下次上传时重新触发淘汰检查

## 待实现

- [x] CloudMigrateWorker：本地 → OSS 异步迁移
- [ ] ThumbnailWorker：图片缩放 / 视频截帧（需引入图片处理库）
- [ ] 定时补偿扫描：兜底检查未迁移的文件

## 面试要点

1. **为什么用 MQ？** — 异步解耦，上传完即返回，冷迁移/缩略图后台处理，提高响应速度
2. **为什么用进程内 channel 而不是 Kafka？** — 当前单进程部署足够；接口抽象已就绪，未来需分布式消费时可替换为 Kafka 而不改业务代码
3. **消息丢失怎么办？** — 进程内 channel 不持久化，重启丢失。通过 `maybeEvictToCloud()` 在下次上传时重新检查磁盘用量触发淘汰，实现最终一致
4. **消息重复怎么办？** — 幂等设计，消费前检查 `storage_type` 是否已为 `oss`，已迁移则跳过
5. **Clean Architecture 如何集成？** — biz 层定义 `MessageProducer` 接口，data 层实现 `goroutineMQ`，Wire 注入，业务逻辑不依赖具体 MQ 实现
6. **channel 满了怎么办？** — 缓冲区 256，`select` 多路复用避免死锁；极端情况下 context 超时返回错误，主流程不受影响
