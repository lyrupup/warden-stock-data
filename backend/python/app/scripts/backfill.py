"""离线批量回补日 K（CLI）：直接在 quant 进程内调用采集逻辑写库，绕过 Go / HTTP 编排。

适用场景：首次全量历史回补（最快路径，无 HTTP 往返、无 Go 调度开销）。
baostock 在单进程内串行；如需并行，可开多个进程按 --shard 分片，每个进程持有独立 baostock 会话：

    python -m app.scripts.backfill --all --shard 0/4 &
    python -m app.scripts.backfill --all --shard 1/4 &
    python -m app.scripts.backfill --all --shard 2/4 &
    python -m app.scripts.backfill --all --shard 3/4 &

复用与 Go 在线采集完全相同的逻辑：日 K + 复权因子 + 自算涨跌停 + ST + 停牌；并写 update_watermarks，
使概览页「行情数据覆盖」正确反映。断点续跑用 --skip-done 跳过已建立水位的代码。
"""
from __future__ import annotations

import argparse
import logging
import time
from datetime import date

from sqlalchemy import select

from app.core.db import session_scope
from app.features.collect import repo, service
from app.models.tables import Security, UpdateWatermark
from app.schemas.collect import CollectKlineRequest

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
logger = logging.getLogger("backfill")


def _load_codes(market: str, include_delisted: bool, skip_done: bool) -> list[str]:
    with session_scope() as session:
        stmt = select(Security.code).where(Security.market == market)
        if not include_delisted:
            stmt = stmt.where(Security.status == 1)
        codes = [c for (c,) in session.execute(stmt.order_by(Security.code)).all()]
        done: set[str] = set()
        if skip_done:
            res = session.execute(
                select(UpdateWatermark.stock_code).where(
                    UpdateWatermark.market == market,
                    UpdateWatermark.last_trade_date.is_not(None),
                )
            )
            done = {c for (c,) in res.all()}
    if done:
        codes = [c for c in codes if c not in done]
    return codes


def _chunks(seq: list[str], size: int):
    for i in range(0, len(seq), size):
        yield seq[i : i + size]


def run(args: argparse.Namespace) -> None:
    market = args.market
    if args.codes:
        codes = [c.strip() for c in args.codes.split(",") if c.strip()]
    else:
        codes = _load_codes(market, args.include_delisted, args.skip_done)

    if args.shard:
        i, n = args.shard
        codes = codes[i::n]

    total = len(codes)
    logger.info(
        "backfill start mode=%s total=%d chunk=%d shard=%s", args.mode, total, args.chunk, args.shard
    )
    if total == 0:
        logger.info("no codes to backfill")
        return

    ok = skipped = failed = 0
    t0 = time.monotonic()
    for chunk in _chunks(codes, args.chunk):
        req = CollectKlineRequest(
            codes=chunk,
            mode=args.mode,
            market=market,
            from_date=args.from_date,
            to_date=args.to_date,
        )
        results = service.collect_kline_batch(req)
        with session_scope() as session:
            for r in results:
                if r.status == "ok":
                    ok += 1
                    if r.latest_trade_date:
                        repo.upsert_watermark(
                            session, market, r.code, date.fromisoformat(r.latest_trade_date)
                        )
                elif r.status == "skipped":
                    skipped += 1
                else:
                    failed += 1
                    logger.warning("failed code=%s reason=%s", r.code, r.reason)
        done = ok + skipped + failed
        elapsed = time.monotonic() - t0
        rate = done / elapsed if elapsed > 0 else 0.0
        logger.info(
            "progress %d/%d ok=%d skipped=%d failed=%d (%.2f code/s)",
            done, total, ok, skipped, failed, rate,
        )

    logger.info(
        "backfill done ok=%d skipped=%d failed=%d in %.0fs",
        ok, skipped, failed, time.monotonic() - t0,
    )


def _shard(s: str) -> tuple[int, int]:
    try:
        i_str, n_str = s.split("/")
        i, n = int(i_str), int(n_str)
    except ValueError as e:  # noqa: BLE001
        raise argparse.ArgumentTypeError("shard 格式应为 i/n，如 0/4") from e
    if not (0 <= i < n):
        raise argparse.ArgumentTypeError("shard 需满足 0 <= i < n，如 0/4")
    return (i, n)


def main() -> None:
    p = argparse.ArgumentParser(description="离线批量回补日 K（直接写库）")
    p.add_argument("--mode", choices=["full", "incremental"], default="full")
    p.add_argument("--codes", help="逗号分隔代码（指定则忽略 --all）")
    p.add_argument("--all", action="store_true", help="回补全市场（默认仅在市股票）")
    p.add_argument("--include-delisted", action="store_true", help="--all 时包含退市股")
    p.add_argument("--from", dest="from_date", help="起始日 YYYY-MM-DD（覆盖默认）")
    p.add_argument("--to", dest="to_date", help="结束日 YYYY-MM-DD")
    p.add_argument("--chunk", type=int, default=50, help="每次采集的代码数（默认 50）")
    p.add_argument("--shard", type=_shard, help="多进程分片 i/n（如 0/4），用于并行加速")
    p.add_argument("--skip-done", action="store_true", help="跳过已有水位的代码（断点续跑）")
    p.add_argument("--market", default="CN")
    args = p.parse_args()
    if not args.codes and not args.all:
        p.error("需指定 --codes 或 --all")
    run(args)


if __name__ == "__main__":
    main()
