# 守望者行情数据服务 · Python 量化数据服务（采集 + 指标计算）

> Warden Stock Data Service · Python Quant Service（`backend/python/`）
>
> 文档版本：v1.0 ｜ 配套文档：[`PRD.md`](./PRD.md) · [`BACKEND.md`](./BACKEND.md) · [`MARKET_DATA.md`](./MARKET_DATA.md) · [`openapi.yaml`](./openapi.yaml)
>
> 本文档是 Python 量化数据服务（下称「**quant 服务**」）的唯一技术依据。遵循项目 `AGENTS.md` 规范，对话与注释一律中文。

---

## 1. 服务定位与边界

quant 服务是行情数据中台新增的**第二个后端服务**，承担两类职责，二者共享一个进程：

| 职责 | 说明 |
|------|------|
| **数据采集（Collector）** | 用 **baostock**（免费、免注册）采集 **日 K 线（不复权 + 前/后复权因子）、换手率、ST 标记、停牌状态、退市信息、证券列表**，并**自算涨跌停价**，写入 PostgreSQL，作为回测/查询底座。 |
| **指标计算（Indicator）** | 用专业金融分析库 **pandas-ta-classic** + numpy/pandas，对库内日 K **实时计算**技术指标（MA/MACD/KDJ/RSI/BOLL/ATR/动量…），返回给 Go 服务。**不落库快照**。 |

> 未来扩展：量化回测引擎（撮合 / 绩效）将作为本服务的第三类职责加入，故命名为「quant 量化服务」而非单纯「collector」。

### 1.1 铁律：不对外、只被 Go 调用

- quant 服务**只暴露内部 API**（`/internal/v1/*`），**不对接前端、不对接外部接入方**。
- 部署时**不映射宿主机端口**，仅在 docker-compose 内网以服务名 `quant:8000` 可达；本地开发绑定 `127.0.0.1`。
- 所有内部接口校验 **`X-Internal-Token`** 共享密钥头（来自环境变量 `INTERNAL_TOKEN`），Go 服务调用时携带，双重保证只被 Go 驱动。
- 对外的鉴权 / 限流 / 凭证 / HMAC 全部仍在 Go 服务（M4/M5），quant 服务不重复实现。

### 1.2 与 gotdx（Go）的职责切分

| 数据 | 负责方 | 说明 |
|------|--------|------|
| 分时走势 / 实时快照 / 指数 | **gotdx（Go）** | 实时性强、gotdx 的强项；维持原链路不变 |
| 日 K 线 + 复权因子 + 涨跌停 + ST + 停牌 + 退市 + 证券列表 | **baostock（quant）** | 公司行为/元数据，gotdx 拿不到 |
| 技术指标实时计算 | **pandas-ta（quant）** | 从 Go 迁入，去掉指标快照落库 |
| K 线纯查询（无指标） | **Go 直查 PG** | 不经 quant，最快路径 |

---

## 2. 技术栈

| 分类 | 选型 | 说明 |
|------|------|------|
| 语言 | **Python 3.12** | |
| Web 框架 | **FastAPI + uvicorn** | 异步、自动 OpenAPI、契合 PRD Web 规范 |
| 数据源 | **baostock** | 免费免注册；日 K / 复权因子 / ST / 停牌 / 退市 / 证券列表 |
| 指标库 | **pandas-ta-classic** + numpy + pandas | 纯 Python 252 指标，2026 活跃；A 股特殊口径（KDJ SMA 平滑、MACD 柱×2 通达信口径）保留**自定义实现**对齐前端 |
| DB 驱动 | **SQLAlchemy 2.x + psycopg(3)** | 连接池、批量 upsert |
| 配置 | **pydantic-settings** + 环境变量 | 不硬编码 |
| 校验 | **pydantic v2** | 请求/响应模型 |
| 测试 | **pytest** | 纯函数（涨跌停、指标口径）先行 |
| 镜像 | **python:3.12-slim**（非 alpine） | baostock/pandas/numpy 在 musl 上易踩坑 |

> 选型理由：pandas-ta-classic 纯 Python、`pip install` 无 C 依赖，安装最稳；A 股 KDJ/MACD 等特殊口径以自定义函数对齐通达信，保证与前端 K 线副图一致。

---

## 3. 目录结构（遵循 core/ 与 features/ 划分）

```
backend/python/
├── app/
│   ├── main.py                      # FastAPI 入口（装配路由、生命周期、内部鉴权中间件）
│   ├── core/                        # 可移植核心模块
│   │   ├── config.py                # pydantic-settings 配置（环境变量）
│   │   ├── db.py                    # SQLAlchemy engine / session / 批量 upsert
│   │   ├── security.py              # X-Internal-Token 校验依赖
│   │   ├── logging.py               # 结构化日志
│   │   └── limit_price.py           # 涨跌停价纯函数（按板块/ST 自算）
│   ├── features/
│   │   ├── collect/                 # 数据采集（baostock）
│   │   │   ├── baostock_client.py   # baostock 登录复用 + 串行调用封装
│   │   │   ├── mapper.py            # baostock 字段 → DB 模型映射
│   │   │   ├── kline.py            # 日 K + 复权因子 + 涨跌停 + ST + 停牌 采集
│   │   │   ├── securities.py        # 证券列表 + 上市/退市/ST 状态采集
│   │   │   └── service.py           # 采集编排（按 codes 批处理，返回每只结果）
│   │   ├── indicator/               # 指标计算
│   │   │   ├── catalog.py           # 指标目录（type/name/group/value_type/params）
│   │   │   ├── compute.py           # pandas-ta + A 股口径计算
│   │   │   └── service.py           # 读库 K 线 → 计算 → 对齐返回
│   │   └── api/                     # 内部路由
│   │       ├── collect_router.py    # POST /internal/v1/collect/*
│   │       ├── indicator_router.py  # POST /internal/v1/indicators*、GET /catalog
│   │       └── health_router.py     # GET /health
│   ├── models/                      # SQLAlchemy 表模型（与 Go GORM 模型同表）
│   ├── schemas/                     # pydantic 请求/响应
│   └── scripts/                     # 离线 CLI
│       └── backfill.py             # 离线批量回补日 K（直接写库，绕过 Go/HTTP）
├── tests/                           # pytest（涨跌停、指标口径、mapper）
├── requirements.txt
├── Dockerfile
├── Makefile
└── README.md
```

> 环境变量不再单独维护：统一使用 `backend/.env`（与 Go 服务共用），本机直跑经 `Makefile` 加载 `../.env`，Docker 经 compose `env_file: ../.env` 注入。

---

## 4. 数据采集（baostock）

### 4.1 baostock 调用约束

- baostock 是**同步阻塞**库，且**全局单连接、非并发安全**：单进程内 `bs.login()` 一次复用，所有调用**串行**（一把进程内锁）；客户端内置空闲重登、socket 超时与查询失败重试，规避连接被服务端静默关闭后 `send_msg` 死循环。
- **合批语义**：Go `job_runner` 把 `batch_size`（默认 20）只代码**一次 HTTP 整批**交给 `/internal/v1/collect/kline`，quant 在锁内顺序处理整批并返回每只结果；`concurrency`（默认 10）控制 Go 端**在途批次 HTTP 数**。由于 baostock 在单个 quant 进程内全局串行，**单 worker 下并发批次只会在 Python 锁内排队**，并不提速；要真正并行需多进程（uvicorn `--workers N` / 多实例），或改用下文离线 CLI 的 `--shard` 分片。
- baostock 代码格式为 `sh.600000` / `sz.000001`；mapper 负责与库内 `600000` 纯代码互转。

### 4.5 离线批量回补 CLI（首次全量最快路径）

baostock **无官方可下载的数据集文件**，全部数据均通过登录 + 查询获取。因此「下载数据集再手动入库」在 baostock 语境下等价于：**本地一次性把全市场查下来直接写库**。为此提供离线 CLI，直接在 quant 进程内复用采集逻辑（日 K + 复权因子 + 自算涨跌停 + ST + 停牌），并写 `update_watermarks`，**绕过 Go/HTTP 编排**，是首次全量历史回补的最快路径：

```bash
cd backend/python
make backfill ARGS="--codes 600519,000001"   # 指定代码
make backfill ARGS="--all"                     # 全市场在市股票（先跑证券列表采集）
make backfill ARGS="--all --include-delisted"  # 含退市股
make backfill ARGS="--all --skip-done"         # 断点续跑：跳过已有水位的代码
# 多进程并行（每进程独立 baostock 会话，按下标分片）：
make backfill ARGS="--all --shard 0/4" & make backfill ARGS="--all --shard 1/4" &
make backfill ARGS="--all --shard 2/4" & make backfill ARGS="--all --shard 3/4" &
```

常用参数：`--mode full|incremental`（默认 full）、`--chunk N`（每次采集代码数，默认 50）、`--from/--to`（覆盖日期区间）、`--shard i/n`（多进程分片并行）、`--skip-done`（断点续跑）。Docker 环境可直接 `docker compose exec quant python -m app.scripts.backfill --all`。

### 4.2 日 K 采集字段（`query_history_k_data_plus`）

请求字段：`date,code,open,high,low,close,preclose,volume,amount,turn,tradestatus,pctChg,isST`，`adjustflag` 取 **2（前复权）** 落 `stock_daily_klines`。

| baostock 字段 | DB 字段（stock_daily_klines） | 说明 |
|---------------|------------------------------|------|
| open/high/low/close | open/high/low/close | 前复权价 |
| preclose | pre_close | 昨收（算涨跌停基准） |
| volume | volume | 成交量 |
| amount | amount | 成交额 |
| turn | turnover_rate | 换手率 % |
| pctChg | pct_chg | 涨跌幅 % |
| tradestatus | trade_status | 1 正常 / 0 停牌 |
| isST | is_st | 1 是 / 0 否（**逐日 point-in-time**） |
| _(自算)_ | limit_up / limit_down | 见 §5 |

> **停牌行**：baostock `tradestatus=0` 行 OHLC 可能等于停牌前价，量为 0；保留入库（`trade_status=0`），供回测识别不可成交。

### 4.3 复权因子采集（`query_adjust_factor`）

落新表 `stock_adjust_factors(market, code, trade_date, fore_factor, back_factor)`，供接入方做**可复现回测**自行复权（前复权基准会随除权漂移，回测更宜用不复权 + 因子 / 后复权）。

### 4.4 证券列表 + 退市 + ST 状态（`query_stock_basic` / `query_all_stock`）

落 `securities`：`code/name/board/status/list_date/delist_date/is_st`。

- `status`：1 上市 / 0 退市（`query_stock_basic` 的 `status` + `outDate` 判定）。
- 退市股**不删除**，标 `status=0` + `delist_date`，历史 K 线保留（避免幸存者偏差）。
- `board` 按代码段判定并补全**北交所**（`4`/`8` 开头、`bj.` 前缀）。
- `is_st`：当前是否 ST（最新名称含 ST/*ST）；逐日历史 ST 已在日 K 的 `is_st` 字段。

### 4.5 交易日历（`query_trade_dates`）

`collect_calendar` 调 baostock `query_trade_dates(start_date, end_date)` 拉区间内**每个自然日**及 `is_trading_day`，经 `map_calendar_row` 映射后 UPSERT 入 `trading_calendars`（`market`+`cal_date` 唯一键，`is_open` 标开/休市，`source='baostock'`）。

- `from_date` 留空用 `BACKFILL_START_DATE`；`to_date` 留空时**默认到当年年底**（`{今年}-12-31`），以拉全当年节假日——baostock `query_trade_dates` 在 `end` 为空时只返回到「最近一个交易日」，故必须显式给到年底。超出 baostock 已发布范围的未来日期会被自动截断，无副作用。
- 作为 Go 调度器「交易日感知」的权威源：Go 侧 `calendar` 作业（`SyncCalendar`）每月刷新、首次启动库内日历不足时自动 bootstrap；K 线增量另用实际成交日反推补登作兜底。

---

## 5. 涨跌停价自算（纯免费方案）

集中在 `core/limit_price.py` 纯函数，采集日 K 时逐行计算并写入 `limit_up`/`limit_down`。

```
基础比例 base_pct:
  主板（沪 60* / 深 000*/001*/003*）           → 10%
  创业板（300*/301*）、科创板（688*/689*）       → 20%
  北交所（8**/4**/bj.）                         → 30%

当日 ST（is_st=1）:
  主板 ST                                      → 5%
  创业板/科创板 ST                              → 仍 20%（注册制后与非 ST 相同）
  北交所 ST                                    → 仍 30%

涨跌停价（四舍五入到分）:
  limit_up   = round(pre_close × (1 + pct), 2)
  limit_down = round(pre_close × (1 - pct), 2)

特殊：新股上市首日 / 科创创业上市前 5 日不设涨跌停 → limit_up/limit_down 置 NULL（标记不约束）。
```

> 判定首日用 `securities.list_date` 与 `trade_date` 比对；`pre_close<=0` 或停牌（无昨收）时置 NULL。规则可单测覆盖（`tests/test_limit_price.py`）。

---

## 6. 指标计算（实时，不落库）

### 6.1 计算链路

```
Go /open/v1/.../indicators
  → POST quant /internal/v1/indicators { codes, period, adjust, types, limit|from|to }
      → quant 从 PG 读 stock_daily_klines（前复权，升序）
      → pandas DataFrame + pandas-ta / A 股自定义口径 计算
      → 返回 { code, trade_date, values:{type:value} } 或逐 bar 序列
  → Go 组装统一响应返回外部
```

- quant **自己读 PG 取 K 线**（已连库），避免 Go↔Python 间大数据传输。
- 指标计算**无状态、不写库**；point-in-time（逐 bar 用 `df.iloc[:i+1]`）保证无未来函数。

### 6.2 指标口径（对齐原 Go / 通达信）

| 指标 | type | 口径 |
|------|------|------|
| MA | `ma5/10/20/30/60`（period 可扩展） | 收盘 SMA |
| MACD | `macd_dif/macd_dea/macd_bar` | EMA12/26、DEA=DIF 的 EMA9、柱=(DIF−DEA)×2（通达信） |
| KDJ | `kdj_k/kdj_d/kdj_j` | RSV(9)；K=SMA(RSV,3,1)、D=SMA(K,3,1) 初值 50；J=3K−2D |
| RSI | `rsi6/rsi12/rsi24` | Wilder 平滑 |
| BOLL | `boll_mid/boll_upper/boll_lower` | MA20 ± 2×总体标准差 |
| ATR | `atr14/atr20` | 真实波幅 Wilder |
| 动量 | `pct_change20/60` | N 日涨跌幅 |
| 迁移因子 | `bias / vol_ratio / amplitude / ma_align …` | 与原 Go factor 口径一致 |

### 6.3 指标目录（Catalog）

`GET /internal/v1/catalog` 返回指标元数据数组（`type/name/group/value_type/params`），Go 的 `/open/v1/meta` 缓存透传。**注册即可见**（遍历计算注册表动态生成）。由于不再有快照，目录**移除 `snapshot` 字段**，新增说明"全部实时计算"。

---

## 7. 内部 API（被 Go 调用）

BasePath：`/internal/v1`，全部需 `X-Internal-Token`。

| Method | Path | 用途 | 请求体 |
|--------|------|------|--------|
| GET | `/health` | 健康检查（含 baostock 登录状态） | — |
| GET | `/internal/v1/catalog` | 指标目录 | — |
| POST | `/internal/v1/collect/securities` | 采集证券列表 + 上市/退市/ST | `{market?}` |
| POST | `/internal/v1/collect/calendar` | 采集官方交易日历（每个自然日开/休市）入 `trading_calendars` | `{market?, from_date?, to_date?}` |
| POST | `/internal/v1/collect/kline` | 采集一批代码的日 K + 复权因子 + 涨跌停 + ST + 停牌（`full` 全量历史；日 K 日常增量改由 Go gotdx 原生写库；带 `from/to` 时即「周级 baostock 对齐」用此端点覆盖区间、`source` 写 `baostock`） | `{codes:[..], mode:"full"\|"incremental", from?, to?}` |
| POST | `/internal/v1/indicators` | 批量实时指标（最新一日或指定日） | `{codes:[..], types:[..], period?, adjust?, trade_date?}` |
| POST | `/internal/v1/indicators/series` | 单只逐 bar 指标序列（K 线带指标用） | `{code, types:[..], period?, adjust?, limit?, offset?, from?, to?}` |

### 7.1 采集返回（供 Go 写作业进度/失败码）

```json
{
  "results": [
    {"code": "600000", "status": "ok",      "rows": 5, "latest_trade_date": "2026-06-09"},
    {"code": "688xxx", "status": "skipped",  "reason": "no_market_data"},
    {"code": "000xxx", "status": "failed",   "reason": "baostock error: ..."}
  ]
}
```

> Go 的 `JobRunner` 把 `status=failed` 计入 `failed_codes`、`skipped` 计入 `skipped_codes`，与现有作业编排完全兼容。

---

## 8. 数据库（与 Go 共享同一 PostgreSQL）

quant 服务与 Go 服务**共用同一个 PostgreSQL 实例与库**。表结构由 **Go 的 `init.sql` / AutoMigrate 统一维护**（单一事实源），quant 只做**读写数据**，不负责建表迁移（避免双写 schema 冲突）。涉及的表：

| 表 | quant 读 | quant 写 |
|----|:-------:|:-------:|
| `stock_daily_klines`（扩列后） | ✅（算指标） | ✅（采集） |
| `stock_adjust_factors`（新） | — | ✅（采集） |
| `securities`（扩列后） | ✅ | ✅（采集） |
| `trading_calendars` | ✅ | ✅（反推校正，可选） |

表结构变更详见 [`BACKEND.md`](./BACKEND.md) §3。

---

## 9. 部署与本地开发

### 9.1 本地开发（Python 直跑）

```bash
cd backend/python
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
make run                        # 读取 ../.env（与 go 共用），uvicorn 127.0.0.1:8000
make backfill ARGS="--all"      # 离线批量回补日 K（直接写库，见 §4.5）
```

- 本地仅绑 `127.0.0.1:8000`，不对外。Go 本地 `make run` 用 `QUANT_BASE_URL=http://127.0.0.1:8000` 调用。
- PostgreSQL/Redis 仍由 `backend/go` 的 `make infra-up` 启动，两服务共用。
- 环境变量统一在 `backend/.env`，无需单独的 `.env`。

### 9.2 线上 Docker

- `backend/deploy/docker-compose.yml` 含 `quant` 服务：`image: warden_stock_data-quant`，`build: ../python`，`depends_on: [postgres(healthy)]`。
- **不写 `ports`**（不映射宿主机），仅内网；Go 容器通过 `QUANT_BASE_URL=http://quant:8000` 访问。
- `env_file: ../.env`，`PG_HOST: postgres` 覆盖。
- `deploy.sh` 执行 `docker compose build backend quant` 后统一 `up -d`。

详见 [`BACKEND.md`](./BACKEND.md) §7。

---

## 10. 环境变量（统一 `backend/.env.example`，quant 相关项）

```bash
QUANT_PORT=8000
QUANT_ENV=dev
INTERNAL_TOKEN=change_me_internal_token   # 与 backend .env 同值，Go 调用校验

# PostgreSQL（与 backend 共用；Docker 部署时 PG_HOST 由 compose 覆盖为 postgres）
PG_HOST=localhost
PG_PORT=5432
PG_USER=postgres
PG_PASSWORD=postgres
PG_DB=warden_data
PG_SSLMODE=disable

# 采集
COLLECT_BAOSTOCK_RETRY=3
BACKFILL_START_DATE=1990-12-19            # 全量回补起始（A 股最早交易日；改大可缩短回补范围）
```

---

## 11. 测试（pytest，口径先行）

| 模块 | 重点 |
|------|------|
| `core/limit_price` | 各板块/ST/北交所/新股首日涨跌停价 |
| `features/indicator/compute` | KDJ/MACD/RSI/BOLL/ATR 与通达信/原 Go 口径一致；数据不足跳过；无未来函数 |
| `features/collect/mapper` | baostock 字段映射、代码格式互转、停牌/退市行处理 |

```bash
cd backend/python && make test    # pytest
```
