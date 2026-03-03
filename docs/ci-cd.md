# CI/CD 配置

## GitHub Actions

本项目使用 GitHub Actions 进行持续集成和发布。

### CI 工作流 (ci.yml)

**触发条件**:
- Push 到 `main` 分支
- 向 `main` 分支发起 PR

**执行步骤**:
1. **Test Job**
   - 检出代码
   - 通过 `go-version-file: go.mod` 安装与项目一致的 Go 版本
   - 设置 `GOTOOLCHAIN=local`，禁止自动下载 toolchain 模块（避免 `go: no such tool "covdata"`）
   - 下载依赖
   - 运行单元测试 (`go test -v -race -coverprofile=coverage.out ./...`)
   - 上传覆盖率报告到 Codecov

2. **Build Job** (依赖 Test 通过)
   - 编译 2 个后端服务的 Linux AMD64 二进制文件:
     - `user-service`
     - `file-service`
   - 上传构建产物

3. **Compose Smoke Job** (依赖后端/前端测试通过)
    - `docker compose up -d --build` 启动全栈容器
    - 校验 Gateway 健康检查 (`/health`)
    - 校验 Frontend 可访问 (`http://localhost:3000/`)
    - 校验 gRPC 端口连通 (`9001/9002`)
    - 无论成功失败都执行 `docker compose down -v`

4. **Docker Images Job**
    - 后端与前端镜像在 Compose Smoke 通过后构建/推送
    - `latest` 标签仅在 `main` 分支 push 时发布

### Release 工作流 (release.yml)

**触发条件**: 推送以 `v` 开头的 tag (如 `v3.0.0`)

**执行步骤**:
1. 检出代码
2. 通过 `go-version-file: go.mod` 安装与项目一致的 Go 版本
3. 运行测试
4. 编译多架构二进制文件:
   - `user-service-linux-amd64` / `arm64`
   - `file-service-linux-amd64` / `arm64`
5. 生成 SHA256 校验和
6. 创建 GitHub Release 并上传文件

## 构建命令

```bash
# 编译所有服务
make build-user     # → bin/user-service
make build-file     # → bin/file-service
make build-gateway  # → bin/gateway

# 或手动编译
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/user-service ./app/user/cmd
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/file-service ./app/file/cmd
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/gateway ./app/gateway/cmd
```

## 容器镜像发布

```bash
# 构建并推送到 GHCR
make image-user VERSION=v3.0.0
make image-file VERSION=v3.0.0
make image-gateway VERSION=v3.0.0
```

## 发布流程

```bash
# 1. 确保所有测试通过
go test ./...

# 2. 创建并推送 tag
git tag v3.0.0
git push origin v3.0.0

# 3. GitHub Actions 自动创建 Release
```

## 面试要点

1. **为什么用 GitHub Actions？** — 与 GitHub 深度集成，配置简单，免费额度充足
2. **为什么 CGO_ENABLED=0？** — 生成静态链接二进制，便于容器化部署 (Alpine 没有 glibc)
3. **为什么同时构建 AMD64 和 ARM64？** — 支持 x86 服务器和 ARM 云服务器 (如 AWS Graviton)
4. **三个服务的 CI 策略？** — 共用 go.mod，一次测试覆盖全部，分别编译
5. **为什么要 `go-version-file + GOTOOLCHAIN=local`？** — 保证 `go` 命令与工具链二进制一致，避免自动 toolchain 导致覆盖率工具缺失
