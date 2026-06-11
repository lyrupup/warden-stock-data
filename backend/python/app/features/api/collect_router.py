"""采集内部接口（仅 Go 经 X-Internal-Token 调用）。

采用同步 def：FastAPI 在线程池执行，baostock 阻塞调用不占用事件循环。
"""
from __future__ import annotations

from fastapi import APIRouter, Depends

from app.core.security import verify_internal_token
from app.features.collect import service
from app.schemas.collect import (
    CollectCalendarRequest,
    CollectKlineRequest,
    CollectResponse,
    CollectSecuritiesRequest,
)

router = APIRouter(prefix="/internal/v1/collect", dependencies=[Depends(verify_internal_token)])


@router.post("/securities")
def collect_securities(req: CollectSecuritiesRequest) -> dict:
    count = service.collect_securities(req)
    return {"count": count}


@router.post("/calendar")
def collect_calendar(req: CollectCalendarRequest) -> dict:
    count = service.collect_calendar(req)
    return {"count": count}


@router.post("/kline", response_model=CollectResponse)
def collect_kline(req: CollectKlineRequest) -> CollectResponse:
    results = service.collect_kline_batch(req)
    return CollectResponse(results=results)
