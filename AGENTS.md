# AI Agent 开发指南

## 代码规范

- 每次创建/修改/删除了 feature 或修复 bug 后，维护 README.md、AGENTS.md 以及 docs 目录
- docs 目录中对每个功能的具体实现进行说明（方便面试时讲清楚）
- 不要生成兼容性代码
- 从 .env 和环境变量中读取敏感配置（.env 优先）
- 常规配置使用 YAML 文件
- 为新功能编写测试，有些只需要 mock 写单元测试，有些则需要单元和集成测试

## 项目架构

```
分层架构 (Clean Architecture):

┌─────────────────────────────────────────┐
│              API Layer                  │
│         (proto + HTTP/gRPC)             │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│            Service Layer                │
│      (api/user/v1, api/file/v1)         │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Biz Layer                  │
│   (UserUsecase, FileUsecase)            │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Data Layer                 │
│    (UserRepo, FileRepo + GORM/Redis)    │
└─────────────────────────────────────────┘
```

## 功能清单

### 用户服务 (internal/biz/user.go)
- [x] Register - 用户注册
- [x] Login - 用户登录
- [x] GetUserInfo - 获取用户信息
- [x] UpdateUserInfo - 更新用户信息

### 文件服务 (internal/biz/file.go)
- [x] CheckUpload - 秒传/断点续传检查
- [x] SaveChunk - 保存分块
- [x] MergeChunks - 合并分块
- [x] ListFiles - 文件列表
- [x] CreateFolder - 创建文件夹
- [x] RenameFile - 重命名
- [x] DeleteFiles - 删除（软删除到回收站）
- [x] MoveFiles - 移动文件
- [x] ListTrash - 回收站列表
- [x] RestoreFiles - 恢复文件
- [x] PermanentDelete - 彻底删除
- [x] CreateShare - 创建分享
- [x] GetShare - 获取分享内容
- [x] SearchFiles - 搜索文件

## 测试策略

| 模块 | 测试类型 | 说明 |
|------|----------|------|
| biz/user.go | 单元测试 | Mock UserRepo |
| biz/file.go | 单元测试 | Mock FileRepo |
| data/user.go | 集成测试 | 需要 MySQL |
| data/file.go | 集成测试 | 需要 MySQL + Redis |
| service/*.go | 单元测试 | Mock Usecase |

## MQ 消息格式

### TransferMessage (file-transfer)
```json
{
  "file_md5": "abc123...",
  "cur_location": "/store/abc123/file.zip",
  "dest_location": "oss://bucket/abc123/file.zip"
}
```

### ThumbnailMessage (file-thumbnail)
```json
{
  "file_id": 123,
  "file_path": "/store/abc123/image.jpg",
  "file_type": "image/jpeg"
}
```

## 环境变量

| 变量 | 说明 | 示例 |
|------|------|------|
| DB_HOST | 数据库地址 | localhost |
| DB_PORT | 数据库端口 | 3306 |
| DB_USER | 数据库用户 | root |
| DB_PASSWORD | 数据库密码 | *** |
| DB_NAME | 数据库名 | cloud_disk |
| REDIS_ADDR | Redis地址 | localhost:6379 |
| JWT_SECRET | JWT密钥 | *** |
| OSS_ENDPOINT | OSS端点 | oss-cn-hangzhou.aliyuncs.com |
| OSS_ACCESS_KEY_ID | OSS AK | *** |
| OSS_ACCESS_KEY_SECRET | OSS SK | *** |
| KAFKA_BROKERS | Kafka地址 | localhost:9092 |
