# 容器化部署

本项目以 **Podman + podman-compose** 为一等公民，同时兼容 Docker / Docker Compose。
Makefile 会自动检测 `podman` / `docker` 命令并使用。

## 部署模式

| | Mode A: local (轻量) | Mode B: s3 (完整) |
|--|---|---|
| **启动命令** | `make images && make up` | `make images-all && make up ENV_FILE=.env.s3 PROFILE="--profile s3"` |
| **服务数** | 6 (consul, redis, user, file, gateway, frontend) | 10 (+ mysql, kafka, seaweedfs, file-worker) |
| **数据库** | SQLite (容器内文件) | MySQL 8.0 |
| **消息队列** | 进程内 goroutine | Kafka |
| **对象存储** | 本地磁盘 | SeaweedFS |

## 服务组成

| 服务 | 镜像 | 端口 | Profiles | 说明 |
|------|------|------|----------|------|
| consul | hashicorp/consul:1.19 | 8500 | - (always) | 服务注册与发现 |
| redis | redis:7-alpine | 6379 | - (always) | 分块状态 + 磁盘用量计数 |
| mysql | mysql:8.0 | 3306 | s3 | 数据库 (完整模式) |
| kafka | apache/kafka:3.7.0 | 9092 | s3 | 消息队列 (完整模式) |
| seaweedfs | chrislusf/seaweedfs:latest | 9333, 8333 | s3 | S3 兼容对象存储 (完整模式) |
| user-service | 自构建 | 9001 (gRPC) | - (always) | 用户服务 |
| file-service | 自构建 | 9002 (gRPC) | - (always) | 文件服务 |
| gateway | 自构建 | 8080 (HTTP) | - (always) | API 网关 |
| file-worker | 自构建 | - | s3 | Kafka 消费：冷迁移 (完整模式) |
| frontend | 自构建 | 3000 (Nginx) | - (always) | Vue 3 SPA |

`profiles: [s3]` 标记的服务只在 `--profile s3` 模式下启动。

## 快速启动

> **为什么分两步（先 build 再 up）？**
> podman-compose 1.x 不支持 Compose spec 的 `build.network` 字段，
> 而 podman 默认 bridge 网络在某些环境无法访问互联网。
> `make images` 统一使用 `--network host` 构建，确保任何环境都能成功。

```bash
# Mode A: 轻量模式
make images              # 构建 backend + frontend 镜像
make up                  # 启动 (默认 .env.local)

# Mode B: 完整模式
make images-all          # 额外构建 file-worker
make up ENV_FILE=.env.s3 PROFILE="--profile s3"

# 仅基础设施 (本地开发)
make infra-up

# 查看日志 / 停止服务
make logs
make down
```

如果你使用 Docker Compose（支持 `build.network`），也可以直接一步到位：
```bash
docker compose --env-file .env.local up -d --build
docker compose --env-file .env.s3 --profile s3 up -d --build
```

## Makefile 命令

```bash
# 镜像构建 (统一 --network host)
make image-user       # 构建用户服务镜像
make image-file       # 构建文件服务镜像
make image-gateway    # 构建网关镜像
make image-worker     # 构建 file-worker 镜像
make image-frontend   # 构建前端镜像
make images           # 构建全部 (不含 worker)
make images-all       # 构建全部 (含 worker)

# Compose 操作
make up               # 启动 (默认 .env.local)
make down             # 停止
make ps               # 查看状态
make logs             # 查看日志
make clean-containers # 停止并清除数据卷

# 自定义 env / profile
make up ENV_FILE=.env.s3 PROFILE="--profile s3"

# 本地开发 (不用容器)
make infra-up         # 启动 Consul + Redis
make run-user         # 本地运行用户服务
make run-file         # 本地运行文件服务
make run-gateway      # 本地运行网关
```

## Dockerfile

多阶段构建，通过 `SERVICE` 构建参数指定服务：

```dockerfile
# Build stage — debian 基础镜像内置 gcc，无需网络安装依赖
FROM golang:1.25 AS builder
ARG SERVICE
COPY . .
# CGO_ENABLED=1 for SQLite support, -mod=vendor 零网络离线编译
RUN CGO_ENABLED=1 go build -mod=vendor -o /app/server ${BUILD_PATH}

# Runtime stage — debian-slim (glibc 兼容 CGO 二进制)
FROM debian:bookworm-slim
RUN mkdir -p /app/data /app/store  # SQLite DB + local file storage
COPY --from=builder /app/server /app/server
```

特点：
- 基于 debian (golang:1.25 + bookworm-slim)，内置 gcc 无需网络下载
- `CGO_ENABLED=1` 支持 SQLite 编译
- `-mod=vendor` 离线构建，配合 `--network host` 保证任何网络环境都能成功
- 支持通过 `--build-arg SERVICE=user|file|gateway|worker` 构建不同服务
- `/app/data/` 存放 SQLite 数据库文件
- `/app/store/` 存放本地模式的合并文件
- worker 和 gateway 不需要 YAML 配置文件，从环境变量读取

## Frontend Dockerfile

前端采用多阶段构建：

1. `node:24-alpine` 阶段安装依赖并执行 `pnpm build`
2. `nginx:1.27-alpine` 阶段仅拷贝 `dist/` 静态文件并提供服务

这样可确保 CI 与本地容器构建行为一致，不依赖宿主机预构建产物。

## 启动顺序

```
Consul → MySQL → Redis → Kafka
    ↓        ↓       ↓
 user-service (等待 MySQL + Consul healthy)
 file-service (等待 MySQL + Redis + Consul healthy)
    ↓
 gateway (等待 Consul + user-service + file-service started)
```

docker-compose.yml 使用 `depends_on` + `condition` 控制启动顺序。

## 环境变量

通过 `.env.local` 或 `.env.s3` 文件配置：

### .env.local (轻量模式)
```bash
DB_DRIVER=sqlite
SQLITE_PATH=/app/data/cloud_disk.db
STORAGE_MODE=local
PRIMARY_MAX_BYTES=10737418240
FILE_STORE_DIR=/app/store
JWT_SECRET=your_jwt_secret
CONSUL_ADDR=consul:8500
REDIS_ADDR=redis:6379
```

### .env.s3 (完整模式)
```bash
DB_DRIVER=mysql
DB_PASSWORD=root123
STORAGE_MODE=s3
PRIMARY_MAX_BYTES=107374182400
KAFKA_BROKERS=kafka:9092
SEAWEEDFS_ENDPOINT=http://seaweedfs:8333
SEAWEEDFS_BUCKET=light-cloud-disk
JWT_SECRET=your_jwt_secret
CONSUL_ADDR=consul:8500
REDIS_ADDR=redis:6379
# OSS (optional)
OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
OSS_ACCESS_KEY_ID=your_key
OSS_ACCESS_KEY_SECRET=your_secret
OSS_BUCKET=your_bucket
```

## 本地开发模式

```bash
# 1. Start infrastructure
make infra-up

# 2. Wait for Consul UI: http://localhost:8500

# 3. Start three services in separate terminals
make run-user      # 终端 1
make run-file      # 终端 2
make run-gateway   # 终端 3

# 4. Test APIs
curl http://localhost:8080/health
curl -X POST http://localhost:8080/api/v1/user/register \
  -H "Content-Type: application/json" \
  -d '{"username":"test","password":"123456"}'
```

## 面试要点

1. **为什么用多阶段构建？** — 构建环境与运行环境分离，镜像更小更安全
2. **为什么 Dockerfile 用 debian 而不是 Alpine？** — CGO (SQLite) 编译需要 gcc，alpine 需要 `apk add` 下载，在 podman bridge 网络受限时会失败；debian 内置 gcc 无需额外网络请求
3. **为什么分两步 build + up？** — podman-compose 1.x 不支持 `build.network: host`，无法在 compose build 时指定宿主机网络；预构建镜像后 `up -d` 可直接使用
4. **Consul 在容器中怎么工作？** — 单节点 server 模式，服务通过 `consul:8500` 访问
5. **服务启动顺序？** — `depends_on` + healthcheck 确保基础设施就绪后才启动应用
