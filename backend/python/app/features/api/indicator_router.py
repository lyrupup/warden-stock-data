"""指标内部接口（仅 Go 经 X-Internal-Token 调用）。"""
from __future__ import annotations

from fastapi import APIRouter, Depends

from app.core.security import verify_internal_token
from app.features.indicator import catalog, service
from app.schemas.indicator import (
    IndicatorRequest,
    IndicatorResponse,
    IndicatorSeriesRequest,
    IndicatorSeriesResponse,
)

router = APIRouter(prefix="/internal/v1", dependencies=[Depends(verify_internal_token)])


@router.get("/catalog")
def get_catalog() -> dict:
    return {"indicators": catalog.catalog(), "default_types": catalog.default_types()}


@router.post("/indicators", response_model=IndicatorResponse)
def indicators(req: IndicatorRequest) -> IndicatorResponse:
    res = service.batch_indicators(
        req.codes, req.types, req.period, req.adjust, req.trade_date, req.market
    )
    return IndicatorResponse(results=res)


@router.post("/indicators/series", response_model=IndicatorSeriesResponse)
def indicators_series(req: IndicatorSeriesRequest) -> IndicatorSeriesResponse:
    res = service.series_indicators(
        req.code, req.types, req.period, req.adjust,
        req.limit, req.offset, req.from_date, req.to_date, req.market,
    )
    return IndicatorSeriesResponse(stock_code=req.code, indicators=res)
