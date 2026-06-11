# Warden Python quant 服务（采集 + 指标计算）

行情数据中台的 Python 服务：用 **baostock** 采集日 K / 复权因子 / 涨跌停（自算）/ ST / 停牌 / 退市 / 证券列表，用 **pandas-ta** 实时计算技术指标。**仅内网、由 Go 服务调用，不对外开放**。

> 完整设计见 [`../../docs/PYTHON_SERVICE.md`](../../docs/PYTHON_SERVICE.md)。

## 快速开始（本地）

环境变量统一在 `backend/.env`（与 Go 服务共用），无需单独 env：

```bash
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
make run                        # 读取 ../.env，127.0.0.1:8000
make test                       # pytest
make backfill ARGS="--all"      # 离线批量回补日 K（直接写库，首次全量最快路径）
```

> 离线回补支持 `--codes/--all/--shard i/n（多进程并行）/--skip-done（断点续跑）`，详见 [`../../docs/PYTHON_SERVICE.md`](../../docs/PYTHON_SERVICE.md) §4.5。baostock 无可下载的数据集文件，离线 CLI 即「本地一次性查全市场直接入库」的等价方案。

## 内部接口（需 `X-Internal-Token`）

| Method | Path | 用途 |
|--------|------|------|
| GET | `/health` | 健康检查 |
| GET | `/internal/v1/catalog` | 指标目录 |
| POST | `/internal/v1/collect/securities` | 采集证券列表 + 上市/退市/ST |
| POST | `/internal/v1/collect/kline` | 采集一批代码日 K + 复权因子 + 涨跌停 + ST + 停牌 |
| POST | `/internal/v1/indicators` | 批量实时指标（最新/指定日） |
| POST | `/internal/v1/indicators/series` | 单只逐 bar 指标序列 |

## 目录

```
app/
├── core/        config / db / security / logging / limit_price（涨跌停纯函数）
├── features/
│   ├── collect/ baostock 采集 + 映射 + upsert + 编排
│   ├── indicator/ pandas-ta 指标计算 + 目录 + 服务
│   └── api/     内部路由
├── models/      SQLAlchemy 表模型（与 backend 同表）
├── schemas/     pydantic 请求/响应
└── scripts/     离线 CLI（backfill.py 批量回补）
```

数据库 schema 由 `backend/deploy/init.sql` 统一维护，本服务只读写数据。
