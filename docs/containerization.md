# 容器化部署

本项目以 **Podman + podman-compose** 为一等公民，同时兼容 Docker / Docker Compose。
Makefile 会自动检测 `podman` / `docker` 命令并使用。

## 服务组成

| 服务 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| consul | hashicorp/consul:1.19 | 8500 | 服务注册与发现 |
| redis | redis:7-alpine | 6379 | 分块状态 + 分布式锁 + 磁盘用量计数 |
| mysql | mysql:8.0 | 3306 | 关系数据库 |
| user-service | 自构建 | 9001 (gRPC) | 用户服务 |
| file-service | 自构建 | 9002 (gRPC) | 文件服务 (纠删码 + 本地存储) |
| gateway | 自构建 | 8080 (HTTP) | API 网关 (一致性哈希路由) |
| frontend | 自构建 | 3000 (Nginx) | Vue 3 SPA |

## 快速启动

> **为什么分两步（先 build 再 up）？**
> podman-compose 1.x 不支持 Compose spec 的 `build.network` 字段，
> 而 podman 默认 bridge 网络在某些环境无法访问互联网。
> `make images` 统一使用 `--network host` 构建，确保任何环境都能成功。

```bash
# 构建镜像并启动
make images              # 构建 backend + frontend 镜像
make up                  # 启动全部服务 (默认 .env)

# 仅基础设施 (本地开发)
make infra-up            # 启动 Consul + MySQL + Redis

# 查看日志 / 停止服务
make logs
make down
```

如果你使用 Docker Compose（支持 `build.network`），也可以直接一步到位：
```bash
docker compose --env-file .env up -d --build
```

## Makefile 命令

```bash
# 镜像构建 (统一 --network host)
make image-user       # 构建用户服务镜像
make image-file       # 构建文件服务镜像
make image-gateway    # 构建网关镜像
make image-frontend   # 构建前端镜像
make images           # 构建全部

# Compose 操作
make up               # 启动 (默认 .env)
make down             # 停止
make ps               # 查看状态
make logs             # 查看日志
make clean-containers # 停止并清除数据卷

# 本地开发 (不用容器)
make infra-up         # 启动 Consul + MySQL + Redis
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
- 支持通过 `--build-arg SERVICE=user|file|gateway` 构建不同服务
- `/app/data/` 存放 SQLite 数据库文件
- `/app/store/` 存放本地文件 + 纠删码分片
- gateway 不需要 YAML 配置文件，从环境变量读取

## Frontend Dockerfile

前端采用多阶段构建：

1. `node:24-alpine` 阶段安装依赖并执行 `pnpm build`
2. `nginx:1.27-alpine` 阶段仅拷贝 `dist/` 静态文件并提供服务

这样可确保 CI 与本地容器构建行为一致，不依赖宿主机预构建产物。

## 启动顺序

```
Consul → MySQL → Redis
    ↓        ↓       ↓
 user-service (等待 MySQL + Consul healthy)
 file-service (等待 MySQL + Redis + Consul healthy)
    ↓
 gateway (等待 Consul + user-service + file-service started)
```

docker-compose.yml 使用 `depends_on` + `condition` 控制启动顺序。

## 环境变量

通过 `.env` 文件配置：

```bash
DB_DRIVER=mysql              # 数据库驱动 (mysql/sqlite)
DB_PASSWORD=root123          # MySQL 密码
SQLITE_PATH=/app/data/cloud_disk.db  # SQLite 路径 (DB_DRIVER=sqlite 时)
PRIMARY_MAX_BYTES=10737418240  # 本地磁盘上限 (10 GB)
FILE_STORE_DIR=/app/store    # 文件存储目录
JWT_SECRET=your_jwt_secret
CONSUL_ADDR=consul:8500
REDIS_ADDR=redis:6379
ERASURE_DATA_SHARDS=4        # 纠删码数据分片数
ERASURE_PARITY_SHARDS=2      # 纠删码校验分片数
ERASURE_MIN_FILE_SIZE=1048576  # 纠删码最小文件 (1 MB)
# OSS (冷存)
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
