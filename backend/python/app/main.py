"""Warden Python quant 服务入口（采集 + 指标计算）。仅内网，由 Go 服务调用。"""
from __future__ import annotations

from fastapi import FastAPI

from app.core.logging import setup_logging
from app.features.api import collect_router, health_router, indicator_router

setup_logging()

app = FastAPI(title="Warden Quant Service", version="1.0", docs_url="/docs")

app.include_router(health_router.router)
app.include_router(collect_router.router)
app.include_router(indicator_router.router)
