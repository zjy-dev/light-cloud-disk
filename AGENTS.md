# AI Agent 开发指南

## 代码规范

- **不要生成兼容性代码!!!**
- 所有工具链用最新版本，可以用 context7 获取最新文档
- 每次创建/修改/删除了 feature 或修复 bug 后，维护 README.md
- docs 目录中对每个功能的具体实现进行说明（方便面试时讲清楚）
- 积极使用 Makefile
- 从 .env 和环境变量中读取敏感配置（.env 优先）
- 常规配置使用 YAML 文件
- 为新功能编写测试，有些只需要 mock 写单元测试，有些则需要单元和集成测试

- 敏感配置从环境变量和 .env 中读，非敏感配置写入 YAML 配置文件

- 后端编程语言使用 Go，框架 Kratos v2
  
- 前端包管理器使用 pnpm, 如果没有 node 环境则用 fnm 配置最新的 lts 版本
- 前端框架用 Vue 3

- python 使用 uv 来管理虚拟环境和运行

- 容器编排使用标准 Compose spec（Podman Compose / Docker Compose 兼容），以 Podman 为一等公民
- Makefile 镜像构建统一使用 `--network host`，解决 podman-compose 不支持 `build.network` 的问题, 启动容器阵列前先 make images, 再 make up 启动
  
- CI/CD 使用 GitHub Actions，容器镜像推送到 GHCR
- GitHub Actions 的 Go 版本必须通过 `go-version-file: go.mod` 读取，并设置 `GOTOOLCHAIN=local`，避免版本漂移或自动 toolchain 导致测试失败


## 项目架构

```
微服务架构:

    Client (HTTP)
         │
         ├─── CheckUpload ──────────────────────────────────┐
         │                                                   │
         │ (返回 UploadPlan: 分块→实例映射)                    │
         │                                                   │
         ├─── PUT chunks 直传 ──► File Service HTTP :9003 ×N │
         │                                                   │
         ▼                                                   │
┌─────────────────────────────────┐                          │
│       Gin API Gateway           │                          │
│  (HTTP :8080, JWT, CORS, Hash) │                          │
└────────┬───────────────┬────────┘                          │
    gRPC │               │ gRPC                              │
         ▼               ▼                                   │
┌────────────────┐ ┌────────────────┐                        │
│  User Service  │ │  File Service  │◄───────────────────────┘
│  (gRPC :9001)  │◄│  (gRPC :9002)  │
│  Kratos v2     │ │  (HTTP :9003)  │
│  MySQL/SQLite  │ │  Kratos v2     │
└───────┬────────┘ └──┬────┬───┬───┘
        │             │    │   │
        ▼             ▼    ▼   ▼
    ┌────────┐   ┌──────┐ ┌─────┐ ┌──────────┐
    │  DB    │   │  DB  │ │Redis│ │ 本地磁盘  │
    └────────┘   └──────┘ └─────┘ └──────────┘

    ← ─ ─ Consul 服务发现 ─ ─ →

    File Service ──→ Kafka MQ ──→ 阿里云 OSS
    (无 Kafka 时降级为 goroutine channel)
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

## 根目录文件与目录说明

### 根目录文件

| 文件 | 作用 |
|------|------|
| `go.mod` / `go.sum` | Go module 定义 (`github.com/J-Y-Zhang/light-cloud-disk`)，锁定全部依赖版本 |
| `Makefile` | 统一构建入口：`make build` / `make test` / `make api` / `make wire` / `make run-*` / `make fe-*` / `make image-*` / `make infra-up` 等 |
| `Dockerfile` | 统一多阶段构建 (CGO_ENABLED=1 支持 SQLite)，基于 debian (golang:1.25 + bookworm-slim，内置 gcc 无需网络)，通过 `--build-arg SERVICE=user\|file\|gateway` 编译 3 种服务到同一镜像规格 |
| `docker-compose.yml` | 容器编排 (Consul + Redis + MySQL + 3 服务 + 前端) |
| `.env.example` | 环境变量模板 |
| `.dockerignore` | Docker 构建排除规则；注意 `vendor/` 有意保留以支持离线构建 |
| `.gitignore` | Git 忽略规则 (`bin/`、`/cmd`、`/worker`、`frontend/dist/`、`.env`、`coverage.out` 等) |
| `README.md` | 项目 README，面向外部用户和面试官 |
| `AGENTS.md` | 本文件，面向 AI Agent 和开发者的内部开发规范 |

### 根目录子目录

| 目录 | 作用 |
|------|------|
| `api/` | Protobuf 接口定义及其生成的 Go 代码 (`user/v1/`, `file/v1/`) |
| `app/` | **核心业务代码**，包含三个微服务 (user, file, gateway) |
| `vendor/` | `go mod vendor` 生成的依赖源码副本；Dockerfile 用 `-mod=vendor` 做**零网络离线构建**，保证 CI/本地/容器三者一致。`.dockerignore` 故意不排除此目录 |
| `third_party/` | 第三方 proto (google/api annotations)，供 `protoc` 编译时引用 |
| `docs/` | 面试 / 设计文档 (architecture / storage / cold-hot-storage / gateway / chunk-upload / presigned-upload / service-discovery / message-queue / distributed-locking / erasure-coding / ci-cd / containerization / frontend / interview) |
| `frontend/` | Vue 3 前端 SPA，独立 pnpm 项目；拥有自己的 `Dockerfile`（Node 构建 → Nginx 运行） |
| `.github/workflows/` | `ci.yml`（Push/PR → 后端测试 + 前端测试 + Compose 冒烟 + 构建 + GHCR 推送）、`release.yml`（tag → 多架构二进制 → GitHub Release） |

### `api/` 目录

```
api/
├── user/v1/
│   ├── user.proto          # 用户服务 gRPC 接口定义
│   ├── user.pb.go          # protoc 生成的消息类型
│   └── user_grpc.pb.go     # protoc 生成的 gRPC 客户端/服务端代码
└── file/v1/
    ├── file.proto
    ├── file.pb.go
    └── file_grpc.pb.go
```

### `app/` 目录 (核心)

```
app/
├── user/                        # 用户服务 (Kratos v2, gRPC :9001)
│   ├── cmd/
│   │   ├── main.go              # 入口：加载配置、创建 Consul 注册器、启动 Kratos App
│   │   ├── wire.go              # Wire 依赖注入声明
│   │   └── wire_gen.go          # Wire 自动生成的注入代码
│   ├── configs/
│   │   └── config.yaml          # 数据库等非敏感配置 (敏感值用 ${ENV_VAR} 占位)
│   └── internal/
│       ├── biz/
│       │   ├── biz.go           # Wire ProviderSet
│       │   ├── user.go          # UserUsecase + UserRepo 接口 + User 实体
│       │   └── user_test.go     # 14 个单元测试 (Mock UserRepo)
│       ├── data/
│       │   ├── data.go          # GORM 初始化 + Wire ProviderSet
│       │   ├── user.go          # UserRepo 实现 (GORM)
│       │   └── user_integration_test.go  # 5 个集成测试 (build tag: integration)
│       ├── service/
│       │   ├── service.go       # Wire ProviderSet
│       │   └── user.go          # gRPC UserService 实现 (调用 UserUsecase)
│       ├── server/
│       │   ├── server.go        # Wire ProviderSet
│       │   └── grpc.go          # gRPC Server 配置
│       └── conf/
│           ├── conf.proto       # 配置 protobuf 定义
│           └── conf.pb.go       # 生成的配置结构体
│
├── file/                        # 文件服务 (Kratos v2, gRPC :9002)
│   ├── cmd/
│   │   ├── main.go              # 入口：加载配置、Consul 注册、发现 user-service、启动
│   │   └── wire.go / wire_gen.go
│   ├── configs/
│   │   └── config.yaml
│   └── internal/
│       ├── biz/
│       │   ├── biz.go
│       │   ├── file.go          # FileUsecase + FileRepo/UserClient/MessageProducer/ErasureEncoder 接口
│       │   └── file_test.go     # 59 个单元测试
│       ├── data/
│       │   ├── data.go          # GORM (MySQL/SQLite) + Redis 初始化
│       │   ├── file.go          # FileRepo 实现 (GORM + Redis + 分布式锁)
│       │   ├── user_client.go   # UserClient 实现 (gRPC → user-service)
│       │   ├── mq_goroutine.go  # goroutineMQ (进程内 buffered channel)
│       │   ├── oss.go           # CloudStorage 实现 (alibabacloud-oss-go-sdk-v2)
│       │   ├── erasure.go       # ErasureEncoder 实现 (klauspost/reedsolomon)
│       │   └── file_integration_test.go  # 7 个集成测试
│       ├── service/
│       │   ├── service.go
│       │   └── file.go          # gRPC FileService 实现
│       ├── server/
│       │   ├── server.go
│       │   └── grpc.go
│       └── conf/
│           ├── conf.proto
│           └── conf.pb.go
│
└── gateway/                     # API 网关 (Gin, HTTP :8080)
    ├── cmd/
    │   └── main.go              # Gin 路由定义、中间件挂载、启动
    ├── configs/
    │   └── config.yaml          # 网关配置 (Consul 地址等)
    └── internal/
        ├── client/
        │   └── client.go        # Consul gRPC 客户端构造 (discovery:///user-service 等)
        ├── handler/
        │   ├── user.go          # 用户相关 HTTP → gRPC 处理器
        │   ├── file.go          # 文件相关 HTTP → gRPC 处理器
        │   └── handler_test.go  # 15 个单元测试
        └── middleware/
            ├── jwt.go           # JWT 认证中间件
            ├── cors.go          # CORS + Logger 中间件
            └── middleware_test.go # 9 个单元测试
```

### `frontend/` 目录

```
frontend/
├── Dockerfile               # 多阶段构建：Node 编译 → Nginx 运行
├── nginx.conf               # Nginx 配置 (SPA fallback + /api 反代)
├── package.json             # pnpm 依赖声明
├── pnpm-lock.yaml
├── vite.config.ts           # Vite 配置 (Tailwind 插件, 路径别名, API 代理 → :8080)
├── tsconfig.json / tsconfig.app.json / tsconfig.node.json
├── index.html               # SPA 入口
└── src/
    ├── main.ts              # Vue 应用入口
    ├── App.vue              # 根组件
    ├── style.css            # Tailwind v4 主题 (light/dark CSS 变量)
    ├── api/
    │   ├── client.ts        # Axios 实例 (baseURL, JWT interceptor, 401 重定向)
    │   ├── user.ts          # 用户 API (register, login, getUserInfo, updateUserInfo)
    │   └── file.ts          # 文件 API (24 个端点，含 4 个 presigned upload)
    ├── composables/
    │   ├── useTheme.ts      # 主题切换 (light/dark/system) + localStorage 持久化
    │   ├── useUpload.ts     # 上传 (direct / presigned) + MD5 秒传 + 断点续传 + 进度追踪
    │   └── __tests__/       # composable 单元测试
    ├── components/
    │   ├── layout/          # AppLayout / AppSidebar / AppHeader
    │   ├── ui/              # ThemeToggle / BaseModal
    │   └── file/            # FileBreadcrumb / FileToolbar / FileItem / UploadProgress / ShareDialog
    ├── stores/
    │   ├── auth.ts          # Pinia: 认证状态 (login/register/logout/fetchUserInfo)
    │   ├── file.ts          # Pinia: 文件列表/导航/排序/选择/CRUD
    │   └── __tests__/       # store 单元测试
    ├── views/               # LoginView / RegisterView / FileBrowserView / TrashView / ProfileView / SharePublicView
    ├── router/
    │   ├── index.ts         # Vue Router 配置 + auth guard
    │   └── __tests__/       # router 单元测试
    └── types/
        └── index.ts         # TypeScript 接口定义 (对齐 proto)
```

## 服务清单

### 用户服务 (app/user/internal/biz/user.go)
- [x] Register - 用户注册
- [x] Login - 用户登录
- [x] GetUserInfo - 获取用户信息
- [x] UpdateUserInfo - 更新用户信息
- [x] UpdateStorageUsed - 更新存储用量 (供 File Service 调用)

### 文件服务 (app/file/internal/biz/file.go)
- [x] CheckUpload - 秒传/断点续传检查 + 上传模式选择 (direct/presigned)
- [x] SaveChunk - 保存分块 (本地磁盘 + Redis 分布式锁 + 用量计数)
- [x] CompleteUpload - 完成上传 (验证分块完整性 + 创建文件记录 + 打散存储 + UserClient.UpdateStorageUsed + 异步 LRU 淘汰)
- [x] GetDownloadPlan - 获取下载计划 (返回各分块所在实例地址，支持并发下载)
- [x] InitPresignedUpload - 初始化预签名上传 (秒传检查 + 会话创建/续传 + OSS InitMultipartUpload + 签发 URL)
- [x] ReportUploadedPart - 上报已上传分块 (写 upload_parts 表)
- [x] CompletePresignedUpload - 完成预签名上传 (OSS CompleteMultipartUpload + 创建文件记录)
- [x] AbortPresignedUpload - 取消预签名上传 (OSS AbortMultipartUpload + 清理会话)
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
- [x] GetDownloadURL - 按 StorageType 签发预签名 URL 或 local:// URL
- [x] OpenLocalFile - 打开本地文件用于 streaming
- [x] GetDiskUsage - 查询主存/冷存磁盘用量
- [x] maybeEvictToCloud - LRU 淘汰 (主存 → OSS)

### API 网关 (app/gateway/)
- [x] HTTP→gRPC 代理 (所有用户/文件操作)
- [x] JWT 认证中间件
- [x] CORS 中间件
- [x] Consul 服务发现客户端
- [x] MD5 一致性哈希路由 (FNV32a, 150 虚拟节点, Consul 健康检查)
- [x] StreamFile - 本地模式流式文件下载 (gRPC server-streaming → HTTP 流)
- [x] 预签名上传代理 (InitPresignedUpload / ReportUploadedPart / CompletePresignedUpload / AbortPresignedUpload)

## 测试策略

| 模块 | 测试类型 | 测试数 | 说明 |
|------|----------|--------|------|
| app/user/internal/biz | 单元测试 | 14 | Mock UserRepo |
| app/file/internal/biz | 单元测试 | 59 | Mock FileRepo + UserClient + MessageProducer + CloudStorage + ErasureEncoder |
| app/gateway/internal/handler | 单元测试 | 15 | Mock gRPC 客户端 |
| app/gateway/internal/middleware | 单元测试 | 9 | JWT + CORS 中间件 |
| app/gateway/internal/client | 单元测试 | 7 | 一致性哈希 + 容错 |
| app/user/internal/data | 集成测试 | 5 | 需要 MySQL (build tag: integration) |
| app/file/internal/data | 集成测试 | 7 | 需要 MySQL + Redis (build tag: integration) |

运行测试:
```bash
go test ./...                              # 全部单元测试 (104 个)
go test -tags=integration ./...            # 包含集成测试 (需要基础设施)
```

## 服务间通信

- **Gateway → User Service**: 通过 Consul 发现 `user-service`，gRPC 调用
- **Gateway → File Service**: 通过 Consul 发现 `file-service`，gRPC 调用
- **File Service → User Service**: 通过 Consul 发现 `user-service`，调用 `UpdateStorageUsed` RPC
- **File Service → Kafka MQ**: CompleteUpload 完成后异步 `maybeEvictToCloud()` → Kafka（开发环境未配置时降级 goroutine channel）
- **Client → File Service HTTP**: 分块直传 File Service 实例 (HTTP :9003)，绕过网关
- **Gateway → File Service HTTP (recovery)**: 主分块实例失败时，网关把 recovery 请求代理到健康实例，由后者从 OSS 恢复分片重建 chunk

## MQ 集成 (Kafka / goroutine 降级)

**Kafka MQ**: segmentio/kafka-go，KRaft 模式。开发环境无 Kafka 时降级为进程内 goroutine channel。

| 文件 | 职责 |
|------|------|
| `app/file/internal/biz/file.go` | `MessageProducer` 接口 + `CloudMigrateMessage`/`ThumbnailMessage` 结构体 + `maybeEvictToCloud()` 淘汰逻辑 |
| `app/file/internal/data/mq_kafka.go` | kafkaProducer (Kafka 写入) + KafkaConsumer (Kafka 消费 + 云迁移处理) |
| `app/file/internal/data/mq_goroutine.go` | goroutineMQ 降级实现 (进程内 buffered channel) + NewMessageProducer 自动选择 |

### CloudMigrateMessage (cloud-migrate)
```json
{
  "file_store_id": 42,
  "file_md5": "abc123...",
  "cur_location": "abc123/file.zip",
  "dest_location": "oss://abc123/file.zip",
  "size": 10485760
}
```

### ThumbnailMessage (file-thumbnail)
```json
{
  "file_id": 123,
  "file_path": "abc123/image.jpg",
  "file_type": "image/jpeg"
}
```

详细说明见 [docs/kafka.md](docs/kafka.md) 和 [docs/message-queue.md](docs/message-queue.md)。

## 环境变量

| 变量 | 说明 | 服务 | 示例 |
|------|------|------|------|
| DB_DRIVER | 数据库驱动 (mysql/sqlite) | user, file | mysql |
| SQLITE_PATH | SQLite 文件路径 | user, file | /app/data/cloud_disk.db |
| DB_HOST | 数据库地址 | user, file | localhost |
| DB_PORT | 数据库端口 | user, file | 3306 |
| DB_USER | 数据库用户 | user, file | root |
| DB_PASSWORD | 数据库密码 | user, file | *** |
| DB_NAME | 数据库名 | user, file | cloud_disk |
| REDIS_ADDR | Redis 地址 | file | localhost:6379 |
| CONSUL_ADDR | Consul 地址 | user, file, gateway | localhost:8500 |
| JWT_SECRET | JWT 密钥 | gateway | *** |
| GATEWAY_ADDR | 网关监听地址 | gateway | :8080 |
| PRIMARY_MAX_BYTES | 本地磁盘主存上限 (字节) | file | 10737418240 (10GB) |
| OSS_ENDPOINT | 阿里云 OSS 端点 | file | oss-cn-hangzhou.aliyuncs.com |
| OSS_REGION | OSS Region | file | cn-hangzhou |
| OSS_BUCKET | OSS Bucket | file | light-cloud-disk |
| OSS_ACCESS_KEY_ID | OSS AK | file | *** |
| OSS_ACCESS_KEY_SECRET | OSS SK | file | *** |
| FILE_TMP_DIR | 分块临时目录 | file | /app/tmp |
| FILE_STORE_DIR | 分块存储目录 | file | /app/store |
| FILE_HTTP_ADDR | File Service HTTP 监听地址 | file | :9003 |
| KAFKA_BROKERS | Kafka broker 地址 (逗号分隔) | file | kafka:9092 |
| ERASURE_DATA_SHARDS | 纠删码数据分片数 | file | 4 |
| ERASURE_PARITY_SHARDS | 纠删码校验分片数 | file | 2 |
| ERASURE_MIN_FILE_SIZE | 纠删码最小文件大小 (字节) | file | 10485760 (10MB) |

## 关键设计决策

1. **双协议 File Service**: File Service 暴露 gRPC (:9002) + HTTP (:9003)。HTTP 用于客户端直传/下载分块，绕过网关。User Service 仅 gRPC。
2. **Gin 网关**: 选择 Gin 而非 Kratos HTTP，因为网关职责是路由/中间件，不需要 Kratos 的 Service/Biz/Data 分层。
3. **Consul 服务发现**: 使用 Kratos 的 `kratos.Registrar()` 注册，客户端使用 `discovery:///service-name` 端点。
4. **跨服务调用**: File Service 定义 `biz.UserClient` 接口，由 `data/user_client.go` 通过 gRPC 实现，解耦业务逻辑和远程调用。
5. **Wire 依赖注入**: 每个 Kratos 服务使用独立的 `wire.go`，Gateway 不使用 Wire (直接构造)。
6. **Monorepo 结构**: 后端留在根目录（避免破坏 Go module 路径），前端位于 `frontend/` 目录。
7. **vendor 离线构建**: `go mod vendor` 将全部依赖源码镜像到仓库，Dockerfile 使用 `-mod=vendor` 实现零网络构建，确保 CI 和本地构建行为一致。
8. **统一 Dockerfile**: 用单个 `Dockerfile` + `SERVICE` 构建参数替代多个 Dockerfile，编译路径为 `./app/${SERVICE}/cmd`。
9. **CloudStorage 接口**: biz 层定义接口，data 层用 alibabacloud-oss-go-sdk-v2 (阿里云 OSS) 实现，有 noop 降级。
10. **Redis 用量计数器**: `disk_usage:local` 用 INCRBY 原子操作追踪，避免每次查 DB 聚合。
11. **Redis 分布式锁**: 分块级 SetNX + Lua 原子释放，CompleteUpload 级 SetNX 防重复完成，保障并发上传安全。
12. **Reed-Solomon 纠删码**: 直传 chunk 落盘后立即生成恢复分片并写入 OSS；legacy local_ec 仍保留本地大文件重建能力。

## 容器化

- 统一 `Dockerfile`：多阶段构建 (golang:1.25 + bookworm-slim)，CGO_ENABLED=1 支持 SQLite，通过 `--build-arg SERVICE=user|file|gateway` 构建不同服务
- 前端独立 `frontend/Dockerfile`：多阶段 (node:24-alpine → nginx:1.27-alpine)
- `docker-compose.yml`：Consul + Redis + MySQL + Kafka + 3 服务 + 前端

## CI/CD

- `ci.yml`：Push/PR 到 `main`/`v2` 分支触发 → 后端测试 → 前端测试 → Compose 冒烟 → 构建二进制 → 推送 GHCR 镜像
- `release.yml`：Push `v*` tag 触发 → 测试 → 编译 amd64/arm64 多架构二进制 → 创建 GitHub Release

## 最近更新（2026-07）

- **打散存储 + 并发下载**: 文件分块打散到多个 File Service 实例，下载时客户端并发拉取各分块
- **Kafka 消息队列**: segmentio/kafka-go (KRaft 模式)，cloud-migrate / file-thumbnail 两个 topic，开发环境无 Kafka 时降级 goroutine channel
- **分块直传**: 客户端通过网关获取 UploadPlan 后直传 File Service HTTP 实例，绕过网关瓶颈
- **一致性哈希容错**: hashRouter 支持标记不健康实例 + 15s 冷却 + 自动恢复
- **分块恢复链路**: 直传 chunk 生成恢复分片写入 OSS，GetDownloadPlan 返回 recovery URL，主实例失败时由健康实例重建 chunk
- **Redis 分布式锁**: 分块级 SetNX + Lua 原子释放，CompleteUpload 级 SetNX 防重复完成
- **上传状态机**: FileStore 增加 `upload_status` (uploading/complete)，秒传仅匹配已完成文件
- MySQL 为默认数据库 (SQLite 可选)
- 统一 Dockerfile (SERVICE=user|file|gateway)，EXPOSE 9003
- 单 .env 配置文件，新增 KAFKA_BROKERS / FILE_HTTP_ADDR 环境变量

## 前端架构

### 技术栈
- **框架**: Vue 3.5 + TypeScript 5.9 (Composition API + `<script setup>`)
- **构建**: Vite 7 + vue-tsc
- **样式**: Tailwind CSS v4 (`@theme` CSS 变量，非 tailwind.config.js)
- **状态管理**: Pinia 3
- **路由**: Vue Router 5 (History mode, lazy-loaded routes)
- **HTTP**: Axios (JWT interceptor, 401 auto-redirect)
- **图标**: lucide-vue-next
- **工具**: @vueuse/core (theme persistence, system preference detection)
- **包管理**: pnpm

### 主题系统

双主题方案，使用 CSS 自定义属性 + Tailwind v4 `@theme`:

| 模式 | 背景色 | 强调色 | 文字色 |
|------|--------|--------|--------|
| Light | #f0f7ff (sky-blue) | #2563eb (blue-600) | #0f172a (slate-900) |
| Dark | #0b1120 (navy) | #60a5fa (blue-400) | #e2e8f0 (slate-200) |

### 前端功能清单

- [x] 用户认证 (登录/注册/登出/JWT 自动续期)
- [x] 文件浏览 (网格/列表视图切换, 排序, 面包屑导航)
- [x] 文件操作 (新建文件夹, 重命名, 删除, 移动, 下载)
- [x] 分块上传 (MD5 秒传, 断点续传, 直传/预签名, 进度显示)
- [x] 回收站管理 (列表, 恢复, 永久删除)
- [x] 文件分享 (创建分享链接, 密码保护, 有效期)
- [x] 公开分享页 (无需登录访问)
- [x] 主题切换 (浅色/深色/跟随系统)
- [x] 文件搜索
- [x] 用户资料编辑
- [x] 存储用量显示

### 前端测试策略

| 模块 | 测试类型 | 说明 |
|------|----------|------|
| stores/auth | 单元测试 | Mock API, 测试 login/register/logout 流程 |
| stores/file | 单元测试 | Mock API, 测试 CRUD/导航/排序/选择 |
| composables/useTheme | 单元测试 | Mock DOM classList, 测试主题切换逻辑 |
| composables/useUpload | 单元测试 | Mock API + crypto, 测试上传流程 |
| router | 单元测试 | 测试 auth guard 重定向逻辑 |

运行前端测试:
```bash
cd frontend && pnpm test            # 运行所有单元测试
cd frontend && pnpm test:coverage   # 运行测试 + 覆盖率报告
cd frontend && pnpm build           # TypeScript 类型检查 + 构建
```
