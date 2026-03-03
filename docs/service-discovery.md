# 服务发现与通信

## 概述

本项目使用 Consul 作为服务注册与发现中心，所有服务间的 gRPC 通信通过 Consul 进行地址解析。

## 服务注册

### 注册流程

```
服务启动 → 连接 Consul → 注册服务名+地址+端口 → 定期心跳 → 服务停止时注销
```

### Kratos 服务注册 (User/File Service)

Kratos 通过 `kratos.Registrar()` 选项自动完成服务注册和注销：

```go
// app/user/cmd/main.go
import (
    "github.com/go-kratos/kratos/contrib/registry/consul/v2"
    consulapi "github.com/hashicorp/consul/api"
)

func main() {
    // 1. 创建 Consul 客户端
    consulCli, _ := consulapi.NewClient(&consulapi.Config{
        Address: os.Getenv("CONSUL_ADDR"), // e.g. "localhost:8500"
    })
    // 2. 创建 Kratos 注册器
    r := consul.New(consulCli)
    // 3. 启动 Kratos app，自动注册
    app := kratos.New(
        kratos.Name("user-service"),
        kratos.Server(grpcSrv),
        kratos.Registrar(r),
    )
    app.Run() // 注册到 Consul
    // app.Stop() 时自动注销
}
```

### 注册信息

| 服务 | Consul 名称 | 端口 | 协议 |
|------|-------------|------|------|
| 用户服务 | user-service | 9001 | gRPC |
| 文件服务 | file-service | 9002 | gRPC |

## 服务发现

### 发现流程

```
客户端需要调用目标服务 → 通过 Consul 查找服务实例 → 获取地址列表 → 选择一个实例 → 建立 gRPC 连接
```

### Gateway 发现后端服务

```go
// app/gateway/internal/client/client.go
import (
    kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
    "github.com/go-kratos/kratos/contrib/registry/consul/v2"
)

func NewServiceClients() *ServiceClients {
    consulCli, _ := consulapi.NewClient(&consulapi.Config{
        Address: os.Getenv("CONSUL_ADDR"),
    })
    r := consul.New(consulCli)

    // 使用 discovery:/// 协议前缀
    userConn, _ := kratosgrpc.DialInsecure(ctx,
        kratosgrpc.WithEndpoint("discovery:///user-service"),
        kratosgrpc.WithDiscovery(r),
    )
    userClient := userv1.NewUserServiceClient(userConn)
    // ...
}
```

### File Service 发现 User Service

File Service 通过相同模式发现 User Service，调用 `UpdateStorageUsed` RPC：

```go
// app/file/cmd/main.go
// 发现 user-service 获取 gRPC 连接
userConn, _ := kratosgrpc.DialInsecure(ctx,
    kratosgrpc.WithEndpoint("discovery:///user-service"),
    kratosgrpc.WithDiscovery(r),
)
userServiceClient := userv1.NewUserServiceClient(userConn)
// 传入 File Service 的 Wire 依赖注入
```

## 通信拓扑

```
                    Consul (注册中心)
                   ╱       │       ╲
              注册╱        │注册     ╲注册
              ╱            │          ╲
    User Service    File Service    Gateway
    (user-service)  (file-service)
         ▲               │              │
         │    gRPC       │              │
         └───(发现)───────┘              │
         ▲                              │
         │          gRPC                │
         └──────────(发现)───────────────┘
                                        │
                                  gRPC  │
         File Service ◄─────(发现)──────┘
```

### 调用关系

| 调用方 | 被调用方 | 方法 | 场景 |
|--------|----------|------|------|
| Gateway | User Service | Register, Login, GetUserInfo, UpdateUserInfo | 所有用户操作 |
| Gateway | File Service | 所有文件操作 RPC | 所有文件操作 |
| File Service | User Service | UpdateStorageUsed | 文件合并完成后更新存储用量 |

## 容错处理

### 服务不可用

File Service 调用 User Service 更新存储用量时，如果 User Service 不可用，仅打印警告日志，不影响文件合并操作：

```go
// app/file/internal/biz/file.go
func (uc *FileUsecase) MergeChunks(...) error {
    // ... 合并文件逻辑
    
    // 更新存储用量 - 容错处理
    if err := uc.userClient.UpdateStorageUsed(ctx, userID, fileSize); err != nil {
        uc.log.Warnf("failed to update storage used for user %d: %v", userID, err)
        // 不 return error，文件合并已成功
    }
    return nil
}
```

### 健康检查

- Consul 定期检查服务健康状态
- 不健康的实例自动从服务列表中移除
- Gateway 通过 Consul 只连接健康实例

## 配置

所有服务通过环境变量 `CONSUL_ADDR` 配置 Consul 地址：

```yaml
# 本地开发
CONSUL_ADDR=localhost:8500

# Docker Compose 内
CONSUL_ADDR=consul:8500
```

## 面试要点

1. **为什么用 Consul 而不是 Eureka/Nacos？** — Consul 支持多数据中心，Go 生态原生支持，Kratos 有官方 contrib
2. **服务发现 vs 硬编码地址？** — 动态扩缩容、故障自动摘除、无需修改配置
3. **`discovery:///` 协议是什么？** — Kratos gRPC 客户端的自定义 resolver，触发 Consul 查询解析服务地址
4. **如果 Consul 挂了？** — 已建立的连接不受影响，新连接无法解析；生产环境 Consul 应部署集群 (3-5 节点)
5. **跨服务调用失败怎么办？** — 关键路径返回错误，非关键路径 (如更新统计) 降级处理仅告警
