# Quick Start: 005-scattered-kafka

## 前提条件

- Go 1.25+
- Node.js 22+ (pnpm)
- Podman 或 Docker + Compose

## 本地开发

```bash
# 1. 启动基础设施 (Consul + Redis + MySQL + Kafka)
make infra-up

# 2. 启动用户服务
make run-user

# 3. 启动文件服务 (会同时启动 gRPC :9002 和 HTTP :9003)
make run-file

# 4. 启动网关
make run-gateway

# 5. 前端开发
cd frontend && pnpm dev
```

## 容器部署

```bash
# 构建镜像
make images

# 启动全部服务 (包含 Kafka)
make up
```

## 环境变量新增

| 变量 | 说明 | 默认值 |
|------|------|--------|
| KAFKA_BROKERS | Kafka broker 地址列表 (逗号分隔) | (空, 降级为 goroutine MQ) |
| FILE_HTTP_ADDR | 文件服务 HTTP 监听地址 | :9003 |

## 测试

```bash
# 后端全部单元测试
go test -race ./...

# 前端测试
cd frontend && pnpm test
```
