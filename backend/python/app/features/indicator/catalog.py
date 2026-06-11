"""指标目录（接入方发现服务支持哪些指标的单一事实源）。

v2：指标全部实时计算，不再有「快照」概念，故目录无 snapshot 字段。
"""
from __future__ import annotations

_CATALOG: list[dict] = [
    {"type": "ma5", "name": "MA5", "group": "trend", "value_type": "number", "params": {"period": 5}},
    {"type": "ma10", "name": "MA10", "group": "trend", "value_type": "number", "params": {"period": 10}},
    {"type": "ma20", "name": "MA20", "group": "trend", "value_type": "number", "params": {"period": 20}},
    {"type": "ma30", "name": "MA30", "group": "trend", "value_type": "number", "params": {"period": 30}},
    {"type": "ma60", "name": "MA60", "group": "trend", "value_type": "number", "params": {"period": 60}},
    {"type": "macd_dif", "name": "MACD DIF", "group": "momentum", "value_type": "number", "params": {"fast": 12, "slow": 26, "signal": 9}},
    {"type": "macd_dea", "name": "MACD DEA", "group": "momentum", "value_type": "number", "params": {"fast": 12, "slow": 26, "signal": 9}},
    {"type": "macd_bar", "name": "MACD 柱", "group": "momentum", "value_type": "number", "params": {"fast": 12, "slow": 26, "signal": 9}},
    {"type": "kdj_k", "name": "KDJ K", "group": "momentum", "value_type": "number", "params": {"n": 9, "k": 3, "d": 3}},
    {"type": "kdj_d", "name": "KDJ D", "group": "momentum", "value_type": "number", "params": {"n": 9, "k": 3, "d": 3}},
    {"type": "kdj_j", "name": "KDJ J", "group": "momentum", "value_type": "number", "params": {"n": 9, "k": 3, "d": 3}},
    {"type": "rsi6", "name": "RSI6", "group": "momentum", "value_type": "number", "params": {"period": 6}},
    {"type": "rsi12", "name": "RSI12", "group": "momentum", "value_type": "number", "params": {"period": 12}},
    {"type": "rsi24", "name": "RSI24", "group": "momentum", "value_type": "number", "params": {"period": 24}},
    {"type": "boll_mid", "name": "BOLL 中轨", "group": "volatility", "value_type": "number", "params": {"n": 20, "k": 2}},
    {"type": "boll_upper", "name": "BOLL 上轨", "group": "volatility", "value_type": "number", "params": {"n": 20, "k": 2}},
    {"type": "boll_lower", "name": "BOLL 下轨", "group": "volatility", "value_type": "number", "params": {"n": 20, "k": 2}},
    {"type": "atr14", "name": "ATR14", "group": "volatility", "value_type": "number", "params": {"period": 14}},
    {"type": "atr20", "name": "ATR20", "group": "volatility", "value_type": "number", "params": {"period": 20}},
    {"type": "pct_change20", "name": "20 日涨跌幅", "group": "momentum", "value_type": "number", "params": {"period": 20}},
    {"type": "pct_change60", "name": "60 日涨跌幅", "group": "momentum", "value_type": "number", "params": {"period": 60}},
    {"type": "bias6", "name": "乖离率 BIAS6", "group": "deviation", "value_type": "number", "params": {"period": 6}},
    {"type": "vol_ratio", "name": "量比", "group": "volume", "value_type": "number", "params": {"period": 5}},
    {"type": "amplitude", "name": "振幅", "group": "volatility", "value_type": "number", "params": {}},
    {"type": "ma_align", "name": "均线多头排列", "group": "trend", "value_type": "bool", "params": {}},
]

# 常用默认指标集合（前端 K 线副图 / 接入方默认请求）
_DEFAULT_TYPES = [
    "ma5", "ma10", "ma20", "ma30", "ma60",
    "macd_dif", "macd_dea", "macd_bar",
    "kdj_k", "kdj_d", "kdj_j",
    "rsi6", "rsi12", "rsi24",
    "boll_mid", "boll_upper", "boll_lower",
    "atr14", "atr20",
    "pct_change20", "pct_change60",
]


def catalog() -> list[dict]:
    return [dict(item, implemented=True) for item in _CATALOG]


def default_types() -> list[str]:
    return list(_DEFAULT_TYPES)
