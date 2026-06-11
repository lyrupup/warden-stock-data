"""表模型（与 backend GORM 模型同表；schema 以 backend/deploy/init.sql 为准）。"""
from __future__ import annotations

from datetime import date, datetime
from decimal import Decimal

from sqlalchemy import (
    BigInteger,
    Boolean,
    Date,
    DateTime,
    Numeric,
    SmallInteger,
    String,
    func,
)
from sqlalchemy.orm import Mapped, mapped_column

from app.models.base import Base


class Security(Base):
    __tablename__ = "securities"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True)
    market: Mapped[str] = mapped_column(String(8), default="CN")
    code: Mapped[str] = mapped_column(String(16))
    name: Mapped[str] = mapped_column(String(64), default="")
    board: Mapped[str] = mapped_column(String(16), default="")
    status: Mapped[int] = mapped_column(SmallInteger, default=1)
    list_date: Mapped[date | None] = mapped_column(Date, nullable=True)
    delist_date: Mapped[date | None] = mapped_column(Date, nullable=True)
    is_st: Mapped[bool] = mapped_column(Boolean, default=False)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())


class StockDailyKline(Base):
    __tablename__ = "stock_daily_klines"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True)
    market: Mapped[str] = mapped_column(String(8), default="CN")
    source: Mapped[str] = mapped_column(String(32), default="baostock")
    stock_code: Mapped[str] = mapped_column(String(16))
    trade_date: Mapped[date] = mapped_column(Date)
    open: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    high: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    low: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    close: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    pre_close: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    volume: Mapped[Decimal | None] = mapped_column(Numeric(20, 0), nullable=True)
    amount: Mapped[Decimal | None] = mapped_column(Numeric(24, 4), nullable=True)
    turnover_rate: Mapped[Decimal | None] = mapped_column(Numeric(10, 4), nullable=True)
    pct_chg: Mapped[Decimal | None] = mapped_column(Numeric(10, 4), nullable=True)
    limit_up: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    limit_down: Mapped[Decimal | None] = mapped_column(Numeric(20, 4), nullable=True)
    trade_status: Mapped[int] = mapped_column(SmallInteger, default=1)
    is_st: Mapped[bool] = mapped_column(Boolean, default=False)
    adjust: Mapped[str] = mapped_column(String(8), default="qfq")
    created_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())


class StockAdjustFactor(Base):
    __tablename__ = "stock_adjust_factors"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True)
    market: Mapped[str] = mapped_column(String(8), default="CN")
    stock_code: Mapped[str] = mapped_column(String(16))
    trade_date: Mapped[date] = mapped_column(Date)
    fore_factor: Mapped[Decimal | None] = mapped_column(Numeric(20, 8), nullable=True)
    back_factor: Mapped[Decimal | None] = mapped_column(Numeric(20, 8), nullable=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())


class UpdateWatermark(Base):
    __tablename__ = "update_watermarks"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True)
    market: Mapped[str] = mapped_column(String(8), default="CN")
    stock_code: Mapped[str] = mapped_column(String(16))
    last_trade_date: Mapped[date | None] = mapped_column(Date, nullable=True)
    updated_at: Mapped[datetime] = mapped_column(DateTime(timezone=True), server_default=func.now())


class TradingCalendar(Base):
    __tablename__ = "trading_calendars"

    id: Mapped[int] = mapped_column(BigInteger, primary_key=True)
    market: Mapped[str] = mapped_column(String(8), default="CN")
    cal_date: Mapped[date] = mapped_column(Date)
    is_open: Mapped[bool] = mapped_column(Boolean, default=False)
    source: Mapped[str] = mapped_column(String(16), default="manual")
