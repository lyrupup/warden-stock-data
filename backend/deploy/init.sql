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
    board VARCHAR(16) NOT NULL DEFAULT '',
    status SMALLINT NOT NULL DEFAULT 1,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_securities_market_code UNIQUE (market, code)
);

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

CREATE TABLE IF NOT EXISTS stock_indicator_snapshots (
    id BIGSERIAL PRIMARY KEY,
    market VARCHAR(8) NOT NULL DEFAULT 'CN',
    stock_code VARCHAR(16) NOT NULL,
    trade_date DATE NOT NULL,
    values JSONB NOT NULL DEFAULT '{}',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uni_indi_snap_market_code_date UNIQUE (market, stock_code, trade_date)
);

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
    job_type VARCHAR(16) NOT NULL,
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
    status VARCHAR(16) NOT NULL DEFAULT 'running',
    total INT NOT NULL DEFAULT 0,
    processed INT NOT NULL DEFAULT 0,
    succeeded INT NOT NULL DEFAULT 0,
    failed INT NOT NULL DEFAULT 0,
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
