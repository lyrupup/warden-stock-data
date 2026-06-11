#!/usr/bin/env bash
# A股历史行情冷备导入脚本
# 用法：cd backend/data-export && ./import.sh [--mode fresh|merge] [--skip-verify]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA_DIR="${SCRIPT_DIR}/data"
MANIFEST="${SCRIPT_DIR}/manifest.json"
ENV_FILE="${SCRIPT_DIR}/../.env"

MODE="fresh"
SKIP_VERIFY=false

usage() {
  cat <<'EOF'
用法: ./import.sh [选项]

将 backend/data-export/data/ 下的 CSV.gz 冷备导入 PostgreSQL。

选项:
  --mode fresh       清空目标表后全量导入（默认，适用于新机器/空库）
  --mode merge       保留已有数据，冲突行按唯一键 UPSERT（适用于补数据）
  --skip-verify      跳过 manifest SHA256 校验
  -h, --help         显示帮助

前置条件:
  1. PostgreSQL 已启动（本地或 Docker compose）
  2. 已执行 backend/deploy/init.sql 建表
  3. backend/.env 中 PG_* 配置正确

示例:
  cd backend/data-export && ./import.sh
  cd backend/data-export && ./import.sh --mode merge
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --skip-verify) SKIP_VERIFY=true; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "未知参数: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ "$MODE" != "fresh" && "$MODE" != "merge" ]]; then
  echo "错误: --mode 仅支持 fresh 或 merge" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  echo "错误: 未找到 $ENV_FILE，请先 cp backend/.env.example backend/.env" >&2
  exit 1
fi

# shellcheck disable=SC1090
set -a
source "$ENV_FILE"
set +a

PG_USER="${PG_USER:-postgres}"
PG_PASSWORD="${PG_PASSWORD:-postgres}"
PG_DB="${PG_DB:-warden_data}"
PG_HOST="${PG_HOST:-localhost}"
PG_PORT="${PG_PORT:-5432}"

DOCKER_CONTAINER=""
if docker ps --format '{{.Names}}' 2>/dev/null | grep -q '^warden_stock_data-postgres-1$'; then
  DOCKER_CONTAINER="warden_stock_data-postgres-1"
fi

psql_exec() {
  if [[ -n "$DOCKER_CONTAINER" ]]; then
    docker exec -i "$DOCKER_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 "$@"
  else
    PGPASSWORD="$PG_PASSWORD" psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 "$@"
  fi
}

psql_copy() {
  if [[ -n "$DOCKER_CONTAINER" ]]; then
    docker exec -i "$DOCKER_CONTAINER" psql -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "$1"
  else
    PGPASSWORD="$PG_PASSWORD" psql -h "$PG_HOST" -p "$PG_PORT" -U "$PG_USER" -d "$PG_DB" -v ON_ERROR_STOP=1 -c "$1"
  fi
}

verify_manifest() {
  echo "==> 校验导出文件完整性..."
  local missing=0
  for f in securities.csv.gz trading_calendars.csv.gz stock_daily_klines.csv.gz stock_adjust_factors.csv.gz update_watermarks.csv.gz; do
    if [[ ! -f "${DATA_DIR}/${f}" ]]; then
      echo "  缺失: ${f}" >&2
      missing=1
    fi
  done
  if [[ $missing -eq 1 ]]; then
    exit 1
  fi

  if [[ "$SKIP_VERIFY" == true ]]; then
    echo "  已跳过 SHA256 校验"
    return
  fi

  if ! command -v shasum >/dev/null 2>&1; then
    echo "  警告: 未找到 shasum，跳过校验" >&2
    return
  fi

  cd "$DATA_DIR"
  local expected actual
  expected=$(python3 -c "import json; m=json.load(open('${MANIFEST}')); print('\n'.join(f['sha256']+'  '+f['name'] for f in m['files']))")
  actual=$(shasum -a 256 *.csv.gz | sort)
  expected_sorted=$(echo "$expected" | sort)
  if [[ "$actual" != "$expected_sorted" ]]; then
    echo "  错误: SHA256 与 manifest.json 不一致" >&2
    echo "  期望:" >&2
    echo "$expected_sorted" >&2
    echo "  实际:" >&2
    echo "$actual" >&2
    exit 1
  fi
  echo "  SHA256 校验通过"
}

truncate_tables() {
  echo "==> 清空目标表（fresh 模式）..."
  psql_exec -c "
    TRUNCATE TABLE
      stock_daily_klines,
      stock_adjust_factors,
      update_watermarks,
      securities,
      trading_calendars
    RESTART IDENTITY;
  "
}

import_fresh() {
  local file="$1" table="$2" columns="$3"
  echo "==> 导入 ${file} -> ${table} ..."
  gunzip -c "${DATA_DIR}/${file}" | psql_copy "\\copy ${table} (${columns}) FROM STDIN WITH (FORMAT csv, HEADER true)"
}

import_merge() {
  local file="$1" table="$2" columns="$3" conflict_sql="$4"
  echo "==> 合并导入 ${file} -> ${table} ..."
  psql_exec -c "
    DROP TABLE IF EXISTS _warden_import_staging;
    CREATE UNLOGGED TABLE _warden_import_staging (LIKE ${table} INCLUDING DEFAULTS);
    ALTER TABLE _warden_import_staging DROP COLUMN IF EXISTS id;
    ALTER TABLE _warden_import_staging DROP COLUMN IF EXISTS created_at;
    ALTER TABLE _warden_import_staging DROP COLUMN IF EXISTS updated_at;
  "
  gunzip -c "${DATA_DIR}/${file}" | psql_copy "\\copy _warden_import_staging (${columns}) FROM STDIN WITH (FORMAT csv, HEADER true)"
  psql_exec -c "${conflict_sql}; DROP TABLE IF EXISTS _warden_import_staging;"
}

echo "==> 连接目标: ${PG_HOST}:${PG_PORT}/${PG_DB}"
if [[ -n "$DOCKER_CONTAINER" ]]; then
  echo "    使用 Docker 容器: ${DOCKER_CONTAINER}"
fi
echo "    导入模式: ${MODE}"

verify_manifest

if [[ "$MODE" == "fresh" ]]; then
  truncate_tables
  import_fresh "securities.csv.gz" "securities" \
    "market,code,name,board,status,list_date,delist_date,is_st"
  import_fresh "trading_calendars.csv.gz" "trading_calendars" \
    "market,cal_date,is_open,source"
  import_fresh "stock_daily_klines.csv.gz" "stock_daily_klines" \
    "market,source,stock_code,trade_date,open,high,low,close,pre_close,volume,amount,turnover_rate,pct_chg,limit_up,limit_down,trade_status,is_st,adjust"
  import_fresh "stock_adjust_factors.csv.gz" "stock_adjust_factors" \
    "market,stock_code,trade_date,fore_factor,back_factor"
  import_fresh "update_watermarks.csv.gz" "update_watermarks" \
    "market,stock_code,last_trade_date"
else
  import_merge "securities.csv.gz" "securities" \
    "market,code,name,board,status,list_date,delist_date,is_st" \
    "INSERT INTO securities (market,code,name,board,status,list_date,delist_date,is_st)
     SELECT market,code,name,board,status,list_date,delist_date,is_st FROM _import_staging
     ON CONFLICT (market, code) DO UPDATE SET
       name=EXCLUDED.name, board=EXCLUDED.board, status=EXCLUDED.status,
       list_date=EXCLUDED.list_date, delist_date=EXCLUDED.delist_date,
       is_st=EXCLUDED.is_st, updated_at=NOW();"

  import_merge "trading_calendars.csv.gz" "trading_calendars" \
    "market,cal_date,is_open,source" \
    "INSERT INTO trading_calendars (market,cal_date,is_open,source)
     SELECT market,cal_date,is_open,source FROM _import_staging
     ON CONFLICT (market, cal_date) DO UPDATE SET
       is_open=EXCLUDED.is_open, source=EXCLUDED.source;"

  import_merge "stock_daily_klines.csv.gz" "stock_daily_klines" \
    "market,source,stock_code,trade_date,open,high,low,close,pre_close,volume,amount,turnover_rate,pct_chg,limit_up,limit_down,trade_status,is_st,adjust" \
    "INSERT INTO stock_daily_klines (market,source,stock_code,trade_date,open,high,low,close,pre_close,volume,amount,turnover_rate,pct_chg,limit_up,limit_down,trade_status,is_st,adjust)
     SELECT market,source,stock_code,trade_date,open,high,low,close,pre_close,volume,amount,turnover_rate,pct_chg,limit_up,limit_down,trade_status,is_st,adjust FROM _import_staging
     ON CONFLICT (market, stock_code, trade_date, adjust) DO UPDATE SET
       source=EXCLUDED.source, open=EXCLUDED.open, high=EXCLUDED.high, low=EXCLUDED.low,
       close=EXCLUDED.close, pre_close=EXCLUDED.pre_close, volume=EXCLUDED.volume,
       amount=EXCLUDED.amount, turnover_rate=EXCLUDED.turnover_rate, pct_chg=EXCLUDED.pct_chg,
       limit_up=EXCLUDED.limit_up, limit_down=EXCLUDED.limit_down,
       trade_status=EXCLUDED.trade_status, is_st=EXCLUDED.is_st;"

  import_merge "stock_adjust_factors.csv.gz" "stock_adjust_factors" \
    "market,stock_code,trade_date,fore_factor,back_factor" \
    "INSERT INTO stock_adjust_factors (market,stock_code,trade_date,fore_factor,back_factor)
     SELECT market,stock_code,trade_date,fore_factor,back_factor FROM _import_staging
     ON CONFLICT (market, stock_code, trade_date) DO UPDATE SET
       fore_factor=EXCLUDED.fore_factor, back_factor=EXCLUDED.back_factor, updated_at=NOW();"

  import_merge "update_watermarks.csv.gz" "update_watermarks" \
    "market,stock_code,last_trade_date" \
    "INSERT INTO update_watermarks (market,stock_code,last_trade_date)
     SELECT market,stock_code,last_trade_date FROM _import_staging
     ON CONFLICT (market, stock_code) DO UPDATE SET
       last_trade_date=EXCLUDED.last_trade_date, updated_at=NOW();"
fi

echo "==> 导入完成，验证行数..."
psql_exec -c "
SELECT 'securities' AS t, count(*) AS n FROM securities WHERE market='CN'
UNION ALL SELECT 'trading_calendars', count(*) FROM trading_calendars WHERE market='CN'
UNION ALL SELECT 'stock_daily_klines', count(*) FROM stock_daily_klines WHERE market='CN' AND trade_date<='2026-06-08'
UNION ALL SELECT 'stock_adjust_factors', count(*) FROM stock_adjust_factors WHERE market='CN' AND trade_date<='2026-06-08'
UNION ALL SELECT 'update_watermarks', count(*) FROM update_watermarks WHERE market='CN';
"

echo "==> 全部完成。导入后可通过增量任务从 2026-06-09 起继续拉取最新数据。"
