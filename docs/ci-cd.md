# CI/CD 配置

## GitHub Actions

本项目使用 GitHub Actions 进行持续集成和发布。

### CI 工作流 (ci.yml)

**触发条件**: 
- Push 到 `main` 或 `v2` 分支
- 向 `main` 或 `v2` 分支发起 PR

**执行步骤**:
1. **Test Job**
   - 检出代码
   - 安装 Go 1.22
   - 下载依赖
   - 运行单元测试 (`go test -v -race -coverprofile=coverage.out ./...`)
   - 上传覆盖率报告到 Codecov

2. **Build Job** (依赖 Test 通过)
   - 编译 Linux AMD64 二进制文件
   - 上传构建产物

### Release 工作流 (release.yml)

**触发条件**: 推送以 `v` 开头的 tag (如 `v2.0.0`)

**执行步骤**:
1. 检出代码
2. 运行测试
3. 编译多架构二进制文件:
   - `user-service-linux-amd64`
   - `file-service-linux-amd64`
   - `user-service-linux-arm64`
   - `file-service-linux-arm64`
4. 生成 SHA256 校验和
5. 创建 GitHub Release 并上传文件

## 发布流程

```bash
# 1. 确保所有测试通过
make test

# 2. 创建并推送 tag
git tag v2.0.0
git push origin v2.0.0

# 3. GitHub Actions 自动创建 Release
```

## 本地构建

```bash
# 编译所有服务
make build

# 或手动编译
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/user-service ./cmd/user
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/file-service ./cmd/file
```

## 面试要点

1. **为什么用 GitHub Actions？** - 与 GitHub 深度集成，配置简单，免费额度充足
2. **为什么 CGO_ENABLED=0？** - 生成静态链接二进制，便于容器化部署
3. **为什么同时构建 AMD64 和 ARM64？** - 支持 x86 服务器和 ARM 云服务器 (如 AWS Graviton)
4. **Release 自动生成？** - 使用 `softprops/action-gh-release` 自动从 commit 生成 release notes
