"""服务配置：从环境变量加载（与 backend 共用 PG_* 约定）。"""
from __future__ import annotations

from functools import lru_cache

from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    # 统一环境变量在 backend/.env（go 与 python 共用）；本机直跑时相对 backend/python 为 ../.env。
    # Docker 部署经 compose env_file 注入进程环境变量，优先级高于此文件。
    model_config = SettingsConfigDict(
        env_file=("../.env", ".env"), case_sensitive=False, extra="ignore"
    )

    quant_port: int = 8000
    quant_env: str = "dev"
    # Go 调用本服务时携带的内部共享密钥
    internal_token: str = "change_me_internal_token"

    pg_host: str = "localhost"
    pg_port: int = 5432
    pg_user: str = "postgres"
    pg_password: str = "postgres"
    pg_db: str = "warden_data"
    pg_sslmode: str = "disable"

    collect_baostock_retry: int = 3
    backfill_start_date: str = "1990-12-19"  # A 股最早交易日；全量回补自上市以来全部历史

    @property
    def database_url(self) -> str:
        return (
            f"postgresql+psycopg://{self.pg_user}:{self.pg_password}"
            f"@{self.pg_host}:{self.pg_port}/{self.pg_db}?sslmode={self.pg_sslmode}"
        )


@lru_cache
def get_settings() -> Settings:
    return Settings()
