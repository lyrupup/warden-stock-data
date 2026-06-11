"""指标接口请求/响应模型。"""
from __future__ import annotations

from pydantic import BaseModel, Field


class IndicatorRequest(BaseModel):
    codes: list[str] = Field(default_factory=list)
    types: list[str] = Field(default_factory=list)
    period: str = "day"
    adjust: str = "qfq"
    trade_date: str | None = None  # 指定交易日（默认最新一日）
    market: str = "CN"


class IndicatorSeriesRequest(BaseModel):
    code: str
    types: list[str] = Field(default_factory=list)
    period: str = "day"
    adjust: str = "qfq"
    limit: int = 0
    offset: int = 0
    from_date: str | None = None
    to_date: str | None = None
    market: str = "CN"


class IndicatorResult(BaseModel):
    stock_code: str
    trade_date: str | None = None
    values: dict[str, str] = Field(default_factory=dict)


class IndicatorResponse(BaseModel):
    results: list[IndicatorResult] = Field(default_factory=list)


class IndicatorSeriesResponse(BaseModel):
    stock_code: str
    indicators: list[IndicatorResult] = Field(default_factory=list)


class CatalogItem(BaseModel):
    type: str
    name: str
    group: str
    value_type: str = "number"
    params: dict = Field(default_factory=dict)
    implemented: bool = True
