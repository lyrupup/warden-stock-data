"""涨跌停价自算（纯函数，便于单测）。

A 股涨跌幅制度（注册制后）：
- 主板（沪 60*/深 000*/001*/002*/003*）       ±10%
- 创业板（300*/301*）、科创板（688*/689*）      ±20%
- 北交所（8**/4**/920*）                        ±30%
- 主板 ST                                       ±5%
- 创业板/科创板 ST                               仍 ±20%（注册制后与非 ST 相同）
- 北交所 ST                                      仍 ±30%
- 新股上市首日 / 科创创业前 5 日不设限            → 返回 (None, None)
"""
from __future__ import annotations

from decimal import ROUND_HALF_UP, Decimal


def board_of(code: str) -> str:
    """按代码段判定板块。兼容 baostock 的 'sh.600000' 与纯代码 '600000'。"""
    c = code.split(".")[-1]
    if c.startswith(("688", "689")):
        return "STAR"  # 科创板
    if c.startswith(("300", "301")):
        return "GEM"   # 创业板
    if c.startswith("920") or c.startswith(("8", "4")):
        return "BSE"   # 北交所
    return "MAIN"      # 主板


def limit_pct(code: str, is_st: bool) -> Decimal:
    """返回该标的当日涨跌幅比例（单边）。"""
    board = board_of(code)
    if board == "BSE":
        return Decimal("0.30")
    if board in ("GEM", "STAR"):
        return Decimal("0.20")  # 创业/科创：ST 也按 20%
    return Decimal("0.05") if is_st else Decimal("0.10")  # 主板：ST 5%，否则 10%


def compute_limit_prices(
    code: str,
    pre_close: Decimal | float | str | None,
    is_st: bool = False,
    is_first_day: bool = False,
) -> tuple[Decimal | None, Decimal | None]:
    """计算涨跌停价（四舍五入到分）；不设限 / 无有效昨收时返回 (None, None)。"""
    if is_first_day or pre_close is None:
        return None, None
    pc = Decimal(str(pre_close))
    if pc <= 0:
        return None, None
    pct = limit_pct(code, is_st)
    up = (pc * (Decimal("1") + pct)).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    down = (pc * (Decimal("1") - pct)).quantize(Decimal("0.01"), rounding=ROUND_HALF_UP)
    return up, down
