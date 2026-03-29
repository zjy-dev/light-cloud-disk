# API 网关实现

## 概述

Gateway 是唯一对浏览器暴露的 HTTP 入口，但它不再代理 chunk 数据本身。现在的职责分成三类：

1. 认证与常规文件管理 API
2. UploadPlan / DownloadPlan 这样的控制面接口
3. chunk recovery 的失败兜底代理

## 关键路由

### 公开路由

- `POST /api/v1/user/register`
- `POST /api/v1/user/login`
- `GET /api/v1/share/:share_id`

### 保护路由

- `POST /api/v1/file/check-upload`
- `POST /api/v1/file/complete-upload`
- `GET /api/v1/file/download-plan/:file_id`
- `GET /api/v1/file/chunks/:md5/:index/recovery`
- 其它用户信息、文件管理、回收站、分享接口

## Gateway 在上传链路中的作用

Gateway 不再接收 chunk 二进制。

它只做两件事：

1. 调用 file-service 的 `CheckUpload`
2. 基于一致性哈希环补全 UploadPlan

生成 UploadPlan 时，Gateway 会：

- 使用 `pickN(fileMD5, totalChunks)` 选择多个 file-service HTTP 地址
- 为每个 chunk 生成 `uploadUrl`
- 把 UploadPlan 返回给浏览器，后续 chunk 直连 file-service HTTP :9003

## Gateway 在下载链路中的作用

`GetDownloadPlan` 的 gRPC 返回值只有主下载地址。Gateway 在 HTTP 响应层额外补充 `backupUrls`：

- 主地址：原始 chunk 所在实例的直连地址
- 备份地址：`/api/v1/file/chunks/:md5/:index/recovery?file_size=...`

前端优先请求主地址，失败后再请求 `backupUrls`。这样正常下载不走网关，只有恢复路径才会经过 Gateway。

## Recovery 代理

`RecoverChunk` handler 的逻辑是：

1. 从 Consul 哈希环选取健康 file-service HTTP 地址
2. 代理请求到 `GET /api/v1/chunks/recover/:md5/:index`
3. 如果某个实例请求失败，立即把它标记为不健康
4. 继续尝试下一个健康实例
5. 成功后把恢复出的 chunk 字节流直接回给浏览器

这层代理只负责故障恢复，不参与正常 chunk 数据传输。

## 一致性哈希路由

Gateway 内部维护 file-service 哈希环：

- 虚拟节点：150
- 冷却时间：15 秒
- Consul 刷新周期：15 秒
- 上传分配：`pickN`
- 控制面 gRPC 路由：`pick`

因此 Gateway 既能把同一个文件的控制请求路由到稳定实例，也能为打散上传生成多实例计划。

## JWT 认证

- Gateway 负责校验登录态并把 `user_id` 写入 Gin Context
- file-service HTTP chunk 端点也会二次校验 Bearer Token
- 因此浏览器既能访问 Gateway API，也能安全直连 file-service HTTP

## 测试重点

当前 handler 测试覆盖了：

- CheckUpload / CompleteUpload / GetDownloadPlan 路径
- presigned upload 代理
- recovery backup URL 的补全逻辑
- JWT / CORS / Logger 中间件

## 面试要点

1. 为什么 Gateway 不再代理上传：否则多实例扩容也无法摆脱单入口带宽瓶颈。
2. 为什么 recovery 放在 Gateway：它拥有健康实例视角，适合在失败时选择一个仍然可用的 file-service 去重建 chunk。
3. 为什么 HTTP 响应而不是 proto 里补 backupUrls：前端对接的是 HTTP API，Gateway 可以在不改 gRPC 合同的情况下补充恢复语义。
