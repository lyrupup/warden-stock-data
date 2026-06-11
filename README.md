# 守望者行情数据服务（Warden Stock Data Service）

> 🦉 一套独立的 **A 股行情数据中台**，把行情拉取 / 缓存 / 落库 / 指标计算沉淀为统一、可扩展、高性能的数据底座，对多个业务系统开放只读行情 API。
>
> 持续侦查全市场行情 —— 让行情数据成为可被任意应用复用的「数据眼睛」。

---

## ✨ 项目由来

本服务从 [`warden-stock-trading`](../warden-stock-trading) 交易系统中**抽离行情数据能力**独立而来。原系统把「行情 + 策略 + 持仓 + 风控 + AI」耦合在一起，其中「行情数据」是最底层、最通用、最值得被多方复用的能力。本服务将其单独成服，对外提供标准化只读行情接口，让交易系统、量化系统、数据看板等都能以统一方式接入。

沿用守望者 **Scan · 侦查** 的能力定位：持续侦查全市场行情，沉淀为高质量、可增量更新、可计算量化因子的行情底座。

---

## 🏛️ 服务架构（Go + Python 双服务）

本服务由两个后端服务协作，前端与外部接入方**只与 Go 服务通信**，Python 服务**不对外、仅由 Go 内网调用**：

```
前端 / 外部接入方 ──► Go 服务（对外唯一入口：鉴权/限流/K线直查/作业调度）
                         │  gotdx ──► 分时 / 实时快照 / 指数
                         │  内部 HTTP（仅 compose 内网 + X-Internal-Token）
                         ▼
                     Python quant 服务（不对外）
                         · baostock 采集：日K / 复权因子 / 涨跌停（自算）/ ST / 停牌 / 退市 / 证券列表
                         · pandas-ta 实时计算技术指标（不落库）
                         ▼
                     PostgreSQL（两服务共用）
```

- **K 线纯查询**（无指标）→ Go 直查 PostgreSQL，最快路径，不经 Python。
- **指标 / 数据回补** → Go 转发 Python（指标实时算、采集 baostock 写库）。
- Python 服务详见 [`docs/PYTHON_SERVICE.md`](docs/PYTHON_SERVICE.md)。

## 🧩 功能模块

| 模块 | 能力 | 简介 |
|------|------|------|
| **M1** 数据源适配与多市场接入 | 适配器 | Go 侧 `IMarketProvider`（**gotdx** 分时/快照/指数）+ Python 侧 **baostock**（日K/复权因子/ST/停牌/退市）；`market` 维度预留 H 股/美股 |
| **M2** 存储与增量更新调度 | 性能核心 | **三类更新作业**（证券列表同步 / 全量日K回补 / 增量日K回补）；Go 调度编排（分批/并发/排队/失败码），实际采集由 Python baostock 执行；触发可选全量或指定代码；**盘后定时**（默认证券同步 8:30、增量日K 17:00 / 分批 20 / 并发 10，交易日历感知） |
| **M3** 技术指标计算 | 量化因子 | 由 **Python quant 服务**用 pandas-ta **实时计算**：MA5~60、MACD/KDJ/RSI/BOLL/ATR、中长期动量、迁移因子；**不再落库指标快照**；Go 转发请求 |
| **M4** 行情数据开放 API | 数据出口 | 纯只读：指数 / 快照 / K 线（含涨跌停/停牌/ST/复权因子）/ 分时 / 指标 / 搜索 / 元数据 |
| **M5** 鉴权与凭证管理 | 安全网关 | 管理员 JWT + 接入方 **secretId/secretKey（HMAC 签名）**；接入方 scope 固定只读 |
| **M6** Web 管理后台 | 运营界面 | 管理员登录、凭证分发（secretKey 一次性展示）、行情展示、数据源 / 作业管理 |

> 本服务**不面向 C 端用户**，只生产与开放**公共行情数据**。接入方通过 secretId/secretKey **只读消费**，无任何数据更新权限。Python quant 服务不对外开放，仅由 Go 服务内网驱动。

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

- **Go 服务**：Go + Gin + GORM；对外唯一入口；中间件含限流 / 超时（context）/ CORS / 日志 / 双鉴权（Admin JWT + HMAC 凭证）；K 线直查、作业调度编排、指标/采集转发 Python；TDD 测试先行。
- **Python quant 服务**：FastAPI + uvicorn + **baostock**（采集日K/复权因子/ST/停牌/退市）+ **pandas-ta-classic**（实时指标）；仅内网、由 Go 经 `X-Internal-Token` 调用。
- **数据源**：分时/快照/指数走 gotdx（Go）；日K及元数据走 baostock（Python，免费免注册）；涨跌停价按板块/ST 规则**自算**。
- **存储**：PostgreSQL（K 线含涨跌停/停牌/ST、复权因子、证券/退市、凭证 / 作业 / 交易日历）+ Redis（缓存 / 限流 / 配额 / nonce）。**指标不落库，实时计算**。
- **调度**：robfig/cron（Go），盘后定时分批回补，交易日历感知；采集动作经 HTTP 驱动 Python。
- **前端**：React + Vite + shadcn/ui + Tailwind CSS，ky + TanStack Query，zustand，lightweight-charts，Light/Dark 主题。
- **部署**：Docker + docker-compose（postgres / redis / backend / quant），配置经环境变量注入；quant 不映射宿主端口。

---

## ⚙️ 关键默认参数

| 配置 | 默认值 | 说明 |
|------|--------|------|
| 盘后更新触发 | 证券同步 `0 30 8 * * *`（8:30）、增量日K `0 0 17 * * *`（17:00） | 仅交易日执行；全量回补默认停用，按需手动触发 |
| 分批大小 | 20 | 每批标的数（Go 切批后逐批驱动 Python baostock 采集） |
| 并发度 | 10 | 批内并发 |
| 首次历史回补 | ≈5 年 | 日 K 线全量回补（baostock，`BACKFILL_START_DATE`） |
| 技术指标 | 实时计算（Python pandas-ta） | **不落库**；按需 MA/MACD/KDJ/RSI/BOLL/ATR/动量等 |
| 涨跌停价 | 按板块/ST 自算 | 主板10% / 创业·科创20% / 北交所30% / 主板ST 5% |
| 开放 API 校验 | HMAC-SHA256 签名 | 时间戳 ±300s + nonce 防重放 |

---

> 目录结构：`backend/go`（Go 服务）、`backend/python`（Python quant 服务）、`backend/deploy`（统一 Docker 部署）；环境变量统一在 `backend/.env`，三者共用。

## 🚀 快速启动（Go 后端）

```bash
cd backend
cp .env.example .env    # 统一环境变量（go / python / Docker 共用）
cd go
make infra-up           # 仅启动 postgres + redis 容器
make tidy && make run   # 自动加载 ../.env 并启动 API

curl http://localhost:8080/health
```

**线上 Docker 一键部署（全部服务）**（服务器 `git pull` 后）：

```bash
cp backend/.env.example backend/.env   # 首次，填写生产配置
cd backend/deploy && ./deploy.sh       # 一次性部署 postgres + redis + python(quant) + go(backend)
```

默认管理员：`admin` / `admin123`（可通过环境变量 `ADMIN_USERNAME` / `ADMIN_PASSWORD` 覆盖）。

开放 API 需 HMAC 签名头，详见 [`docs/BACKEND.md`](docs/BACKEND.md) §2.2。管理 API 使用 `Authorization: Bearer <jwt>`。

```bash
# 管理员登录
curl -s -X POST http://localhost:8080/admin/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"admin123"}'
```

后端目录结构与技术说明见 [`docs/BACKEND.md`](docs/BACKEND.md)；单元测试：`cd backend/go && make test`。

构建二进制（gotdx 为唯一行情源）：

```bash
cd backend/go && make build
./bin/warden-server
```

历史 K 线回补由 **Python quant 服务**经 baostock 执行，通过 Go 管理后台「更新作业」触发（全量日K回补 / 增量日K回补），Go 负责分批/并发/进度编排。

---

## 🚀 快速启动（Python quant 服务）

quant 服务（位于 `backend/python`）负责 baostock 采集与实时指标计算，**仅供 Go 内网调用**，本地绑定 `127.0.0.1:8000`。环境变量与 Go 共用 `backend/.env`，无需单独的 env 文件。

```bash
cd backend/python
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
make run                        # 读取 ../.env（与 go 共用），uvicorn 127.0.0.1:8000
```

Go 服务通过 `QUANT_BASE_URL`（默认 `http://127.0.0.1:8000`）+ `INTERNAL_TOKEN` 调用。PostgreSQL/Redis 仍由 `backend/go` 的 `make infra-up` 启动，两服务共用。详见 [`docs/PYTHON_SERVICE.md`](docs/PYTHON_SERVICE.md)。

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
| 开放 API 接入文档 | `/api-docs`（首页右上角「API 文档」入口） |

> 启动前端会自动执行 `npm run sync:docs`（`predev`/`prebuild` 钩子），把 [`docs/API_GUIDE.md`](docs/API_GUIDE.md) 复制到 `frontend/public/api-guide.md`，由 `/api-docs` 页面运行时渲染。

---

## 📚 文档

- 产品需求文档（PRD，按 M1~M6 模块）：[`docs/PRD.md`](docs/PRD.md)
- **开放 API 接入说明（第三方接入指南：secretKey 使用 + 开放接口）**：[`docs/API_GUIDE.md`](docs/API_GUIDE.md)
- 后端技术开发文档（数据库 / API / TDD / 数据源 / 调度 / HMAC）：[`docs/BACKEND.md`](docs/BACKEND.md)
- **Python quant 服务文档（采集 + 指标计算）**：[`docs/PYTHON_SERVICE.md`](docs/PYTHON_SERVICE.md)
- 行情数据链路（K 线 / 分时 / 采集 / 指标）：[`docs/MARKET_DATA.md`](docs/MARKET_DATA.md)
- 前端技术开发文档（管理后台）：[`docs/FRONTEND.md`](docs/FRONTEND.md)
- OpenAPI 接口定义（管理 API + 开放 API）：[`docs/openapi.yaml`](docs/openapi.yaml)

---

## 🗺️ 里程碑

| 阶段 | 目标 |
|------|------|
| V1.0 MVP | A 股行情中台闭环：gotdx 适配 + 增量/盘后调度/全市场扫描 + MA5~MA60 + 只读开放 API + 凭证管理 + 管理后台 |
| V1.1 | HMAC 签名增强、凭证审计与告警、作业可观测增强 |
| V1.2 | KDJ/MACD/RSI/BOLL/ATR 指标扩展（量化策略/回测）|
| V1.3 | ✅ **双服务重构**：新增 Python quant 服务（baostock 采集日K/复权因子/涨跌停/ST/停牌/退市 + pandas-ta 实时指标）；Go 专注对外 API/鉴权/K线直查/作业调度；指标快照落库下线 |
| V2.0 | H 股 / 美股接入、分市场调度、多源对账；Python 内置回测引擎 |

---

> ⚠️ 免责：本服务仅提供行情数据，不构成投资建议；数据可能因数据源延迟 / 异常而存在偏差（以 `stale` 标记提示）。公开 / 逆向行情源仅适合非商业用途，商用请评估合规并接入持牌数据厂商。
