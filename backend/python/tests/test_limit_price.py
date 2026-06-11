from decimal import Decimal

from app.core.limit_price import board_of, compute_limit_prices


def test_board_of():
    assert board_of("600000") == "MAIN"
    assert board_of("000001") == "MAIN"
    assert board_of("002415") == "MAIN"
    assert board_of("300750") == "GEM"
    assert board_of("301000") == "GEM"
    assert board_of("688981") == "STAR"
    assert board_of("830799") == "BSE"
    assert board_of("430047") == "BSE"
    assert board_of("920099") == "BSE"
    assert board_of("sh.600000") == "MAIN"  # 兼容 baostock 前缀


def test_limit_main():
    up, down = compute_limit_prices("600000", Decimal("10"))
    assert up == Decimal("11.00")
    assert down == Decimal("9.00")


def test_limit_main_st_is_5pct():
    up, down = compute_limit_prices("600000", 10, is_st=True)
    assert up == Decimal("10.50")
    assert down == Decimal("9.50")


def test_limit_gem_st_still_20pct():
    up, down = compute_limit_prices("300750", 10, is_st=True)
    assert up == Decimal("12.00")
    assert down == Decimal("8.00")


def test_limit_star_20pct():
    up, down = compute_limit_prices("688981", 10)
    assert up == Decimal("12.00")
    assert down == Decimal("8.00")


def test_limit_bse_30pct():
    up, down = compute_limit_prices("830799", 10)
    assert up == Decimal("13.00")
    assert down == Decimal("7.00")


def test_limit_rounding_half_up():
    # 9.99 * 1.1 = 10.989 → 10.99
    up, _ = compute_limit_prices("600000", Decimal("9.99"))
    assert up == Decimal("10.99")


def test_first_day_no_limit():
    assert compute_limit_prices("600000", 10, is_first_day=True) == (None, None)


def test_no_preclose():
    assert compute_limit_prices("600000", None) == (None, None)
    assert compute_limit_prices("600000", 0) == (None, None)
