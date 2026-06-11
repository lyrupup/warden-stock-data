"""采集接口请求/响应模型。"""
from __future__ import annotations

from pydantic import BaseModel, Field


class CollectSecuritiesRequest(BaseModel):
    market: str = "CN"


class CollectCalendarRequest(BaseModel):
    market: str = "CN"
    from_date: str | None = None  # YYYY-MM-DD，留空用 BACKFILL_START_DATE
    to_date: str | None = None    # 留空到 baostock 已发布的最新日历


class CollectKlineRequest(BaseModel):
    codes: list[str] = Field(default_factory=list)
    # full: 全量历史回补；incremental: 增量（仅补水位之后，由 Go 传 from 控制起点）
    mode: str = "incremental"
    from_date: str | None = None  # YYYY-MM-DD
    to_date: str | None = None
    market: str = "CN"


class CollectResult(BaseModel):
    code: str
    status: str  # ok / skipped / failed
    rows: int = 0
    latest_trade_date: str | None = None
    reason: str | None = None


class CollectResponse(BaseModel):
    results: list[CollectResult] = Field(default_factory=list)
