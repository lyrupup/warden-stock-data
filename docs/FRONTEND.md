# 守望者行情数据服务 · 前端技术开发文档

> Warden Stock Data Service · Admin Console Frontend
>
> 文档版本：v1.0 ｜ 创建日期：2026-06-05
> 配套文档：[`PRD.md`](./PRD.md) · [`BACKEND.md`](./BACKEND.md) · [`openapi.yaml`](./openapi.yaml)
>
> 本服务前端为**管理后台（Admin Console）**，仅面向管理员，对应 PRD **M6**（并在个股详情内置 **M7** 分时做 T 研判）。功能：管理员登录、接入凭证（secretId/secretKey）分发与管理、行情数据展示、分时做 T 研判、数据源与更新作业管理。

---

## 1. 技术栈与选型

### 1.1 主要技术栈

| 分类 | 选型 | 说明 |
|------|------|------|
| 框架 | **React 18 + TypeScript** | 函数式组件 + Hooks，全量 TS |
| 构建 | **Vite 5** | 快速冷启动与 HMR |
| UI 组件 | **shadcn/ui** | 基于 Radix + Tailwind，CLI 安装到 `components/ui/` |
| 样式 | **Tailwind CSS 3** | utility-first，支持 Light/Dark 主题 |
| 路由 | **React Router v6** | 集中式路由 + 登录守卫 |
| 服务端状态 | **TanStack Query v5** | 请求缓存、轮询（作业进度）、重试、失效 |
| 全局状态 | **zustand** | 管理员认证、主题 |
| 请求器 | **ky** | 基于 fetch，封装于 `core/http-client/` |
| 表单 | **react-hook-form + zod** | 凭证创建 / 作业配置表单与校验 |
| 图表 | **lightweight-charts** | K 线图（含 MA5/10/20/30/60 叠加 + 成交量副图）、分时图 |
| 国际化 | **i18next + react-i18next** | 语言包置于 `core/i18n/locales/`（V1 中文为主） |
| 测试 | **Vitest + Testing Library + MSW** | 单测 + 组件测试 + 接口 Mock |
| 代码规范 | ESLint + Prettier | 统一风格 |

### 1.2 强制编码约束（贯穿全项目，遵循 AGENTS.md）

1. **具名导出**：禁止 `export default`，统一 `export`。
2. **类型前缀**：`type` 用 `T`、`enum` 用 `E`、`interface` 用 `I`。
3. **目录命名**：组件/hook 目录用 **kebab-case**，通过 `index.ts` 统一导出。
4. **分层**：`Component → Hook/Store → Core → Backend`，禁止跨层直连。
5. **DRY**：相同/相似逻辑出现 ≥ 3 次抽象到 `components/common/` 或 `hooks/`。
6. **样式**：优先 Tailwind utility class，用 `cn()` 合并 className。
7. **decimal 安全**：行情 / 指标数值后端为 **decimal 字符串**，必须经 `lib/decimal.ts` 转 number 后再计算 / 格式化，**禁止直接算术或 `.toFixed()`**。
8. **凭证安全**：secretKey 仅创建 / 轮换时由后端返回一次，前端**只在弹窗一次性展示并提供复制**，绝不持久化、不写入任何 store / localStorage。

---

## 2. 项目目录结构

```
src/
├── components/
│   ├── ui/                      # shadcn/ui 原始组件（CLI 生成，勿改）
│   └── common/
│       ├── data-table/          # 通用表格（排序/分页）
│       ├── quote-cell/          # 行情涨跌色单元格
│       ├── confirm-dialog/      # 二次确认（吊销凭证等危险操作）
│       ├── secret-reveal-dialog/# secretKey 一次性展示 + 复制
│       ├── page-header/         # 页面标题/操作区
│       ├── kline-chart/         # K 线图 + 均线叠加 + 成交量副图
│       ├── intraday-chart/      # 分时图：价格线 + 均价线 + 昨收基准 + 分时量副图
│       └── empty-state/         # 空态/错误降级
├── features/
│   ├── auth/                    # M6 管理员登录
│   ├── credentials/             # M5/M6 凭证分发与管理
│   ├── market/                  # M6 行情数据展示（指数/个股/K线/指标）
│   └── ops/                     # M6 数据源 + 更新作业 + 数据新鲜度
├── core/
│   ├── http-client/             # ky 封装 + 拦截器（管理员 JWT 注入）
│   ├── api/                     # 各业务 service 工厂
│   ├── auth/                    # token 管理
│   ├── i18n/                    # i18next + locales/
│   └── theme/                   # 主题（Light/Dark）provider
├── hooks/
│   ├── use-paged-query/
│   └── use-polling-query/       # 作业进度轮询
├── stores/
│   ├── auth-store.ts            # 管理员 token / 信息
│   └── theme-store.ts
├── types/                       # 全局类型（与 openapi 对齐）
├── lib/                         # cn / decimal / format / date
├── routes/                      # 路由配置 + 登录守卫
├── styles/
├── App.tsx
└── main.tsx
```

每个 `features/<module>/` 内部统一结构：

```
features/credentials/
├── components/        # 模块专属组件（凭证表格、创建表单、详情抽屉）
├── hooks/             # 封装 TanStack Query（useCredentials / useCreateCredential ...）
├── api.ts             # 模块接口（基于 core/api）
├── types.ts           # 模块类型（TCredential / TCredentialSecret ...）
└── index.ts           # 统一具名导出
```

---

## 3. 核心功能包（依赖清单）

| 包 | 用途 | 所在层 |
|----|------|-------|
| `react` / `react-dom` | 框架 | 全局 |
| `vite` / `@vitejs/plugin-react` | 构建 | 工程 |
| `tailwindcss` / `postcss` / `autoprefixer` | 样式 | 全局 |
| `class-variance-authority` / `clsx` / `tailwind-merge` | `cn` | `lib/` |
| `@radix-ui/*`（随 shadcn 安装） | 无障碍基础组件 | `components/ui/` |
| `react-router-dom` | 路由 | `routes/` |
| `@tanstack/react-query` | 服务端状态 | `features/*/hooks` |
| `zustand` | 全局状态 | `stores/` |
| `ky` | HTTP 请求 | `core/http-client/` |
| `react-hook-form` / `zod` / `@hookform/resolvers` | 表单与校验 | `features/*` |
| `lightweight-charts` | K 线 + 均线图 | `features/market` |
| `i18next` / `react-i18next` | 国际化 | `core/i18n/` |
| `dayjs` | 日期 / 交易日 | `lib/` |
| `vitest` / `@testing-library/react` / `msw` | 测试 | 工程 |

---

## 4. 核心基础设施实现

### 4.1 HTTP 请求器（core/http-client）

统一 ky 实例：注入管理员 JWT（`Authorization: Bearer`）、解包 `{code,message,data}`、`code!=0` 抛 `AppError`、`40001` 自动登出跳登录页。

```typescript
// core/http-client/http-client.ts
import ky from "ky";
import { useAuthStore } from "@/stores/auth-store";

export class AppError extends Error {
  constructor(public code: number, message: string) {
    super(message);
  }
}

export const httpClient = ky.create({
  prefixUrl: import.meta.env.VITE_API_BASE_URL, // 指向 /admin
  hooks: {
    beforeRequest: [
      (req) => {
        const token = useAuthStore.getState().token;
        if (token) req.headers.set("Authorization", `Bearer ${token}`);
      },
    ],
    afterResponse: [
      async (_req, _opts, res) => {
        const body = await res.clone().json<{ code: number; message: string; data: unknown }>();
        if (body.code === 40001) useAuthStore.getState().logout();
        if (body.code !== 0) throw new AppError(body.code, body.message);
      },
    ],
  },
});
```

> 管理后台只调用管理 API（`/admin/*`）。行情展示所需的指数 / 个股 / K 线 / 指标数据，由后端在 `/admin` 下提供等价只读视图接口（复用 service 层），前端**无需**实现 HMAC 签名（HMAC 仅用于外部接入方直连 `/open/v1`）。

### 4.2 decimal 工具（lib/decimal.ts）

```typescript
// 后端 decimal 字段为字符串；统一转 number 再计算/格式化
export const toNumber = (v: string | number | null | undefined): number =>
  v == null ? 0 : typeof v === "number" ? v : Number(v);

export const formatPct = (v: string | number) => `${toNumber(v).toFixed(2)}%`;
export const changeColor = (v: string | number) =>
  toNumber(v) > 0 ? "text-red-500" : toNumber(v) < 0 ? "text-green-500" : "text-muted-foreground";
```

### 4.3 全局状态（zustand）

```typescript
// stores/auth-store.ts
interface IAuthState {
  token: string | null;
  admin: TAdmin | null;
  login: (token: string, admin: TAdmin) => void;
  logout: () => void;
}
// token 持久化到 localStorage；logout 清空并跳 /login
```

### 4.4 作业进度轮询（hooks/use-polling-query）

封装 TanStack Query 的 `refetchInterval`：当前跟踪作业 `status` 为 `running`/`waiting` 时每 2s 轮询 `/admin/jobs/runs/:runId`，进入终态后停止。

作业页列表轮询：作业列表 `/admin/jobs` 每 30s 刷新；执行记录 `/admin/jobs/runs` 每 10s 刷新（与单条进度的 2s 高频轮询区分，降低空闲请求量）。

---

## 5. 路由与页面规划

### 5.1 路由表

| 路径 | 页面 | 模块 | 守卫 |
|------|------|------|------|
| `/login` | 管理员登录 | M6 | 公开 |
| `/` | 概览（数据新鲜度 / 数据源健康 / 今日作业） | M6 | 需登录 |
| `/credentials` | 凭证列表与管理 | M5/M6 | 需登录 |
| `/credentials/:id` | 凭证详情 + 调用审计 | M5/M6 | 需登录 |
| `/market` | 行情中心（大盘指数概览） | M6 | 需登录 |
| `/market/quote` | 个股行情搜索（搜索 → 点击结果跳转详情） | M6 | 需登录 |
| `/market/quote/:code` | 个股行情详情（快照 + 分时 + 做 T 研判 + K 线 + 均线 + 指标） | M6 / M7 | 需登录 |
| `/ops/datasources` | 数据源管理与健康 | M1/M6 | 需登录 |
| `/ops/jobs` | 更新作业配置与执行记录 | M2/M6 | 需登录 |

### 5.2 全局布局

左侧导航（概览 / 凭证 / 行情 / 运维）+ 顶栏（主题切换、管理员菜单、登出）。登录守卫：无 token 重定向 `/login`。

---

## 6. 各页面核心功能与实现方案

### 6.1 登录（features/auth）

- 账号密码表单（react-hook-form + zod）→ `POST /admin/auth/login` → 存 token + 跳首页。

### 6.2 凭证管理（features/credentials）— 核心

- **列表**：`GET /admin/credentials`，表格列：接入方名称、secretId（脱敏 + 复制）、scope（read）、限流 QPS、日配额、状态、最近调用、调用量。
- **创建**：表单填接入方名称 / 限流 / 配额 / 有效期 → `POST /admin/credentials` → 成功后用 `secret-reveal-dialog` **一次性展示 secretId + secretKey 明文**，提供「复制」与醒目提示「secretKey 仅此一次可见，请妥善保存」。关闭弹窗后不可再获取。
- **轮换**：`POST /admin/credentials/:id/rotate` → 同样一次性展示新 secretKey。
- **吊销 / 启停**：`confirm-dialog` 二次确认 → `DELETE` / `PUT`。
- **详情**：`GET /admin/credentials/:id`，展示调用审计（按日 call_count / error_count / 最近调用）。
- **接入指引**：详情页内置「HMAC 签名接入示例」代码片段（如何构造 X-Signature），方便接入方对接。

> secretKey **只存在于创建/轮换响应的内存中**，展示后即丢弃；禁止写入 store / 缓存 / 日志。

### 6.3 行情展示（features/market）

- **大盘指数**：`/market` 卡片网格展示指数（现价、涨跌额、涨跌幅，涨跌色由 `changeColor`）。
- **个股行情搜索**：`/market/quote` 搜索框（`GET /admin/market/search`）→ 点击结果 `navigate` 跳转 `/market/quote/:code` 详情页（不再内联展示）。导航项 `end:false`，详情页下「个股行情」保持高亮。
- **个股行情详情**：`/market/quote/:code` 按路由 `code` 拉取数据，三态处理：
  - **加载中**：拉取快照时展示 Loading（spinner + 提示）。
  - **错误 / 无数据**：股票不存在或拉取失败时展示错误卡片与「返回搜索」入口。
  - **成功**：个股快照卡（现价 / 开高低收 / 量额 / 换手率，stale 时「数据延迟」徽标）+ **分时图**（`intraday-chart`，价格线 + 均价线 + 昨收基准线 + 分时量副图 + 乖离副图 + 做 T 研判，60s 轮询，见 §6.5）+ **K 线图**（`kline-chart`，日/周/月 + 复权切换 + MA5/10/20/30/60 叠加 + 成交量副图）+ 指标面板。

### 6.4 运维（features/ops）

- **数据源**：`GET /admin/datasources` 展示 source/market/优先级/健康状态；`PUT` 配置启停 / 连接池；「探测」按钮 → `POST .../healthcheck`。
- **更新作业**：
  - 作业配置表单：cron 表达式（默认 `0 0 17 * * *`）、分批大小（默认 20）、并发度（默认 10）、启停 → `PUT /admin/jobs/:id`。
  - 手动触发：选择类型（全量/增量/快照/指标）+ 市场 + 可选代码 → `POST /admin/jobs/:id/run`，返回 runId 后跳到进度视图。
  - 执行记录：`GET /admin/jobs/runs` 分页表格；运行中行用 `use-polling-query` 轮询进度条（processed/total）；可取消运行中作业。
  - 编辑作业：每个作业卡片提供「编辑」按钮，弹窗修改名称 / 市场 / cron / 分批 / 并发 / 启停（`job_type` 只读）；`cron_expr` 前端粗校验 + 服务端 `PUT /admin/jobs/:id` 权威校验，错误信息回显弹窗。
  - 数据新鲜度：`GET /admin/freshness` 展示「全市场更新到哪个交易日 / 最近扫描时间 / 证券数 / 行情数据覆盖率（已入库股票数 ÷ 证券总数）」。

### 6.5 分时交易研判 / 做 T（M7，components/common/intraday-chart）

> 在个股详情分时图之上的**纯前端**日内做 T（T+0 低吸高抛）辅助研判，不接交易、仅供参考。计算与渲染分层清晰，便于单测与复用。

- **计算层（lib，纯函数 + vitest）**：
  - `lib/intraday-signals.ts`：`computeIntradayMetrics`（逐分钟乖离 / 量比 / 累计均量，做 T 与趋势 BS 的共同基座）+ `computeIntradaySignals`（均价线交叉「趋势 B/S」）。
  - `lib/daytrade.ts`：`computeDayTradeBaseline`（历史 ATR / refPct / 支撑压力，排除今日）、`computeDayTrend`（6 因子趋势态）、`computeDayTradePlan`（是否适合做 T 与模式）、`computeDayTradeSignals`（高抛低吸轨道 + 吸/抛信号）、`computeSessionMaturity`（盘中可靠度）。参数默认见 `DEFAULT_DAYTRADE_PARAMS`。
- **渲染层（`intraday-chart.tsx`，lightweight-charts v5）**：主图（价 / 均价 / 昨收 / 高抛低吸轨道带 / 吸·抛箭头）+ 量副图（量柱 + 量能门槛线）+ 乖离副图；各 pane 注入浮动图例，hover 同步各自指标值。
- **交互**：
  - **模式切换**：「做 T（吸/抛）」/「趋势（B/S）」一键切换主图标注与下方面板。
  - **研判面板**：趋势态 + 评分、建议模式、预期振幅、历史 ATR、轨道宽度、信号明细，每项带 `info` 悬浮说明；顶部「可靠度」徽标按交易时长分四档。
  - **调参面板**：振幅门槛 / 轨道宽度 / 信号冷却 / 低吸缩量上限 / 趋势斜率窗口 / 历史回看 6 项滑块，改动**实时**重算研判与轨道，可一键重置默认；每项带 `info` 提示影响因素。
- **数据来源**：复用 `useStockIntraday`（分时，60s 轮询）与 `useStockKline(code, "day", "qfq")`（历史基准，独立于上方 K 线周期选择器）。
- **配色 / 弹层**：A 股涨红跌绿；info 提示与下拉等弹层统一用 `bg-popover`（需在 `tailwind.config.js` 与 `globals.css` 定义 `popover` 配色变量，否则透明）。
- 详细计算口径与风险说明见 [`MARKET_DATA.md`](./MARKET_DATA.md) §4.9~§4.10。

---

## 7. 与后端的契约对齐

- 类型以 [`openapi.yaml`](./openapi.yaml) 为单一事实源；`types/` 下的 `T*` 与之对齐（可用 openapi-typescript 生成）。
- 所有 decimal 字段在 `types.ts` 标为 `string`，经 `lib/decimal.ts` 消费。
- 错误码与后端 `errcode` 段位一致（见 BACKEND.md §1.4），统一在 `AppError` 处理与 toast 提示。

---

## 8. 测试策略

| 层 | 工具 | 重点 |
|----|------|------|
| 工具函数 | Vitest | `decimal.toNumber`、涨跌色、日期 |
| Hook | Vitest + MSW | 凭证 CRUD、作业轮询停止条件 |
| 组件 | Testing Library + MSW | secretKey 弹窗仅展示一次、吊销二次确认、K 线均线渲染 |

---

## 9. 环境变量

```bash
# .env
VITE_API_BASE_URL=http://localhost:8080/admin   # 管理后台只打 /admin
VITE_APP_TITLE=守望者行情数据后台
```

---

## 10. 并行开发约定（前端视角）

- 接口契约以 `openapi.yaml` 为准，先对齐契约再开发。
- 分支按模块：`feat/fe-auth`、`feat/fe-credentials`、`feat/fe-market`、`feat/fe-ops`。
- 核心功能变更同步更新 PRD / FRONTEND / openapi / README（见 `AGENTS.md`）。
