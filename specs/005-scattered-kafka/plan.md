# Implementation Plan: Kafka MQ + 分块打散存储 + 并发下载 + 一致性哈希容错

**Branch**: `005-scattered-kafka` | **Date**: 2025-07-22 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/005-scattered-kafka/spec.md`

## Summary

将文件上传流程从"网关代理分块→单实例合并"改为"网关分配上传计划→客户端直连文件服务实例→分块永久打散存储"。同时将进程内 goroutine MQ 替换为 Kafka，增强一致性哈希容错机制，并实现客户端并发下载。

## Technical Context

**Language/Version**: Go 1.25, TypeScript 5.9  
**Primary Dependencies**: Kratos v2, Gin, GORM, go-redis/v8, segmentio/kafka-go, klauspost/reedsolomon, Vue 3, Vite 7  
**Storage**: MySQL (default) / SQLite + Redis + local disk + Alibaba Cloud OSS  
**Testing**: `go test -race`, Vitest  
**Target Platform**: Linux server (Podman/Docker) + Web browser  
**Project Type**: Microservices (web-service)  
**Performance Goals**: 上传/下载吞吐量随实例数线性增长  
**Constraints**: CGO_ENABLED=1, vendor offline build, Podman 兼容  
**Scale/Scope**: 3+ file-service instances, 10k+ files

## Constitution Check

| Principle | Status | Notes |
|-----------|--------|-------|
| I. Clean Architecture | ✅ Pass | Kafka producer/consumer 在 data 层，biz 层只依赖 MessageProducer 接口 |
| II. Single-Mode with DB Flexibility | ⚠️ Amendment | 将 goroutine MQ 改为 Kafka，但保留 goroutine 降级模式（无 KAFKA_BROKERS 时） |
| III. Interface-Driven Communication | ✅ Pass | MessageProducer 接口不变，新增 Kafka 实现 |
| IV. Test-First Discipline | ✅ Pass | 所有新代码需写测试 |
| V. Configuration Hygiene | ✅ Pass | KAFKA_BROKERS 走 .env 环境变量 |
| VI. Vendor-Locked Offline Build | ✅ Pass | kafka-go 加入 vendor |
| VII. Documentation as Deliverable | ✅ Pass | 更新 docs + README |

## Project Structure

### Documentation (this feature)

```text
specs/005-scattered-kafka/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Research notes
├── data-model.md        # Data model changes
├── quickstart.md        # Quick start guide
├── checklists/
│   └── requirements.md  # Quality checklist
└── tasks.md             # Task list
```

### Source Code (repository root)

```text
api/file/v1/
├── file.proto           # 修改: 新增 GetUploadPlan/CompleteUpload/GetDownloadPlan RPC；移除 MergeChunks
├── file.pb.go           # regenerated
└── file_grpc.pb.go      # regenerated

app/file/
├── cmd/
│   ├── main.go          # 修改: 添加 HTTP server + Kafka consumer 启动
│   ├── wire.go          # 修改: 添加 NewKafkaProducer + NewHTTPServer provider
│   └── wire_gen.go      # regenerated
├── configs/
│   └── config.yaml      # 修改: 添加 kafka 配置
└── internal/
    ├── biz/
    │   ├── file.go      # 重大修改: 移除 MergeChunks，新增 GetUploadPlan/CompleteUpload/GetDownloadPlan
    │   └── file_test.go # 重写受影响的测试
    ├── data/
    │   ├── data.go      # 修改: ProviderSet 添加 Kafka + HTTP
    │   ├── file.go      # 修改: 新增 chunk_records 表操作，移除 MergeChunkData
    │   ├── mq_goroutine.go  # 保留作为降级方案
    │   ├── mq_kafka.go      # 新增: Kafka producer + consumer 实现
    │   └── file_integration_test.go # 更新集成测试
    ├── service/
    │   └── file.go      # 修改: 新增 RPC 方法映射
    └── server/
        ├── grpc.go      # 现有
        └── http.go      # 新增: Gin HTTP server (分块上传/下载端点)

app/gateway/
├── cmd/
│   └── main.go          # 修改: 新增上传计划/下载计划路由
└── internal/
    ├── client/
    │   └── client.go    # 修改: 增强哈希环容错（N 次跳过失败节点）
    ├── handler/
    │   ├── file.go      # 修改: 替换 UploadChunk/MergeChunks 为 GetUploadPlan/CompleteUpload/GetDownloadPlan
    │   └── handler_test.go # 更新测试
    └── middleware/
        └── jwt.go       # 现有不变

frontend/src/
├── api/
│   └── file.ts          # 修改: 新增 getUploadPlan/completeUpload/getDownloadPlan API
├── composables/
│   └── useUpload.ts     # 重大修改: 实现直连文件服务实例的分块上传
├── composables/
│   └── useDownload.ts   # 新增: 并发下载分块 + 本地拼接
└── types/
    └── index.ts         # 修改: 新增上传计划/下载计划类型

docker-compose.yml       # 修改: 添加 Kafka (KRaft), 多 file-service 实例, 暴露 HTTP 端口
Dockerfile               # 修改: file service 暴露 HTTP 端口
.env.example             # 修改: 新增 KAFKA_BROKERS, FILE_HTTP_ADDR
Makefile                 # 修改: 新增 Kafka 相关 make target
```

**Structure Decision**: 沿用现有 monorepo 结构。每个 file-service 实例新增一个 Gin HTTP server 处理分块上传/下载，共享同一个二进制。

## Phase 0: Research

### Key Design Decisions

1. **文件服务 HTTP 端点**: file-service 在 gRPC 之外增加一个 Gin HTTP server (`:9003`)。客户端直连此端口上传/下载分块。JWT 验证通过 shared secret（与 gateway 相同的 JWT_SECRET）。

2. **上传流程变更**:
   - 旧: Client → Gateway (HTTP) → File Service (gRPC UploadChunk) → 本地磁盘 → MergeChunks → 完整文件
   - 新: Client → Gateway (GetUploadPlan) → 获取各分块目标地址 → Client 直连各 File Service (HTTP PUT /chunks) → Client → Gateway (CompleteUpload) → 验证所有分块 → 创建文件记录

3. **无合并存储模型**: 分块永久存储在各实例本地磁盘。新增 `chunk_records` 表记录每个分块的实例地址和磁盘路径。FileStore 表保留但不再存储合并后的文件路径。

4. **下载流程**:
   - Client → Gateway (GetDownloadPlan) → 返回所有分块的下载 URL 列表
   - Client 并发 HTTP GET 各实例的 `/chunks/:md5/:index` → 本地拼接

5. **Kafka 集成**: 使用 segmentio/kafka-go，KRaft 模式（无 ZooKeeper）。Topic: `cloud-migrate`, `file-thumbnail`。file-service 启动时同时启动 consumer goroutine。

6. **一致性哈希容错**: 
   - `pick()` 改为 `pickN(key, n)` 返回 N 个候选节点
   - 上传时分块哈希到主节点，纠删码校验分片分布到后续候选节点
   - 下载时主节点不可用则尝试备选节点（纠删码重建）
   - Consul 健康检查间隔从 15s 保持不变

7. **纠删码调整**: 从整文件编码改为按分块编码。每个分块独立生成 parity shards，分布到不同实例。

## Phase 1: Design

### Data Model Changes

参见 [data-model.md](data-model.md)。

核心变更:
- 新增 `chunk_records` 表: (file_md5, chunk_index, instance_addr, store_path, chunk_size, checksum)
- `file_stores` 表: 移除 `store_path` 的合并文件语义，新增 `total_chunks` 字段
- `erasure_shards` 表: `file_store_id` 改为关联 chunk_record (`chunk_record_id`)
- 移除 `MergeChunkData` repo 方法
- 新增 `CreateChunkRecord`, `FindChunkRecords`, `FindChunkRecordByIndex` repo 方法

### API Contract Changes

参见 [contracts/](contracts/) 目录。

Proto 变更:
- 新增 `GetUploadPlan` RPC: 返回 `{chunks: [{index, target_addr, upload_url}], token}`
- 新增 `CompleteUpload` RPC: 验证所有分块已上传，创建文件记录
- 新增 `GetDownloadPlan` RPC: 返回 `{chunks: [{index, download_url, instance_addr}]}`
- 修改 `CheckUpload`: 返回中增加 `upload_plan` 字段（合并到一次调用减少 RTT）
- 移除 `MergeChunks` RPC
- 保留 `UploadChunk` RPC 仅用于单实例降级模式

### Gateway Changes

- 移除 `UploadChunk` 和 `MergeChunks` HTTP 处理器
- 新增 `CompleteUpload` 和 `GetDownloadPlan` 处理器
- `CheckUpload` 处理器扩展返回上传计划
- hash router `pickN()` 方法用于生成上传计划

### Frontend Changes

- `useUpload.ts`: 从网关获取上传计划 → 并发直连各实例上传分块 → 通知网关完成
- 新增 `useDownload.ts`: 获取下载计划 → 并发拉取分块 → Blob 拼接 → 触发下载
- 文件下载不再通过网关流式传输（小文件仍可通过网关代理）

## Complexity Tracking

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| Kafka 外部依赖 | 多实例间可靠异步任务分发 | goroutine channel 无法跨进程通信 |
| HTTP server in file-service | 客户端直连上传/下载绕过网关 | gRPC binary stream 对浏览器不友好 |
