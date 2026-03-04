"""Manages Playwright browser contexts with persistent profiles."""

import asyncio
import logging
import os
import uuid
from datetime import datetime
from pathlib import Path

import psutil
from playwright.async_api import Error as PlaywrightError
from playwright.async_api import TimeoutError as PlaywrightTimeoutError
from playwright.async_api import async_playwright

from .actions import execute_action
from .models import (
    ActionSpec,
    DoActionResponse,
    ScreenshotResponse,
    StartSessionResponse,
    StopSessionResponse,
)

logger = logging.getLogger(__name__)


# ---------------------------------------------------------------------------
# Retry utility (P5)
# ---------------------------------------------------------------------------


async def with_retry(fn, retries: int = 2, backoff: float = 2.0):
    """Call *fn* (a coroutine factory), retrying on transient Playwright errors.

    Only retries on TimeoutError / PlaywrightError.  Any other exception
    propagates immediately.  After *retries* failed attempts the original
    exception is re-raised.
    """
    for attempt in range(retries + 1):
        try:
            return await fn()
        except (PlaywrightTimeoutError, PlaywrightError) as exc:
            if attempt == retries:
                raise
            wait = backoff * (attempt + 1)
            logger.warning("Transient Playwright error (attempt %d/%d): %s. Retrying in %.1fs",
                           attempt + 1, retries + 1, exc, wait)
            await asyncio.sleep(wait)


# ---------------------------------------------------------------------------
# Lock helpers (P5)
# ---------------------------------------------------------------------------


def _read_lock_pid(lock_file: Path) -> int | None:
    """Parse PID from a Chrome SingletonLock file.  Returns None on failure."""
    try:
        content = lock_file.read_text(encoding="utf-8", errors="ignore").strip()
        # Chrome writes "hostname-pid" on some platforms; try last segment first
        for part in reversed(content.split("-")):
            try:
                return int(part)
            except ValueError:
                continue
    except OSError:
        pass
    return None


def _check_and_clear_lock(lock_file: Path) -> None:
    """Remove a stale SingletonLock, or raise if the owning process is alive."""
    pid = _read_lock_pid(lock_file)
    if pid is not None and psutil.pid_exists(pid):
        raise ValueError(
            f"Browser profile is in use by PID {pid}. Close the browser first."
        )
    lock_file.unlink(missing_ok=True)
    logger.warning("Removed stale lock file: %s (dead/unknown process)", lock_file)


# ---------------------------------------------------------------------------
# Session data class
# ---------------------------------------------------------------------------


class BrowserSession:
    """Holds a live browser context and its metadata."""

    def __init__(self, session_id, context, page, client_id, app, profile_path):
        self.session_id = session_id
        self.context = context
        self.page = page
        self.client_id = client_id
        self.app = app
        self.profile_path = profile_path


# ---------------------------------------------------------------------------
# BrowserManager
# ---------------------------------------------------------------------------


class BrowserManager:
    """Manages Playwright browser contexts with persistent profiles."""

    def __init__(
        self,
        profiles_root: str,
        slow_mo: int = 50,
        nav_timeout: int = 45000,
        traces_root: str | None = None,
        action_log=None,  # P2: optional ActionLogDB
        policy_engine=None,  # P3: optional PolicyEngine
    ):
        self._profiles_root = str(Path(profiles_root).resolve())  # force absolute
        self._slow_mo = slow_mo
        self._nav_timeout = nav_timeout
        self._traces_root = traces_root or os.environ.get("DATA_DIR", "./data") + "/traces"
        self._sessions: dict[str, BrowserSession] = {}
        self._pw = None
        self._action_log = action_log
        self._policy_engine = policy_engine

    async def _ensure_playwright(self):
        if self._pw is None:
            self._pw = await async_playwright().start()

    def _profile_path(self, client_id: str, app: str) -> str:
        return os.path.join(self._profiles_root, client_id, app)

    def _traces_dir(self, session_id: str) -> str:
        path = str(Path(self._traces_root).resolve() / session_id)
        Path(path).mkdir(parents=True, exist_ok=True)
        return path

    async def start_session(self, client_id: str, app: str) -> StartSessionResponse:
        """Launch headed browser with persistent profile for client+app."""
        await self._ensure_playwright()
        profile_path = self._profile_path(client_id, app)

        # Check for concurrent use of same profile
        for s in self._sessions.values():
            if s.profile_path == profile_path:
                raise ValueError(f"Profile already in use: {profile_path}")

        # Ensure profile directory exists
        Path(profile_path).mkdir(parents=True, exist_ok=True)

        # P5: process-aware lock handling
        lock_file = Path(profile_path) / "SingletonLock"
        if lock_file.exists():
            _check_and_clear_lock(lock_file)

        context = await self._pw.chromium.launch_persistent_context(
            user_data_dir=profile_path,
            headless=False,
            slow_mo=self._slow_mo,
            viewport={"width": 1280, "height": 900},
        )
        context.set_default_navigation_timeout(self._nav_timeout)

        page = context.pages[0] if context.pages else await context.new_page()
        session_id = str(uuid.uuid4())

        session = BrowserSession(
            session_id=session_id,
            context=context,
            page=page,
            client_id=client_id,
            app=app,
            profile_path=profile_path,
        )
        self._sessions[session_id] = session
        logger.info("Started session %s for %s/%s", session_id, client_id, app)

        return StartSessionResponse(
            session_id=session_id,
            client_id=client_id,
            app=app,
            profile_path=profile_path,
        )

    async def do_action(
        self, session_id: str, action_spec: ActionSpec, bypass_policy: bool = False
    ) -> DoActionResponse:
        """Execute an allowlisted read action. Refuses write actions in v1."""
        session = self._sessions.get(session_id)
        if not session:
            return DoActionResponse(
                success=False,
                action_id=str(uuid.uuid4()),
                error=f"Session not found: {session_id}",
            )

        traces_dir = self._traces_dir(session_id)

        # P3: determine overnight mode for policy evaluation
        is_overnight = False
        if self._policy_engine is not None:
            is_overnight = self._policy_engine.is_overnight(datetime.now().hour)

        try:
            # P5: wrap execution in retry for transient Playwright errors
            result = await with_retry(
                lambda: execute_action(
                    session.page,
                    action_spec,
                    traces_dir,
                    action_log=self._action_log,
                    policy_engine=None if bypass_policy else self._policy_engine,
                    client_id=session.client_id,
                    is_overnight=is_overnight,
                    session_id=session_id,
                )
            )
        except Exception as e:
            # On failure, keep browser open for manual inspection
            logger.error("Action failed: %s", e, exc_info=True)
            screenshot_path = str(
                Path(traces_dir) / f"error_{datetime.now():%Y%m%d_%H%M%S}.png"
            )
            try:
                await session.page.screenshot(path=screenshot_path)
            except Exception:
                pass
            result = DoActionResponse(
                success=False,
                action_id=str(uuid.uuid4()),
                error=str(e),
            )

        return result

    async def screenshot(self, session_id: str) -> ScreenshotResponse:
        """Capture current page screenshot."""
        session = self._sessions.get(session_id)
        if not session:
            raise ValueError(f"Session not found: {session_id}")

        traces_dir = self._traces_dir(session_id)
        timestamp = datetime.now()
        path = str(Path(traces_dir) / f"screenshot_{timestamp:%Y%m%d_%H%M%S}.png")
        await session.page.screenshot(path=path, full_page=False)

        return ScreenshotResponse(path=path, timestamp=timestamp)

    async def stop_session(self, session_id: str) -> StopSessionResponse:
        """Close browser context and save profile state."""
        session = self._sessions.pop(session_id, None)
        if not session:
            return StopSessionResponse(success=False, session_id=session_id)

        await session.context.close()  # saves profile state to disk
        logger.info("Stopped session %s", session_id)
        return StopSessionResponse(success=True, session_id=session_id)

    async def close_all(self) -> None:
        """Shut down all sessions and the Playwright instance."""
        for session_id in list(self._sessions.keys()):
            await self.stop_session(session_id)
        if self._pw:
            await self._pw.stop()
            self._pw = None
