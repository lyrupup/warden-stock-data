"""内部鉴权：仅允许携带正确 X-Internal-Token 的 Go 服务调用。"""
from __future__ import annotations

from fastapi import Header, HTTPException, status

from app.core.config import get_settings


async def verify_internal_token(x_internal_token: str | None = Header(default=None)) -> None:
    expected = get_settings().internal_token
    if not x_internal_token or x_internal_token != expected:
        raise HTTPException(
            status_code=status.HTTP_401_UNAUTHORIZED,
            detail="invalid or missing X-Internal-Token",
        )
