# Tasks: Kafka MQ + 分块打散存储 + 并发下载 + 一致性哈希容错

**Input**: Design documents from `/specs/005-scattered-kafka/`
**Prerequisites**: plan.md (required), spec.md (required), research.md, data-model.md

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: 添加 Kafka 依赖，更新 Proto 定义，准备容器编排和环境配置

- [ ] T001 [US3] 添加 segmentio/kafka-go 依赖到 `go.mod` 并 `go mod vendor`
- [ ] T002 修改 `api/file/v1/file.proto`: 移除 MergeChunks RPC；新增 CompleteUpload/GetDownloadPlan RPC 和 message 定义 (UploadPlan, ChunkAssignment, ChunkLocation)；修改 CheckUploadReply 增加 upload_plan 字段；运行 `make api` 重新生成 `file.pb.go` 和 `file_grpc.pb.go`
- [ ] T003 修改 `docker-compose.yml`: 添加 Kafka (apache/kafka:3.9 KRaft 模式) 服务；file-service 添加 KAFKA_BROKERS/FILE_HTTP_ADDR 环境变量并暴露 HTTP 端口 9003
- [ ] T004 [P] 修改 `.env.example`: 新增 KAFKA_BROKERS 和 FILE_HTTP_ADDR 变量

**Checkpoint**: Proto 接口定义完成，依赖就绪，Kafka 容器可启动

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: 新增 ChunkRecord 实体、Repo 接口、数据层实现——所有用户故事的前置依赖

**⚠️ CRITICAL**: Phase 3+ 的所有任务依赖本阶段完成

- [ ] T005 修改 `app/file/internal/biz/file.go`: 新增 ChunkRecord/UploadPlan/ChunkAssignment/DownloadPlan/ChunkLocation 实体定义；新增 `StorageScattered = "scattered"` 常量；FileStore 新增 `TotalChunks int32` 字段
- [ ] T006 修改 `app/file/internal/biz/file.go`: FileRepo 接口新增 chunk_records 相关方法 (CreateChunkRecord, FindChunkRecords, FindChunkRecordByIndex, CountChunkRecords, DeleteChunkRecords, UpdateChunkStorageLocation)；移除 MergeChunkData
- [ ] T007 修改 `app/file/internal/data/data.go`: GORM AutoMigrate 注册 ChunkRecordPO 模型
- [ ] T008 修改 `app/file/internal/data/file.go`: 新增 ChunkRecordPO 结构体定义 (含 UNIQUE 索引 file_md5+file_size+chunk_index) + 6 个 chunk_records GORM 操作实现；移除 MergeChunkData 实现；新增 TotalChunks 字段到 FileStorePO

**Checkpoint**: ChunkRecord 数据模型和 Repo 接口就绪

---

## Phase 3: User Story 1 — 分块打散上传 (Priority: P1) 🎯 MVP

**Goal**: 客户端绕过网关，直连文件服务实例上传分块

**Independent Test**: 上传多分块文件，验证分块分散存储到不同实例，网关仅处理元数据

### Implementation

- [ ] T009 修改 `app/file/internal/biz/file.go`: 移除 `MergeChunks()` 和 `uploadMergedFile()` 方法
- [ ] T010 修改 `app/file/internal/biz/file.go`: 实现 `CompleteUpload()` — 验证 chunk_records 数量 == total_chunks，创建 FileStore (storage_type=scattered) + File 记录，调用 UserClient.UpdateStorageUsed，触发异步 maybeEvictToCloud
- [ ] T011 修改 `app/file/internal/biz/file.go`: 新增 `CreateChunkRecord()` 公开方法供 HTTP server 调用
- [ ] T012 新增 `app/file/internal/server/http.go`: 创建 ChunkHTTPServer (Gin)，实现 Kratos transport.Server 接口 (Start/Stop)；挂载 JWT 中间件 + CORS；注册 `PUT /api/v1/chunks/:md5/:index` (上传) 和 `GET /api/v1/chunks/:md5/:index` (下载)
- [ ] T013 修改 `app/file/internal/service/file.go`: 替换 MergeChunks → 新增 CompleteUpload 和 GetDownloadPlan gRPC 方法映射
- [ ] T014 修改 `app/file/cmd/wire.go` + `wire_gen.go`: wireApp 返回 `(*grpc.Server, *biz.FileUsecase, func(), error)`；移除 newApp 从 Wire.Build
- [ ] T015 修改 `app/file/cmd/main.go`: 手动构造 ChunkHTTPServer (读取 FILE_HTTP_ADDR/JWT_SECRET)；newApp 接收 variadic `...transport.Server` extras；Kratos metadata 写入 `http_port`
- [ ] T016 修改 `app/gateway/internal/client/client.go`: hashRouter 新增 `httpAddrs map[string]string`；refresh() 读取 Consul 实例的 `http_port` metadata 构建 HTTP 地址映射；新增 `pickN(key, n)` 方法返回 N 个不同物理节点；新增 `GetHTTPAddrs()` 和 `PickNFileHTTPAddrs()` 方法
- [ ] T017 修改 `app/gateway/internal/handler/file.go`: 修改 `CheckUpload` 处理器——当 mode=direct 且非秒传时调用 `PickNFileHTTPAddrs` 生成上传计划；新增 `CompleteUpload` 处理器；移除 `UploadChunk` 和 `MergeChunks` 处理器
- [ ] T018 修改 `app/gateway/cmd/main.go`: 更新路由表——移除 `/file/upload-chunk` 路由；新增 `/file/complete-upload` 路由

### Tests

- [ ] T019 [P] [US1] 修改 `app/file/internal/biz/file_test.go`: 移除 MergeChunks 相关测试；新增 CompleteUpload 测试 (成功、缺少分块、重复完成)；更新 mock 接口 (移除 MergeChunkData，新增 6 个 ChunkRecord mock 方法)
- [ ] T020 [P] [US1] 修改 `app/gateway/internal/handler/handler_test.go`: 移除 UploadChunk/MergeChunks 测试和 multipartBody helper；新增 CheckUpload (含上传计划) 测试和 CompleteUpload 测试

**Checkpoint**: 分块直连上传端到端可用——US1 Acceptance Scenarios 1-3 全部可验证

---

## Phase 4: User Story 2 — 无合并存储与并发下载 (Priority: P1)

**Goal**: 分块永久存储 + 客户端并发下载拼接

**Independent Test**: 上传多分块文件后请求下载，验证客户端从多实例并发获取分块并拼接成原文件

### Implementation

- [ ] T021 修改 `app/file/internal/biz/file.go`: 实现 `GetDownloadPlan()` — 查询 chunk_records 获取所有分块位置；local 分块生成 HTTP 下载 URL；OSS 分块生成预签名 URL
- [ ] T022 修改 `app/gateway/internal/handler/file.go`: 新增 `GetDownloadPlan` 处理器
- [ ] T023 修改 `app/gateway/cmd/main.go`: 添加 `/file/download-plan` 路由
- [ ] T024 修改 `frontend/src/types/index.ts`: 新增 ChunkAssignment/UploadPlan/CompleteUploadReply/ChunkLocation/DownloadPlanReply TypeScript 接口；移除 UploadChunkReply/MergeChunksReply；更新 CheckUploadReply
- [ ] T025 修改 `frontend/src/api/file.ts`: 新增 `completeUpload()` 和 `getDownloadPlan()` API 方法；移除 `uploadChunk()` 和 `mergeChunks()` 方法
- [ ] T026 修改 `frontend/src/composables/useUpload.ts`: 重构——从 CheckUpload 响应获取 upload_plan → fetch() PUT 到各实例的 upload_url → 调用 completeUpload；MAX_CONCURRENT_UPLOADS=4；状态 'merging' → 'completing'
- [ ] T027 新增 `frontend/src/composables/useDownload.ts`: 实现并发下载——getDownloadPlan → 并发 fetch 各分块 → Blob 拼接 → 触发浏览器下载
- [ ] T028 修改 `frontend/src/stores/file.ts`: downloadFile 使用 getDownloadPlan——单分块直接 open URL，多分块用并发 fetch + Blob 拼接
- [ ] T029 修改 `frontend/src/components/file/UploadProgress.vue`: 状态文案 'merging' → 'completing' (3 处)

### Tests

- [ ] T030 [P] [US2] 修改 `app/file/internal/biz/file_test.go`: 新增 GetDownloadPlan 测试 (本地分块、OSS 分块、混合分块)
- [ ] T031 [P] [US2] 修改 `frontend/src/composables/__tests__/useUpload.test.ts`: 重写上传测试——mock completeUpload + global fetch 替代 uploadChunk/mergeChunks
- [ ] T032 [P] [US2] 修改 `frontend/src/stores/__tests__/file.test.ts`: 更新 downloadFile 测试——mock getDownloadPlan 替代 getDownloadURL

**Checkpoint**: 上传/下载全流程端到端可用 (散列存储 + 并发下载)——US2 Acceptance Scenarios 1-2 可验证

---

## Phase 5: User Story 3 — Kafka 消息队列 (Priority: P2)

**Goal**: 替换 goroutine channel MQ 为 Kafka，保留 goroutine 降级

**Independent Test**: 触发 LRU 淘汰后，验证迁移消息通过 Kafka 发布并被消费

### Implementation

- [ ] T033 新增 `app/file/internal/data/mq_kafka.go`: 实现 kafkaProducer (biz.MessageProducer 接口) — 两个 kafka.Writer (cloud-migrate/file-thumbnail topics)；实现 KafkaConsumer (Kratos transport.Server) — kafka.Reader + GroupID + handleCloudMigrate 迁移逻辑 + CommitMessages
- [ ] T034 修改 `app/file/internal/data/mq_goroutine.go`: `NewMessageProducer` 检查 KAFKA_BROKERS 环境变量——有则创建 kafkaProducer，无则创建 goroutineMQ
- [ ] T035 修改 `app/file/internal/conf/conf.proto` + 重新生成 `conf.pb.go`: Bootstrap 新增 `Kafka kafka = 5` 字段；新增 `message Kafka { repeated string brokers, string cloud_migrate_topic, string thumbnail_topic }`
- [ ] T036 修改 `app/file/internal/biz/file.go`: FileUsecase 新增 `Repo()` 和 `CloudStore()` 公开访问器方法 (供 KafkaConsumer 调用)
- [ ] T037 修改 `app/file/cmd/main.go`: 当 KAFKA_BROKERS 非空时创建 KafkaConsumer 并传入 newApp 作为额外 transport.Server

**Checkpoint**: Kafka MQ 可用，goroutine MQ 保留为降级方案——US3 Acceptance Scenarios 1-3 可验证

---

## Phase 6: User Story 4 — 一致性哈希容错 (Priority: P2)

**Goal**: 增强哈希环故障处理——不健康实例跳过 + 冷却恢复

**Independent Test**: 3 实例中停止 1 个，验证上传/下载正确降级到 2 个实例

### Implementation

- [ ] T038 修改 `app/gateway/internal/client/client.go`: hashRouter 新增 `unhealthy map[string]time.Time` + `unhealthyCooldown = 30s` 常量；pick() 顺时针遍历时跳过不健康实例 (全部不健康时 fallback)；pickN() 跳过不健康实例
- [ ] T039 修改 `app/gateway/internal/client/client.go`: 新增 `MarkUnhealthy(httpAddr)` 方法 (反向查找 HTTP→gRPC 地址)；ServiceClients 新增 `MarkFileInstanceUnhealthy()` 公开代理
- [ ] T040 修改 `app/gateway/internal/client/client.go`: refresh() 中清理过期的不健康标记 (超过 cooldown 的条目)

### Tests

- [ ] T041 [P] [US4] 修改 `app/gateway/internal/client/client_test.go`: 新增 TestHashRouter_Pick_SkipsUnhealthy、TestHashRouter_PickN_SkipsUnhealthy、TestHashRouter_MarkUnhealthy 测试；更新 rebuildRing helper 初始化 unhealthy map

**Checkpoint**: 哈希环容错完整——US4 Acceptance Scenarios 1+3 可验证

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: 文档、配置、容器化更新

- [ ] T042 修改 `Dockerfile`: file service EXPOSE 9003
- [ ] T043 [P] 新增 `docs/scattered-storage.md`: 分块打散存储设计文档 (上传/下载流程、数据模型、哈希路由、HTTP server)
- [ ] T044 [P] 新增 `docs/kafka.md`: Kafka MQ 设计文档 (topics、producer/consumer、降级策略、配置)
- [ ] T045 修改 `docs/architecture.md`: 更新 MQ 引用 (goroutine → Kafka)
- [ ] T046 修改 `docs/chunk-upload.md`: 更新流程图 (直传 File Service HTTP)、上传协议 (PUT binary)、面试要点
- [ ] T047 修改 `README.md`: 更新架构图、功能列表、API 路由表、MQ 节、环境变量 (KAFKA_BROKERS/FILE_HTTP_ADDR)、测试数量、changelog
- [ ] T048 修改 `AGENTS.md`: 更新架构图 (File Service HTTP :9003 + Kafka)、服务清单 (MergeChunks→CompleteUpload+GetDownloadPlan)、MQ 节 (Kafka+降级)、环境变量、测试数量、设计决策、最近更新
- [ ] T049 修改 `app/file/configs/config.yaml`: 添加 kafka 配置占位

**Checkpoint**: 所有文档与代码同步

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: 无依赖——可立即开始
- **Phase 2 (Foundation)**: 依赖 T002 (proto 定义)——BLOCKS 所有用户故事
- **Phase 3 (US1 上传)**: 依赖 Phase 2 完成
- **Phase 4 (US2 下载)**: 依赖 T012 (HTTP server)——可与 Phase 3 后半部分并行
- **Phase 5 (US3 Kafka)**: 依赖 T001——与 Phase 3/4 相互独立可并行
- **Phase 6 (US4 容错)**: 依赖 T016 (pickN)——可与 Phase 4/5 并行
- **Phase 7 (Polish)**: 依赖所有功能阶段完成

### User Story Independence

| US | 依赖 | 可并行于 |
|----|------|----------|
| US1 (P1 上传) | Phase 2 | — (第一个实施) |
| US2 (P1 下载) | US1 的 HTTP server (T012) | US3, US4 |
| US3 (P2 Kafka) | T001 (kafka-go 依赖) | US2, US4 |
| US4 (P2 容错) | T016 (pickN 方法) | US3 |

### Parallel Opportunities per Phase

**Phase 1**: T003 ∥ T004 (不同文件)
**Phase 3**: T019 ∥ T020 (不同测试文件)
**Phase 4**: T030 ∥ T031 ∥ T032 (不同测试文件)
**Phase 7**: T043 ∥ T044 (不同 docs 文件)

---

## Implementation Strategy

**MVP**: Phase 1 → Phase 2 → Phase 3 (US1) = 分块直传可用
**Increment 2**: Phase 4 (US2) = 并发下载可用，端到端完整
**Increment 3**: Phase 5 (US3) ∥ Phase 6 (US4) = Kafka + 容错
**Final**: Phase 7 = 文档同步

---

## Summary

| Phase | Tasks | 任务数 |
|-------|-------|--------|
| Phase 1: Setup | T001–T004 | 4 |
| Phase 2: Foundation | T005–T008 | 4 |
| Phase 3: US1 上传 | T009–T020 | 12 |
| Phase 4: US2 下载 | T021–T032 | 12 |
| Phase 5: US3 Kafka | T033–T037 | 5 |
| Phase 6: US4 容错 | T038–T041 | 4 |
| Phase 7: Polish | T042–T049 | 8 |

**Total**: 49 tasks (12 parallelizable)
**Per-story counts**: US1=12, US2=12, US3=5, US4=4
**Suggested MVP scope**: Phase 1 + Phase 2 + Phase 3 (US1) = 20 tasks
