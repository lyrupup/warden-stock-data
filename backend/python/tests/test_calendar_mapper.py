from datetime import date

from app.features.collect.mapper import map_calendar_row


def test_map_calendar_row_trading_day():
    row = {"calendar_date": "2026-06-11", "is_trading_day": "1"}
    out = map_calendar_row(row)
    assert out == {
        "market": "CN",
        "cal_date": date(2026, 6, 11),
        "is_open": True,
        "source": "baostock",
    }


def test_map_calendar_row_closed_day():
    row = {"calendar_date": "2026-06-13", "is_trading_day": "0"}
    out = map_calendar_row(row)
    assert out["is_open"] is False
    assert out["cal_date"] == date(2026, 6, 13)


def test_map_calendar_row_invalid_date():
    assert map_calendar_row({"calendar_date": "", "is_trading_day": "1"}) is None
    assert map_calendar_row({"calendar_date": "bad", "is_trading_day": "1"}) is None
