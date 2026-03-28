# 轻网盘 Light Cloud Disk

[![CI](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml)
[![Release](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/release.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/releases)

基于 Kratos v2 的微服务云存储系统，采用 gRPC 服务拆分 + Gin API 网关 + Consul 服务发现架构。支持**双模式部署**：轻量模式（SQLite + 本地磁盘 + 进程内 MQ）适合个人使用，完整模式（MySQL + SeaweedFS + Kafka）适合企业级扩展。两种模式均支持 LRU 冷迁移到阿里云 OSS。前端使用 Vue 3 + TypeScript + Tailwind CSS 构建。

**Monorepo 结构**: 后端 Go 代码在项目根目录，前端 Vue 3 SPA 在 `frontend/` 目录。

## 架构概览

### 双模式部署

| | Mode A: local (轻量) | Mode B: s3 (完整) |
|--|---|---|
| **数据库** | SQLite | MySQL 8.0 |
| **消息队列** | 进程内 goroutine channel | Kafka |
| **主存储** | 本地磁盘 | SeaweedFS |
| **文件下载** | Gateway 流式代理 | 预签名 URL 直连 |
| **冷存储** | 阿里云 OSS (可选) | 阿里云 OSS |
| **启动命令** | `make images && make up` | `make images-all && make up ENV_FILE=.env.s3 PROFILE="--profile s3"` |

### 完整模式架构图 (Mode B)

```
                    ┌──────────────────────┐
                    │    Gin API Gateway   │
                    │     (HTTP :8080)     │
                    └────┬────────────┬────┘
                         │  gRPC      │  gRPC
               ┌─────────┘            └──────────┐
               ▼                                  ▼
    ┌─────────────────────┐           ┌─────────────────────┐
    │   User Service      │           │   File Service      │
    │   (gRPC :9001)      │◄──gRPC────│   (gRPC :9002)      │
    │   Kratos v2         │           │   Kratos v2         │
    └────────┬────────────┘           └──┬─────┬─────┬──────┘
             │                           │     │     │
             ▼                           ▼     ▼     ▼
    ┌─────────────────┐           ┌──────┐ ┌─────┐ ┌───────────┐
    │     MySQL       │           │MySQL │ │Redis│ │ SeaweedFS │
    └─────────────────┘           └──────┘ └─────┘ └───────────┘
                                                        │
     ← ─ ─ ─  Consul 服务发现  ─ ─ ─ →                   │
                                                        ▼
    File Service ──→ Kafka ──→ file-worker ──→ ┌──────────┐
    (cloud-migrate)                            │阿里云 OSS │
                                               └──────────┘
```

### 轻量模式架构图 (Mode A)

```
                    ┌──────────────────────┐
                    │    Gin API Gateway   │
                    │     (HTTP :8080)     │
                    └────┬────────────┬────┘
                         │  gRPC      │  gRPC
               ┌─────────┘            └──────────┐
               ▼                                  ▼
    ┌─────────────────────┐           ┌─────────────────────┐
    │   User Service      │           │   File Service      │
    │   (gRPC :9001)      │◄──gRPC────│   (gRPC :9002)      │
    │   SQLite            │           │   SQLite + 本地磁盘  │
    └─────────────────────┘           └──────────┬──────────┘
                                                 │
    ← ─ ─ ─ Consul 服务发现 ─ ─ ─ →            Redis
                                                 │
                                      goroutine MQ (进程内)
                                                 │
                                          ┌──────┴──────┐
                                          │ 阿里云 OSS   │ (可选冷迁移)
                                          └─────────────┘
```

- **User Service**: 用户注册/登录、信息管理、存储配额 (gRPC-only, Kratos v2)
- **File Service**: 分块上传、秒传、文件管理、回收站、分享、冷热分层存储 (gRPC-only, Kratos v2)
- **API Gateway**: HTTP 路由、JWT 认证、CORS、gRPC 代理 (Gin)
- **SeaweedFS**: S3 兼容对象存储，合并后文件主存 (温数据)
- **阿里云 OSS**: 冷数据归档，由 file-worker 异步迁移
- **Consul**: 服务注册与发现

## 技术栈

| 组件 | 技术选型 | 版本 |
|------|----------|------|
| **后端** | | |
| 微服务框架 | Kratos | v2.9.2 |
| API 网关 | Gin | v1.12 |
| 服务发现 | Consul | 1.19 |
| 通信协议 | gRPC (服务间) + HTTP (客户端) | - |
| ORM | GORM | v1.25.12 |
| 数据库 | MySQL 8.0 / SQLite (双模式) | - |
| 缓存 | Redis | v8.11.5 |
| 消息队列 | Kafka / goroutine channel (双模式) | v3.7 |
| 对象存储 (温) | SeaweedFS (S3 API) | latest |
| 对象存储 (冷) | 阿里云 OSS | SDK v1.4 |
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
- [x] 分块上传 (大文件支持，5MB/块)
- [x] 秒传 (基于 MD5 去重)
- [x] 断点续传 (Redis 记录上传状态)
- [x] 预签名分块上传 (客户端直传 SeaweedFS/OSS，支持跨设备断点续传)
- [x] 双模式上传 (direct: 经后端 / presigned: 客户端直传，自动切换)
- [x] 双模式存储 (Mode A: 本地磁盘主存 / Mode B: SeaweedFS 主存)
- [x] LRU 自动冷迁移 (主存超阈值 → MQ → 迁入 OSS)
- [x] 可插拔数据库 (MySQL / SQLite via DB_DRIVER)
- [x] 可插拔消息队列 (Kafka / 进程内 goroutine channel)
- [x] 本地文件流式下载 (StreamFileContent server-streaming RPC)
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
│   │   │   └── worker/          # file-worker (Kafka 消费进程)
│   │   ├── internal/            # 内部实现
│   │   │   ├── biz/            # 业务逻辑 (FileUsecase)
│   │   │   ├── data/           # 数据访问 (MySQL + Redis + Kafka)
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
├── Dockerfile                   # 统一多阶段构建 (SERVICE=user|file|gateway|worker)
├── docker-compose.yml           # 完整编排 (9 服务)
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
# Mode A: 轻量模式 (SQLite + 本地磁盘)
cp .env.local .env

# Mode B: 完整模式 (MySQL + SeaweedFS + Kafka)
cp .env.s3 .env
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
make infra-up       # 启动 MySQL + Redis + Consul + Kafka
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
# ========== Mode A: 轻量模式 (SQLite + 本地磁盘) ==========
# 1) 构建所有镜像 (backend + frontend)
make images
# 2) 启动服务
make up                  # 等价于 podman-compose --env-file .env.local up -d

# ========== Mode B: 完整模式 (MySQL + SeaweedFS + Kafka) ==========
make images-all          # 额外构建 file-worker 镜像
make up ENV_FILE=.env.s3 PROFILE="--profile s3"

# ========== 常用命令 ==========
make ps                  # 查看容器状态
make logs                # 查看日志
make down                # 停止所有容器
make clean-containers    # 停止并清除数据卷
```

如果你使用 Docker Compose，也可以直接运行：
```bash
docker compose --env-file .env.local up -d --build       # 轻量模式
docker compose --env-file .env.s3 --profile s3 up -d --build  # 完整模式
```

说明：
- 使用 Compose profiles，`--profile s3` 会额外启动 MySQL, Kafka, SeaweedFS, file-worker
- 轻量模式只启动 Consul + Redis + user-service + file-service + gateway + frontend
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
| POST | /api/v1/file/check-upload | 秒传/断点续传检查 |
| POST | /api/v1/file/upload-chunk | 上传文件分块 |
| POST | /api/v1/file/merge-chunks | 合并分块 |
| GET | /api/v1/files | 文件列表 |
| POST | /api/v1/file/folder | 创建文件夹 |
| PUT | /api/v1/file/rename | 重命名文件 |
| DELETE | /api/v1/files | 删除文件 (移入回收站) |
| PUT | /api/v1/file/move | 移动文件 |
| GET | /api/v1/file/download/:file_id | 获取下载链接 (预签名 URL) |
| GET | /api/v1/file/stream/:file_id | 流式下载 (本地模式) |
| GET | /api/v1/files/search | 搜索文件 |
| GET | /api/v1/disk-usage | 磁盘/SeaweedFS 用量 |
| GET | /api/v1/trash | 回收站列表 |
| POST | /api/v1/trash/restore | 恢复文件 |
| DELETE | /api/v1/trash | 彻底删除 |
| POST | /api/v1/share | 创建分享 |

说明：
- `POST /api/v1/file/upload-chunk` 使用 `multipart/form-data` 上传分块二进制（字段：`file_md5`、`chunk_index`、`chunk_size`、`chunk_file`）。

## 服务通信

```
Client ──HTTP──▶ Gateway ──gRPC──▶ User Service
                    │                    ▲
                    │──gRPC──▶ File Service ──gRPC──┘
                                (UpdateStorageUsed)
                                    │
                         ┌──────────┴──────────┐
                         ▼                      ▼
                    SeaweedFS (S3)         阿里云 OSS
                         │    Kafka              ▲
                         └──→ cloud-migrate ──→ file-worker
```

- Gateway 通过 Consul 发现 `user-service` 和 `file-service`
- File Service 通过 Consul 发现 `user-service`，调用 `UpdateStorageUsed` 更新存储用量
- File Service 合并后上传 SeaweedFS，超阈值时通过 Kafka 驱动 worker 迁移到 OSS
- 下载时按 `storage_type` 签发 SeaweedFS 或 OSS 预签名 URL

## 消息队列

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| cloud-migrate | FileService (LRU 淘汰) | file-worker / goroutine MQ | 主存 → 阿里云 OSS 冷迁移 |
| file-thumbnail | FileService | file-worker / goroutine MQ | 生成文件缩略图 |

- **Mode B**: `segmentio/kafka-go` 生产端 + 独立 `file-worker` 消费进程
- **Mode A**: 进程内 goroutine channel (256 缓冲)，替代 Kafka + file-worker
- 未配置 Kafka 且无 SeaweedFS 时，自动使用 goroutine MQ

## 环境变量

| 变量 | 说明 | 服务 | 示例 |
|------|------|------|------|
| DB_DRIVER | 数据库驱动 (mysql/sqlite) | user, file | sqlite |
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
| STORAGE_MODE | 存储模式 | file | local / s3 |
| PRIMARY_MAX_BYTES | 主存上限 (字节) | file | 10737418240 (10GB) |
| FILE_TMP_DIR | 分块临时目录 | file | /app/tmp |
| FILE_STORE_DIR | 本地文件存储目录 | file | /app/store |
| SEAWEEDFS_ENDPOINT | SeaweedFS S3 端点 | file, worker | http://seaweedfs:8333 |
| SEAWEEDFS_BUCKET | SeaweedFS Bucket | file, worker | light-cloud-disk |
| SEAWEEDFS_ACCESS_KEY | SeaweedFS AK | file, worker | - |
| SEAWEEDFS_SECRET_KEY | SeaweedFS SK | file, worker | - |
| OSS_ENDPOINT | 阿里云 OSS 端点 | file, worker | oss-cn-hangzhou.aliyuncs.com |
| OSS_ACCESS_KEY_ID | OSS AK | file, worker | *** |
| OSS_ACCESS_KEY_SECRET | OSS SK | file, worker | *** |
| KAFKA_BROKERS | Kafka 地址 (空则用 goroutine MQ) | file, file-worker | localhost:9092 |
| KAFKA_GROUP_ID | 消费者组 ID | file-worker | file-worker-group |
| KAFKA_CLOUD_MIGRATE_TOPIC | 冷迁移 topic | file-worker | cloud-migrate |
| KAFKA_THUMBNAIL_TOPIC | 缩略图 topic | file-worker | file-thumbnail |

## 测试覆盖

| 模块 | 测试类型 | 测试数 | 说明 |
|------|----------|--------|------|
| app/user/internal/biz | 单元测试 | 14 | Mock UserRepo |
| app/file/internal/biz | 单元测试 | 38 | Mock FileRepo + UserClient + MessageProducer + ObjectStorage + CloudStorage |
| app/gateway/internal/handler | 单元测试 | 15 | Mock gRPC 客户端 |
| app/gateway/internal/middleware | 单元测试 | 9 | JWT + CORS |
| app/user/internal/data | 集成测试 | 5 | 需要 MySQL (build tag) |
| app/file/internal/data | 集成测试 | 7 | 需要 MySQL + Redis (build tag) |

## 前端功能

- 登录 / 注册页面
- 文件浏览器 (网格/列表双视图，排序，面包屑导航)
- 文件上传 (分块上传，进度面板，秒传/断点续传)
- 文件管理 (新建文件夹、重命名、删除、移动、下载)
- 回收站 (恢复、彻底删除、清空)
- 文件分享 (创建分享链接、设密码、过期时间)
- 公开分享页面 (密码验证、文件下载)
- 用户资料设置 (昵称、邮箱、存储用量可视化)
- 深色 / 浅色主题切换 (浅色为天蓝风格，深色为深海军蓝)
- 响应式布局，可收起侧边栏

## 文档

- [微服务架构设计](docs/architecture.md)
- [双模式存储架构](docs/dual-mode-storage.md)
- [冷热分层存储](docs/cold-hot-storage.md)
- [API 网关实现](docs/gateway.md)
- [分块上传实现](docs/chunk-upload.md)
- [预签名分块上传](docs/presigned-upload.md)
- [服务发现与通信](docs/service-discovery.md)
- [消息队列集成](docs/message-queue.md)
- [CI/CD 配置](docs/ci-cd.md)
- [容器化部署](docs/containerization.md)
- [前端架构与设计](docs/frontend.md)

## 更新日志

### v6.0.0 (2026)
- **预签名分块上传**: 客户端直传 SeaweedFS/OSS，支持跨设备断点续传 (4 个新 RPC)
- **MD5 一致性哈希路由**: FNV32a + 150 虚拟节点，上传请求按文件 MD5 路由到固定 File Service 实例
- **双模式上传**: CheckUpload 自动选择 direct/presigned 模式，前端透明切换
- **双模式存储架构**: Mode A (轻量: SQLite + 本地磁盘 + goroutine MQ) / Mode B (完整: MySQL + SeaweedFS + Kafka)
- 可插拔数据库: MySQL / SQLite (DB_DRIVER 环境变量切换)
- 可插拔消息队列: Kafka / 进程内 goroutine channel (自动根据 KAFKA_BROKERS 选择)
- 本地文件流式下载: StreamFileContent server-streaming RPC + Gateway 流式代理
- Docker Compose profiles 双模式: `--profile s3` 启动完整模式，默认轻量模式
- 统一存储配置: STORAGE_MODE + PRIMARY_MAX_BYTES 控制主存行为
- Dockerfile 基于 debian (golang:1.25 + bookworm-slim)，内置 gcc 无需网络下载
- 前端双模式下载: local:// URL 自动走 blob 流式下载
- 98 个后端单元测试 + 50 个前端测试
- **安全加固**: presigned 操作 user_id 鉴权、服务端独立计算 totalParts、hashRouter 连接泄漏修复、事务原子删除、前端失败自动 abort

### v5.0.0 (2026)
- **冷热分层存储**: 主存 (Mode A: 本地磁盘 / Mode B: SeaweedFS) → LRU 淘汰 → 阿里云 OSS 冷归档
- LRU 自动冷迁移: SeaweedFS 超阈值 → Kafka cloud-migrate → file-worker 异步搬迁到 OSS
- 磁盘满保护: CheckUpload 检测 → 自动切换预签名上传
- 磁盘用量查询 API (GetDiskUsage)
- 下载链接按 StorageType 路由 (SeaweedFS/OSS 预签名 URL)
- SeaweedFS (aws-sdk-go-v2/s3) + 阿里云 OSS (alibabacloud-oss-go-sdk-v2) 集成
- Redis 原子计数器追踪 local/seaweedfs 磁盘用量
- file-worker 完整实现: SeaweedFS→OSS 数据搬迁 + DB/Redis 状态更新
- Docker Compose 新增 SeaweedFS 服务
- 71 个单元测试 (biz 33 + handler 15 + middleware 9 + user/biz 14)

### v4.0.0 (2026)
- 新增 Vue 3 前端 (TypeScript + Vite + Tailwind CSS v4)

### v4.0.1 (2026)
- CI 新增 Compose 冒烟验证（全栈容器启动 + Gateway/Frontend/后端端口连通性检查）
- 修复前端镜像依赖宿主机 `dist` 的构建问题，改为 Docker 多阶段自构建
- Compose 中 Kafka 镜像固定到 `3.6.2`，避免 `latest` 漂移
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
- MQ 从 RabbitMQ 迁移到 Kafka
- 添加存储配额管理

### v1.0.0 (2020)
- 基于 go-micro 的初始版本
- 基础文件上传/下载功能


本项目旨在充分利用服务器本地磁盘来节省成本. 我提供了轻量和全量两套利用本地磁盘的方案, 轻量方案直接利用本地磁盘, 我用自定义逻辑做了秒传, 分块续传和断点续传. 全量方案则在本地磁盘建立私有 S3 服务来实现那些功能.

我给项目设置了最大可在本地磁盘存储多少数据. 在 Redis 中维护当前的用量, 当用量超过一定阈值后, 用 LRU 淘汰本地的冷文件并用消息队列异步将冷数据上云并更新 Redis 当前用量. Redis 冷启动或断线重连后会做一次全表查询查实际用量.

上传时如果判断本地磁盘不够用后端就会预签一个 oss 的 上传 url 给前端用.

秒传: 如果查询 Redis 发现 md5 和 大小都相同的文件(这在数据库中要建立联合索引), 就会触发秒传, 直接在 MySQL 中把这个文件逻辑上复制一份即可(无需优化, 秒传是少量场景, 限流即可). Redis 冷启动时从 MySQL 中获取当前的文件列表的 md5 和 大小列表(设置一个最大的获取值, 如果太多了就按照上次访问时间排序只获取一部分).

轻量方案分块上传和断点续传:

给每个文件分配一个 id, 用 redis 的 set 维护已经上传的分块块号. 所有分块上传完后合并并把文件记录写进 MySQL(可以用 redis + mq 优化), 再把 md5 和 大小 写进 redis 中即可.

> 如果有恶意的人上传了很多分块, 但是一个都没有传完, 导致服务器本地磁盘全是垃圾怎么办?


文件删除:
