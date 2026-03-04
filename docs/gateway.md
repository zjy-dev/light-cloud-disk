# API 网关实现

## 概述

API Gateway 是系统的唯一 HTTP 入口，负责请求路由、JWT 认证、CORS 处理，以及 HTTP 到 gRPC 的协议转换。基于 Gin 框架实现。

## 架构

```
                         HTTP :8080
                             │
┌────────────────────────────┼────────────────────────────┐
│                     Gin Router                          │
│                                                         │
│  ┌──────────────┐  ┌─────────────┐  ┌──────────────┐  │
│  │  Logger MW   │→│  CORS MW    │→│  Recovery MW │  │
│  └──────────────┘  └─────────────┘  └──────────────┘  │
│                                                         │
│  /api/v1 (公开)              /api/v1 (保护)             │
│  ├── POST /user/register     ┌─────────────┐            │
│  ├── POST /user/login        │  JWT Auth   │            │
│  └── GET  /share/:id         └──────┬──────┘            │
│                              ├── GET  /user/info         │
│                              ├── PUT  /user/info         │
│                              ├── POST /file/check-upload │
│                              ├── ...其他文件操作           │
│                              └── POST /share             │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Handler Layer                       │   │
│  │  UserHandler ──gRPC──▶ user-service              │   │
│  │  FileHandler ──gRPC──▶ file-service              │   │
│  └─────────────────────────────────────────────────┘   │
│                                                         │
│  ┌─────────────────────────────────────────────────┐   │
│  │              Client Layer (Consul)               │   │
│  │  discovery:///user-service → gRPC conn           │   │
│  │  discovery:///file-service → gRPC conn           │   │
│  └─────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## 目录结构

```
app/gateway/
├── cmd/
│   └── main.go              # Gin 路由定义、启动
├── internal/
│   ├── client/
│   │   └── client.go        # Consul gRPC 客户端构造
│   ├── handler/
│   │   ├── user.go          # 用户 HTTP 处理器
│   │   ├── file.go          # 文件 HTTP 处理器
│   │   └── handler_test.go  # 13 个单元测试
│   └── middleware/
│       ├── jwt.go           # JWT 认证中间件
│       ├── cors.go          # CORS + Logger 中间件
│       └── middleware_test.go # 8 个单元测试
└── configs/
    └── config.yaml
```

## JWT 认证中间件

### 工作流程

```
请求 → 检查 Authorization Header → 解析 Bearer Token → 验证签名 → 提取 user_id → 写入 Context
```

### 关键实现

```go
func JWTAuth() gin.HandlerFunc {
    secret := os.Getenv("JWT_SECRET")
    return func(c *gin.Context) {
        // 1. Extract token from Authorization header
        authHeader := c.GetHeader("Authorization")
        parts := strings.SplitN(authHeader, " ", 2)
        // 2. Validate Bearer prefix
        // 3. Parse and verify JWT
        token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
            return []byte(secret), nil
        })
        // 4. Read user_id from claims
        claims := token.Claims.(jwt.MapClaims)
        userID := int64(claims["user_id"].(float64))
        // 5. Store user_id in Gin context for downstream handlers
        c.Set("user_id", userID)
    }
}
```

### 路由分组

- **公开路由** (无需认证): 注册、登录、获取分享
- **保护路由** (需 JWT): 所有用户信息和文件操作

## HTTP → gRPC 协议转换

Handler 层负责将 HTTP 请求转换为 gRPC 调用：

```go
func (h *UserHandler) Register(c *gin.Context) {
    // 1. Bind HTTP JSON body to proto request
    var req userv1.RegisterRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    // 2. Call gRPC method
    reply, err := h.clients.User.Register(c.Request.Context(), &req)
    // 3. Return proto reply as JSON
    c.JSON(http.StatusOK, reply)
}
```

上传接口 `POST /api/v1/file/upload-chunk` 是一个特例：Gateway 使用 `multipart/form-data` 解析分块文件（二进制），再组装成 `UploadChunkRequest` 转发到 File Service。

对于需要认证的路由，Handler 从 Gin Context 获取 `user_id` 并注入请求：

```go
func (h *FileHandler) ListFiles(c *gin.Context) {
    userID := c.GetInt64("user_id")  // JWT 中间件写入
    // ...
    reply, err := h.clients.File.ListFiles(ctx, &filev1.ListFilesRequest{
        UserId: userID,
        // ...
    })
}
```

## CORS 中间件

允许跨域请求，支持：
- 所有源 (`*`)
- 常用 HTTP 方法 (GET, POST, PUT, DELETE, OPTIONS)
- Authorization 和 Content-Type 头
- 预检请求 (OPTIONS) 直接返回 204

## 测试

### Handler 测试 (13 个)

使用 Mock gRPC 客户端测试 HTTP 处理逻辑：

```go
userClient := &mockUserClient{
    registerFn: func(ctx context.Context, in *userv1.RegisterRequest, ...) (*userv1.RegisterReply, error) {
        return &userv1.RegisterReply{UserId: 1, Username: in.Username}, nil
    },
}
h := NewUserHandler(newTestClients(userClient, &mockFileClient{}))
// Build HTTP request and verify response
```

### Middleware 测试 (8 个)

覆盖 JWT 的各种边界情况：
- 有效 token
- 缺少 Authorization header
- 格式错误 (非 Bearer)
- Token 过期
- 错误密钥
- 缺少 user_id claim

## 面试要点

1. **为什么用 Gateway 模式？** — 统一入口，集中认证，前端只需一个 baseURL
2. **为什么不在每个微服务里做认证？** — 避免重复代码，认证逻辑集中维护
3. **JWT vs Session？** — JWT 无状态，适合微服务；Session 需要共享存储
4. **proto 直接作为 HTTP 响应？** — Gin 的 `c.JSON()` 使用 `encoding/json` 序列化 proto struct，字段名为 proto 的 `json` tag (snake_case)
5. **Consul 客户端连接复用？** — 启动时建立连接，Consul 负责地址解析和负载均衡
