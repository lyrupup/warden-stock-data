# AGENTS 开发指南

本文件为本项目（warden-stock-trading）的 AI 协作开发规范。任何前端或后端开发任务，都必须遵守以下约定。

## 项目文档索引

| 文档 | 路径 | 用途 |
|------|------|------|
| 需求文档（PRD） | `docs/PRD.md` | 系统整体功能、业务流程、需求边界 |
| 前端技术文档 | `docs/FRONTEND.md` | 前端架构、技术栈、目录与编码规范 |
| 后端技术文档 | `docs/BACKEND.md` | 后端架构、技术栈、服务与数据设计 |
| 接口文档 | `docs/openapi.yaml` | API 契约、请求/响应结构、错误码 |
| 行情数据链路 | `docs/MARKET_DATA.md` | K 线 / 分时数据的拉取逻辑、分层、存储与缓存策略 |
| 项目说明 | `README.md` | 项目概览、启动方式、整体说明 |

---

## 一、全栈开发规范（核心约束）

### 技术选型决策

根据业务场景选择对应技术栈：

| 业务场景 | 技术栈 | 请求器 | 适用说明 |
|---------|--------|-------|---------|
| 纯 Web 业务 | React + Vite + shadcn/ui + Tailwind CSS | ky + TanStack Query | 管理后台、Web 应用、数据看板等 |
| 跨三端（iOS/Android/Web） | Expo RN + Tamagui | ky-universal + TanStack Query | 需要同时覆盖移动端和 Web 的业务 |
| 跨双端追求性能 | Flutter + GetX | Dio | iOS/Android 高性能渲染场景 |
| SSR 服务 | Next.js | ky + TanStack Query | 仅做数据获取和页面渲染，不直接连数据库 |
| 轻量级 API 服务 | Bun + Hono + 中间件 | — | BFF 层、简单 CRUD、代理转发 |
| 核心后端服务 | Go + Gin + GORM | — | 主业务逻辑、需要高并发和事务支持的服务 |

### 通用编码约束（贯穿所有技术栈）

1. **封装解耦**：逻辑职责单一，模块间通过接口/类型约束交互，禁止跨层直接调用。
2. **目录规范**：严格划分 `core/`（核心可移植模块）与 `features/`（业务模块），功能目录使用 kebab-case 命名，通过 `index.ts` 统一导出。
3. **DRY 原则**：相同/相似逻辑出现 >= 3 次时，必须抽象为公共模块。
4. **具名导出**：TypeScript 项目禁止 `export default`，统一使用具名导出 `export`。
5. **类型前缀**：type 用 `T` 前缀（`TUser`），enum 用 `E` 前缀（`EStatus`），interface 用 `I` 前缀（`IService`）。
6. **请求器选型**：纯 Web 用 ky，Expo 用 ky-universal，Flutter 用 Dio，均搭配 TanStack Query（Flutter 除外）。
7. **状态管理**：TypeScript 前端统一使用 zustand 管理跨组件状态。
8. **多语言支持**：i18n 模块放在 `core/i18n/`，语言包放在 `core/i18n/locales/`。

### 后端核心要求

- 所有 API 必须实现限流和超时中断（context 传播）中间件。
- 核心业务操作必须支持接口层和数据库层的逻辑中断 + 事务回滚。
- 采用 TDD 驱动开发，先写测试再写实现。
- 使用中间件封装认证、日志、CORS、限流、超时等公共逻辑。

### 基础设施要求

- PostgreSQL、Redis、RabbitMQ 均通过 Docker 容器启动。
- 每个服务独立容器，通过 docker-compose 统一编排。
- 所有环境配置通过环境变量注入，不硬编码。

### Docker 部署规范

1. **PostgreSQL 数据持久化到本项目目录**：PostgreSQL 的数据目录 `/var/lib/postgresql/data` 必须通过 bind mount 持久化到**本项目内的目录**（如 `backend/deploy/pgdata`），不得使用项目外的宿主机路径或匿名卷，确保数据可随项目管理与查看。该数据目录须加入 `.gitignore`，禁止提交到仓库。
2. **镜像名称自动加项目名前缀**：后端启动 Docker 服务时，构建出的镜像名称必须自动带上项目名前缀（前缀取当前项目名，如 `<project>-backend`）。通过在 `docker-compose.yml` 顶层设置 `name: <project>`（统一项目名与容器/网络前缀），并为自建服务显式指定 `image: <project>-<service>` 实现。

---

## 二、开发前置阅读要求

### 通用前提

进行任何前端或后端开发前，**必须先阅读 `docs/PRD.md`**，把握系统整体功能与业务上下文，避免脱离需求实现。

### 前端开发流程

进行前端开发前，须按顺序阅读：

1. `docs/PRD.md` —— 明确需求与业务功能
2. `docs/FRONTEND.md` —— 明确前端架构与技术规范
3. `docs/openapi.yaml` —— 明确接口契约与数据结构

完成阅读后再开始编写前端代码。

### 后端开发流程

进行后端开发前，须按顺序阅读：

1. `docs/PRD.md` —— 明确需求与业务功能
2. `docs/BACKEND.md` —— 明确后端架构与技术规范
3. `docs/openapi.yaml` —— 明确接口契约与数据结构

阅读后**先编写 TDD 测试用例**，再开始编写后端实现代码（测试先行）。

---

## 三、文档同步要求

当涉及**核心功能的新增或修改**时，必须同步更新以下文档，保持代码与文档一致：

- `README.md` —— 项目说明
- `docs/PRD.md` —— 需求文档
- `docs/FRONTEND.md` —— 前端技术文档
- `docs/BACKEND.md` —— 后端技术文档
- `docs/openapi.yaml` —— 接口文档

> 任一核心功能变更未同步文档，视为该任务未完成。

---

## 四、协作输出与提交规范

### 对话输出语言

- 与用户的对话输出一律使用**中文**，不得使用英文作答。

### Git 提交规范

1. **强制账号配置**：提交前必须检查 `git config user.name` 与 `git config user.email`。任一为空时**阻断提交**，并提醒用户先完成 git 账号配置（`git config user.name` / `git config user.email`），不得擅自代为设置。
2. **提交信息格式**：`git commit` 信息遵循 Conventional Commits。类型与作用域前缀（如 `feat(scope)`、`fix`、`refactor`、`docs`、`chore` 等）**保持英文**；冒号之后的描述（含标题描述与正文）**使用中文**撰写。例如：`refactor(backend): 移除 stub 行情源并统一 gotdx 构建`。
3. **禁止 AI 署名**：提交信息中不得添加 Cursor / cursoragent 等 AI 工具的 `Co-authored-by` 或任何类似署名，提交的 author 与 committer 仅为用户本人。
