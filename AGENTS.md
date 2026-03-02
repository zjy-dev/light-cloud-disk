# AI Agent 开发指南

## 代码规范

- **不要生成兼容性代码!!!**
- 所有工具链用最新版本，可以用 context7 获取最新文档
- 每次创建/修改/删除了 feature 或修复 bug 后，维护 README.md、AGENTS.md 以及 docs 目录
- docs 目录中对每个功能的具体实现进行说明（方便面试时讲清楚）
- 不要生成兼容性代码
- 从 .env 和环境变量中读取敏感配置（.env 优先）
- 常规配置使用 YAML 文件
- 为新功能编写测试，有些只需要 mock 写单元测试，有些则需要单元和集成测试

- 要有单元测试和集成测试
- 敏感配置从环境变量和 .env 中读，非敏感配置写入 YAML 配置文件

- 后端编程语言使用 Go，框架 Kratos v2
  
- 前端包管理器实用 pnpm, 如果没有 node 环境则用 fnm 配置最新的 lts 版本
- 前端框架用 Vue 3

- 容器编排使用标准 Compose spec（Docker Compose / Podman Compose 兼容）
  
- CI/CD 使用 GitHub Actions，容器镜像推送到 GHCR


## 项目架构

```
微服务架构:

    Client (HTTP)
         │
         ▼
┌─────────────────────────────────┐
│       Gin API Gateway           │
│     (HTTP :8080, JWT, CORS)     │
└────────┬───────────────┬────────┘
    gRPC │               │ gRPC
         ▼               ▼
┌────────────────┐ ┌────────────────┐
│  User Service  │ │  File Service  │
│  (gRPC :9001)  │◄│  (gRPC :9002)  │
│  Kratos v2     │ │  Kratos v2     │
└───────┬────────┘ └───┬───────┬───┘
        │              │       │
        ▼              ▼       ▼
    ┌────────┐   ┌────────┐ ┌───────┐
    │ MySQL  │   │ MySQL  │ │ Redis │
    └────────┘   └────────┘ └───────┘

    ← ─ ─ Consul 服务发现 ─ ─ →
```

每个 Kratos 服务内部采用 Clean Architecture 分层:

```
┌─────────────────────────────────────────┐
│            Service Layer                │
│   (gRPC handler, proto request/reply)   │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Biz Layer                  │
│   (UserUsecase, FileUsecase)            │
│   (定义 Repo/Client 接口)               │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Data Layer                 │
│   (实现 Repo 接口: GORM/Redis/gRPC)     │
└─────────────────────────────────────────┘
```

## 服务清单

### 用户服务 (app/user/internal/biz/user.go)
- [x] Register - 用户注册
- [x] Login - 用户登录
- [x] GetUserInfo - 获取用户信息
- [x] UpdateUserInfo - 更新用户信息
- [x] UpdateStorageUsed - 更新存储用量 (供 File Service 调用)

### 文件服务 (app/file/internal/biz/file.go)
- [x] CheckUpload - 秒传/断点续传检查
- [x] SaveChunk - 保存分块
- [x] MergeChunks - 合并分块 (完成后调用 UserClient.UpdateStorageUsed)
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

### API 网关 (app/gateway/)
- [x] HTTP→gRPC 代理 (所有用户/文件操作)
- [x] JWT 认证中间件
- [x] CORS 中间件
- [x] Consul 服务发现客户端

## 测试策略

| 模块 | 测试类型 | 测试数 | 说明 |
|------|----------|--------|------|
| app/user/internal/biz | 单元测试 | 14 | Mock UserRepo |
| app/file/internal/biz | 单元测试 | 25 | Mock FileRepo + UserClient |
| app/gateway/internal/handler | 单元测试 | 13 | Mock gRPC 客户端 |
| app/gateway/internal/middleware | 单元测试 | 8 | JWT + CORS 中间件 |
| app/user/internal/data | 集成测试 | 5 | 需要 MySQL (build tag: integration) |
| app/file/internal/data | 集成测试 | 7 | 需要 MySQL + Redis (build tag: integration) |

运行测试:
```bash
go test ./...                              # 全部单元测试 (60个)
go test -tags=integration ./...            # 包含集成测试 (需要基础设施)
```

## 服务间通信

- **Gateway → User Service**: 通过 Consul 发现 `user-service`，gRPC 调用
- **Gateway → File Service**: 通过 Consul 发现 `file-service`，gRPC 调用
- **File Service → User Service**: 通过 Consul 发现 `user-service`，调用 `UpdateStorageUsed` RPC

## MQ 消息格式 (预留)

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

| 变量 | 说明 | 服务 | 示例 |
|------|------|------|------|
| DB_HOST | 数据库地址 | user, file | localhost |
| DB_PORT | 数据库端口 | user, file | 3306 |
| DB_USER | 数据库用户 | user, file | root |
| DB_PASSWORD | 数据库密码 | user, file | *** |
| DB_NAME | 数据库名 | user, file | cloud_disk |
| REDIS_ADDR | Redis 地址 | file | localhost:6379 |
| CONSUL_ADDR | Consul 地址 | user, file, gateway | localhost:8500 |
| JWT_SECRET | JWT 密钥 | gateway | *** |
| GATEWAY_ADDR | 网关监听地址 | gateway | :8080 |
| OSS_ENDPOINT | OSS 端点 | file | oss-cn-hangzhou.aliyuncs.com |
| OSS_ACCESS_KEY_ID | OSS AK | file | *** |
| OSS_ACCESS_KEY_SECRET | OSS SK | file | *** |
| KAFKA_BROKERS | Kafka 地址 | file | localhost:9092 |

## 关键设计决策

1. **gRPC-only 服务**: User/File Service 只暴露 gRPC，不暴露 HTTP。HTTP 统一由 Gateway 处理。
2. **Gin 网关**: 选择 Gin 而非 Kratos HTTP，因为网关职责是路由/中间件，不需要 Kratos 的 Service/Biz/Data 分层。
3. **Consul 服务发现**: 使用 Kratos 的 `kratos.Registrar()` 注册，客户端使用 `discovery:///service-name` 端点。
4. **跨服务调用**: File Service 定义 `biz.UserClient` 接口，由 `data/user_client.go` 通过 gRPC 实现，解耦业务逻辑和远程调用。
5. **Wire 依赖注入**: 每个 Kratos 服务使用独立的 `wire.go`，Gateway 不使用 Wire (直接构造)。
