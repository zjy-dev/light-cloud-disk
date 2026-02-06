# 轻网盘 Light Cloud Disk

[![CI](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/ci.yml)
[![Release](https://github.com/zjy-dev/light-cloud-disk/actions/workflows/release.yml/badge.svg)](https://github.com/zjy-dev/light-cloud-disk/releases)

基于 Kratos v2 的云存储服务，用于学习微服务架构和面试展示。

## 技术栈

| 组件 | 技术选型 | 版本 |
|------|----------|------|
| 微服务框架 | Kratos | v2.9.2 |
| 通信协议 | gRPC + HTTP | - |
| ORM | GORM | v1.25.12 |
| 缓存 | Redis | v8.11.5 |
| 消息队列 | Kafka | v0.4.47 |
| 对象存储 | 阿里云 OSS | - |
| 依赖注入 | Wire | v0.6.0 |
| 认证 | JWT | - |

## 功能特性

### 用户服务
- [x] 用户注册/登录 (JWT认证)
- [x] 用户信息管理
- [x] 存储配额管理 (默认10GB)

### 文件服务
- [x] 分块上传 (大文件支持，5MB/块)
- [x] 秒传 (基于MD5去重)
- [x] 断点续传 (Redis记录上传状态)
- [x] 文件夹管理 (树形结构)
- [x] 文件搜索 (模糊匹配)
- [x] 回收站 (软删除+30天自动清理)
- [x] 文件分享 (链接+提取码+过期时间)
- [x] 异步转存OSS (Kafka消息队列)

## 项目结构

```
.
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

### 1. 环境准备
- Go 1.22+
- MySQL 8.0+
- Redis 6.0+
- Kafka (可选)

### 2. 安装工具
```bash
make init
```

### 3. 生成代码
```bash
make api    # 生成proto代码
make conf   # 生成配置代码
make wire   # 生成依赖注入代码
```

### 4. 配置环境变量
```bash
cp .env.example .env
# 编辑 .env 填入实际配置
```

### 5. 运行服务
```bash
make run-user   # 启动用户服务 (HTTP:8000, gRPC:9000)
make run-file   # 启动文件服务
```

### 6. 运行测试
```bash
make test
```

## API 接口

### 用户服务
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/user/register | 用户注册 |
| POST | /api/v1/user/login | 用户登录 |
| GET | /api/v1/user/info | 获取用户信息 |
| PUT | /api/v1/user/info | 更新用户信息 |

### 文件服务
| 方法 | 路径 | 说明 |
|------|------|------|
| GET | /api/v1/file/upload/check | 检查秒传/断点续传 |
| POST | /api/v1/file/upload/chunk | 上传文件分块 |
| POST | /api/v1/file/upload/merge | 合并分块 |
| GET | /api/v1/files | 获取文件列表 |
| GET | /api/v1/file/download | 获取下载链接 |
| DELETE | /api/v1/file | 删除文件 |
| PUT | /api/v1/file/rename | 重命名 |
| POST | /api/v1/folder | 创建文件夹 |
| PUT | /api/v1/file/move | 移动文件 |
| GET | /api/v1/trash | 回收站列表 |
| POST | /api/v1/trash/restore | 恢复文件 |
| DELETE | /api/v1/trash | 彻底删除 |
| POST | /api/v1/share | 创建分享 |
| GET | /api/v1/share/{id} | 获取分享 |
| GET | /api/v1/files/search | 搜索文件 |

## 消息队列场景

| Topic | 生产者 | 消费者 | 用途 |
|-------|--------|--------|------|
| file-transfer | FileService | TransferWorker | 文件异步转存OSS |
| file-thumbnail | FileService | ThumbnailWorker | 生成文件缩略图 |
| trash-cleanup | CronJob | CleanupWorker | 回收站过期清理 |
| share-expire | CronJob | ShareWorker | 分享链接过期处理 |

## 文档

- [分块上传实现](docs/chunk-upload.md)
- [消息队列使用场景](docs/message-queue.md)
- [CI/CD 配置](docs/ci-cd.md)
- [容器化部署](docs/containerization.md)

## 更新日志

### v2.0.0 (2025)
- 使用 Kratos v2 重构
- 新增文件夹管理
- 新增回收站功能
- 新增文件分享
- 新增文件搜索
- MQ 从 RabbitMQ 迁移到 Kafka
- 添加存储配额管理

### v1.0.0 (2020)
- 基于 go-micro 的初始版本
- 基础文件上传/下载功能
