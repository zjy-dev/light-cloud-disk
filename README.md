# 轻网盘 Light Cloud Disk

[![CI](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml)
[![Release](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/release.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/releases)

基于 Kratos v2 的微服务云存储系统，采用 gRPC 服务拆分 + Gin API 网关 + Consul 服务发现架构。前端使用 Vue 3 + TypeScript + Tailwind CSS 构建。

**Monorepo 结构**: 后端 Go 代码在项目根目录，前端 Vue 3 SPA 在 `frontend/` 目录。

## 架构概览

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
    └────────┬────────────┘           └────┬───────┬────────┘
             │                             │       │
             ▼                             ▼       ▼
    ┌─────────────────┐           ┌────────────┐ ┌───────┐
    │     MySQL       │           │   MySQL    │ │ Redis │
    └─────────────────┘           └────────────┘ └───────┘

    ← ─ ─ ─  Consul 服务发现  ─ ─ ─ →
```

- **User Service**: 用户注册/登录、信息管理、存储配额 (gRPC-only, Kratos v2)
- **File Service**: 分块上传、秒传、文件管理、回收站、分享 (gRPC-only, Kratos v2)
- **API Gateway**: HTTP 路由、JWT 认证、CORS、gRPC 代理 (Gin)
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
| 缓存 | Redis | v8.11.5 |
| 消息队列 | Kafka (预留) | v3.6 |
| 依赖注入 | Wire | v0.6.0 |
| 认证 | JWT (golang-jwt/jwt v5) | v5 |
| 容器编排 | Docker/Podman Compose | - |
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
- [x] 文件夹管理 (树形结构)
- [x] 文件搜索 (模糊匹配)
- [x] 回收站 (软删除 + 恢复)
- [x] 文件分享 (链接 + 提取码 + 过期时间)
- [x] 文件移动/重命名
- [x] 下载链接获取

### 网关
- [x] JWT 认证中间件 (保护路由)
- [x] CORS 跨域中间件
- [x] 请求日志中间件
- [x] gRPC 代理 (HTTP → gRPC 协议转换)
- [x] Consul 服务发现 (自动发现后端服务)

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
cp .env.example .env
# 编辑 .env 填入实际配置
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
```bash
docker-compose up -d   # 或 podman-compose up -d
```

说明：
- 前端镜像采用多阶段构建（容器内执行 `pnpm install && pnpm build`），不再依赖宿主机预先生成 `frontend/dist/`
- `docker-compose.yml` 已包含前端、网关、用户服务、文件服务、Consul、MySQL、Redis、Kafka 全量服务

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
| GET | /api/v1/file/download/:file_id | 获取下载链接 |
| GET | /api/v1/files/search | 搜索文件 |
| GET | /api/v1/trash | 回收站列表 |
| POST | /api/v1/trash/restore | 恢复文件 |
| DELETE | /api/v1/trash | 彻底删除 |
| POST | /api/v1/share | 创建分享 |

## 服务通信

```
Client ──HTTP──▶ Gateway ──gRPC──▶ User Service
                    │                    ▲
                    │──gRPC──▶ File Service ──gRPC──┘
                                (UpdateStorageUsed)
```

- Gateway 通过 Consul 发现 `user-service` 和 `file-service`
- File Service 通过 Consul 发现 `user-service`，调用 `UpdateStorageUsed` 更新存储用量
- 所有 gRPC 连接使用 Kratos gRPC 客户端 + Consul 服务发现

## 消息队列

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| file-transfer | FileService | file-worker (TransferWorker) | 文件异步转存 OSS |
| file-thumbnail | FileService | file-worker (ThumbnailWorker) | 生成文件缩略图 |

客户端使用 `segmentio/kafka-go`，未配置 Kafka 时自动降级为 `noopProducer`。

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
| KAFKA_BROKERS | Kafka 地址 | file, file-worker | localhost:9092 |
| KAFKA_GROUP_ID | 消费者组 ID | file-worker | file-worker-group |
| KAFKA_TRANSFER_TOPIC | 转存 topic | file-worker | file-transfer |
| KAFKA_THUMBNAIL_TOPIC | 缩略图 topic | file-worker | file-thumbnail |
| FILE_TMP_DIR | 分块临时目录 | file | /app/tmp |
| FILE_STORE_DIR | 合并后文件目录 | file | /app/store |
| DOWNLOAD_URL_PREFIX | 下载 URL 前缀 | file | http://localhost:8080/downloads |

## 测试覆盖

| 模块 | 测试类型 | 测试数 | 说明 |
|------|----------|--------|------|
| app/user/internal/biz | 单元测试 | 14 | Mock UserRepo |
| app/file/internal/biz | 单元测试 | 30 | Mock FileRepo + UserClient + MessageProducer |
| app/gateway/internal/handler | 单元测试 | 13 | Mock gRPC 客户端 |
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
- [API 网关实现](docs/gateway.md)
- [分块上传实现](docs/chunk-upload.md)
- [服务发现与通信](docs/service-discovery.md)
- [消息队列集成](docs/message-queue.md)
- [CI/CD 配置](docs/ci-cd.md)
- [容器化部署](docs/containerization.md)
- [前端架构与设计](docs/frontend.md)

## 更新日志

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
