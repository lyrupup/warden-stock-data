"""健康检查（无需内部 token，供容器探活）。"""
from __future__ import annotations

from fastapi import APIRouter

from app.features.collect.baostock_client import is_logged_in

router = APIRouter()


@router.get("/health")
def health() -> dict:
    return {"status": "ok", "baostock_logged_in": is_logged_in()}
