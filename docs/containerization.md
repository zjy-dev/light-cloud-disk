# 容器化部署

本项目优先支持 Podman，同时兼容 Docker。

## 容器运行时

| 运行时 | 优先级 | 说明 |
|--------|--------|------|
| Podman | 一等公民 | 无守护进程，rootless，更安全 |
| Docker | 兼容 | 完全兼容 |

## 快速开始

### 使用 Podman

```bash
# 启动所有服务
podman-compose up -d

# 仅启动基础设施 (本地开发)
podman-compose up -d mysql redis kafka

# 查看日志
podman-compose logs -f

# 停止服务
podman-compose down
```

### 使用 Docker

```bash
# 启动所有服务
docker-compose up -d

# 查看状态
docker-compose ps

# 停止并清理
docker-compose down -v
```

## Makefile 命令

Makefile 自动检测容器运行时（优先 Podman）：

```bash
make images        # 构建所有镜像
make image-user    # 构建用户服务镜像
make image-file    # 构建文件服务镜像

make up            # 启动所有服务
make down          # 停止所有服务
make logs          # 查看日志
make ps            # 查看状态

make infra-up      # 仅启动基础设施 (MySQL, Redis, Kafka)
make infra-down    # 停止基础设施

make clean-containers  # 清理容器和卷
```

## 服务端口

| 服务 | HTTP | gRPC |
|------|------|------|
| user-service | 8000 | 9000 |
| file-service | 8001 | 9001 |
| MySQL | 3306 | - |
| Redis | 6379 | - |
| Kafka | 9092 | - |

## 环境变量

通过 `.env` 文件配置：

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
VERSION=v2.0.0
```

## Dockerfile 说明

使用多阶段构建，最终镜像约 20MB：

```dockerfile
# 构建阶段: golang:1.22-alpine
# 运行阶段: alpine:3.19
```

特点：
- `CGO_ENABLED=0` 静态编译
- 使用 `docker.io/library/` 前缀确保 Podman 兼容
- 支持通过 `--build-arg` 指定服务和版本

## 本地开发模式

```bash
# 1. 启动基础设施
make infra-up

# 2. 本地运行服务 (热重载)
make run-user
make run-file

# 3. 开发完成后停止
make infra-down
```

## 生产部署

```bash
# 1. 设置环境变量
cp .env.example .env
vim .env

# 2. 构建镜像
VERSION=v2.0.0 make images

# 3. 启动服务
VERSION=v2.0.0 podman-compose up -d

# 4. 查看日志
podman-compose logs -f user-service file-service
```

## 面试要点

1. **为什么优先 Podman？**
   - 无守护进程 (daemonless)
   - 支持 rootless 容器，更安全
   - 兼容 Docker 命令和镜像
   - Red Hat/Fedora 默认容器运行时

2. **多阶段构建的好处？**
   - 构建环境与运行环境分离
   - 最终镜像更小 (~20MB vs ~1GB)
   - 不包含编译工具，更安全

3. **为什么用 Alpine？**
   - 体积小 (~5MB)
   - 安全更新及时
   - 足够运行静态编译的 Go 二进制
