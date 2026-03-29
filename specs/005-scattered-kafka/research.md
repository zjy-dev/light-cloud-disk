# Research: Kafka + 分块打散存储

## segmentio/kafka-go

- Go 原生 Kafka 客户端，无 CGO 依赖
- 支持 KRaft (无需 ZooKeeper)
- Producer: `kafka.Writer` 自动重试、批量发送
- Consumer: `kafka.Reader` 消费者组、偏移量自动提交
- 版本: v0.4.x (最新稳定)

## KRaft Mode Kafka Docker

- Kafka 3.7+ 官方镜像 `apache/kafka` 支持 KRaft
- 无需 ZooKeeper，单节点部署足够开发使用
- 环境变量: `KAFKA_NODE_ID`, `KAFKA_PROCESS_ROLES`, `KAFKA_LISTENERS`, `KAFKA_CONTROLLER_QUORUM_VOTERS`

## 浏览器并发 HTTP 限制

- 现代浏览器对同一 origin 最多 6 个并发连接
- 不同 file-service 实例是不同 origin，不受此限制
- 需要处理 CORS（file-service HTTP 端点需返回正确的 CORS 头）

## 一致性哈希容错策略

- **虚拟节点**: 150 vnodes/instance (已有)
- **N 副本策略**: `pickN(key, n)` 顺时针走 N 个不同物理节点
- **故障跳过**: 下载时第一个节点超时 → 尝试下一个候选
- **纠删码互补**: 数据分片和校验分片分布在不同实例上，单实例故障可重建

## JWT Token 传递

- Gateway 已有 JWT 验证和签发逻辑
- 方案: 客户端获取上传计划时附带现有 JWT token
- File-service HTTP server 复用相同的 JWT_SECRET 进行验证
- 无需额外的 token 签发流程

## Gin HTTP Server in File Service

- File service 当前仅有 gRPC server
- 新增 Gin HTTP server 可与 gRPC server 共存于同一进程
- 各自监听不同端口 (gRPC :9002, HTTP :9003)
- Wire 注入 HTTP server 到 Kratos App 的 `kratos.Server()` 列表中
