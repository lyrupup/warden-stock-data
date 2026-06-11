"""指标服务：从 PG 读日 K → 计算 → 对齐返回。无状态、不落库。"""
from __future__ import annotations

import math

import pandas as pd
from sqlalchemy import select

from app.core.db import session_scope
from app.features.indicator.catalog import default_types
from app.features.indicator.compute import compute_indicators
from app.models.tables import StockDailyKline
from app.schemas.indicator import IndicatorResult

# 预热根数：保证 MA60 / RSI24 等长周期指标可算出
_PREHEAT = 260


def _fmt(v) -> str | None:
    if v is None:
        return None
    try:
        f = float(v)
    except (TypeError, ValueError):
        return None
    if math.isnan(f) or math.isinf(f):
        return None
    return f"{f:.4f}"


def _rows_to_df(rows: list[StockDailyKline]) -> pd.DataFrame:
    def fv(x):
        return float(x) if x is not None else float("nan")

    return pd.DataFrame(
        {
            "trade_date": [r.trade_date for r in rows],
            "open": [fv(r.open) for r in rows],
            "high": [fv(r.high) for r in rows],
            "low": [fv(r.low) for r in rows],
            "close": [fv(r.close) for r in rows],
            "volume": [fv(r.volume) for r in rows],
        }
    )


def _load_upto(session, market: str, code: str, adjust: str, upto: str | None, n: int) -> list[StockDailyKline]:
    """取截止 upto（含）最近 n 根日 K，升序返回。"""
    stmt = select(StockDailyKline).where(
        StockDailyKline.market == market,
        StockDailyKline.stock_code == code,
        StockDailyKline.adjust == adjust,
    )
    if upto:
        stmt = stmt.where(StockDailyKline.trade_date <= upto)
    stmt = stmt.order_by(StockDailyKline.trade_date.desc()).limit(n)
    rows = session.execute(stmt).scalars().all()
    return list(reversed(rows))


def batch_indicators(
    codes: list[str],
    types: list[str] | None,
    period: str = "day",
    adjust: str = "qfq",
    trade_date: str | None = None,
    market: str = "CN",
) -> list[IndicatorResult]:
    """批量取每只标的「最新一日（或指定交易日）」的指标值。仅支持日 K。"""
    if period not in ("", "day"):
        return []
    types = types or default_types()
    adjust = adjust or "qfq"
    out: list[IndicatorResult] = []
    with session_scope() as session:
        for code in codes:
            rows = _load_upto(session, market, code, adjust, trade_date, _PREHEAT)
            if not rows:
                continue
            df = _rows_to_df(rows)
            series = compute_indicators(df, types)
            i = len(df) - 1
            vals: dict[str, str] = {}
            for t, s in series.items():
                v = _fmt(s.iloc[i])
                if v is not None:
                    vals[t] = v
            if vals:
                out.append(
                    IndicatorResult(stock_code=code, trade_date=rows[-1].trade_date.isoformat(), values=vals)
                )
    return out


def series_indicators(
    code: str,
    types: list[str] | None,
    period: str = "day",
    adjust: str = "qfq",
    limit: int = 0,
    offset: int = 0,
    from_date: str | None = None,
    to_date: str | None = None,
    market: str = "CN",
) -> list[IndicatorResult]:
    """逐 bar 指标序列（K 线带指标用）。读全序列（含历史预热）计算后按窗口返回。仅支持日 K。"""
    if period not in ("", "day"):
        return []
    types = types or default_types()
    adjust = adjust or "qfq"
    out: list[IndicatorResult] = []
    with session_scope() as session:
        stmt = select(StockDailyKline).where(
            StockDailyKline.market == market,
            StockDailyKline.stock_code == code,
            StockDailyKline.adjust == adjust,
        )
        if to_date:
            stmt = stmt.where(StockDailyKline.trade_date <= to_date)
        stmt = stmt.order_by(StockDailyKline.trade_date.asc())
        rows = session.execute(stmt).scalars().all()
        if not rows:
            return out
        n = len(rows)
        # 窗口下标集合：from/to 优先；否则 limit+offset（跳过最近 offset 根、取 limit 根）
        if from_date or to_date:
            window = [i for i, r in enumerate(rows) if (not from_date or r.trade_date.isoformat() >= from_date)]
        elif limit > 0:
            end = max(0, n - offset)
            start = max(0, end - limit)
            window = list(range(start, end))
        else:
            window = list(range(n))

        df = _rows_to_df(rows)
        series = compute_indicators(df, types)
        for i in window:
            vals: dict[str, str] = {}
            for t, s in series.items():
                v = _fmt(s.iloc[i])
                if v is not None:
                    vals[t] = v
            if vals:
                out.append(
                    IndicatorResult(stock_code=code, trade_date=rows[i].trade_date.isoformat(), values=vals)
                )
    return out
