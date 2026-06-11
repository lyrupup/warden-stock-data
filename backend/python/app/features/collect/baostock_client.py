"""baostock 客户端封装：进程内登录复用 + 串行调用（baostock 非并发安全）+ 失效连接自愈。

真正的并发由 Go job_runner 控制（逐批 HTTP 调用本服务）；本服务内对 baostock 调用全程持锁串行。

健壮性设计（针对 baostock 长连接被服务端关闭后复用导致的死循环）：
- baostock `send_msg` 的接收循环在 socket 被对端关闭后会对 EOF（空字节）空转，
  永不命中结束分隔符 → 100% CPU 死循环、永不返回。
- 为此：(1) 全局 socket 默认超时，避免半开连接 recv 永久阻塞；
  (2) 空闲超阈值主动重登录（换新连接），避免复用已被关闭的连接；
  (3) 查询异常时强制 reset socket + login 重试一次（不对失效连接调 logout）。
"""
from __future__ import annotations

import contextlib
import logging
import signal
import socket
import threading
import time

import baostock as bs
import pandas as pd

logger = logging.getLogger(__name__)

# baostock 底层 socket 无超时；给一个默认超时，避免半开连接 recv 永久阻塞。
# 在任何 bs.login() 建连之前设置，使新建 socket 继承该超时。
_SOCKET_TIMEOUT_SECONDS = 30.0
socket.setdefaulttimeout(_SOCKET_TIMEOUT_SECONDS)

# baostock 默认每页仅 500 条，结果集靠 next() 逐页拉取，每页一次网络往返（实测每页 ~3s）。
# 一只股票 35 年日 K ≈ 8500 行需翻 18 页 ≈ 56s，是全量回补慢的根因。
# 调大每页条数让单只历史一页返回（实测 8585 行：56s → 0s 遍历），全量提速数倍。
_PER_PAGE_COUNT = 10000
try:
    import baostock.common.contants as _bs_const  # 注意：baostock 包内拼写即为 contants

    _bs_const.BAOSTOCK_PER_PAGE_COUNT = _PER_PAGE_COUNT
except Exception as _e:  # noqa: BLE001
    logger.warning("set baostock per_page_count failed: %s", _e)

# 空闲超过该时长则视连接可能已被服务端关闭，下次查询前主动重新登录换新连接。
_IDLE_RELOGIN_SECONDS = 60.0

# 单次查询硬超时看门狗：正常单只全量历史 ≤ ~30s，给足余量。
# 用于打断「socket 被对端半关后 send_msg 对 EOF 空转的 100% CPU 死循环」——
# 该循环不阻塞（recv 立即返回空字节），socket 超时无效，唯有 SIGALRM 能从纯 Python 循环中断出来。
_QUERY_TIMEOUT_SECONDS = 90.0

_lock = threading.Lock()
_state = {"logged_in": False, "last_active": 0.0}


class _QueryTimeout(Exception):
    """单次 baostock 查询超过硬超时（疑似死连接空转）。"""


@contextlib.contextmanager
def _hard_timeout(seconds: float):
    """基于 SIGALRM 的硬超时。仅在主线程可用（signal 限制）。

    非主线程（如 FastAPI 线程池）退化为无超时，依赖 socket 超时与空闲重登录兜底；
    密集全量回补走 CLI 主线程，正好覆盖死循环高发场景。
    """
    if threading.current_thread() is not threading.main_thread():
        yield
        return

    def _on_alarm(signum, frame):
        raise _QueryTimeout(f"query exceeded {seconds:.0f}s")

    prev = signal.signal(signal.SIGALRM, _on_alarm)
    signal.setitimer(signal.ITIMER_REAL, seconds)
    try:
        yield
    finally:
        signal.setitimer(signal.ITIMER_REAL, 0)
        signal.signal(signal.SIGALRM, prev)


def _reset_socket_locked() -> None:
    """强制丢弃 baostock 底层 socket。

    不可对失效连接调用 bs.logout()：其内部 send_msg 在 EOF 时会死循环（与 query 相同）。
    直接 close 并清空 context.default_socket，再由 login() 建新连接。
    """
    try:
        import baostock.common.context as ctx

        sock = getattr(ctx, "default_socket", None)
        if sock is not None:
            try:
                sock.close()
            except OSError:
                pass
            setattr(ctx, "default_socket", None)
    except Exception as e:  # noqa: BLE001
        logger.debug("reset baostock socket ignored: %s", e)
    _state["logged_in"] = False


def _login_locked() -> None:
    res = bs.login()
    if res.error_code != "0":
        raise RuntimeError(f"baostock login failed: {res.error_code} {res.error_msg}")
    _state["logged_in"] = True
    _state["last_active"] = time.monotonic()
    logger.info("baostock logged in")


def _relogin_locked(reason: str) -> None:
    logger.warning("baostock relogin (%s)", reason)
    _reset_socket_locked()
    _login_locked()


def _ensure_fresh_login_locked() -> None:
    """确保已登录，且连接未因长时间空闲而可能失效（空闲超阈值则换新连接）。"""
    if not _state["logged_in"]:
        _login_locked()
        return
    idle = time.monotonic() - _state["last_active"]
    if idle >= _IDLE_RELOGIN_SECONDS:
        _relogin_locked(f"idle {idle:.0f}s >= {_IDLE_RELOGIN_SECONDS:.0f}s")


def _run_query_locked(fn, *args, **kwargs) -> pd.DataFrame:
    rs = fn(*args, **kwargs)
    if rs is None:
        return pd.DataFrame()
    if rs.error_code != "0":
        raise RuntimeError(f"baostock query failed: {rs.error_code} {rs.error_msg}")
    rows = []
    while rs.error_code == "0" and rs.next():
        rows.append(rs.get_row_data())
    _state["last_active"] = time.monotonic()
    return pd.DataFrame(rows, columns=rs.fields)


def query_df(fn, *args, **kwargs) -> pd.DataFrame:
    """串行执行一次 baostock 查询并转为 DataFrame（登录 + 调用 + 遍历全在锁内）。

    失效连接自愈：空闲超阈值先换新连接；查询抛错（含 socket 超时 / 硬超时空转）则重登录重试一次。
    重试仍超时则抛出，由上层将该只标的计为失败并继续，避免整批永久卡死。
    """
    with _lock:
        _ensure_fresh_login_locked()
        try:
            with _hard_timeout(_QUERY_TIMEOUT_SECONDS):
                return _run_query_locked(fn, *args, **kwargs)
        except (OSError, socket.timeout, RuntimeError, _QueryTimeout) as e:
            _relogin_locked(f"query error: {e}")
            with _hard_timeout(_QUERY_TIMEOUT_SECONDS):
                return _run_query_locked(fn, *args, **kwargs)


def is_logged_in() -> bool:
    return _state["logged_in"]
