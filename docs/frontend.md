# 前端架构文档

## 概述

前端基于 Vue 3 + TypeScript 构建，采用 Composition API + `<script setup>` 语法。通过 Vite 开发服务器代理 `/api` 到后端网关 (`localhost:8080`)，实现前后端分离开发。

## 技术栈

| 技术 | 版本 | 用途 |
|------|------|------|
| Vue 3 | 3.5 | UI 框架 (Composition API) |
| TypeScript | 5.9 | 类型安全 |
| Vite | 7 | 构建工具 + 开发服务器 |
| Tailwind CSS | v4 | 原子化 CSS (`@theme` CSS 变量) |
| Pinia | 3 | 状态管理 |
| Vue Router | 5 | 路由 (History mode) |
| Axios | 1.x | HTTP 客户端 |
| @vueuse/core | 14 | 组合式工具 (theme, storage) |
| lucide-vue-next | 0.576 | 图标库 |
| Vitest | 3.x | 单元测试 |

## 目录结构详解

```
frontend/src/
├── api/              # HTTP 请求层
│   ├── client.ts     # Axios 实例配置
│   ├── user.ts       # 用户 API (4 个端点)
│   └── file.ts       # 文件 API (15 个端点)
├── composables/      # 组合式函数 (可复用逻辑)
│   ├── useTheme.ts   # 主题管理
│   └── useUpload.ts  # 分块上传逻辑
├── components/
│   ├── layout/       # 布局组件
│   │   ├── AppLayout.vue    # 主布局 (sidebar + header + content)
│   │   ├── AppSidebar.vue   # 侧边栏 (导航 + 存储用量)
│   │   └── AppHeader.vue    # 顶部栏 (搜索 + 主题 + 用户)
│   ├── ui/           # 通用 UI 组件
│   │   ├── ThemeToggle.vue  # 主题切换按钮
│   │   └── BaseModal.vue    # 通用模态框
│   └── file/         # 文件相关组件
│       ├── FileBreadcrumb.vue  # 面包屑导航
│       ├── FileToolbar.vue     # 操作工具栏
│       ├── FileItem.vue        # 文件/文件夹卡片
│       ├── UploadProgress.vue  # 上传进度面板
│       └── ShareDialog.vue     # 分享对话框
├── stores/           # Pinia 状态管理
│   ├── auth.ts       # 认证状态
│   └── file.ts       # 文件状态
├── views/            # 页面视图
│   ├── LoginView.vue
│   ├── RegisterView.vue
│   ├── FileBrowserView.vue
│   ├── TrashView.vue
│   ├── ProfileView.vue
│   └── SharePublicView.vue
├── router/           # 路由配置
│   └── index.ts
├── types/            # TypeScript 类型定义
│   └── index.ts
├── style.css         # 全局样式 + Tailwind 主题
├── App.vue           # 根组件
└── main.ts           # 应用入口
```

## 核心模块详解

### 1. API 客户端 (`src/api/client.ts`)

**设计思路**: 封装 Axios 实例，统一处理 JWT 认证和错误响应。

**实现要点**:
- `baseURL` 设为 `/api/v1`，Vite 开发服务器将其代理到 `http://localhost:8080`
- 请求拦截器: 从 `localStorage` 读取 token，自动添加 `Authorization: Bearer <token>` 头
- 响应拦截器: 捕获 401 状态码，清除本地 token 并跳转到登录页
- 超时设置 30 秒

**面试要点**: 解释为什么选择拦截器模式而非在每个 API 调用中手动添加 token —— 避免重复代码，统一管理认证状态，单一职责。

### 2. 主题系统 (`src/composables/useTheme.ts`)

**设计思路**: 支持三种模式 (light/dark/system)，使用 CSS 自定义属性实现主题切换。

**实现要点**:
- 使用 `@vueuse/core` 的 `useStorage` 持久化主题偏好到 `localStorage`
- 使用 `usePreferredDark` 监听系统主题变化
- 通过 `document.documentElement.classList.toggle('dark')` 切换 CSS 类
- Tailwind v4 的 `@theme` 指令定义两套 CSS 变量 (light/dark)

**面试要点**:
- 为什么用 CSS 变量而非 JavaScript 控制颜色? —— 性能更好，浏览器原生渲染，无 FOUC
- 为什么不直接用 Tailwind 的 `dark:` 前缀? —— v4 推荐 CSS 变量方案，更灵活，支持 system 模式

### 3. 分块上传 (`src/composables/useUpload.ts`)

**设计思路**: 实现大文件分块上传，支持秒传和断点续传。

**流程**:
```
选择文件 → 计算 MD5 hash → 检查秒传(CheckUpload)
  ├─ 可秒传 → 直接合并(MergeChunks) → 完成
  └─ 不可秒传 → 获取已上传分块列表
       → 逐块上传(跳过已传) → 合并(MergeChunks) → 完成
```

**实现要点**:
- 分块大小 5MB (`CHUNK_SIZE = 5 * 1024 * 1024`)
- 使用 `spark-md5` 分块计算 MD5
- 分块通过 `multipart/form-data` 二进制上传（不做 Base64 编码）
- 全局任务队列 (`tasks` ref)，支持多文件并行上传
- 状态追踪: `pending → hashing → uploading → merging → done | error`

**面试要点**:
- 为什么用 MD5? —— 这里用于秒传去重和完整性校验，速度比 SHA-256 更快
- 为什么改成 FormData? —— 直接传二进制 chunk，避免 Base64 约 33% 膨胀和额外编码开销
- 断点续传原理: 后端返回已上传分块索引列表，前端跳过这些分块

### 4. 认证状态 (`src/stores/auth.ts`)

**设计思路**: 集中管理用户认证状态，包括 token 存储、用户信息、登录/注册/登出流程。

**实现要点**:
- Token 持久化到 `localStorage` (key: `light-cloud-token`)
- `isAuthenticated` 计算属性: `!!token.value`
- Login: 调用 API → 存储 token + user → 跳转首页
- Register: 调用 API → 跳转登录页
- Logout: 清空 token + user → 跳转登录页

### 5. 文件状态 (`src/stores/file.ts`)

**设计思路**: 管理文件列表、导航、排序、选择等文件浏览器核心状态。

**实现要点**:
- `sortedFiles` 计算属性: 文件夹始终排在前面，然后按选定字段排序
- `breadcrumb` 面包屑导航: 维护路径栈，支持任意层级跳转
- `selectedIds` 使用 `Set<number>` 管理多选状态
- 搜索和浏览共用 `files` 状态，通过 `searchKeyword` 区分模式

### 6. 路由配置 (`src/router/index.ts`)

**路由表**:

| 路径 | 组件 | 认证 | 说明 |
|------|------|------|------|
| `/login` | LoginView | guest | 登录页 |
| `/register` | RegisterView | guest | 注册页 |
| `/share/:shareId` | SharePublicView | guest | 公开分享页 |
| `/` | AppLayout > FileBrowserView | auth | 文件浏览 |
| `/trash` | AppLayout > TrashView | auth | 回收站 |
| `/profile` | AppLayout > ProfileView | auth | 个人设置 |

**路由守卫**:
- `meta.auth` 路由: 无 token 则重定向到 `/login`
- `meta.guest` 路由: 有 token 则重定向到 `/` (除 share-view 外)

## 主题系统详解

### CSS 变量方案

```css
/* 浅色主题 (默认) */
:root {
  --color-bg-primary: #f0f7ff;     /* 天蓝色背景 */
  --color-accent: #2563eb;          /* 蓝色强调 */
  --color-text-primary: #0f172a;    /* 深色文字 */
}

/* 深色主题 */
.dark {
  --color-bg-primary: #0b1120;     /* 深海蓝背景 */
  --color-accent: #60a5fa;          /* 亮蓝强调 */
  --color-text-primary: #e2e8f0;    /* 浅色文字 */
}
```

### 设计规范

- **字体**: DM Sans (正文) + Outfit (标题)
- **圆角**: 8px (卡片), 6px (按钮), 12px (模态框)
- **过渡**: 200ms ease (颜色切换)
- **间距**: 4px 基数

## API 端点映射

前端 API 模块与后端网关端点的对应关系:

### 用户 API (`src/api/user.ts`)

| 方法 | 前端函数 | 后端端点 | 认证 |
|------|----------|----------|------|
| POST | `register()` | `/api/v1/user/register` | No |
| POST | `login()` | `/api/v1/user/login` | No |
| GET | `getUserInfo()` | `/api/v1/user/info` | Yes |
| PUT | `updateUserInfo()` | `/api/v1/user/info` | Yes |

### 文件 API (`src/api/file.ts`)

| 方法 | 前端函数 | 后端端点 | 认证 |
|------|----------|----------|------|
| GET | `listFiles()` | `/api/v1/files` | Yes |
| GET | `searchFiles()` | `/api/v1/files/search` | Yes |
| POST | `createFolder()` | `/api/v1/file/folder` | Yes |
| PUT | `renameFile()` | `/api/v1/file/rename` | Yes |
| DELETE | `deleteFiles()` | `/api/v1/files` | Yes |
| PUT | `moveFiles()` | `/api/v1/file/move` | Yes |
| GET | `getDownloadURL()` | `/api/v1/file/download/:id` | Yes |
| POST | `checkUpload()` | `/api/v1/file/check-upload` | Yes |
| POST | `uploadChunk()` | `/api/v1/file/upload-chunk` | Yes |
| POST | `mergeChunks()` | `/api/v1/file/merge-chunks` | Yes |
| GET | `listTrash()` | `/api/v1/trash` | Yes |
| POST | `restoreFiles()` | `/api/v1/trash/restore` | Yes |
| DELETE | `permanentDelete()` | `/api/v1/trash` | Yes |
| POST | `createShare()` | `/api/v1/share` | Yes |
| GET | `getShare()` | `/api/v1/share/:shareId` | No |

## 测试架构

使用 Vitest 作为测试框架，搭配 `@pinia/testing` 和 `@vue/test-utils`。

### 测试分层

| 层级 | 测试对象 | Mock 策略 |
|------|----------|-----------|
| Stores | auth.ts, file.ts | Mock `@/api/*` 模块 |
| Composables | useTheme.ts, useUpload.ts | Mock DOM API + `@/api/*` |
| Router | index.ts (auth guard) | Mock `localStorage` |

### 运行测试

```bash
pnpm test              # 运行所有测试 (watch mode)
pnpm test:run          # 运行所有测试 (CI mode, 单次)
pnpm test:coverage     # 运行测试 + 覆盖率
```

## 构建部署

### 开发

```bash
cd frontend
pnpm install       # 安装依赖
pnpm dev           # 启动开发服务器 (http://localhost:3000)
```

Vite 开发服务器将 `/api` 请求代理到 `http://localhost:8080` (Go 网关)。

### 生产构建

```bash
cd frontend
pnpm build         # TypeScript 类型检查 + Vite 构建
```

产物输出到 `frontend/dist/`，可通过 Nginx 或 CDN 部署。

### 与后端集成

生产环境推荐方式:
1. **Nginx 反向代理**: 静态文件指向 `dist/`，`/api` 代理到网关
2. **CDN + 网关**: 前端部署到 CDN，网关处理 API 和 CORS
