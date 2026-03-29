# 服务发现与通信

## 概述

系统所有内部服务发现都通过 Consul 完成。除了常规 gRPC 地址，file-service 还会把 HTTP 端口作为元数据注册到 Consul，供 Gateway 生成 UploadPlan 和 recovery 代理使用。

## 注册内容

| 服务 | Consul 名称 | 主端口 | 元数据 |
|------|-------------|--------|--------|
| User Service | `user-service` | 9001/gRPC | 无 |
| File Service | `file-service` | 9002/gRPC | `http_port=9003` |

这意味着 Gateway 拿到的不只是 gRPC 连接地址，还能把 gRPC 地址映射到相应的 file-service HTTP 地址。

## 发现链路

### Gateway

Gateway 启动时会：

1. 用 `discovery:///user-service` 建立 User Service gRPC 客户端
2. 用 `discovery:///file-service` 建立 File Service gRPC 客户端
3. 额外维护一个 Consul 哈希环，用于：
   - `pick(key)`: 控制面 gRPC 路由
   - `pickN(key, n)`: 生成 UploadPlan 的多个 HTTP 地址
   - `MarkUnhealthy(addr)`: 对失败实例执行 15 秒冷却剔除

### File Service

File Service 通过相同的 `discovery:///user-service` 机制发现 User Service，调用 `UpdateStorageUsed`。

## 当前通信拓扑

```
Gateway ──gRPC──▶ user-service
Gateway ──gRPC──▶ file-service
File Service ──gRPC──▶ user-service

Gateway ──HTTP address metadata──▶ file-service HTTP :9003
Client  ──HTTP direct upload/download──▶ file-service HTTP :9003
```

## 为什么要把 HTTP 端口也放进 Consul

file-service 的主业务是“gRPC 控制面 + HTTP 数据面”。如果只注册 gRPC 端口，Gateway 无法生成 chunk 直传地址，也无法在 recovery 代理时选择一个健康实例去重建 chunk。

## 健康状态与冷却

Gateway 的哈希环除了依赖 Consul 健康检查，还会对运行时失败做本地冷却：

- 冷却时长：15 秒
- 触发时机：直连请求或 recovery 代理命中失败实例
- 恢复方式：冷却过期后自动重试；Consul 刷新时也会清理过期状态

这样既能响应瞬时故障，也不会永久拉黑实例。

## 面试要点

1. 为什么不只靠 gRPC discovery：上传和下载的数据面是 HTTP，必须把 HTTP 地址也纳入服务发现结果。
2. 为什么还要本地 unhealthy 冷却：Consul 的健康状态刷新有间隔，运行时失败需要更快的本地避让。
3. 为什么 pick 和 pickN 都要有：前者保证控制请求稳定命中，后者负责为 chunk 生成多实例上传计划。
