"""baostock 数据拉取（日 K / 复权因子 / 证券列表）。"""
from __future__ import annotations

import baostock as bs
import pandas as pd

from app.features.collect import baostock_client as bc
from app.features.collect.mapper import to_bs_code

_K_FIELDS = "date,open,high,low,close,preclose,volume,amount,turn,tradestatus,pctChg,isST"


def fetch_kline(code: str, start_date: str | None, end_date: str | None, market: str = "CN") -> pd.DataFrame:
    return bc.query_df(
        bs.query_history_k_data_plus,
        to_bs_code(code, market),
        _K_FIELDS,
        start_date=start_date or "",
        end_date=end_date or "",
        frequency="d",
        adjustflag="2",  # 前复权
    )


def fetch_adjust_factor(code: str, start_date: str | None, end_date: str | None, market: str = "CN") -> pd.DataFrame:
    return bc.query_df(
        bs.query_adjust_factor,
        code=to_bs_code(code, market),
        start_date=start_date or "",
        end_date=end_date or "",
    )


def fetch_securities() -> pd.DataFrame:
    return bc.query_df(bs.query_stock_basic)


def fetch_trade_dates(start_date: str | None, end_date: str | None) -> pd.DataFrame:
    """交易日历：返回区间内每个自然日及其是否交易（字段 calendar_date / is_trading_day）。"""
    return bc.query_df(
        bs.query_trade_dates,
        start_date=start_date or "",
        end_date=end_date or "",
    )
