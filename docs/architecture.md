# 微服务架构设计

## 概述

本项目采用微服务架构，将云盘系统拆分为三个独立部署的服务：User Service、File Service 和 API Gateway。服务间通过 gRPC 通信，使用 Consul 进行服务发现。

## 架构图

```
    Client (浏览器/APP)
         │ HTTP
         ▼
┌─────────────────────────────────┐
│       Gin API Gateway           │
│     (HTTP :8080)                │
│     - JWT 认证                  │
│     - CORS 跨域                │
│     - MD5 一致性哈希路由         │
│     - HTTP→gRPC 协议转换        │
└────────┬───────────────┬────────┘
    gRPC │               │ gRPC
    (Consul 发现)        (Consul 发现 + hash ring)
         ▼               ▼
┌────────────────┐ ┌────────────────┐
│  User Service  │ │  File Service  │
│  (gRPC :9001)  │◄│  (gRPC :9002)  │
│  Kratos v2     │ │  Kratos v2     │
│                │ │                │
│  - 用户注册/登录│ │  - 分块上传    │
│  - 信息管理     │ │  - 秒传/续传   │
│  - 存储配额     │ │  - 文件管理    │
│                │ │  - 纠删码编码   │
│                │ │  - 回收站/分享  │
└───────┬────────┘ └──┬────┬───┬───┘
        │             │    │   │
        ▼             ▼    ▼   ▼
    ┌────────┐   ┌──────┐ ┌─────┐ ┌───────────┐
    │ MySQL  │   │MySQL │ │Redis│ │ 本地磁盘   │
    │/SQLite │   │/SQLite│ │     │ │ (EC shards)│
    └────────┘   └──────┘ └─────┘ └───────────┘

    ← ─ ─ Consul 服务注册与发现 ─ ─ →

    File Service ──→ goroutine MQ ──→ 阿里云 OSS (冷存)
    (LRU 淘汰触发)

    分布式锁: Redis SETNX + Lua 脚本
    纠删码:   Reed-Solomon 4+2 (本地容错)
```

## 分层架构 (Clean Architecture)

每个 Kratos 服务内部采用四层架构：

```
┌─────────────────────────────────────────┐
│            Service Layer                │
│   - 接收 gRPC 请求                      │
│   - 参数校验和转换                      │
│   - 调用 Biz 层                         │
│   - 组装 proto Reply                    │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Biz Layer                  │
│   - 核心业务逻辑                        │
│   - 定义 Repo 接口 (依赖倒置)           │
│   - 定义 Client 接口 (跨服务调用)       │
│   - 不依赖任何具体实现                  │
└─────────────────────────────────────────┘
                    ↓
┌─────────────────────────────────────────┐
│              Data Layer                 │
│   - 实现 Repo 接口 (GORM/Redis)        │
│   - 实现 Client 接口 (gRPC)            │
│   - 数据库模型定义                      │
└─────────────────────────────────────────┘
```

### 依赖倒置示例

File Service 需要调用 User Service 更新存储用量。在 Biz 层定义接口，Data 层通过 gRPC 实现：

```go
// biz/file.go - interface definition
type UserClient interface {
    UpdateStorageUsed(ctx context.Context, userID int64, sizeDelta int64) error
}

// data/user_client.go - gRPC implementation
type userClientImpl struct {
    client userv1.UserServiceClient // gRPC 客户端
}

func (c *userClientImpl) UpdateStorageUsed(ctx context.Context, userID, sizeDelta int64) error {
    _, err := c.client.UpdateStorageUsed(ctx, &userv1.UpdateStorageUsedRequest{
        UserId:    userID,
        SizeDelta: sizeDelta,
    })
    return err
}
```

## 关键设计决策

### 1. gRPC-only 后端服务

User/File Service 只暴露 gRPC 端口，不暴露 HTTP。

**原因**:
- HTTP 统一由 Gateway 处理，避免多入口
- gRPC 性能更优，适合服务间通信
- Proto 定义即接口文档，强类型约束

### 2. Gin 作为 Gateway

选择 Gin 而非 Kratos HTTP 作为 API 网关。

**原因**:
- 网关职责是路由/中间件/代理，不需要 Kratos 的 Service/Biz/Data 分层
- Gin 生态丰富，中间件成熟
- Gateway 不处理业务逻辑，无需 Wire 依赖注入

### 3. Consul 服务发现

使用 Consul 作为注册中心，而非硬编码地址。

**原因**:
- 服务实例动态增减时自动发现
- 支持健康检查
- Kratos 原生支持 (`kratos.Registrar()`)

### 4. 跨服务调用使用接口隔离

File Service 的 `biz.UserClient` 是接口，由 `data.userClientImpl` 通过 gRPC 实现。

**原因**:
- Biz 层不依赖网络细节
- 单元测试可 Mock UserClient
- 未来可替换通信方式 (如 HTTP/MQ)

### 5. Wire 依赖注入

每个 Kratos 服务使用独立的 `wire.go` 配置依赖注入。

**原因**:
- 编译时检查依赖关系
- 自动生成注入代码
- 避免手动构造复杂依赖树

## 面试要点

1. **为什么拆分微服务？** — 用户服务和文件服务有不同的扩展需求，文件服务 I/O 密集需要更多实例
2. **为什么不用 HTTP 网关直接调 HTTP 后端？** — gRPC 比 HTTP/JSON 性能高 3-5 倍，强类型，双向流
3. **服务发现解决什么问题？** — 服务实例动态变化时无需修改配置，负载均衡，故障摘除
4. **Clean Architecture 的好处？** — 业务逻辑与基础设施解耦，可测试性强，技术选型可替换
5. **Gateway 模式的优势？** — 统一入口，集中认证/限流，前端只需对接一个地址
