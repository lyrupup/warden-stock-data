# 守望者行情数据服务（Warden Stock Data Service）

> 🦉 一套独立的 **A 股行情数据中台**，把行情拉取 / 缓存 / 落库 / 指标计算沉淀为统一、可扩展、高性能的数据底座，对多个业务系统开放只读行情 API。
>
> 持续侦查全市场行情 —— 让行情数据成为可被任意应用复用的「数据眼睛」。

---

## ✨ 项目由来

本服务从 [`warden-stock-trading`](../warden-stock-trading) 交易系统中**抽离行情数据能力**独立而来。原系统把「行情 + 策略 + 持仓 + 风控 + AI」耦合在一起，其中「行情数据」是最底层、最通用、最值得被多方复用的能力。本服务将其单独成服，对外提供标准化只读行情接口，让交易系统、量化系统、数据看板等都能以统一方式接入。

沿用守望者 **Scan · 侦查** 的能力定位：持续侦查全市场行情，沉淀为高质量、可增量更新、可计算量化因子的行情底座。

---

## 🧩 功能模块

| 模块 | 能力 | 简介 |
|------|------|------|
| **M1** 数据源适配与多市场接入 | 适配器 | `IMarketProvider` 屏蔽数据源差异；A 股主力 **gotdx**（连接池 + 测速 + 字段映射）；`market` 维度抽象预留 H 股 / 美股；可插拔多源主备降级 |
| **M2** 存储与增量更新调度 | 性能核心 | Redis 缓存 → Provider → 快照兜底；**增量更新**（按交易日水位）；**全市场扫描**算指标快照；**盘后定时分批批量更新**（默认 17:00 / 分批 20 / 并发 10，交易日历感知） |
| **M3** 技术指标计算 | 量化因子 | 输出 **MA5/10/20/30/60**，迁移振幅/乖离/排列等因子；指标接口可扩展 KDJ/MACD/RSI/BOLL |
| **M4** 行情数据开放 API | 数据出口 | 纯只读：指数 / 快照 / K 线 / 指标 / 搜索 / 元数据 |
| **M5** 鉴权与凭证管理 | 安全网关 | 管理员 JWT + 接入方 **secretId/secretKey（HMAC 签名）**；接入方 scope 固定只读 |
| **M6** Web 管理后台 | 运营界面 | 管理员登录、凭证分发（secretKey 一次性展示）、行情展示、数据源 / 作业管理 |

> 本服务**不面向 C 端用户**，只生产与开放**公共行情数据**。接入方通过 secretId/secretKey **只读消费**，无任何数据更新权限。

---

## 🔐 接入方式（Consumer）

外部应用经管理员分发 `secretId` / `secretKey` 后，调用开放 API `/open/v1/*`，每个请求带 HMAC 签名头：

```
X-Secret-Id:  <secretId>
X-Timestamp:  <unix 毫秒>
X-Nonce:      <一次性随机串>
X-Signature:  Base64(HMAC_SHA256(secretKey, StringToSign))

StringToSign = METHOD\nPATH\nCanonicalQuery\nX-Secret-Id\nX-Timestamp\nX-Nonce\nSHA256Hex(body)
```

时间戳偏差 ±300s、nonce 防重放。详见 [`docs/BACKEND.md`](docs/BACKEND.md) §2.2。

---

## 📐 技术方案

> 详见 PRD 第 7 章与 BACKEND.md，遵循项目 `AGENTS.md` 规范。

- **后端**：Go + Gin + GORM；中间件含限流 / 超时（context）/ CORS / 日志 / 双鉴权（Admin JWT + HMAC 凭证）；TDD 测试先行。
- **数据源**：`IMarketProvider` 适配器；A 股 gotdx（`-tags gotdx` 注入，默认回退 stub）；预留 Tushare / 港美股源。
- **存储**：PostgreSQL（K 线 / 快照 / 指标快照 / 凭证 / 作业 / 交易日历）+ Redis（缓存 / 限流 / 配额 / nonce）。
- **调度**：robfig/cron，盘后定时分批增量更新，交易日历感知。
- **前端**：React + Vite + shadcn/ui + Tailwind CSS，ky + TanStack Query，zustand，lightweight-charts，Light/Dark 主题。
- **部署**：Docker + docker-compose，配置经环境变量注入。

---

## ⚙️ 关键默认参数

| 配置 | 默认值 | 说明 |
|------|--------|------|
| 盘后更新触发 | `0 0 17 * * *`（17:00） | 仅交易日执行 |
| 分批大小 | 20 | 每批标的数 |
| 并发度 | 10 | 批内并发，与 gotdx 连接池对齐 |
| 首次历史回补 | 5 年 | 日 K 线全量回补 |
| 全市场扫描默认指标 | MA5/10/20/30/60 | 其余按需开启 |
| 开放 API 校验 | HMAC-SHA256 签名 | 时间戳 ±300s + nonce 防重放 |

---

## 🚀 快速启动（管理后台前端）

```bash
cd frontend
cp .env.example .env    # 按需修改 VITE_API_BASE_URL
npm install
npm run dev             # http://localhost:5173
```

开发模式下 Vite 会将 `/admin` 代理到 `http://localhost:8080`（需后端服务已启动）。生产构建：`npm run build`。

| 页面 | 路径 |
|------|------|
| 登录 | `/login` |
| 概览 | `/` |
| 凭证管理 | `/credentials` |
| 行情中心 | `/market` |
| 个股行情 | `/market/quote` |
| 数据源 | `/ops/datasources` |
| 更新作业 | `/ops/jobs` |

---

## 📚 文档

- 产品需求文档（PRD，按 M1~M6 模块）：[`docs/PRD.md`](docs/PRD.md)
- 后端技术开发文档（数据库 / API / TDD / 数据源 / 调度 / HMAC）：[`docs/BACKEND.md`](docs/BACKEND.md)
- 前端技术开发文档（管理后台）：[`docs/FRONTEND.md`](docs/FRONTEND.md)
- OpenAPI 接口定义（管理 API + 开放 API）：[`docs/openapi.yaml`](docs/openapi.yaml)

---

## 🗺️ 里程碑

| 阶段 | 目标 |
|------|------|
| V1.0 MVP | A 股行情中台闭环：gotdx 适配 + 增量/盘后调度/全市场扫描 + MA5~MA60 + 只读开放 API + 凭证管理 + 管理后台 |
| V1.1 | HMAC 签名增强、凭证审计与告警、作业可观测增强 |
| V1.2 | KDJ/MACD/RSI/BOLL 指标扩展 |
| V2.0 | H 股 / 美股接入、分市场调度、多源对账 |

---

> ⚠️ 免责：本服务仅提供行情数据，不构成投资建议；数据可能因数据源延迟 / 异常而存在偏差（以 `stale` 标记提示）。公开 / 逆向行情源仅适合非商业用途，商用请评估合规并接入持牌数据厂商。
