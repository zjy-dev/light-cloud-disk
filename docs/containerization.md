# 容器化部署

本项目支持 Docker Compose 和 Podman Compose。

## 服务组成

| 服务 | 镜像 | 端口 | 说明 |
|------|------|------|------|
| consul | hashicorp/consul:1.19 | 8500 (UI+API) | 服务注册与发现 |
| mysql | mysql:8.0 | 3306 | 主数据库 |
| redis | redis:7-alpine | 6379 | 分块上传状态缓存 |
| kafka | bitnami/kafka:3.6 | 9092 | 消息队列 (预留) |
| user-service | 自构建 | 9001 (gRPC) | 用户服务 |
| file-service | 自构建 | 9002 (gRPC) | 文件服务 |
| gateway | 自构建 | 8080 (HTTP) | API 网关 |

## 快速启动

```bash
# 启动所有服务
docker-compose up -d   # 或 podman-compose up -d

# 仅启动基础设施 (本地开发)
docker-compose up -d consul mysql redis kafka

# 查看日志
docker-compose logs -f gateway user-service file-service

# 停止
docker-compose down
```

## Makefile 命令

```bash
make image-user       # 构建用户服务镜像
make image-file       # 构建文件服务镜像
make image-gateway    # 构建网关镜像

make infra-up         # 启动基础设施 (MySQL + Redis + Consul + Kafka)
make infra-down       # 停止基础设施

make run-user         # 本地运行用户服务
make run-file         # 本地运行文件服务
make run-gateway      # 本地运行网关
```

## Dockerfile

多阶段构建，通过 `SERVICE` 构建参数指定服务：

```dockerfile
# 构建阶段
FROM golang:1.25-alpine AS builder
ARG SERVICE
COPY . .
RUN go build -o /app/server ./app/${SERVICE}/cmd

# 运行阶段
FROM alpine:3.21
COPY --from=builder /app/server /app/server
COPY app/${SERVICE}/configs/ /app/configs/
CMD ["/app/server", "-conf", "/app/configs/"]
```

特点：
- `CGO_ENABLED=0` 静态编译，无外部依赖
- 最终镜像约 20MB
- 支持通过 `--build-arg SERVICE=user|file|gateway` 构建不同服务

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

通过 `.env` 文件或直接设置环境变量：

```bash
# 数据库
DB_PASSWORD=root123
DB_NAME=cloud_disk

# JWT
JWT_SECRET=your_jwt_secret

# OSS (可选)
OSS_ENDPOINT=oss-cn-hangzhou.aliyuncs.com
OSS_ACCESS_KEY_ID=your_key
OSS_ACCESS_KEY_SECRET=your_secret
OSS_BUCKET_NAME=your_bucket

# 版本
VERSION=v3.0.0
```

## 本地开发模式

```bash
# 1. 启动基础设施
make infra-up

# 2. 等待 Consul UI 可用: http://localhost:8500

# 3. 在不同终端启动三个服务
make run-user      # 终端 1
make run-file      # 终端 2
make run-gateway   # 终端 3

# 4. 测试 API
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
