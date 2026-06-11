---
name: warden-stock-data-import
description: >-
  理解并导入 warden-stock-data 项目的 A 股历史行情冷备（backend/data-export）。
  当用户需要恢复数据库、新机器部署、从冷备导入 A 股 K 线/证券/交易日历数据、
  或询问 data-export 目录含义时使用。
---

# A 股历史行情冷备导入

## 目录用途

`backend/data-export/` 存放 **PostgreSQL 中 A 股（market=CN）历史行情的快照冷备**，用于：

- 电脑数据丢失后快速恢复
- 新机器部署时跳过从 baostock 全量回补（节省数小时）
- 离线迁移数据库

**不包含**：实时快照（`stock_quotes`/`index_quotes`）、管理员账号、API 凭证、定时任务配置等运营数据。这些由服务首次启动或管理后台重新配置。

## 目录结构

```
backend/data-export/
├── SKILL.md           # 本文件（大模型操作指南）
├── manifest.json      # 导出元信息、行数、SHA256 校验
├── import.sh          # 一键导入脚本
└── data/              # 压缩 CSV 数据文件（约 584 MB）
    ├── securities.csv.gz
    ├── trading_calendars.csv.gz
    ├── stock_daily_klines.csv.gz      # 最大，约 1680 万行
    ├── stock_adjust_factors.csv.gz
    └── update_watermarks.csv.gz
```

当前冷备截止交易日：**2026-06-08**（见 `manifest.json` 的 `cutoff_date`）。

## 数据文件说明

所有文件均为 **gzip 压缩的 CSV**，首行为表头，UTF-8 编码，不含 `id` 列（导入时由数据库 SERIAL 自动生成）。

### 1. securities.csv.gz → `securities` 表

A 股证券主数据，5533 条。

| 字段 | 类型 | 说明 |
|------|------|------|
| market | VARCHAR(8) | 固定 `CN` |
| code | VARCHAR(16) | 证券代码，如 `600000` |
| name | VARCHAR(64) | 证券名称 |
| board | VARCHAR(16) | 板块：主板/创业板/科创板/北交所 |
| status | SMALLINT | 1=上市，0=退市 |
| list_date | DATE | 上市日，可空 |
| delist_date | DATE | 退市日，可空 |
| is_st | BOOLEAN | 当前是否 ST |

唯一约束：`(market, code)`

### 2. trading_calendars.csv.gz → `trading_calendars` 表

交易日历，截止 2026-06-08 共 12956 条。

| 字段 | 类型 | 说明 |
|------|------|------|
| market | VARCHAR(8) | `CN` |
| cal_date | DATE | 日历日期 |
| is_open | BOOLEAN | 是否交易日 |
| source | VARCHAR(16) | 数据来源，通常 `manual` 或 `baostock` |

唯一约束：`(market, cal_date)`

### 3. stock_daily_klines.csv.gz → `stock_daily_klines` 表

**核心数据**：个股日 K 线（前复权 qfq），截止 2026-06-08 共 **16,808,093** 行。

| 字段 | 类型 | 说明 |
|------|------|------|
| market | VARCHAR(8) | `CN` |
| source | VARCHAR(32) | 数据源，默认 `baostock` |
| stock_code | VARCHAR(16) | 股票代码 |
| trade_date | DATE | 交易日 |
| open/high/low/close | NUMERIC(20,4) | OHLC |
| pre_close | NUMERIC(20,4) | 昨收（涨跌停基准） |
| volume | NUMERIC(20,0) | 成交量（股） |
| amount | NUMERIC(24,4) | 成交额 |
| turnover_rate | NUMERIC(10,4) | 换手率 % |
| pct_chg | NUMERIC(10,4) | 涨跌幅 % |
| limit_up/limit_down | NUMERIC(20,4) | 自算涨跌停价，不设限日为 NULL |
| trade_status | SMALLINT | 1=正常，0=停牌 |
| is_st | BOOLEAN | 当日是否 ST（point-in-time） |
| adjust | VARCHAR(8) | 复权类型，固定 `qfq` |

唯一约束：`(market, stock_code, trade_date, adjust)`

### 4. stock_adjust_factors.csv.gz → `stock_adjust_factors` 表

复权因子，截止 2026-06-08 共 59235 条。供接入方做可复现回测自行复权。

| 字段 | 类型 | 说明 |
|------|------|------|
| market | VARCHAR(8) | `CN` |
| stock_code | VARCHAR(16) | 股票代码 |
| trade_date | DATE | 除权除息日 |
| fore_factor | NUMERIC(20,8) | 前复权因子 |
| back_factor | NUMERIC(20,8) | 后复权因子 |

唯一约束：`(market, stock_code, trade_date)`

### 5. update_watermarks.csv.gz → `update_watermarks` 表

增量更新水位线，5207 条。`last_trade_date` 已截断至冷备截止日。

| 字段 | 类型 | 说明 |
|------|------|------|
| market | VARCHAR(8) | `CN` |
| stock_code | VARCHAR(16) | 股票代码 |
| last_trade_date | DATE | 该股票已采集到的最后交易日 |

唯一约束：`(market, stock_code)`

导入后增量任务会从 `last_trade_date` 的下一天继续拉取，无需重跑全量。

## 导入前置条件

1. **PostgreSQL 已启动**
   - 本地开发：`cd backend/go && make infra-up`（仅 postgres + redis）
   - Docker 全量：`cd backend/deploy && ./deploy.sh`
2. **已建表**：首次启动 postgres 容器会自动执行 `backend/deploy/init.sql`
3. **环境变量**：`backend/.env` 存在且 `PG_*` 配置正确（可从 `backend/.env.example` 复制）

## 推荐导入流程（给大模型执行）

### 步骤 1：确认冷备文件完整

```bash
cd backend/data-export/data
ls -lh *.csv.gz
shasum -a 256 *.csv.gz   # 与 manifest.json 中 sha256 对比
```

期望行数（见 `manifest.json`）：

| 文件 | 行数 |
|------|------|
| securities.csv.gz | 5,533 |
| trading_calendars.csv.gz | 12,956 |
| stock_daily_klines.csv.gz | 16,808,093 |
| stock_adjust_factors.csv.gz | 59,235 |
| update_watermarks.csv.gz | 5,207 |

### 步骤 2：选择导入模式

| 模式 | 场景 | 行为 |
|------|------|------|
| `fresh`（默认） | 新机器空库 / 完全覆盖 | TRUNCATE 五张表后全量 COPY |
| `merge` | 已有部分数据需补全 | 临时表 + ON CONFLICT UPSERT |

### 步骤 3：执行导入

```bash
chmod +x backend/data-export/import.sh
cd backend/data-export && ./import.sh
```

大表导入约需 **5–15 分钟**（取决于磁盘与 CPU）。脚本结束时会打印各表行数供核对。

### 步骤 4：导入后增量补齐

冷备截止 2026-06-08。若当前日期更晚，导入后启动服务并触发增量采集：

```bash
# 本地
cd backend/go && make run          # Go API
cd backend/python && make run    # Python 采集服务

# 或通过管理后台 / API 触发 incremental 任务
```

增量任务读取 `update_watermarks`，从各股票 `last_trade_date + 1` 拉取至最新交易日，无需重跑 baostock 全量回补。

## 手动导入（脚本不可用时的备选）

连接信息来自 `backend/.env`。若 Docker postgres 容器名为 `warden_stock_data-postgres-1`：

```bash
# 示例：导入 securities（fresh 模式需先 TRUNCATE）
gunzip -c backend/data-export/data/securities.csv.gz | \
  docker exec -i warden_stock_data-postgres-1 \
  psql -U postgres -d warden_data -v ON_ERROR_STOP=1 \
  -c "\copy securities (market,code,name,board,status,list_date,delist_date,is_st) FROM STDIN WITH (FORMAT csv, HEADER true)"
```

**导入顺序**（与 `manifest.json` 的 `import_order` 一致）：

1. securities
2. trading_calendars
3. stock_daily_klines（最耗时）
4. stock_adjust_factors
5. update_watermarks

fresh 模式 TRUNCATE 语句：

```sql
TRUNCATE TABLE stock_daily_klines, stock_adjust_factors, update_watermarks, securities, trading_calendars RESTART IDENTITY;
```

## 验证导入成功

```bash
docker exec -i warden_stock_data-postgres-1 psql -U postgres -d warden_data -c "
SELECT 'securities' t, count(*) n FROM securities WHERE market='CN'
UNION ALL SELECT 'klines<=2026-06-08', count(*) FROM stock_daily_klines WHERE market='CN' AND trade_date<='2026-06-08'
UNION ALL SELECT 'watermarks', count(*) FROM update_watermarks WHERE market='CN';
"
```

期望：`securities=5533`，`klines<=2026-06-08=16808093`，`watermarks=5207`。

抽样检查：

```bash
gunzip -c backend/data-export/data/stock_daily_klines.csv.gz | head -3
```

## 重新生成冷备（维护者）

当需要更新截止日时，从数据库重新导出：

```bash
mkdir -p backend/data-export/data
CUT=2026-06-08   # 修改为目标截止日

docker exec -i warden_stock_data-postgres-1 psql -U postgres -d warden_data -v ON_ERROR_STOP=1 \
  -c "\copy (SELECT market,code,name,board,status,list_date,delist_date,is_st FROM securities WHERE market='CN' ORDER BY code) TO STDOUT WITH (FORMAT csv, HEADER true)" \
  | gzip > backend/data-export/data/securities.csv.gz

# 其余四表同理，K 线加 WHERE trade_date<=DATE '$CUT'
# 完成后更新 manifest.json（行数、sha256、cutoff_date）
```

## 注意事项

- `data/*.csv.gz` 体积约 **584 MB**，已加入 `.gitignore`，需自行备份（网盘/U 盘/对象存储）
- 冷备仅含 **前复权（qfq）** 日 K；回测若需不复权请结合 `stock_adjust_factors` 自行复权
- `fresh` 模式会清空五张表，**不可恢复**，执行前确认目标库无需要保留的增量数据
- Schema 以 `backend/deploy/init.sql` 为准；若表结构变更需同步更新导出列与 `import.sh`
