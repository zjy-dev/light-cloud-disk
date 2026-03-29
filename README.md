# 轻网盘 Light Cloud Disk

[![CI](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml)
[![Release](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/release.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/releases)

基于 Kratos v2 的微服务云存储系统，采用 gRPC 服务拆分 + Gin API 网关 + Consul 服务发现架构。支持 MySQL / SQLite 可切换数据库，本地磁盘作为主存，按 LRU 异步冷迁移到阿里云 OSS；文件分块打散上传到多个 file-service 实例（绕过网关），下载时优先直连分块实例，实例故障时通过 Reed-Solomon 恢复分片和 OSS 兜底恢复。Redis 分布式锁保障并发上传安全，Kafka 负责异步冷迁移。前端使用 Vue 3 + TypeScript + Tailwind CSS 构建。

**Monorepo 结构**: 后端 Go 代码在项目根目录，前端 Vue 3 SPA 在 `frontend/` 目录。

## 架构概览

```
                    ┌──────────────────────┐
                    │    Gin API Gateway   │
                    │     (HTTP :8080)     │
                    │  JWT / CORS / Hash   │
                    │  UploadPlan 分发      │
                    └────┬────────────┬────┘
                         │  gRPC      │  gRPC
               ┌─────────┘            └──────────┐
               ▼                                  ▼
    ┌─────────────────────┐           ┌─────────────────────┐
    │   User Service      │           │   File Service ×N   │
    │   (gRPC :9001)      │◄──gRPC────│   (gRPC :9002)      │
    │   Kratos v2         │           │   (HTTP :9003)      │
    └────────┬────────────┘           └──┬─────┬─────┬──────┘
             │                           │     │     │
             ▼                           ▼     ▼     ▼
    ┌─────────────────┐           ┌──────┐ ┌─────┐ ┌───────────┐
    │  MySQL / SQLite │           │  DB  │ │Redis│ │ 本地磁盘   │
    └─────────────────┘           └──────┘ └─────┘ └───────────┘
                                                        │
     ← ─ ─ ─  Consul 服务发现  ─ ─ ─ →             Kafka MQ
                                                        │
                                                 ┌──────┴──────┐
  Client ──── PUT chunks ────────────────────►   │ 阿里云 OSS   │
  (直传 file-service HTTP, 绕过网关)              └─────────────┘
```

- **User Service**: 用户注册/登录、信息管理、存储配额 (gRPC-only, Kratos v2)
- **File Service**: 分块打散上传/下载、秒传、文件管理、回收站、分享、冷热分层存储、每块恢复分片生成与重建 (gRPC + HTTP, Kratos v2)
- **API Gateway**: HTTP 路由、JWT 认证、CORS、MD5 一致性哈希路由、UploadPlan 分发、gRPC 代理 (Gin)
- **Kafka**: 异步消息队列 (cloud-migrate / file-thumbnail)，本地开发未配置时可降级到 goroutine channel
- **阿里云 OSS**: 冷数据归档，由 Kafka 消费者异步迁移
- **Consul**: 服务注册与发现，HTTP 端口元数据

## 技术栈

| 组件 | 技术选型 | 版本 |
|------|----------|------|
| **后端** | | |
| 微服务框架 | Kratos | v2.9.2 |
| API 网关 | Gin | v1.12 |
| 服务发现 | Consul | 1.19 |
| 通信协议 | gRPC (服务间) + HTTP (客户端) | - |
| ORM | GORM | v1.25.12 |
| 数据库 | MySQL 8.0 / SQLite (DB_DRIVER 切换) | - |
| 缓存 / 分布式锁 | Redis | v8.11.5 |
| 消息队列 | Apache Kafka (KRaft，开发环境可降级 goroutine channel) | 3.9 |
| 对象存储 (冷) | 阿里云 OSS | SDK v1.4 |
| 纠删码 | klauspost/reedsolomon (Reed-Solomon 4+2) | v1.13.3 |
| 依赖注入 | Wire | v0.6.0 |
| 认证 | JWT (golang-jwt/jwt v5) | v5 |
| 容器编排 | Podman / Docker Compose | - |
| **前端** | | |
| 框架 | Vue 3 (Composition API) | v3.5 |
| 构建工具 | Vite | v7.3 |
| 语言 | TypeScript | v5.9 |
| 样式 | Tailwind CSS | v4.2 |
| 状态管理 | Pinia | v3.0 |
| 路由 | Vue Router | v5.0 |
| HTTP 客户端 | Axios | v1.13 |
| 图标 | Lucide Vue Next | latest |
| 包管理器 | pnpm | v10+ |

## 功能特性

### 用户服务
- [x] 用户注册/登录 (JWT 认证)
- [x] 用户信息管理
- [x] 存储配额管理 (默认 10GB)
- [x] 跨服务存储用量更新 (UpdateStorageUsed RPC)

### 文件服务
- [x] 分块打散上传 (大文件分块直传到多个 file-service 实例，绕过网关)
- [x] 秒传 (基于 MD5 去重)
- [x] 断点续传 (Redis SET 记录上传状态)
- [x] 预签名分块上传 (客户端直传 OSS，支持跨设备断点续传)
- [x] 上传模式自动切换 (direct: 经后端打散 / presigned: 客户端直传 OSS)
- [x] UploadPlan 分发 (网关生成分块→实例分配方案, 客户端并行上传)
- [x] LRU 自动冷迁移 (本地磁盘超阈值 → Kafka → 迁入 OSS)
- [x] Kafka 消息队列 (cloud-migrate / file-thumbnail, 开发环境未配置时降级 goroutine channel)
- [x] 分块恢复分片 (每个直传 chunk 上传后立即生成 Reed-Solomon 恢复分片并写入 OSS)
- [x] Redis 分布式锁 (分块级 SetNX + Lua 释放，合并级 SetNX 防重复)
- [x] 可切换数据库 (MySQL / SQLite via DB_DRIVER)
- [x] 并行下载 (GetDownloadPlan → 多实例并发获取分块，浏览器端重组)
- [x] 磁盘满保护 (CheckUpload 检测 → 自动切换预签名上传)
- [x] 磁盘用量查询 (GetDiskUsage API)
- [x] 文件夹管理 (树形结构)
- [x] 文件搜索 (模糊匹配)
- [x] 回收站 (软删除 + 恢复)
- [x] 文件分享 (链接 + 提取码 + 过期时间)
- [x] 文件移动/重命名
- [x] 下载链接获取 (按 StorageType 签发预签名 URL 或本地流式代理)

### 网关
- [x] JWT 认证中间件 (保护路由)
- [x] CORS 跨域中间件
- [x] 请求日志中间件
- [x] gRPC 代理 (HTTP → gRPC 协议转换)
- [x] Consul 服务发现 (自动发现后端服务)
- [x] MD5 一致性哈希路由 (上传请求按文件 MD5 路由到固定 File Service 实例)
- [x] UploadPlan 分发 (pickN 选择多实例, 生成分块→实例分配方案)
- [x] 一致性哈希容错 (不健康实例 15s 冷却剔除, 过期自动重试)
- [x] 下载恢复代理 (主分块实例失败时回退到网关 recovery 端点重建分块)
- [x] 预签名上传代理 (4 个端点: init / report-part / complete / abort)

## 项目结构

```
.
├── frontend/                     # 前端 (Vue 3 SPA)
│   ├── src/
│   │   ├── api/                 # API 客户端 (Axios)
│   │   ├── components/          # 组件
│   │   │   ├── layout/         # 布局 (Sidebar, Header)
│   │   │   ├── file/           # 文件相关组件
│   │   │   └── ui/             # 通用 UI 组件
│   │   ├── composables/         # Vue Composables
│   │   ├── router/              # Vue Router
│   │   ├── stores/              # Pinia 状态管理
│   │   ├── types/               # TypeScript 类型
│   │   └── views/               # 页面视图
│   ├── vite.config.ts
│   └── package.json
├── api/                          # Protobuf API 定义
│   ├── user/v1/                 # 用户服务 proto + 生成代码
│   └── file/v1/                 # 文件服务 proto + 生成代码
├── app/                          # 微服务目录
│   ├── user/                    # 用户服务
│   │   ├── cmd/                 # 入口 (main.go, wire.go)
│   │   ├── internal/            # 内部实现
│   │   │   ├── biz/            # 业务逻辑 (UserUsecase)
│   │   │   ├── data/           # 数据访问 (MySQL)
│   │   │   ├── service/        # gRPC 服务实现
│   │   │   ├── server/         # gRPC 服务器配置
│   │   │   └── conf/           # 配置定义 (proto)
│   │   └── configs/            # 配置文件 (YAML)
│   ├── file/                    # 文件服务
│   │   ├── cmd/                 # 入口
│   │   ├── internal/            # 内部实现
│   │   │   ├── biz/            # 业务逻辑 (FileUsecase)
│   │   │   ├── data/           # 数据访问 (DB + Redis + OSS + 纠删码)
│   │   │   ├── service/        # gRPC 服务实现
│   │   │   ├── server/         # gRPC 服务器配置
│   │   │   └── conf/           # 配置定义
│   │   └── configs/            # 配置文件
│   └── gateway/                 # API 网关
│       ├── cmd/                 # 入口 (Gin 路由)
│       ├── internal/
│       │   ├── client/         # gRPC 客户端 (Consul 发现)
│       │   ├── handler/        # HTTP 处理器
│       │   └── middleware/     # JWT、CORS、Logger
│       └── configs/            # 配置文件
├── third_party/                 # 第三方 proto
├── docs/                        # 功能文档
├── vendor/                      # go mod vendor 依赖副本 (离线构建)
├── Dockerfile                   # 统一多阶段构建 (SERVICE=user|file|gateway)
├── docker-compose.yml           # 容器编排 (Consul + Redis + MySQL + 3 服务 + 前端)
├── Makefile                     # 构建/运行/生成命令
└── go.mod
```

## 快速开始

### 环境准备
- Go 1.25+
- Node.js 24+ (推荐用 fnm 管理)
- pnpm 10+
- MySQL 8.0+
- Redis 7+
- Consul 1.19+
- protoc + protoc-gen-go + protoc-gen-go-grpc

> CI 中的 Go 版本通过 `actions/setup-go` 的 `go-version-file: go.mod` 自动同步，并设置 `GOTOOLCHAIN=local` 禁止自动切换工具链，避免覆盖率阶段出现 `covdata` 缺失。

### 安装工具
```bash
make init
```

### 生成代码
```bash
make api          # 生成 proto 代码
make conf-user    # 生成用户服务配置代码
make conf-file    # 生成文件服务配置代码
make wire-user    # 生成用户服务依赖注入
make wire-file    # 生成文件服务依赖注入
```

### 配置环境变量
```bash
cp .env.example .env
# Edit .env and fill in actual values
```

### 前端安装与运行
```bash
cd frontend
pnpm install        # 安装依赖
pnpm dev            # 启动开发服务器 (http://localhost:3000)
pnpm build          # 构建生产版本
```

### 本地运行后端 (需先启动基础设施)
```bash
make infra-up       # 启动 MySQL + Redis + Consul
make run-user       # 启动用户服务 (gRPC :9001)
make run-file       # 启动文件服务 (gRPC :9002)
make run-gateway    # 启动 API 网关 (HTTP :8080)
```

> 前端开发服务器会自动代理 `/api` 请求到 `localhost:8080` (Gateway)。

### 容器化运行

本项目以 **Podman + podman-compose** 为一等公民，同时兼容 Docker / Docker Compose。
Makefile 会自动检测 `podman` / `docker` 命令并使用。

> **为什么分两步（先 build 再 up）？**
> podman-compose 1.x 不支持 Compose spec 的 `build.network` 字段，
> 而 podman 默认 bridge 网络在某些环境无法访问互联网。
> `make images` 统一使用 `--network host` 构建，确保任何环境都能成功。

```bash
# 1) 构建所有镜像 (backend + frontend)
make images
# 2) 启动服务
make up

# ========== 常用命令 ==========
make ps                  # 查看容器状态
make logs                # 查看日志
make down                # 停止所有容器
make clean-containers    # 停止并清除数据卷
```

如果你使用 Docker Compose，也可以直接运行：
```bash
docker compose --env-file .env up -d --build
```

说明：
- 默认启动 Consul + Redis + MySQL + user-service + file-service + gateway + frontend
- 前端镜像采用多阶段构建（容器内执行 `pnpm install && pnpm build`），不再依赖宿主机预先生成 `frontend/dist/`

### 运行测试
```bash
go test ./...                                  # 全部单元测试
go test -v ./app/user/internal/biz/            # 用户 biz 测试
go test -v ./app/file/internal/biz/            # 文件 biz 测试
go test -v ./app/gateway/internal/handler/     # 网关 handler 测试
go test -v ./app/gateway/internal/middleware/   # 网关中间件测试
```

## API 接口

所有 HTTP 请求通过 Gateway (:8080) 访问。

### 公开路由 (无需认证)

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/user/register | 用户注册 |
| POST | /api/v1/user/login | 用户登录 |
| GET | /api/v1/share/:share_id | 获取分享内容 |
| GET | /health | 健康检查 |

### 保护路由 (需 JWT Token)

请求头: `Authorization: Bearer <token>`

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/user/info | 获取用户信息 |
| PUT | /api/v1/user/info | 更新用户信息 |
| POST | /api/v1/file/check-upload | 秒传/断点续传检查 (返回 UploadPlan) |
| POST | /api/v1/file/complete-upload | 完成上传 (所有分块上传后调用) |
| GET | /api/v1/file/download-plan/:file_id | 获取下载计划 (分块位置列表) |
| GET | /api/v1/files | 文件列表 |
| POST | /api/v1/file/folder | 创建文件夹 |
| PUT | /api/v1/file/rename | 重命名文件 |
| DELETE | /api/v1/files | 删除文件 (移入回收站) |
| PUT | /api/v1/file/move | 移动文件 |
| GET | /api/v1/file/download/:file_id | 获取下载链接 (预签名 URL) |
| GET | /api/v1/files/search | 搜索文件 |
| GET | /api/v1/disk-usage | 磁盘用量 |
| GET | /api/v1/trash | 回收站列表 |
| POST | /api/v1/trash/restore | 恢复文件 |
| DELETE | /api/v1/trash | 彻底删除 |
| POST | /api/v1/share | 创建分享 |

说明：
- 分块直传 file-service HTTP 端口 (9003): `PUT /api/v1/chunks/:md5/:index`, `GET /api/v1/chunks/:md5/:index`
- 分块恢复端点: `GET /api/v1/file/chunks/:md5/:index/recovery?file_size=...`（由网关代理到健康 file-service 实例执行重建）
- 客户端根据 CheckUpload 返回的 UploadPlan，直接上传分块到对应的 file-service 实例

## 服务通信

```
Client ──HTTP──▶ Gateway ──gRPC──▶ User Service
   │                 │                    ▲
   │                 │──gRPC──▶ File Service ×N ──gRPC──┘
   │                               (UpdateStorageUsed)
   │                                    │
   │── PUT /api/v1/chunks ─────▶  HTTP :9003  (直传分块)
   │── GET /api/v1/chunks ◀────   HTTP :9003  (直下分块)
                                        │
                                    Kafka MQ
                                        │
                                        ▼
                                  阿里云 OSS
```

- Gateway 通过 Consul 发现 `user-service` 和 `file-service`
- CheckUpload 返回 UploadPlan，客户端直传分块到 file-service HTTP 端口
- File Service 通过 Consul 发现 `user-service`，调用 `UpdateStorageUsed` 更新存储用量
- 直传分块写入本地后立即生成恢复分片并上传 OSS，供故障恢复使用
- 超阈值时通过 Kafka 异步迁移到 OSS (开发环境无 Kafka 时降级为 goroutine channel)
- 下载时 GetDownloadPlan 返回主下载地址和备份恢复地址，客户端并行下载并在浏览器重组

## 消息队列

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| cloud-migrate | FileService (LRU 淘汰) | Kafka Consumer | 本地磁盘 → 阿里云 OSS 冷迁移 |
| file-thumbnail | FileService | Kafka Consumer | 生成文件缩略图 |

- 默认使用 Apache Kafka 3.9 (KRaft 模式，单节点)
- `KAFKA_BROKERS` 环境变量为空时，开发环境自动降级为进程内 goroutine channel (256 缓冲)
- 生产者使用 `segmentio/kafka-go` 的 `kafka.Writer`，key 为文件 MD5
- 消费者实现 Kratos `transport.Server` 接口，随服务生命周期启停

## 环境变量

| 变量 | 说明 | 服务 | 示例 |
|------|------|------|------|
| DB_DRIVER | 数据库驱动 (mysql/sqlite) | user, file | mysql |
| SQLITE_PATH | SQLite 文件路径 | user, file | /app/data/cloud_disk.db |
| DB_HOST | MySQL 地址 | user, file | localhost |
| DB_PORT | MySQL 端口 | user, file | 3306 |
| DB_USER | MySQL 用户 | user, file | root |
| DB_PASSWORD | MySQL 密码 | user, file | *** |
| DB_NAME | 数据库名 | user, file | cloud_disk |
| REDIS_ADDR | Redis 地址 | file | localhost:6379 |
| CONSUL_ADDR | Consul 地址 | user, file, gateway | localhost:8500 |
| JWT_SECRET | JWT 密钥 | gateway | *** |
| GATEWAY_ADDR | 网关监听地址 | gateway | :8080 |
| PRIMARY_MAX_BYTES | 本地磁盘主存上限 (字节) | file | 10737418240 (10GB) |
| FILE_TMP_DIR | 分块临时目录 | file | /app/tmp |
| FILE_STORE_DIR | 本地文件存储目录 | file | /app/store |
| OSS_ENDPOINT | 阿里云 OSS 端点 | file | oss-cn-hangzhou.aliyuncs.com |
| OSS_ACCESS_KEY_ID | OSS AK | file | *** |
| OSS_ACCESS_KEY_SECRET | OSS SK | file | *** |
| ERASURE_DATA_SHARDS | 纠删码数据分片数 | file | 4 |
| ERASURE_PARITY_SHARDS | 纠删码校验分片数 | file | 2 |
| ERASURE_MIN_FILE_SIZE | 纠删码最小文件大小 (字节) | file | 10485760 (10MB) |
| KAFKA_BROKERS | Kafka broker 地址 (逗号分隔, 空则降级为 goroutine) | file | localhost:9092 |
| FILE_HTTP_ADDR | file-service 分块直传 HTTP 监听地址 | file | :9003 |

## 测试覆盖

| 模块 | 测试类型 | 测试数 | 说明 |
|------|----------|--------|------|
| app/user/internal/biz | 单元测试 | 14 | Mock UserRepo |
| app/file/internal/biz | 单元测试 | 59 | Mock FileRepo + UserClient + MessageProducer + CloudStorage + ErasureEncoder |
| app/gateway/internal/handler | 单元测试 | 15 | Mock gRPC 客户端 |
| app/gateway/internal/client | 单元测试 | 7 | 一致性哈希 + 容错 |
| app/gateway/internal/middleware | 单元测试 | 9 | JWT + CORS |
| app/user/internal/data | 集成测试 | 5 | 需要 MySQL (build tag) |
| app/file/internal/data | 集成测试 | 7 | 需要 MySQL + Redis (build tag) |

## 前端功能

- 登录 / 注册页面
- 文件浏览器 (网格/列表双视图，排序，面包屑导航)
- 文件上传 (分块打散上传到多实例，进度面板，秒传/断点续传)
- 文件管理 (新建文件夹、重命名、删除、移动、并行下载)
- 回收站 (恢复、彻底删除、清空)
- 文件分享 (创建分享链接、设密码、过期时间)
- 公开分享页面 (密码验证、文件下载)
- 用户资料设置 (昵称、邮箱、存储用量可视化)
- 深色 / 浅色主题切换 (浅色为天蓝风格，深色为深海军蓝)
- 响应式布局，可收起侧边栏

## 文档

- [微服务架构设计](docs/architecture.md)
- [存储架构](docs/storage.md)
- [冷热分层存储](docs/cold-hot-storage.md)
- [API 网关实现](docs/gateway.md)
- [分块上传实现](docs/chunk-upload.md)
- [预签名分块上传](docs/presigned-upload.md)
- [服务发现与通信](docs/service-discovery.md)
- [消息队列集成](docs/message-queue.md)
- [Kafka 消息队列](docs/kafka.md)
- [分块打散存储](docs/scattered-storage.md)
- [分布式锁](docs/distributed-locking.md)
- [纠删码](docs/erasure-coding.md)
- [CI/CD 配置](docs/ci-cd.md)
- [容器化部署](docs/containerization.md)
- [前端架构与设计](docs/frontend.md)

## 更新日志

### v8.0.0 (2026)
- **分块打散存储**: 文件分块直传到多个 file-service 实例 (HTTP :9003)，绕过网关瓶颈
- **恢复分片链路**: 直传 chunk 后立即生成 Reed-Solomon 恢复分片并写入 OSS，下载失败时通过 recovery URL 重建分块
- **Kafka 消息队列**: Apache Kafka 3.9 (KRaft) 负责异步冷迁移，开发环境无 Kafka 时自动降级
- **并行下载**: GetDownloadPlan API 返回分块位置，浏览器端并行下载并重组
- **一致性哈希容错**: 不健康实例临时剔除 15s (cooldown)，过期后自动重试
- **UploadPlan 分发**: Gateway 使用 pickN 选择多实例，生成分块→实例分配方案
- CompleteUpload / GetDownloadPlan 替代 MergeChunks / GetDownloadURL
- chunk_records 表记录分块在各实例的分布
- File Service 新增 HTTP 端口 (Gin, JWT, Kratos transport.Server)
- 前端 useUpload 重写为 fetch PUT 直传，useDownload 新增并行下载

### v7.0.0 (2026)
- **分布式架构重构**: 统一为本地磁盘主存 + 阿里云 OSS 冷存，移除 SeaweedFS / file-worker 双路径
- **Reed-Solomon 纠删码**: 保留 legacy local_ec 能力，并为散列分块引入恢复分片元数据
- **Redis 分布式锁**: 分块级 SetNX + Lua 释放，CompleteUpload 级 SetNX 防重复完成，支持并发安全上传
- **上传状态机**: FileStore 增加 `upload_status` (uploading/complete)，秒传仅匹配已完成文件
- **数据模型优化**: `erasure_shards` 表存储分片元数据，MD5 改为 (file_md5, size) 联合唯一索引
- 移除双模式部署，MySQL 为默认数据库 (SQLite 可选)
- Kafka 为默认消息队列实现，开发环境保留 goroutine 降级路径
- 统一 Dockerfile (SERVICE=user|file|gateway)
- 单 .env 配置文件，新增 ERASURE_* 环境变量

### v6.0.0 (2026)
- **预签名分块上传**: 客户端直传 OSS，支持跨设备断点续传 (4 个新 RPC)
- **MD5 一致性哈希路由**: FNV32a + 150 虚拟节点，上传请求按文件 MD5 路由到固定 File Service 实例
- 本地文件流式下载: StreamFileContent server-streaming RPC + Gateway 流式代理
- Dockerfile 基于 debian (golang:1.25 + bookworm-slim)，内置 gcc 无需网络下载
- 98 个后端单元测试 + 50 个前端测试
- **安全加固**: presigned 操作 user_id 鉴权、服务端独立计算 totalParts、hashRouter 连接泄漏修复、事务原子删除、前端失败自动 abort

### v5.0.0 (2026)
- **冷热分层存储**: 本地磁盘 → LRU 淘汰 → 阿里云 OSS 冷归档
- 磁盘满保护: CheckUpload 检测 → 自动切换预签名上传
- 磁盘用量查询 API (GetDiskUsage)
- 阿里云 OSS (alibabacloud-oss-go-sdk-v2) 集成
- Redis 原子计数器追踪磁盘用量

### v4.0.0 (2026)
- 新增 Vue 3 前端 (TypeScript + Vite + Tailwind CSS v4)

### v4.0.1 (2026)
- CI 新增 Compose 冒烟验证（全栈容器启动 + Gateway/Frontend/后端端口连通性检查）
- 修复前端镜像依赖宿主机 `dist` 的构建问题，改为 Docker 多阶段自构建
- Monorepo 结构 (前后端同仓)
- 深色/浅色双主题 (浅色天蓝风格)
- 文件浏览器 (网格/列表视图、搜索、排序)
- 分块上传进度面板 (支持秒传/断点续传)
- 文件分享 (创建链接 + 公开访问页)
- 回收站管理
- 用户资料设置

### v3.0.0 (2026)
- 从伪微服务重构为真正的微服务架构
- 拆分独立 User Service (gRPC-only, Kratos v2)
- 拆分独立 File Service (gRPC-only, Kratos v2)
- 新增 Gin API Gateway (HTTP→gRPC 代理)
- 新增 Consul 服务注册与发现
- File Service 通过 gRPC 调用 User Service (替代共享 DB 访问)
- 60 个单元测试 + 12 个集成测试

### v2.0.0 (2025)
- 使用 Kratos v2 重构
- 新增文件夹管理
- 新增回收站功能
- 新增文件分享
- 新增文件搜索
- 添加存储配额管理

### v1.0.0 (2020)
- 基于 go-micro 的初始版本
- 基础文件上传/下载功能


本项目以服务器本地磁盘为主存，通过 UploadPlan + 一致性哈希把分块分散到多个 file-service 实例，避免网关成为上传和下载瓶颈。每个直传 chunk 落盘后都会立即生成恢复分片并写入 OSS；客户端下载时优先直连主分块地址，失败后自动回退到 recovery URL，由健康实例从恢复分片重建该 chunk。

Redis 维护本地磁盘用量和上传中的分块状态，超过阈值时通过 Kafka 异步触发 LRU 冷迁移。磁盘不足时，后端直接切换到预签名 OSS multipart 上传模式，不阻塞主流程。

秒传以 `(file_md5, size)` 的已完成 FileStore 为准，命中后只创建新的逻辑文件记录，不复制物理数据。断点续传则通过 chunk_records 和 Redis 分块状态共同确认缺失块，只重传未完成部分。
