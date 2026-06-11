"""baostock 字段 → DB 行映射，含代码格式互转、涨跌停自算。"""
from __future__ import annotations

from datetime import date
from decimal import Decimal, InvalidOperation

from app.core.limit_price import board_of, compute_limit_prices

_BOARD_CN = {"MAIN": "主板", "GEM": "创业板", "STAR": "科创板", "BSE": "北交所"}


def to_bs_code(code: str, market: str = "CN") -> str:
    """纯代码 → baostock 代码（sh./sz./bj.）。"""
    c = code.split(".")[-1]
    if c.startswith("920") or c.startswith(("8", "4")):
        return f"bj.{c}"
    if c.startswith(("6", "5", "9")):  # 沪：6 主板/科创、5 基金、900 沪B
        return f"sh.{c}"
    return f"sz.{c}"  # 深：0/3/2 等


def from_bs_code(bs_code: str) -> str:
    """baostock 代码 → 纯代码。"""
    return bs_code.split(".")[-1]


def board_cn(code: str) -> str:
    return _BOARD_CN.get(board_of(code), "主板")


def _dec(x) -> Decimal | None:
    if x is None or x == "":
        return None
    try:
        return Decimal(str(x))
    except (InvalidOperation, ValueError):
        return None


def _pdate(s) -> date | None:
    if not s:
        return None
    try:
        return date.fromisoformat(str(s))
    except ValueError:
        return None


def map_kline_row(code: str, row, list_date: date | None, market: str = "CN") -> dict:
    """baostock 日 K 行 → stock_daily_klines 行（含自算涨跌停）。"""
    td = _pdate(row.get("date"))
    is_st = str(row.get("isST", "0")) == "1"
    trade_status = 1 if str(row.get("tradestatus", "1")) == "1" else 0
    pre_close = _dec(row.get("preclose"))
    # 上市首日及之前不设涨跌停（简化：未含科创/创业前 5 日特例）
    is_first_day = list_date is not None and td is not None and td <= list_date
    limit_up, limit_down = compute_limit_prices(code, pre_close, is_st=is_st, is_first_day=is_first_day)
    return {
        "market": market,
        "source": "baostock",
        "stock_code": code,
        "trade_date": td,
        "open": _dec(row.get("open")),
        "high": _dec(row.get("high")),
        "low": _dec(row.get("low")),
        "close": _dec(row.get("close")),
        "pre_close": pre_close,
        "volume": _dec(row.get("volume")),
        "amount": _dec(row.get("amount")),
        "turnover_rate": _dec(row.get("turn")),
        "pct_chg": _dec(row.get("pctChg")),
        "limit_up": limit_up,
        "limit_down": limit_down,
        "trade_status": trade_status,
        "is_st": is_st,
        "adjust": "qfq",
    }


def map_factor_row(code: str, row, market: str = "CN") -> dict:
    """baostock 复权因子行 → stock_adjust_factors 行。"""
    return {
        "market": market,
        "stock_code": code,
        "trade_date": _pdate(row.get("dividOperateDate")),
        "fore_factor": _dec(row.get("foreAdjustFactor")),
        "back_factor": _dec(row.get("backAdjustFactor")),
    }


def map_calendar_row(row, market: str = "CN") -> dict | None:
    """baostock query_trade_dates 行 → trading_calendars 行；无日期返回 None。"""
    d = _pdate(row.get("calendar_date"))
    if d is None:
        return None
    return {
        "market": market,
        "cal_date": d,
        "is_open": str(row.get("is_trading_day", "0")) == "1",
        "source": "baostock",
    }


def map_security_row(row, market: str = "CN") -> dict | None:
    """baostock query_stock_basic 行 → securities 行；非股票（指数/其它）返回 None。"""
    if str(row.get("type", "1")) != "1":  # 1=股票
        return None
    code = from_bs_code(row.get("code", ""))
    if not code:
        return None
    name = row.get("code_name", "") or ""
    status = 1 if str(row.get("status", "1")) == "1" else 0
    return {
        "market": market,
        "code": code,
        "name": name,
        "board": board_cn(code),
        "status": status,
        "list_date": _pdate(row.get("ipoDate")),
        "delist_date": _pdate(row.get("outDate")),
        "is_st": "ST" in name.upper(),
    }
