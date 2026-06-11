"""采集编排：按 Go 传入的一批代码顺序采集（baostock 串行），每只用 savepoint 隔离失败。"""
from __future__ import annotations

import logging
from datetime import date

from sqlalchemy import select

from app.core.config import get_settings
from app.core.db import session_scope
from app.features.collect import baostock_api, repo
from app.features.collect.mapper import (
    map_calendar_row,
    map_factor_row,
    map_kline_row,
    map_security_row,
)
from app.models.tables import Security
from app.schemas.collect import (
    CollectCalendarRequest,
    CollectKlineRequest,
    CollectResult,
    CollectSecuritiesRequest,
)

logger = logging.getLogger(__name__)


def collect_securities(req: CollectSecuritiesRequest) -> int:
    """采集证券列表 + 上市/退市/ST 状态（全量 upsert）。"""
    df = baostock_api.fetch_securities()
    rows = []
    for _, r in df.iterrows():
        m = map_security_row(r, req.market)
        if m:
            rows.append(m)
    with session_scope() as session:
        return repo.upsert_securities(session, rows)


def collect_calendar(req: CollectCalendarRequest) -> int:
    """采集 baostock 官方交易日历（区间内每个自然日的开/休市），UPSERT 入 trading_calendars。

    from_date 留空时用 BACKFILL_START_DATE；to_date 留空时**默认到当年年底**（拉全当年节假日，
    而非只到最近一个交易日——baostock query_trade_dates 在 end 为空时仅返回到最近交易日）。
    超出 baostock 已发布范围的未来日期会被自动截断，无副作用。
    """
    settings = get_settings()
    start = req.from_date or settings.backfill_start_date
    end = req.to_date or f"{date.today().year}-12-31"
    df = baostock_api.fetch_trade_dates(start, end)
    rows = []
    for _, r in df.iterrows():
        m = map_calendar_row(r, req.market)
        if m:
            rows.append(m)
    with session_scope() as session:
        return repo.upsert_calendars(session, rows)


def _load_list_dates(session, codes: list[str], market: str) -> dict:
    if not codes:
        return {}
    res = session.execute(
        select(Security.code, Security.list_date).where(
            Security.market == market, Security.code.in_(codes)
        )
    )
    return {code: ld for code, ld in res.all()}


def collect_kline_batch(req: CollectKlineRequest) -> list[CollectResult]:
    """采集一批代码的日 K + 复权因子 + 涨跌停 + ST + 停牌。

    baostock 单次查询可达数十秒；Go 端 concurrency 会并发多批 HTTP。
    **禁止**在 session_scope 内做 baostock I/O，否则每批占一条 PG 连接直至整批跑完，
  连接池（默认 10）耗尽后后续请求 QueuePool timeout → 增量作业「没动静」或整批失败。
    """
    settings = get_settings()
    with session_scope() as session:
        list_dates = _load_list_dates(session, req.codes, req.market)

    results: list[CollectResult] = []
    for code in req.codes:
        try:
            results.append(_collect_one(code, req, list_dates.get(code), settings))
        except Exception as e:  # noqa: BLE001
            logger.warning("collect kline failed code=%s err=%s", code, e)
            results.append(CollectResult(code=code, status="failed", reason=str(e)))
    return results


def _collect_one(code: str, req: CollectKlineRequest, list_date, settings) -> CollectResult:
    if req.mode == "full":
        start = req.from_date or settings.backfill_start_date
    else:
        start = req.from_date or ""  # 增量：Go 传水位日作为 from
    end = req.to_date or ""

    # 阶段 1：baostock 拉取（不占 DB 连接）
    df = baostock_api.fetch_kline(code, start, end, req.market)
    if df.empty:
        return CollectResult(code=code, status="skipped", reason="no_market_data")
    rows = [map_kline_row(code, r, list_date, req.market) for _, r in df.iterrows()]
    rows = [r for r in rows if r["trade_date"] is not None]
    if not rows:
        return CollectResult(code=code, status="skipped", reason="no_market_data")

    frows: list[dict] = []
    try:
        fdf = baostock_api.fetch_adjust_factor(code, start, end, req.market)
        if not fdf.empty:
            frows = [map_factor_row(code, r, req.market) for _, r in fdf.iterrows()]
    except Exception as e:  # noqa: BLE001
        logger.warning("collect adjust factor failed code=%s err=%s", code, e)

    # 阶段 2：写库（短连接）
    with session_scope() as session:
        with session.begin_nested():
            n = repo.upsert_klines(session, rows)
            if frows:
                repo.upsert_factors(session, frows)

    latest = max(r["trade_date"] for r in rows)
    return CollectResult(code=code, status="ok", rows=n, latest_trade_date=latest.isoformat())
