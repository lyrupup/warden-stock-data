"""采集落库：PostgreSQL UPSERT（on conflict do update）。"""
from __future__ import annotations

from datetime import date

from sqlalchemy import func
from sqlalchemy.dialects.postgresql import insert

from app.models.tables import (
    Security,
    StockAdjustFactor,
    StockDailyKline,
    TradingCalendar,
    UpdateWatermark,
)

_KLINE_UPDATE = [
    "source", "open", "high", "low", "close", "pre_close", "volume", "amount",
    "turnover_rate", "pct_chg", "limit_up", "limit_down", "trade_status", "is_st",
]
_SECURITY_UPDATE = ["name", "board", "status", "list_date", "delist_date", "is_st"]


def _chunks(rows: list, size: int = 500):
    for i in range(0, len(rows), size):
        yield rows[i : i + size]


def upsert_klines(session, rows: list[dict]) -> int:
    rows = [r for r in rows if r.get("trade_date") is not None]
    if not rows:
        return 0
    for chunk in _chunks(rows):
        stmt = insert(StockDailyKline).values(chunk)
        stmt = stmt.on_conflict_do_update(
            index_elements=["market", "stock_code", "trade_date", "adjust"],
            set_={c: stmt.excluded[c] for c in _KLINE_UPDATE},
        )
        session.execute(stmt)
    return len(rows)


def upsert_factors(session, rows: list[dict]) -> int:
    rows = [r for r in rows if r.get("trade_date") is not None]
    if not rows:
        return 0
    for chunk in _chunks(rows):
        stmt = insert(StockAdjustFactor).values(chunk)
        stmt = stmt.on_conflict_do_update(
            index_elements=["market", "stock_code", "trade_date"],
            set_={
                "fore_factor": stmt.excluded["fore_factor"],
                "back_factor": stmt.excluded["back_factor"],
            },
        )
        session.execute(stmt)
    return len(rows)


def upsert_watermark(session, market: str, code: str, last_trade_date: date) -> None:
    """推进个股更新水位（仅当更新到更晚交易日时前移），与 Go 侧水位语义一致。

    概览页「行情数据覆盖」分母按 update_watermarks 行数统计，离线回补后写水位可使覆盖率正确反映。
    """
    stmt = insert(UpdateWatermark).values(
        market=market, stock_code=code, last_trade_date=last_trade_date, updated_at=func.now()
    )
    stmt = stmt.on_conflict_do_update(
        index_elements=["market", "stock_code"],
        set_={"last_trade_date": stmt.excluded.last_trade_date, "updated_at": func.now()},
        where=UpdateWatermark.last_trade_date.is_(None)
        | (UpdateWatermark.last_trade_date < stmt.excluded.last_trade_date),
    )
    session.execute(stmt)


def upsert_calendars(session, rows: list[dict]) -> int:
    """交易日历 UPSERT（按 market+cal_date 唯一键覆盖 is_open/source）。"""
    rows = [r for r in rows if r.get("cal_date") is not None]
    if not rows:
        return 0
    for chunk in _chunks(rows):
        stmt = insert(TradingCalendar).values(chunk)
        stmt = stmt.on_conflict_do_update(
            index_elements=["market", "cal_date"],
            set_={"is_open": stmt.excluded.is_open, "source": stmt.excluded.source},
        )
        session.execute(stmt)
    return len(rows)


def upsert_securities(session, rows: list[dict]) -> int:
    rows = [r for r in rows if r.get("code")]
    if not rows:
        return 0
    for chunk in _chunks(rows):
        stmt = insert(Security).values(chunk)
        stmt = stmt.on_conflict_do_update(
            index_elements=["market", "code"],
            set_={c: stmt.excluded[c] for c in _SECURITY_UPDATE},
        )
        session.execute(stmt)
    return len(rows)
