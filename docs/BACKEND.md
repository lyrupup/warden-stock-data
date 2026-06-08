# 守望者行情数据服务 · 后端技术开发文档

> Warden Stock Data Service · Backend Technical Design
>
> 文档版本：v1.0 ｜ 创建日期：2026-06-05
> 配套文档：[`PRD.md`](./PRD.md) · [`FRONTEND.md`](./FRONTEND.md) · [`openapi.yaml`](./openapi.yaml)
>
> 本文档是后端实现的唯一技术依据，遵循 PRD 模块划分（M1~M6）与项目 `AGENTS.md` 规范。开发遵循 **TDD（测试先行）**。

---

## 1. 技术栈与架构

### 1.1 技术栈

| 分类 | 选型 | 说明 |
|------|------|------|
| 语言/框架 | **Go 1.26 + Gin** | 高并发、强类型核心服务 |
| ORM | **GORM** | PostgreSQL 驱动，所有操作传播 context |
| 数据库 | **PostgreSQL 16** | K 线 / 快照 / 指标 / 凭证 / 作业 / 交易日历 |
| 缓存 | **Redis 7** | 行情缓存、限流、凭证配额计数、nonce 防重放、分布式锁 |
| 调度 | **robfig/cron v3** | M2 盘后定时全市场增量更新 |
| 行情数据源 | **bensema/gotdx**（直连通达信，A 股主力）；预留 Tushare / 港美股源 | 经 `IMarketProvider` 适配器接入，连接池封装 |
| 鉴权 | **JWT**（管理员后台）+ **HMAC-SHA256 签名**（开放 API 凭证） | 双鉴权体系 |
| 配置 | **viper** + 环境变量 | 不硬编码 |
| 校验 | **go-playground/validator** | DTO 参数校验 |
| 测试 | **testing + testify + go.uber.org/mock** | TDD、表驱动、Mock |
| 文档 | **OpenAPI 3.0**（openapi.yaml） | 前后端契约 |
| 加密 | **AES-GCM / bcrypt** | 数据源密钥加密；管理员密码与 secretKey 哈希 |

### 1.2 分层架构

```
                 ┌──────────── 管理 API (/admin) ──── Admin JWT 中间件
请求 → Router → Middleware ┤
                 └──────────── 开放 API (/open) ──── HMAC 凭证校验中间件（只读）
                                          ↓
                              Handler → Service → Repository → PostgreSQL
                                          ↓            ↓
                                     Cache(Redis)   Integration（行情源适配器）
                                          ↓            ↓
                                     Scheduler     Indicator（指标引擎）
```

| 层 | 职责 | 禁止 |
|----|------|------|
| Handler | 参数绑定/校验、调 Service、组装响应 | 业务逻辑、直连 DB |
| Service | 业务编排、事务、调 Repository/集成/指标引擎 | 处理 HTTP |
| Repository | DB CRUD（`WithContext`） | 业务逻辑 |
| Integration | 行情源适配器（gotdx…），实现 `IMarketProvider` | 业务逻辑 |
| Indicator | 技术指标纯函数计算（MA…） | I/O、DB |
| Scheduler | cron 调度 + 更新作业执行器 | 直接写 Handler 逻辑 |

### 1.3 目录结构

```
backend/
├── cmd/
│   ├── server/main.go         # HTTP 服务入口（装配路由 + 启动调度器）
│   └── backfill/main.go       # 一次性历史 K 线回补 CLI（首次 5 年回补）
├── internal/
│   ├── handler/               # admin/(后台) open/(开放API) 两组 handler
│   ├── service/               # market quote kline indicator credential job admin
│   ├── repository/            # kline quote indicator credential job calendar security
│   ├── model/                 # GORM 模型
│   ├── dto/                   # request/ response/
│   ├── middleware/            # adminauth hmacauth ratelimit quota timeout logger cors recovery
│   ├── integration/
│   │   └── market/            # provider.go(接口) gotdx_*.go factory.go
│   ├── indicator/             # indicator.go(接口+注册) ma.go 迁移因子 catalog.go
│   ├── scheduler/             # cron.go job_runner.go(分批/限速/续跑)
│   ├── mock/                  # mockgen 生成
│   └── router/router.go
├── pkg/
│   ├── errcode/ response/ database/ cache/ crypto/ signature/(HMAC) utils/
├── config/config.yaml
├── deploy/{docker-compose.yml,Dockerfile,init.sql}
├── test/                      # 集成测试
└── go.mod
```

> 遵循 `AGENTS.md`：`core` 可移植模块（indicator / signature / cache）与 `features` 业务解耦；具名导出；接口前缀 `I`。

### 1.4 统一响应与错误码

```go
// pkg/response
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
// Success: code=0；分页 data={list,total,page,size}
```

错误码段位：`10xxx` 通用、`20xxx` 行情/K线、`21xxx` 指标、`22xxx` 更新作业、`23xxx` 数据源、`40xxx` 管理员鉴权、`41xxx` 开放 API 凭证鉴权、`42xxx` 限流/配额。

| 错误码 | 含义 |
|--------|------|
| 10001 | 参数错误 |
| 10002 | 资源不存在 |
| 10408 | 请求超时（context 取消） |
| 20001 | 行情数据源异常（已降级返回 stale 快照） |
| 20002 | 股票不存在 |
| 21001 | 指标参数非法 |
| 22001 | 更新作业冲突（已有同类作业运行中） |
| 40001 | 管理员未登录 / token 非法 |
| 41001 | 缺少凭证签名头 |
| 41002 | secretId 不存在 / 已吊销 / 已过期 |
| 41003 | 签名校验失败 |
| 41004 | 时间戳过期 / nonce 重放 |
| 41005 | 凭证 scope 不足（试图访问非只读资源） |
| 42001 | 触发限流（QPS） |
| 42002 | 超出日调用配额 |

### 1.5 数值精度约定（重要）

所有**金额 / 价格 / 数量 / 比率 / 指标值**后端统一用 `shopspring/decimal.Decimal`（DB 为 `numeric`），其 `MarshalJSON` 序列化为**带引号的 JSON 字符串**（如 `"10.5000"`）。对应 openapi 的 `Decimal` schema（`type: string`）。前端必须经统一 decimal 工具转 number 后再计算，禁止直接算术 / `.toFixed()`。

---

## 2. 鉴权与权限设计（双体系）

> PRD 铁律：管理员可读写；接入方（Consumer）**只读、零写权限**。

### 2.1 管理员鉴权（管理 API `/admin`）

- 账号密码登录（`POST /admin/auth/login`），密码 **bcrypt** 哈希存储，校验通过签发 **JWT**（HS256，含 `admin_id`、`exp`）。
- 管理 API 经 `AdminAuth` 中间件：解析 Bearer JWT → 注入 `admin_id` 到 `gin.Context`。失败返回 `40001`。
- 预留多管理员与角色（`role` 字段），V1 单管理员够用。

### 2.2 开放 API 凭证鉴权（开放 API `/open`，HMAC 签名）

接入方请求需带以下 Header：

| Header | 说明 |
|--------|------|
| `X-Secret-Id` | 凭证公开标识 secretId |
| `X-Timestamp` | 请求 Unix 毫秒时间戳 |
| `X-Nonce` | 一次性随机串（防重放） |
| `X-Signature` | HMAC-SHA256 签名（Base64） |

**签名串（StringToSign）构造**：

```
StringToSign = METHOD + "\n" +
               PATH + "\n" +
               CanonicalQuery + "\n" +     // query 参数按 key 字典序排序后 a=1&b=2
               X-Secret-Id + "\n" +
               X-Timestamp + "\n" +
               X-Nonce + "\n" +
               SHA256Hex(RequestBody)      // GET 时 body 为空串的 sha256
Signature = Base64( HMAC_SHA256(secretKey, StringToSign) )
```

`HmacAuth` 中间件校验流程（见 `pkg/signature` + `middleware/hmacauth.go`）：

1. 取 `X-Secret-Id` → 查凭证（带缓存）；不存在 / 已吊销 / 过期 → `41002`。
2. 校验 `X-Timestamp` 与服务器时间偏差在 **±300s** 内，否则 `41004`。
3. 校验 `X-Nonce` 未在 Redis 出现过（`SETNX warden:nonce:{secretId}:{nonce}` TTL=300s），重放 → `41004`。
4. 用库内 secretKey 明文（**仅哈希存储用于展示校验，签名校验需可还原的密钥**，故 secretKey 经 AES-GCM 加密存储、内存解密参与签名）重算签名，比对 `X-Signature`，不一致 → `41003`。
5. 校验凭证 `scope` 含 `read`；开放 API 路由组本就不挂任何写路由 → 双重保证只读（`41005` 兜底）。
6. 注入 consumer 上下文（`consumer_id`、`scope`、限流配额），交由 `RateLimit`/`Quota` 中间件。

> **密钥存储取舍**：secretKey 需要参与服务端签名重算，故采用 **AES-GCM 可逆加密**存储（密钥来自 `CONFIG_ENC_KEY`），而非单向哈希；同时落一份 `secret_key_hash` 仅用于「创建后再次校验展示」。明文 secretKey **仅创建时返回一次**。

### 2.3 路由分组与权限边界

```go
// 开放 API：仅注册只读 GET 路由，经 HMAC + 限流 + 配额
open := r.Group("/open/v1")
open.Use(middleware.HmacAuth(...), middleware.RateLimitByCredential(...), middleware.Quota(...))
open.GET(...)   // 没有任何 POST/PUT/DELETE

// 管理 API：经 Admin JWT
admin := r.Group("/admin")
admin.POST("/auth/login", ...)          // 登录免鉴权
adminAuthed := admin.Group("", middleware.AdminAuth(...))
adminAuthed.GET/POST/PUT/DELETE(...)    // 写操作全部在此
```

---

## 3. 数据库设计文档

### 3.1 设计约定

- 表名小写下划线复数；金额/价格/指标 `NUMERIC(20,4)`，比率 `NUMERIC(10,4)`，量 `NUMERIC(20,0)`。
- 公共行情数据**不含 user_id**（本服务无 C 端用户）；统一含 `market`（CN/HK/US）与 `source`（gotdx…）维度。
- 软删除 `deleted_at`（管理类表）；行情明细表按 `(market, code, trade_date)` 唯一。
- 唯一约束使用命名约束 `CONSTRAINT uni_<table>_<cols> UNIQUE(...)`，与 GORM `uniqueIndex` 推断名对齐，避免 `AutoMigrate` DROP 不存在约束失败。
- `cmd/server/main.go` 逐个 `AutoMigrate(m)`，单表失败 `slog.Warn` 跳过；生产以 `init.sql` 为准。

### 3.2 ER 概览

```
公共行情（按 market/source 维度，无 user_id）
  data_sources                              数据源配置
  securities (market,code,name,status)      证券列表
  trading_calendars (market,date,is_open)   交易日历（自维护 + 反推校正）
  index_quotes / stock_quotes               实时快照（兜底降级）
  stock_daily_klines (market,code,date,OHLCV,adjust)  日 K 线序列
  stock_indicator_snapshots (market,code,date,values JSONB)  指标快照（全市场扫描底座）
  update_watermarks (market,code,last_trade_date)     增量更新水位
  update_jobs 1──N update_job_runs          更新作业 + 执行记录

管理与安全
  admins                                    管理员
  api_credentials                           接入凭证（secret_id/secret_key_cipher/scope/限流/配额）
  credential_access_logs                    凭证调用审计（按日聚合）
```

### 3.3 核心表结构（init.sql 摘要）

```sql
-- 数据源配置
CREATE TABLE IF NOT EXISTS data_sources (
    id BIGSERIAL PRIMARY KEY,
    source VARCHAR(32) NOT NULL,            -- gotdx / tushare ...
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    name VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 0,        -- 主备降级顺序，越小越优先
    config JSONB NOT NULL DEFAULT '{}',     -- 连接池/超时/地址池/加密 token
    health VARCHAR(16) NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_data_sources_market_source UNIQUE (market, source)
);

-- 证券列表
CREATE TABLE IF NOT EXISTS securities (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    code VARCHAR(16) NOT NULL,
    name VARCHAR(64) NOT NULL DEFAULT '',
    board VARCHAR(16) NOT NULL DEFAULT '',  -- 主板/创业板/科创板/北交所
    status SMALLINT NOT NULL DEFAULT 1,     -- 1 上市 0 退市/停牌
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_securities_market_code UNIQUE (market, code)
);
CREATE INDEX IF NOT EXISTS idx_securities_market ON securities(market);
-- 代码/名称模糊搜索走 trgm GIN 索引（ILIKE '%kw%' 不再全表扫描）
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_securities_code_trgm ON securities USING gin (code gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_securities_name_trgm ON securities USING gin (name gin_trgm_ops);

-- 交易日历（自维护 + gotdx K 线日期反推校正）
CREATE TABLE IF NOT EXISTS trading_calendars (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    cal_date DATE NOT NULL,
    is_open BOOLEAN NOT NULL DEFAULT FALSE, -- 是否交易日
    source VARCHAR(16) NOT NULL DEFAULT 'manual', -- manual / inferred
    CONSTRAINT uni_trading_calendars_market_date UNIQUE (market, cal_date)
);
CREATE INDEX IF NOT EXISTS idx_trading_calendars_market_open ON trading_calendars(market, is_open);

-- 个股日 K 线序列（前复权存储）
CREATE TABLE IF NOT EXISTS stock_daily_klines (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    source VARCHAR(32) NOT NULL DEFAULT 'gotdx',
    stock_code VARCHAR(16) NOT NULL,
    trade_date DATE NOT NULL,
    open NUMERIC(20,4), high NUMERIC(20,4), low NUMERIC(20,4), close NUMERIC(20,4),
    volume NUMERIC(20,0), amount NUMERIC(24,4),
    adjust VARCHAR(8) NOT NULL DEFAULT 'qfq',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_klines_market_code_date_adj UNIQUE (market, stock_code, trade_date, adjust)
);
CREATE INDEX IF NOT EXISTS idx_klines_code_date ON stock_daily_klines(market, stock_code, trade_date);

-- 个股技术指标快照（全市场扫描底座，指标值 JSONB 可扩展）
CREATE TABLE IF NOT EXISTS stock_indicator_snapshots (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL,
    trade_date DATE NOT NULL,
    values JSONB NOT NULL DEFAULT '{}',     -- {"ma5":..,"ma10":..,"ma20":..,"ma30":..,"ma60":..}
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_indi_snap_market_code_date UNIQUE (market, stock_code, trade_date)
);
CREATE INDEX IF NOT EXISTS idx_indi_snap_code_date ON stock_indicator_snapshots(market, stock_code, trade_date);

-- 增量更新水位
CREATE TABLE IF NOT EXISTS update_watermarks (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL,
    last_trade_date DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_watermark_market_code UNIQUE (market, stock_code)
);

-- 行情快照（兜底降级，沿用原系统）
CREATE TABLE IF NOT EXISTS index_quotes (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    index_code VARCHAR(16) NOT NULL, index_name VARCHAR(64) NOT NULL,
    price NUMERIC(20,4), change_amount NUMERIC(20,4), change_percent NUMERIC(10,4),
    volume NUMERIC(20,0), amount NUMERIC(24,4),
    trade_date DATE NOT NULL, snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_index_quotes_code_date ON index_quotes(index_code, trade_date);

CREATE TABLE IF NOT EXISTS stock_quotes (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL, stock_name VARCHAR(64) NOT NULL DEFAULT '',
    price NUMERIC(20,4), open NUMERIC(20,4), high NUMERIC(20,4), low NUMERIC(20,4), prev_close NUMERIC(20,4),
    change_percent NUMERIC(10,4), volume NUMERIC(20,0), amount NUMERIC(24,4), turnover_rate NUMERIC(10,4),
    trade_date DATE NOT NULL, snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_stock_quotes_code_date ON stock_quotes(stock_code, trade_date);

-- 更新作业
CREATE TABLE IF NOT EXISTS update_jobs (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    job_type VARCHAR(16) NOT NULL,          -- full / incremental / snapshot / indicator
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    cron_expr VARCHAR(64) NOT NULL DEFAULT '0 0 17 * * *', -- 默认每日 17:00
    batch_size INT NOT NULL DEFAULT 20,     -- 默认分批 20
    concurrency INT NOT NULL DEFAULT 10,    -- 默认并发 10
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 作业执行记录
CREATE TABLE IF NOT EXISTS update_job_runs (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'running', -- running/done/failed/canceled
    total INT NOT NULL DEFAULT 0,
    processed INT NOT NULL DEFAULT 0,
    succeeded INT NOT NULL DEFAULT 0,
    failed INT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    error_msg TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_job_runs_job ON update_job_runs(job_id, started_at);

-- 管理员
CREATE TABLE IF NOT EXISTS admins (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(64) NOT NULL,
    password_hash VARCHAR(128) NOT NULL,
    role VARCHAR(16) NOT NULL DEFAULT 'admin',
    status SMALLINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_admins_username UNIQUE (username)
);

-- 接入凭证
CREATE TABLE IF NOT EXISTS api_credentials (
    id BIGSERIAL PRIMARY KEY,
    secret_id VARCHAR(40) NOT NULL,
    secret_key_cipher TEXT NOT NULL,        -- AES-GCM 加密的 secretKey（签名重算用）
    secret_key_hash VARCHAR(128) NOT NULL,  -- 哈希（仅校验展示）
    consumer_name VARCHAR(128) NOT NULL,
    scope VARCHAR(32) NOT NULL DEFAULT 'read',
    rate_limit INT NOT NULL DEFAULT 20,     -- QPS
    daily_quota INT NOT NULL DEFAULT 100000,
    status SMALLINT NOT NULL DEFAULT 1,     -- 1 启用 0 吊销
    expire_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_api_credentials_secret_id UNIQUE (secret_id)
);

-- 凭证调用审计（按日聚合）
CREATE TABLE IF NOT EXISTS credential_access_logs (
    id BIGSERIAL PRIMARY KEY,
    credential_id BIGINT NOT NULL,
    stat_date DATE NOT NULL,
    call_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    last_access_at TIMESTAMPTZ,
    CONSTRAINT uni_access_log_cred_date UNIQUE (credential_id, stat_date)
);
```

### 3.4 GORM 模型示例

```go
// internal/model/kline.go
type StockDailyKline struct {
    ID        uint            `gorm:"primarykey" json:"id"`
    Market    string          `gorm:"size:8;not null;default:'CN'" json:"market"`
    Source    string          `gorm:"size:32;not null;default:'gotdx'" json:"source"`
    StockCode string          `gorm:"size:16;not null;index:idx_klines_code_date" json:"stock_code"`
    TradeDate time.Time       `gorm:"type:date;not null;index:idx_klines_code_date" json:"trade_date"`
    Open      decimal.Decimal `gorm:"type:numeric(20,4)" json:"open"`
    High      decimal.Decimal `gorm:"type:numeric(20,4)" json:"high"`
    Low       decimal.Decimal `gorm:"type:numeric(20,4)" json:"low"`
    Close     decimal.Decimal `gorm:"type:numeric(20,4)" json:"close"`
    Volume    decimal.Decimal `gorm:"type:numeric(20,0)" json:"volume"`
    Amount    decimal.Decimal `gorm:"type:numeric(24,4)" json:"amount"`
    Adjust    string          `gorm:"size:8;not null;default:'qfq'" json:"adjust"`
}
func (StockDailyKline) TableName() string { return "stock_daily_klines" }
```

> **分时走势（实时透传，不落库）**：`StockIntraday` / `IntradayPoint` 为纯领域对象（无 GORM 表），由 `QuoteService.Intraday` 经 `IMarketProvider.Intraday` 调 gotdx `StockTickChart` 拉当日分时图（价格 / 均价 / 该分钟成交量）并补一次盘口快照取昨收。gotdx 分时点本身**不含时间戳**，按索引推算 A 股交易分钟（上午 09:30–11:30、下午 13:00–15:00 共 240 点），时区固定东八区。
>
> **最近交易日 + 历史回退**：先以「日 K 最后一根的日期」反推最近交易日（`latestTradeDate`）；当日分时为空（**非交易日 / 盘前**）时自动回退到该交易日的历史分时 `StockHistoryTickChart`，响应中的 `trade_date` 即实际返回数据所属交易日。前端无需感知是否交易日，直接按 `trade_date` 展示。

---

## 4. API 接口文档

> 机器可读契约见 [`openapi.yaml`](./openapi.yaml)。本节为人读摘要，二者保持一致。
>
> 两套 BasePath：管理 API `/admin`（Admin JWT）；开放 API `/open/v1`（HMAC 凭证，**纯只读**）。

### 4.1 通用约定

- 响应体：`{ "code": 0, "message": "ok", "data": ... }`；分页 `data={list,total,page,size}`。
- 时间 ISO8601；数值字段为 decimal 字符串（见 §1.5）。
- 开放 API 必带 HMAC 签名头（见 §2.2）；管理 API 必带 `Authorization: Bearer <jwt>`。

### 4.2 管理 API（`/admin`，Admin JWT）

**认证**

| Method | Path | 说明 |
|--------|------|------|
| POST | `/admin/auth/login` | 管理员登录 `{username,password}` → `{token}` |
| GET | `/admin/auth/me` | 当前管理员信息 |

**凭证管理（M5）**

| Method | Path | 说明 |
|--------|------|------|
| GET | `/admin/credentials` | 凭证列表（含调用量、状态） |
| POST | `/admin/credentials` | 创建凭证 `{consumer_name,rate_limit,daily_quota,expire_at}` → **一次性返回 secretKey 明文** |
| GET | `/admin/credentials/:id` | 凭证详情 + 调用审计 |
| POST | `/admin/credentials/:id/rotate` | 轮换 secretKey（一次性返回新明文） |
| PUT | `/admin/credentials/:id` | 修改配额 / 启停 |
| DELETE | `/admin/credentials/:id` | 吊销凭证 |

**数据源与更新作业（M2/M1）**

| Method | Path | 说明 |
|--------|------|------|
| GET | `/admin/datasources` | 数据源列表 + 健康状态 |
| PUT | `/admin/datasources/:id` | 配置数据源（启停 / 连接池 / 优先级） |
| POST | `/admin/datasources/:id/healthcheck` | 触发连通性探测 |
| GET | `/admin/jobs` | 更新作业列表 |
| PUT | `/admin/jobs/:id` | 配置作业（名称 / 市场 / cron / 分批 / 并发 / 启停；`job_type` 与时间字段不可改，`cron_expr` 服务端校验格式） |
| POST | `/admin/jobs/:id/run` | 手动触发一次（`{type,market,codes?}`） → `{runId}` |
| POST | `/admin/jobs/runs/:runId/cancel` | 取消运行中作业 |
| GET | `/admin/jobs/runs` | 执行记录（分页，含进度） |
| GET | `/admin/jobs/runs/:runId` | 单次执行详情 / 进度 |
| GET | `/admin/freshness` | 数据新鲜度（全市场更新到哪个交易日、最近扫描时间、已入库行情股票数 `kline_stock_count`）。性能：最新交易日走 `(market, trade_date)` 复合索引；`kline_stock_count` 取 `update_watermarks` 行数（每股一行），避免在数百万行 K 线上做 `MAX` 全表扫描与 `COUNT(DISTINCT)` |

> 管理 API 也可调用全部开放 API 的只读能力用于后台行情展示（后台前端直接复用 `/open/v1` 的 service 层或带管理员上下文的内部只读接口）。

### 4.3 开放 API（`/open/v1`，HMAC 凭证，只读）

| Method | Path | 说明 |
|--------|------|------|
| GET | `/open/v1/indices` | 大盘指数（`?market=CN`） |
| GET | `/open/v1/quotes?codes=600000,000001` | 批量个股快照 |
| GET | `/open/v1/stocks/:code` | 单只个股快照 |
| GET | `/open/v1/stocks/:code/kline?period=day&adjust=qfq&limit=120` | K 线（支持 `from`/`to` 交易日区间，回测取历史区间用） |
| GET | `/open/v1/stocks/:code/intraday` | 分时走势（实时透传：价格线 + 均价线 + 分时量；不落库；非交易日自动回退最近交易日，`trade_date` 标注实际日期） |
| GET | `/open/v1/stocks/:code/indicators?types=ma5,ma10,ma20,ma30,ma60` | 单只实时指标 |
| GET | `/open/v1/indicators?codes=...&types=ma5,ma60&trade_date=` | 批量指标（读快照） |
| GET | `/open/v1/search?kw=` | 股票搜索（优先查本地 `securities` 表 + trgm 索引；库空时回源行情提供方） |
| GET | `/open/v1/securities?market=CN` | 证券列表 |
| GET | `/open/v1/meta` | 市场列表 / 指标目录 / 数据新鲜度 |

> 开放 API 路由组**不存在任何 POST/PUT/DELETE**（权限边界第一道防线，可由 `router_test.go` 断言）。

---

## 5. 核心功能模块设计方案

### M1 数据源适配与多市场接入

- **接口（迁移自原系统并扩展 `market` 与 `StockList`）**：

```go
// internal/integration/market/provider.go
type IMarketProvider interface {
    Market() string  // CN / HK / US
    Source() string  // gotdx / tushare ...
    Indices(ctx context.Context) ([]model.IndexQuote, error)
    Quotes(ctx context.Context, codes []string) ([]model.StockQuote, error)
    Kline(ctx context.Context, code, period, adjust string) ([]model.Kline, error)
    Search(ctx context.Context, kw string) ([]model.StockBrief, error)
    StockList(ctx context.Context) ([]model.StockBrief, error)  // 全市场证券列表（回补/扫描用）
    HealthCheck(ctx context.Context) error
}
```

- **工厂 + 主备降级链**：

```go
// 按 market 选主源，失败回退备源（priority 排序，均实现 IMarketProvider）
func NewProvider(market string, sources []DataSourceConfig) IMarketProvider {
    // gotdx(CN) 主；可包一层 fallbackProvider 串联备源
}
```

- **gotdx 适配器**：`gotdx_pool.go`（连接池，见下）、`gotdx_provider.go`（`safeCall` 防 panic、懒加载证券名索引、`withClient(ctx, fn)` 借池执行）、`gotdx_mapper.go`（价格 /100、/1000 量纲还原，换手率 /10000）。`StockList` 基于 `StockAll(market)` 并过滤为 A 股个股。
- **单元测试**：以测试内置的 fake provider（`internal/service/fake_provider_test.go`）注入确定性行情数据，不依赖外部网络。
- **扩展**：新增市场/源 = 新增实现 `IMarketProvider` 的适配器 + 工厂注册，零侵入 M2/M3/M4。

#### gotdx 连接池（`internal/integration/market/gotdx_pool.go`）

通达信 `gotdx.Client` 是有状态 TCP 连接，单次 `Connect()` 握手实测约 1-4s。早期实现每请求新建/断开连接，导致缓存未命中时接口动辄数秒。连接池复用已建立连接，消除重复握手。

- **容量模型**：`tokens`（带缓冲 channel，cap=maxConn 且初始填满）为「总连接配额」，持有 token 才能新建连接，把同时存在的连接数严格限制在 `maxConn`；`idle` 缓存空闲健康连接，复用期间持有其 token。任意时刻 `idle 连接 + 借出连接 = 已创建连接 = maxConn - len(tokens) ≤ maxConn`。
- **借还**：`Acquire` 优先复用 `idle`，无空闲则消费 token 新建，配额耗尽时阻塞等待归还或 `ctx` 取消/超时；`Release` 对健康连接放回 `idle`，对出错/panic 连接 `Disconnect` 并归还 token，由后续请求重建（通达信节点不稳定，坏连接续用会读写错位，丢弃最稳妥）。
- **健康判定**：`safeCall` 捕获 fn 的 error 与 panic；任一发生即视为连接不可复用。`WithClient` 对失败最多重试 3 次（每次取/建新连接）。
- **并发**：`idle` / `tokens` 均为 channel，天然并发安全；移除了旧实现对所有 TDX 请求的全局串行锁，缓存未命中时可并发使用至多 `maxConn` 条连接。
- **生命周期**：`maxConn` 由 `MARKET_GOTDX_MAX_CONN`（默认 10）配置；进程关停时经 `FallbackProvider.Close()` → `GotdxProvider.Close()` → `GotdxPool.Close()` 断开空闲连接。

模块接口：

```go
type GotdxPool struct { /* idle / tokens / newClient ... */ }

func NewGotdxPool(maxConn int) *GotdxPool
func (p *GotdxPool) WithClient(ctx context.Context, fn func(*gotdx.Client) error) error
func (p *GotdxPool) Close()

// GotdxProvider 借池执行（各 IMarketProvider 方法内部调用）
func (p *GotdxProvider) withClient(ctx context.Context, fn func(*gotdx.Client) error) error
func (p *GotdxProvider) Close() error
```

### M2 存储与增量更新调度（性能核心）

- **取数链路（读，迁移自原系统）**：`Redis 命中 → Provider → stock_quotes 快照兜底(stale)`，TTL 盘中短、盘后长。
- **指数行情**：`Redis 命中（TTL 5min）→ index_quotes 库内最新快照快速返回 + 后台 singleflight 刷新 TDX → 仅库空时同步拉取`。避免 Redis 过期后每次新建 TDX 连接阻塞首页 ~4s。指数目录由 `cnIndexCatalog()` 统一维护（沪/深/北：上证指数、上证50、沪深300、科创综指、科创50、中证500/1000、深证成指、创业板指/50、深证100、中小100、北证50 等），新增指数仅需在该目录追加；映射经 `mapIndexQuote` 完整填充 `price/change_amount/change_percent/volume/amount`。
- **增量更新（写）**：

```
全市场增量（job_type=incremental）前置筛选:
  latest = MAX(stock_daily_klines.trade_date)   // 全市场最新交易日；库空则全量
  pending = securities 中满足「无 watermark 或 watermark < latest」的代码
            // 已是最新的标的直接跳过，不再请求数据源

对每只待更新标的:
  wm = watermark(market, code)            // 上次更新到的交易日
  bars = provider.Kline(code, "day", "qfq")  // gotdx 取最新 N 根
  newBars = bars[trade_date > wm]          // 仅取水位之后
  upsert stock_daily_klines(newBars)       // 有新数据才写
  // 水位对齐到「数据源可得最新」与 latest（全市场最新交易日）的较新者：
  watermark = max(bars 最新日期, latest)
```

> **按 watermark 筛待更新（性能）**：`incremental` 类型且未显式指定 `codes` 时，`JobRunner.Run` 先用 `UpdateService.PendingCodes` 以「库内 K 线最大交易日」为基准过滤，仅对落后/新股发起数据源请求；盘后多数标的已最新时，请求量可从全市场约 5200 次降到个位/十位数。`full`/`indicator` 类型与显式指定 `codes` 不参与该过滤。

> **水位对齐避免重复拉取（停牌/次新股）**：`IncrementalOne` 拉取成功后，即便本轮无新 K 线，也将 `watermark` 推进到「数据源可得最新日期」与「全市场最新交易日 `marketLatest`」中的较新者。否则停牌 / 尚未复牌的次新股因个股最新交易日永远落后于全市场，会被每轮增量反复选中、无效拉取（症状：增量 `pending` 数始终不归零）。对齐后这些个股不再被选中，直到全市场出现新交易日才会重试。`marketLatest` 由 `JobRunner.Run` 调 `UpdateService.MarketLatestTradeDate` 取一次后传入逐只处理。

> **作业类型**：`incremental`（增量 K 线+指标）、`full`（全量回补 K 线+逐日指标）、`indicator`/`snapshot`（仅指标）、`securities`（**仅同步证券列表**：调一次 `SyncSecuritiesList` 发现新股/改名，几秒完成，不触达 K 线接口，作为新股入库的轻量入口）。默认作业由 `EnsureDefaults` 按 `(job_type, market)` 幂等补建（`daily-incremental` 17:00、`securities-sync` 8:30），存量库也会自动补上后加入的默认作业。

- **全市场扫描计算**（补齐原系统遗留缺口）：拉全市场 `securities` → 并发（信号量 channel + WaitGroup，迁移自原粗筛引擎）逐只拉 K 线 → 调 M3 指标引擎算默认指标集合 `defaultSnapshotTypes` → upsert `stock_indicator_snapshots`。单只失败跳过不中断，`ctx` 取消即时退出。
  - **默认落库指标集合（回测友好）**：`ma5/10/20/30/60` + `macd_dif/dea/bar` + `kdj_k/d/j` + `rsi6/12/24` + `boll_mid/upper/lower` + `atr14/atr20` + `pct_change20/60`（见 `internal/service/update_service.go:defaultSnapshotTypes`）。指标值统一存 `values`（JSONB），新增指标无需改表结构；接入方做量化回测时按 `trade_date` 直接读 point-in-time 历史指标，无需重算。
  - **数据不足处理**：`ComputeAll` 任一指标数据不足即跳过该日快照（早期不足 60 根的历史日因 MA60 无法计算自然跳过），保证 point-in-time 一致；其余长尾指标（如 bias、vol_ratio）不入默认快照，走单只实时接口按需计算。
  - **JSONB 合并写入（避免子集扫描丢字段）**：`IndicatorRepository.UpsertSnapshot` 冲突时按 `values = COALESCE(已有,'{}') || excluded.values` **JSONB 合并**（同键以新值为准），而非整行覆盖。好处：① 用更小的 `types` 子集再次扫描不会抹掉该日已落库的其它指标；② 新增指标可增量补字段，无需整库回填即可逐步补齐（但要把历史区间一次性补齐仍建议跑 `full` 作业逐日回算）。注意：若从默认集合移除某指标，合并语义会**保留其历史旧值**（陈旧键），如需清除需单独处理。
  - **快照更新触发**：日常由默认 `daily-incremental`（17:00）逐日写「最新交易日」快照（`ScanIndicators`）；要为历史区间补齐新增指标，需跑 `full` 作业 / `cmd/backfill`（`BackfillIndicators` 逐日 point-in-time 回算）。`indicator`/`incremental` 作业只更新最新一天。
- **逐历史交易日指标快照（point-in-time，回测友好）**：`stock_indicator_snapshots` 以 `(market, code, trade_date)` 为唯一键，每个历史交易日一行。扫描支持两种模式：
  - **增量模式**（盘后默认）：只为「最新交易日」算并 upsert 一行。
  - **回补模式**（首次 / 指定区间）：对 `[from, to]` 内每个交易日 `t`，用截止 `t` 的 K 线序列切片 `Bars[0:idx(t)+1]` 计算指标并 upsert，**无未来函数**。接入方回测时按 `trade_date` 直接读历史指标，无需重算。

```go
// 逐日回补：对一只标的的完整 K 线序列，按日切片滚动计算指标快照
func (s *scanService) backfillIndicators(code string, bars []model.Kline) {
    for i := range bars {                       // bars 升序
        series := factor.Series{Bars: bars[:i+1]} // 截止第 i 日，point-in-time
        vals := indicator.ComputeAll(series, maTypes) // ma5/10/20/30/60
        s.repo.UpsertSnapshot(code, bars[i].Date, vals)
    }
}
```
- **盘后定时分批批量更新**（PRD 决策：默认 17:00 / 分批 20 / 并发 10）：

```go
// internal/scheduler/cron.go
c := cron.New(cron.WithSeconds())
c.AddFunc(job.CronExpr /*默认 "0 0 17 * * *"*/, func() {
    if !calendar.IsTradingDay(today) { return }   // 交易日历感知，非交易日跳过
    runner.RunIncremental(ctx, job)               // 分批 batch=20、并发=10、限速
})
```

  - **分批 + 限速**：把全市场代码切成 `batch_size=20` 一批，批间 sleep 限速，批内 `concurrency=10` 并发，规避数据源限频/封禁。
  - **断点续跑**：`update_job_runs.processed` 记进度，失败批重试；进程重启可从 watermark 续跑（天然幂等：upsert + 水位）。
  - **同类型排队（FIFO）**：手动触发 / 定时触发统一走 `JobRunner.Submit`。若同 `job_type` 已有 `running`，新作业落 `status=waiting` 入队而非并行启动；当前作业终态后 `promoteNext` 自动提升队首 waiting 执行。`waiting` 作业可被用户取消（无执行 goroutine，直接置 `canceled`）。`update_job_runs` 增加 `job_type`/`market` 字段以支撑队列定位与接力执行。
  - **启动恢复**：进程启动先把残留 `running` 置 `failed`（孤儿清理），再对存在 `waiting` 的每个类型 `ResumeWaiting` 提升队首，避免重启后队列卡死。
- **交易日历**：自维护表为主（按年导入交易所休市安排），每次增量更新用「实际拉到的 K 线最大日期」反推校正 `trading_calendars`（`source='inferred'`），补漏临时休市。

### M3 技术指标计算

- **接口与注册（可扩展）**：

```go
// internal/indicator/indicator.go
type IIndicator interface {
    Type() string                              // ma5 / ma10 / macd ...
    Compute(s Series, params Params) (Value, error)
}
var registry = map[string]IIndicator{}
func Register(i IIndicator) { registry[i.Type()] = i }
```

- **MA（V1 交付）**：迁移原系统 `factor.MA`，注册 ma5/ma10/ma20/ma30/ma60（period 可参数化）。纯函数、表驱动单测覆盖。
- **迁移因子**：`ma_align / bias / amplitude / amplitude_streak / pct_change / field / vol_ratio` 全部迁移，注册进 catalog 供 `/open/v1/meta` 暴露。
- **经典技术指标（已落地，量化策略/回测刚需）**：在均线/迁移因子基础上补齐以下指标，均经 `IIndicator` 接口注册、纯函数、表驱动单测覆盖（`internal/indicator/{macd,rsi,kdj,boll,atr}.go` + `oscillators_test.go`）。多值指标按子项拆为独立 `type` 注册，契合单值 `Compute` 接口与 JSONB 扁平快照：

  | 指标 | 注册 type | 口径/默认参数 |
  |------|-----------|---------------|
  | MACD | `macd_dif` `macd_dea` `macd_bar` | EMA12/EMA26、DEA=DIF 的 EMA9、柱=(DIF-DEA)×2（通达信口径） |
  | KDJ | `kdj_k` `kdj_d` `kdj_j` | RSV(9)；K=SMA(RSV,3,1)、D=SMA(K,3,1) 初值 50；J=3K-2D |
  | RSI | `rsi6` `rsi12` `rsi24` | Wilder 平滑；avgLoss=0 时 RSI=100 |
  | BOLL | `boll_mid` `boll_upper` `boll_lower` | 中轨 MA20、上/下轨 ±2×收盘价总体标准差 |
  | ATR | `atr14` `atr20` | 真实波幅 Wilder 平滑，用于动态止损/轨道宽度 |
  | 动量 | `pct_change20` `pct_change60` | 中长期 N 日涨跌幅 |

- **扩展预留**：新增指标 = 实现 `IIndicator` 接口 + `Register` + 在 `catalog.go` 的 `metaTable` 追加元数据，零侵入 M2/M4；多值指标继续按子项拆 type。
- **指标目录动态派生（接入方发现能力的单一事实源）**：`Catalog()`（`internal/indicator/catalog.go`）**遍历 registry 动态生成**——「注册即可见」，避免手写常量与实现漂移。每项含：
  - `type` / `name` / `group`（分类）/ `value_type`（number\|bool）/ `params`（计算口径，如 `{fast:12,slow:26,signal:9}`）；
  - `implemented`：由「是否已注册」派生，恒为 true；
  - `snapshot`：由「是否在 `DefaultSnapshotTypes`」派生，标识该指标**能否经 `/open/v1/indicators` 批量按交易日 point-in-time 读取**（回测构建策略的关键判断）。
- **默认快照指标集合下沉指标包**：`indicator.DefaultSnapshotTypes` 统一维护盘后逐日快照默认落库的指标集合，service（`update_service.go`）直接引用，避免重复定义；`/open/v1/meta` 额外暴露 `default_snapshot_types`，接入方一眼可知哪些指标可按日回放。
- **`/open/v1/meta` 响应**（`MetaService.Meta`）：`markets` + `indicators`（目录）+ `default_snapshot_types` + `freshness`。`indicators` 与 `default_snapshot_types` 即接入方发现「支持哪些指标、哪些可批量回放」的入口。
- **两种时机**：默认读 `stock_indicator_snapshots`（M2 盘后批量算好，高并发只读）；未入快照或单只明细时实时拉 K 线计算，结果一致。
- **归属边界（重要，见 PRD M3）**：本服务**只做因子「计算」并对外输出数值**；原系统 `rule` 规则组合引擎（`left op right` + and/or 选股条件求值）**不迁入本服务、不暴露对外 API**——它属于「怎么用因子」的业务策略层，归接入方。即使 `ma_align`/`amplitude_streak` 等布尔因子也只是确定性计算，仍属计算层留在本服务。

### M4 行情数据开放 API

- Handler 仅做绑定 + 调 service + 组装；service 复用 M1/M2/M3。
- 指标接口：`?types=ma5,ma60` → 优先快照、缺失则实时算；批量接口强制走快照（性能）。
- **K 线带指标**：`/stocks/{code}/kline?indicators=ma5,macd_bar,...` 传入 `indicators` 时，额外返回与 bars 按 `trade_date` 对齐的**逐 bar 指标**，响应体变为 `{bars, indicators}`（不传则仍只返回 bars 数组，向后兼容）。由 `IndicatorService.KlineIndicators` 实现「**快照优先 + 实时补齐**」：日 K + 前复权且指标在默认快照集合内 → 读 `stock_indicator_snapshots` 区间快照（`GetSnapshotsRange`，含完整历史预热、口径统一）；周/月 K、后复权、非默认指标（如 `ma120`）或快照缺口 → 在返回 bars 上逐 bar `bars[:i+1]` point-in-time 实时计算补齐。单指标数据不足则该 bar 跳过该指标、不影响其它指标。供前端 K 线图叠加 MA/BOLL 与 MACD/KDJ/RSI/ATR/动量副图绘制（前端不再手算）。
- 所有接口透传 `ctx`（超时中断），降级返回 stale。

### M5 鉴权与凭证管理

- `pkg/signature`：`Sign(secretKey, stringToSign)` / `Verify(...)` 纯函数，TDD 覆盖（正常 / 篡改 / 过期 / 重放）。
- secretId 生成：`AKID` 前缀 + 32 位随机；secretKey：48 位高熵随机，AES-GCM 加密落库 + 哈希落库，明文仅创建/轮换返回一次。
- 凭证缓存：secretId → 凭证（含解密 secretKey）缓存于内存/Redis，TTL 60s，吊销时主动失效。

### M6 管理后台

- 后端仅提供管理 API；前端实现见 [`FRONTEND.md`](./FRONTEND.md)。

### M7 分时交易研判（做 T，前端计算 / 后端供数）

- **后端职责**：仅提供研判所需数据——分时序列（复用 M1 `StockTickChart` → M4 分时接口，逐分钟 价/均价/量 + 昨收）与历史日 K（M4 K 线接口）。本期**不新增任何研判计算接口与持久化**。
- **计算归属**：做 T 研判（趋势态、是否适合做 T、高抛低吸 B/S、轨道、盘中可靠度）属「怎么用数据」的策略层，与 M3 因子计算同理对待——**放在前端纯函数**实现并由 vitest 覆盖（见 [`MARKET_DATA.md`](./MARKET_DATA.md) §4.10 与 `FRONTEND.md`），后端不承载。
- **逐分钟无未来函数**：研判仅用「当前及之前」的分时点，盘中实时与收盘后在同一时刻结论一致；历史 ATR 等基准来自昨日数据，稳定可靠。
- **扩展点（预留，未实现）**：若未来需服务端统一口径，可将 `atr20`、`day_amplitude` 等做 T 基准指标下沉为 M3 因子并经 M2 盘后批量落库，前端改读快照；研判主体仍可留前端。

---

## 6. TDD 驱动测试文档

### 6.1 流程与组织

- **测试先行**：先写 `_test.go`（红）→ 实现（绿）→ 重构。
- Service 用 mockgen 生成的 mock 替换 Repository / Provider，**禁止单测真实外呼**。
- 纯函数（指标、签名、增量水位计算、交易日历推断）必须表驱动覆盖。

### 6.2 测试矩阵（按模块）

| 模块 | 测试重点 | 类型 |
|------|---------|------|
| M1 适配 | mapper 量纲换算、工厂选源、连接池复用 | 单元 |
| M1 连接池 | 健康连接复用、坏连接/ panic 丢弃重建、配额守恒、ctx 取消（`gotdx_pool_test.go`） | 单元 |
| M2 增量 | 水位推进、仅取水位后数据、幂等 upsert、批次切分、ctx 取消退出 | 单元 |
| M2 调度 | 交易日历感知（非交易日跳过）、分批限速、续跑 | 单元 + 集成 |
| M2 扫描 | 全市场并发不中断、指标快照落库 | 单元 |
| M3 指标 | MA5/10/20/30/60 数值、数据不足、迁移因子 | 表驱动单元 |
| M5 签名 | 正常 / 篡改 / 时间戳过期 / nonce 重放 / 吊销 / scope | 单元 |
| M4 API | 鉴权拦截、只读路由断言（无写路由）、降级 stale | Handler + 集成 |

### 6.3 指标表驱动测试模板

```go
func TestMA(t *testing.T) {
    bars := buildBars([]float64{10, 11, 12, 13, 14}) // close 序列
    cases := []struct{ period int; want string; err bool }{
        {5, "12", false},
        {6, "", true}, // 数据不足
    }
    for _, c := range cases {
        v, err := indicator.MA(factor.Series{Bars: bars}, c.period)
        if c.err { require.Error(t, err); continue }
        require.NoError(t, err)
        require.Equal(t, c.want, v.String())
    }
}
```

### 6.4 签名校验测试要点

```go
func TestVerify(t *testing.T) {
    // 正常签名通过；改 body/path/query → 失败；ts 偏移 >300s → 过期；同 nonce 二次 → 重放拒绝
}
```

### 6.5 覆盖率

- `make test` 跑全量；核心包（indicator / signature / scheduler 增量逻辑）覆盖率 ≥ 80%。

---

## 7. 基础设施与部署

### 7.1 docker-compose（节选）

```yaml
services:
  postgres: { image: postgres:16, environment: [...], volumes: [./deploy/init.sql:/docker-entrypoint-initdb.d/init.sql] }
  redis:    { image: redis:7 }
  backend:  { build: ./backend, depends_on: [postgres, redis], env_file: ./backend/.env, ports: ["8080:8080"] }
```

### 7.2 关键环境变量

```bash
APP_PORT=8080
APP_ENV=dev
JWT_SECRET=change_me                 # 管理员 JWT
CONFIG_ENC_KEY=32_bytes_key_for_aes_gcm________  # secretKey/数据源 token 加密

# PostgreSQL / Redis
PG_HOST=localhost PG_PORT=5432 PG_USER=postgres PG_PASSWORD=postgres PG_DB=warden_data PG_SSLMODE=disable
REDIS_HOST=localhost REDIS_PORT=6379 REDIS_PASSWORD= REDIS_DB=0

# 行情数据源
MARKET_PROVIDER=gotdx                # gotdx(通达信，唯一行情源)
MARKET_GOTDX_MAX_CONN=10             # gotdx 连接池（与扫描并发对齐）

# 盘后定时更新默认（可被 DB job 配置覆盖）
JOB_CRON=0 0 17 * * *                # 每日 17:00
JOB_BATCH_SIZE=20
JOB_CONCURRENCY=10
BACKFILL_YEARS=5                     # 首次历史回补 5 年

# HMAC 签名
SIGN_TS_SKEW_SEC=300                 # 时间戳允许偏差
SIGN_NONCE_TTL_SEC=300               # nonce 防重放窗口
```

### 7.3 中间件装配顺序

```
管理 API： Recovery → Logger → CORS → RateLimit(全局) → Timeout(ctx) → AdminAuth
开放 API： Recovery → Logger → CORS → RateLimit(全局) → Timeout(ctx) → HmacAuth → RateLimitByCredential → Quota
```

> 全市场扫描 / 历史回补由 CLI（`cmd/backfill`）或调度器执行，不走 HTTP timeout；开放/管理慢接口（批量指标）可在 timeout 中间件做路由级超时覆盖。

---

## 8. 并行开发约定（后端视角）

- 接口契约以 `openapi.yaml` 为准；新增/改动接口先改契约再实现。
- 分支按模块：`feat/m1-provider`、`feat/m2-scheduler`、`feat/m3-indicator`、`feat/m5-credential` 等。
- 任一核心功能变更需同步更新 PRD / BACKEND / FRONTEND / openapi / README（见 `AGENTS.md`）。
- 迁移自 `warden-stock-trading` 的代码（适配层 / 因子引擎）保留其单测，按本服务包路径调整 import。
