"""技术指标计算（基于 pandas/numpy，口径对齐通达信/原 Go 因子引擎）。

依赖专业指标库 pandas-ta-classic 作为扩展底座（未来可直接接入 OBV/CCI/DMI 等）；
A 股特殊口径指标（KDJ 的 SMA 平滑、MACD 柱×2 通达信口径）采用本模块自实现以保证一致性。
输入为升序日 K 的 DataFrame（列：open/high/low/close/volume），输出与索引对齐的 Series。
所有计算仅用「当前及之前」数据，逐 bar 切片即 point-in-time，无未来函数。
"""
from __future__ import annotations

import re

import numpy as np
import pandas as pd


def _ema(s: pd.Series, n: int) -> pd.Series:
    return s.ewm(span=n, adjust=False).mean()


def _wilder(s: pd.Series, n: int) -> pd.Series:
    # Wilder 平滑等价于 alpha = 1/n 的 EWM
    return s.ewm(alpha=1.0 / n, adjust=False).mean()


def _tdx_sma(s: pd.Series, n: int, m: int, init: float = 50.0) -> pd.Series:
    """通达信 SMA(X, N, M)：Y = (X*M + Y_prev*(N-M)) / N，带初值递推（KDJ 用）。"""
    alpha = m / n
    arr = s.to_numpy(dtype=float)
    out = np.full(len(arr), np.nan)
    prev = init
    for i, x in enumerate(arr):
        if np.isnan(x):
            out[i] = prev
            continue
        prev = alpha * x + (1 - alpha) * prev
        out[i] = prev
    return pd.Series(out, index=s.index)


def _macd(close: pd.Series, fast: int = 12, slow: int = 26, signal: int = 9):
    dif = _ema(close, fast) - _ema(close, slow)
    dea = _ema(dif, signal)
    bar = (dif - dea) * 2  # 通达信柱口径
    return dif, dea, bar


def _kdj(high: pd.Series, low: pd.Series, close: pd.Series, n: int = 9):
    low_n = low.rolling(n).min()
    high_n = high.rolling(n).max()
    rng = (high_n - low_n).replace(0, np.nan)
    rsv = (close - low_n) / rng * 100
    k = _tdx_sma(rsv, 3, 1, init=50.0)
    d = _tdx_sma(k, 3, 1, init=50.0)
    j = 3 * k - 2 * d
    return k, d, j


def _rsi(close: pd.Series, n: int) -> pd.Series:
    delta = close.diff()
    gain = delta.clip(lower=0)
    loss = (-delta).clip(lower=0)
    avg_gain = _wilder(gain, n)
    avg_loss = _wilder(loss, n)
    rs = avg_gain / avg_loss
    rsi = 100 - 100 / (1 + rs)
    return rsi.where(avg_loss != 0, 100.0)


def _atr(high: pd.Series, low: pd.Series, close: pd.Series, n: int) -> pd.Series:
    prev_close = close.shift(1)
    tr = pd.concat(
        [(high - low), (high - prev_close).abs(), (low - prev_close).abs()],
        axis=1,
    ).max(axis=1)
    return _wilder(tr, n)


def _boll(close: pd.Series, n: int = 20, k: float = 2.0):
    mid = close.rolling(n).mean()
    std = close.rolling(n).std(ddof=0)  # 总体标准差
    return mid, mid + k * std, mid - k * std


def _compute_one(t: str, close, high, low, vol) -> pd.Series | None:
    if m := re.fullmatch(r"ma(\d+)", t):
        return close.rolling(int(m.group(1))).mean()
    if m := re.fullmatch(r"rsi(\d+)", t):
        return _rsi(close, int(m.group(1)))
    if m := re.fullmatch(r"atr(\d+)", t):
        return _atr(high, low, close, int(m.group(1)))
    if m := re.fullmatch(r"pct_change(\d+)", t):
        return close.pct_change(int(m.group(1))) * 100
    if m := re.fullmatch(r"bias(\d+)", t):
        n = int(m.group(1))
        ma = close.rolling(n).mean()
        return (close - ma) / ma * 100
    if t in ("macd_dif", "macd_dea", "macd_bar"):
        dif, dea, bar = _macd(close)
        return {"macd_dif": dif, "macd_dea": dea, "macd_bar": bar}[t]
    if t in ("kdj_k", "kdj_d", "kdj_j"):
        k, d, j = _kdj(high, low, close)
        return {"kdj_k": k, "kdj_d": d, "kdj_j": j}[t]
    if t in ("boll_mid", "boll_upper", "boll_lower"):
        mid, up, lo = _boll(close)
        return {"boll_mid": mid, "boll_upper": up, "boll_lower": lo}[t]
    if t == "vol_ratio":
        return vol / vol.rolling(5).mean()
    if t == "amplitude":
        return (high - low) / close.shift(1) * 100
    if t == "ma_align":
        ma5, ma10, ma20 = close.rolling(5).mean(), close.rolling(10).mean(), close.rolling(20).mean()
        ma30, ma60 = close.rolling(30).mean(), close.rolling(60).mean()
        cond = (ma5 > ma10) & (ma10 > ma20) & (ma20 > ma30) & (ma30 > ma60)
        return cond.astype(float)
    return None


def compute_indicators(df: pd.DataFrame, types: list[str]) -> dict[str, pd.Series]:
    """计算请求的指标，返回 {type: Series}（与 df.index 对齐）。未知 type 跳过。"""
    if df.empty:
        return {}
    close = df["close"].astype(float)
    high = df["high"].astype(float)
    low = df["low"].astype(float)
    vol = df["volume"].astype(float)
    out: dict[str, pd.Series] = {}
    for t in types:
        s = _compute_one(t, close, high, low, vol)
        if s is not None:
            out[t] = s
    return out
