# 分块打散存储设计

## 概述

分块打散存储是 light-cloud-disk 的核心存储策略：文件在上传时被分成多个分块（chunk），每个分块直接上传到不同的 file-service 实例，绕过 API Gateway 瓶颈。下载时客户端从多个实例并行获取分块并在本地重组。

## 架构

```
                              ┌──────────────────┐
                              │   API Gateway     │
                              │ 1. CheckUpload    │
                              │ 2. 返回 UploadPlan │
                              └───────┬──────────┘
                                      │ (upload plan)
                                      ▼
    ┌──────────────────────────────────────────────────────┐
    │                    客户端 (浏览器)                      │
    │  根据 UploadPlan 直接 PUT 分块到各 file-service 实例    │
    └───┬──────────┬──────────┬──────────┬─────────────────┘
        │          │          │          │
        ▼          ▼          ▼          ▼
   ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
   │ File   │ │ File   │ │ File   │ │ File   │
   │Svc #1  │ │Svc #2  │ │Svc #3  │ │Svc #1  │
   │chunk 0 │ │chunk 1 │ │chunk 2 │ │chunk 3 │
   └────────┘ └────────┘ └────────┘ └────────┘
```

## 上传流程

### 1. CheckUpload (Gateway → File Service)

客户端先发送文件元信息（MD5、大小、文件名）到 Gateway：

- Gateway 通过一致性哈希选择 file-service 实例处理 CheckUpload RPC
- 如果文件已存在（秒传），直接返回 `canFastUpload: true`
- 否则返回 `uploadMode: "direct"` 和一个 **UploadPlan**

### 2. UploadPlan 生成

Gateway 使用 `pickN(fileMD5, instanceCount)` 从哈希环选择多个 file-service 实例，生成分块分配方案：

```json
{
  "totalChunks": 4,
  "chunkSize": 5242880,
  "assignments": [
    {"chunkIndex": 0, "targetAddr": "10.0.0.1:9003", "uploadUrl": "http://10.0.0.1:9003/api/v1/chunks/abc123/0"},
    {"chunkIndex": 1, "targetAddr": "10.0.0.2:9003", "uploadUrl": "http://10.0.0.2:9003/api/v1/chunks/abc123/1"},
    {"chunkIndex": 2, "targetAddr": "10.0.0.3:9003", "uploadUrl": "http://10.0.0.3:9003/api/v1/chunks/abc123/2"},
    {"chunkIndex": 3, "targetAddr": "10.0.0.1:9003", "uploadUrl": "http://10.0.0.1:9003/api/v1/chunks/abc123/3"}
  ]
}
```

### 3. 直接上传 (Client → File Service HTTP)

客户端按照 UploadPlan，直接通过 HTTP PUT 上传各分块到对应的 file-service 实例：

- 每个 file-service 实例暴露 HTTP 端口 (默认 9003)
- PUT `/api/v1/chunks/:md5/:index` — 上传分块
- JWT Bearer Token 认证
- 最大并发上传数：4

### 4. CompleteUpload (Gateway → File Service)

所有分块上传完成后，客户端通过 Gateway 调用 CompleteUpload：

- 验证所有分块记录完整
- 创建文件记录
- 更新用户存储用量

## 下载流程

### 1. GetDownloadPlan (Gateway → File Service)

客户端请求下载时，Gateway 转发 GetDownloadPlan RPC：

- 查询 `chunk_records` 表获取所有分块位置
- 返回 DownloadPlan，包含每个分块的下载 URL

### 2. 并行下载 (Client ← File Service HTTP)

- 单分块文件：直接打开下载 URL
- 多分块文件：浏览器端并行下载所有分块，在内存中重组，然后触发下载
- GET `/api/v1/chunks/:md5/:index` — 下载分块
- 最大并发下载数：4

## 数据模型

### chunk_records 表

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键 |
| file_md5 | string | 文件 MD5 |
| chunk_index | int32 | 分块序号 |
| node_addr | string | 存储该分块的实例地址 |
| store_path | string | 本地磁盘存储路径 |
| size | int64 | 分块大小 |
| created_at | time | 创建时间 |

### file_stores 表新增字段

| 字段 | 类型 | 说明 |
|------|------|------|
| total_chunks | int32 | 文件总分块数 |
| storage_type | string | "scattered" 表示分块打散存储 |

## 一致性哈希路由

- **算法**: FNV-32a
- **虚拟节点**: 150 个/物理节点
- **刷新周期**: 15 秒 (Consul Health API)
- **容错**: 不健康实例临时剔除 30 秒，过期后自动重试

### pickN 算法

从哈希环上 key 的位置开始顺时针遍历，收集 N 个不同物理节点的 HTTP 地址，跳过被标记为不健康的实例。

## File Service HTTP Server

每个 file-service 实例除 gRPC 端口外，额外暴露一个 HTTP 端口用于分块直传/直下：

```
PUT  /api/v1/chunks/:md5/:index  — 上传分块
GET  /api/v1/chunks/:md5/:index  — 下载分块
```

- 使用 Gin 框架
- JWT Bearer Token 认证（与 Gateway 共享密钥）
- 实现 Kratos `transport.Server` 接口，纳入服务生命周期管理
- HTTP 端口通过 Consul Metadata `http_port` 注册，供 Gateway 发现
