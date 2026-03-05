# 容器化部署

本项目支持 Docker Compose 和 Podman Compose，提供**双模式部署**。

## 部署模式

| | Mode A: local (轻量) | Mode B: s3 (完整) |
|--|---|---|
| **启动命令** | `docker compose --env-file .env.local up -d` | `docker compose --env-file .env.s3 --profile s3 up -d` |
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

```bash
# Mode A: 轻量模式
docker compose --env-file .env.local up -d --build

# Mode B: 完整模式
docker compose --env-file .env.s3 --profile s3 up -d --build

# 仅基础设施 (本地开发)
docker compose up -d consul redis

# 查看日志
docker compose logs -f gateway user-service file-service

# 停止服务
docker compose down
```

## Makefile 命令

```bash
make image-user       # 构建用户服务镜像
make image-file       # 构建文件服务镜像
make image-gateway    # 构建网关镜像

make infra-up         # 启动基础设施 (MySQL + Redis + Consul + Kafka)
make infra-down       # Stop services基础设施

make run-user         # 本地运行用户服务
make run-file         # 本地运行文件服务
make run-gateway      # 本地运行网关
```

## Dockerfile

多阶段构建，通过 `SERVICE` 构建参数指定服务：

```dockerfile
# Build stage
FROM golang:1.25-alpine AS builder
RUN apk add --no-cache gcc musl-dev  # CGO for SQLite
ARG SERVICE
COPY . .
# CGO_ENABLED=1 for SQLite support
RUN CGO_ENABLED=1 go build -mod=vendor -o /app/server ${BUILD_PATH}

# Runtime stage
FROM alpine:3.21
RUN mkdir -p /app/data /app/store  # SQLite DB + local file storage
COPY --from=builder /app/server /app/server
```

特点：
- `CGO_ENABLED=1` + musl-dev 支持 SQLite 编译
- 最终镜像约 25MB
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
2. **为什么 Alpine？** — 体积小 (~5MB)，安全更新及时
3. **Consul 在容器中怎么工作？** — 单节点 server 模式，服务通过 `consul:8500` 访问
4. **服务启动顺序？** — `depends_on` + healthcheck 确保基础设施就绪后才启动应用
