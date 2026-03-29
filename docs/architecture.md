# 微服务架构设计

## 概述

当前系统由三个长期运行的服务组成：

- User Service：用户与配额
- File Service：打散上传、分块下载、冷迁移、恢复重建
- API Gateway：HTTP 入口、鉴权、服务发现、上传计划和恢复代理

主路径的设计目标很明确：上传和下载的数据面尽量绕过网关，网关只承担控制面和失败兜底。

## 当前拓扑

```
Client
  │
  ├── CheckUpload / CompleteUpload / GetDownloadPlan ─▶ Gateway :8080
  │                                                      │
  │                                                      ├── gRPC ─▶ User Service :9001
  │                                                      └── gRPC ─▶ File Service :9002
  │
  ├── PUT /api/v1/chunks/:md5/:index ──────────────────▶ File Service HTTP :9003 ×N
  ├── GET /api/v1/chunks/:md5/:index ──────────────────▶ File Service HTTP :9003 ×N
  └── GET /api/v1/file/chunks/:md5/:index/recovery ───▶ Gateway recovery proxy
                                                           │
                                                           └── 健康实例从 OSS 恢复分片重建 chunk

File Service ──▶ Kafka ──▶ OSS
          └────▶ Redis / MySQL(or SQLite)
```

## 关键数据流

### 1. 打散上传

1. 客户端请求 `CheckUpload`。
2. Gateway 基于一致性哈希生成 UploadPlan。
3. 浏览器并发直传 chunk 到多个 file-service HTTP 实例。
4. 每个 chunk 写本地磁盘、登记 `chunk_records`，并生成恢复分片到 OSS。
5. `CompleteUpload` 只校验元数据完整性并创建 FileStore/File，不做合并。

### 2. 并发下载

1. 客户端请求 `GetDownloadPlan`。
2. Gateway 返回每块的主下载地址和 recovery 备用地址。
3. 浏览器优先直连主 chunk 地址，失败后再请求 recovery URL。
4. recovery 请求由健康实例从 OSS 恢复分片重建 chunk 并返回。

### 3. 冷热迁移

1. `maybeEvictToCloud()` 在本地热存超过阈值时选取 LRU 候选。
2. 迁移任务写入 Kafka。
3. 消费端根据 `storage_type` 处理 local 或 scattered 对象。
4. 迁移完成后更新 `file_stores` / `chunk_records`，并修正 Redis 用量计数器。

## Clean Architecture

Kratos 服务内部仍保持 Service → Biz → Data 分层：

- Service：gRPC handler 和参数映射
- Biz：上传、下载、恢复、迁移等核心规则
- Data：GORM、Redis、OSS、Kafka、gRPC 客户端的具体实现

Gateway 不走 Kratos 四层，原因是它只做路由、中间件和协议转换，不承载业务状态。

## 设计取舍

### 为什么 File Service 同时暴露 gRPC 和 HTTP

- gRPC 给内部控制面调用：CheckUpload、CompleteUpload、GetDownloadPlan、文件管理
- HTTP 给浏览器数据面直传和直下 chunk
- 这样既保留了强类型 RPC，又避免浏览器被 gRPC 流式协议绑定

### 为什么恢复链路走 OSS 而不是副本表

- 主下载路径仍走实例直连，带宽效率最高
- 故障恢复时只要任意一个健康实例还在，就可以从 OSS 恢复分片无状态重建 chunk
- 不需要维护跨实例 chunk 副本一致性

### 为什么网关还保留 recovery 代理

- 失败才走 recovery，平时不消耗网关带宽
- Gateway 已经持有 Consul 视角下的健康实例列表，适合挑选健康实例代理恢复请求

## 面试要点

1. 为什么数据面绕过网关：上传和下载吞吐受网关带宽限制太明显，控制面与数据面必须拆开。
2. 为什么还要 chunk_records：没有 chunk 级元数据，就无法做并发下载、实例切换和 chunk 迁移。
3. 为什么恢复分片写 OSS：实例级故障后，本地磁盘不可达，恢复材料必须放在跨实例可用的位置。
