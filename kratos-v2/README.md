# 轻网盘 v2

基于 Kratos v2 重构的网盘项目，用于学习和面试展示。

## 技术栈

| 组件 | 技术选型 |
|------|----------|
| 微服务框架 | Kratos v2 |
| 通信协议 | gRPC + HTTP |
| ORM | GORM |
| 缓存 | Redis |
| 消息队列 | Kafka |
| 对象存储 | 阿里云 OSS |
| 依赖注入 | Wire |
| 链路追踪 | OpenTelemetry |

## 功能特性

### 用户服务
- 用户注册/登录 (JWT认证)
- 用户信息管理
- 存储配额管理

### 文件服务
- 分块上传 (大文件支持)
- 秒传 (基于MD5去重)
- 断点续传 (Redis记录上传状态)
- 文件夹管理 (树形结构)
- 文件搜索
- 回收站 (软删除+30天自动清理)
- 文件分享 (链接+提取码+过期时间)
- 异步转存OSS (Kafka消息队列)

## 项目结构

```
kratos-v2/
├── api/                    # protobuf API定义
│   ├── user/v1/           # 用户服务API
│   └── file/v1/           # 文件服务API
├── cmd/                    # 服务入口
│   ├── user/              # 用户服务
│   └── file/              # 文件服务
├── internal/
│   ├── biz/               # 业务逻辑层 (UseCase)
│   ├── data/              # 数据访问层 (Repository)
│   ├── service/           # 服务实现层
│   ├── server/            # HTTP/gRPC服务器
│   ├── conf/              # 配置定义
│   └── mq/                # 消息队列
├── configs/               # 配置文件
├── docs/                  # 功能文档
├── third_party/           # 第三方proto
├── Makefile
└── go.mod
```

## 快速开始

### 1. 安装依赖
```bash
make init
```

### 2. 生成代码
```bash
make api    # 生成proto代码
make conf   # 生成配置代码
make wire   # 生成依赖注入代码
```

### 3. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 填入实际配置
```

### 4. 运行服务
```bash
make run-user   # 启动用户服务
make run-file   # 启动文件服务
```

## MQ 使用场景

| Topic | 用途 |
|-------|------|
| file-transfer | 本地文件异步转存OSS |
| file-thumbnail | 上传完成后生成缩略图 |
| trash-cleanup | 回收站30天过期清理 |
| share-expire | 分享链接过期清理 |

## API 文档

服务启动后访问: `http://localhost:8000/q/swagger-ui`
