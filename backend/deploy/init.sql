-- 守望者行情数据服务 · 数据库初始化（与 BACKEND.md / GORM 模型对齐）

CREATE TABLE IF NOT EXISTS data_sources (
    id BIGSERIAL PRIMARY KEY,
    source VARCHAR(32) NOT NULL,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    name VARCHAR(64) NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    priority INT NOT NULL DEFAULT 0,
    config JSONB NOT NULL DEFAULT '{}',
    health VARCHAR(16) NOT NULL DEFAULT 'unknown',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_data_sources_market_source UNIQUE (market, source)
);

CREATE TABLE IF NOT EXISTS securities (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    code VARCHAR(16) NOT NULL,
    name VARCHAR(64) NOT NULL DEFAULT '',
    board VARCHAR(16) NOT NULL DEFAULT '',  -- 主板/创业板/科创板/北交所
    status SMALLINT NOT NULL DEFAULT 1,     -- 1 上市 / 0 退市
    list_date DATE,                         -- 上市日（新股首日涨跌停判定 + 回测起点）
    delist_date DATE,                       -- 退市日（避免幸存者偏差：退市股不删除，保留历史 K 线）
    is_st BOOLEAN NOT NULL DEFAULT FALSE,   -- 当前是否 ST（逐日历史 ST 在 stock_daily_klines.is_st）
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_securities_market_code UNIQUE (market, code)
);

-- 证券代码/名称模糊搜索（ILIKE '%kw%'）走 trgm GIN 索引，避免全表扫描。
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX IF NOT EXISTS idx_securities_code_trgm ON securities USING gin (code gin_trgm_ops);
CREATE INDEX IF NOT EXISTS idx_securities_name_trgm ON securities USING gin (name gin_trgm_ops);

-- 个股日 K 线序列（前复权存储，数据源 baostock）。
-- 每个交易日「一行取齐」回测可成交性数据：OHLCV + 昨收 + 换手率 + 涨跌幅 + 自算涨跌停 + 停牌 + 当日 ST。
CREATE TABLE IF NOT EXISTS stock_daily_klines (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    source VARCHAR(32) NOT NULL DEFAULT 'baostock',
    stock_code VARCHAR(16) NOT NULL,
    trade_date DATE NOT NULL,
    open NUMERIC(20,4), high NUMERIC(20,4), low NUMERIC(20,4), close NUMERIC(20,4),
    pre_close NUMERIC(20,4),                -- 昨收（涨跌停基准）
    volume NUMERIC(20,0), amount NUMERIC(24,4),
    turnover_rate NUMERIC(10,4),            -- 换手率 %
    pct_chg NUMERIC(10,4),                  -- 涨跌幅 %
    limit_up NUMERIC(20,4),                 -- 自算涨停价（按板块/ST；不设限日为 NULL）
    limit_down NUMERIC(20,4),               -- 自算跌停价
    trade_status SMALLINT NOT NULL DEFAULT 1, -- 1 正常 / 0 停牌（停牌日不可成交）
    is_st BOOLEAN NOT NULL DEFAULT FALSE,   -- 当日是否 ST（point-in-time，回测过滤/涨跌停制度）
    adjust VARCHAR(8) NOT NULL DEFAULT 'qfq',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_klines_market_code_date_adj UNIQUE (market, stock_code, trade_date, adjust)
);

-- 概览「数据新鲜度」与增量筛选需按市场取最新交易日（MAX(trade_date)）：
-- 复合索引避免在数百万行 K 线上全表扫描。
CREATE INDEX IF NOT EXISTS idx_klines_market_trade_date ON stock_daily_klines (market, trade_date);
-- 运维页「按 source 聚合覆盖」需 distinct(stock_code)+min/max(trade_date) 分组统计：
-- 覆盖索引使其走索引内只读扫描，数千万行下从约 40s 降到约 5s。
CREATE INDEX IF NOT EXISTS idx_klines_source_code_date ON stock_daily_klines (market, source, stock_code, trade_date);

-- 复权因子表（baostock query_adjust_factor）：供接入方做可复现回测自行复权
-- （前复权基准随除权漂移，回测宜用不复权 + 因子 / 后复权）。
CREATE TABLE IF NOT EXISTS stock_adjust_factors (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL,
    trade_date DATE NOT NULL,
    fore_factor NUMERIC(20,8),              -- 前复权因子
    back_factor NUMERIC(20,8),              -- 后复权因子
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_adj_factor_market_code_date UNIQUE (market, stock_code, trade_date)
);
CREATE INDEX IF NOT EXISTS idx_adj_factor_code_date ON stock_adjust_factors (market, stock_code, trade_date);

CREATE TABLE IF NOT EXISTS index_quotes (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    index_code VARCHAR(16) NOT NULL,
    index_name VARCHAR(64) NOT NULL,
    price NUMERIC(20,4),
    change_amount NUMERIC(20,4),
    change_percent NUMERIC(10,4),
    volume NUMERIC(20,0),
    amount NUMERIC(24,4),
    trade_date DATE NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stock_quotes (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL,
    stock_name VARCHAR(64) NOT NULL DEFAULT '',
    price NUMERIC(20,4),
    open NUMERIC(20,4),
    high NUMERIC(20,4),
    low NUMERIC(20,4),
    prev_close NUMERIC(20,4),
    change_percent NUMERIC(10,4),
    volume NUMERIC(20,0),
    amount NUMERIC(24,4),
    turnover_rate NUMERIC(10,4),
    trade_date DATE NOT NULL,
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS update_jobs (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(64) NOT NULL,
    job_type VARCHAR(32) NOT NULL,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    cron_expr VARCHAR(64) NOT NULL DEFAULT '0 0 17 * * *',
    batch_size INT NOT NULL DEFAULT 20,
    concurrency INT NOT NULL DEFAULT 10,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS update_job_runs (
    id BIGSERIAL PRIMARY KEY,
    job_id BIGINT NOT NULL,
    job_type VARCHAR(32) NOT NULL DEFAULT '',
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    total INT NOT NULL DEFAULT 0,
    processed INT NOT NULL DEFAULT 0,
    succeeded INT NOT NULL DEFAULT 0,
    failed INT NOT NULL DEFAULT 0,
    skipped INT NOT NULL DEFAULT 0,
    failed_codes TEXT NOT NULL DEFAULT '',
    skipped_codes TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ,
    error_msg TEXT NOT NULL DEFAULT ''
);

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

CREATE TABLE IF NOT EXISTS trading_calendars (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    cal_date DATE NOT NULL,
    is_open BOOLEAN NOT NULL DEFAULT FALSE,
    source VARCHAR(16) NOT NULL DEFAULT 'manual',
    CONSTRAINT uni_trading_calendars_market_date UNIQUE (market, cal_date)
);

CREATE TABLE IF NOT EXISTS update_watermarks (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL,
    last_trade_date DATE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_watermark_market_code UNIQUE (market, stock_code)
);

CREATE TABLE IF NOT EXISTS credential_access_logs (
    id BIGSERIAL PRIMARY KEY,
    credential_id BIGINT NOT NULL,
    stat_date DATE NOT NULL,
    call_count BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    last_access_at TIMESTAMPTZ,
    CONSTRAINT uni_access_log_cred_date UNIQUE (credential_id, stat_date)
);

CREATE TABLE IF NOT EXISTS api_credentials (
    id BIGSERIAL PRIMARY KEY,
    secret_id VARCHAR(40) NOT NULL,
    secret_key_cipher TEXT NOT NULL,
    secret_key_hash VARCHAR(128) NOT NULL,
    consumer_name VARCHAR(128) NOT NULL,
    scope VARCHAR(32) NOT NULL DEFAULT 'read',
    rate_limit INT NOT NULL DEFAULT 20,
    daily_quota INT NOT NULL DEFAULT 100000,
    status SMALLINT NOT NULL DEFAULT 1,
    expire_at TIMESTAMPTZ,
    created_by BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_api_credentials_secret_id UNIQUE (secret_id)
);
