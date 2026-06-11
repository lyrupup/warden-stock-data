import pandas as pd

from app.features.indicator.compute import compute_indicators


def _df(closes: list[float]) -> pd.DataFrame:
    n = len(closes)
    return pd.DataFrame(
        {
            "open": closes,
            "high": [c + 0.5 for c in closes],
            "low": [c - 0.5 for c in closes],
            "close": closes,
            "volume": [100.0] * n,
        }
    )


def test_ma5():
    out = compute_indicators(_df([10, 11, 12, 13, 14]), ["ma5"])
    assert abs(out["ma5"].iloc[-1] - 12.0) < 1e-9


def test_ma_insufficient_is_nan():
    out = compute_indicators(_df([10, 11]), ["ma5"])
    assert pd.isna(out["ma5"].iloc[-1])


def test_macd_keys_present():
    out = compute_indicators(_df(list(range(1, 80))), ["macd_dif", "macd_dea", "macd_bar"])
    assert set(out) == {"macd_dif", "macd_dea", "macd_bar"}
    assert out["macd_dif"].notna().iloc[-1]


def test_kdj_bounded():
    closes = [10 + (i % 5) for i in range(40)]
    out = compute_indicators(_df(closes), ["kdj_k", "kdj_d", "kdj_j"])
    k = out["kdj_k"].iloc[-1]
    assert 0 <= k <= 100


def test_unknown_type_skipped():
    out = compute_indicators(_df([10, 11, 12]), ["not_an_indicator"])
    assert out == {}
