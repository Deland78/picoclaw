"""Unit tests for browser_worker — Playwright mocked with AsyncMock."""

from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
from playwright.async_api import TimeoutError as PlaywrightTimeoutError

from services.browser_worker.models import ActionSpec
from services.browser_worker.playwright_runner import BrowserManager, with_retry


@pytest.fixture
def tmp_profiles(tmp_path):
    """Temporary profiles directory."""
    return str(tmp_path / "profiles")


@pytest.fixture
def tmp_traces(tmp_path):
    """Temporary traces directory."""
    traces = tmp_path / "traces"
    traces.mkdir()
    return str(traces)


@pytest.fixture
def mock_playwright():
    """Mocked Playwright instance — no real browser launched."""
    with patch(
        "services.browser_worker.playwright_runner.async_playwright"
    ) as mock_pw_fn:
        mock_pw = AsyncMock()
        mock_pw_fn.return_value.start = AsyncMock(return_value=mock_pw)

        # Mock persistent context
        mock_context = AsyncMock()
        mock_page = AsyncMock()
        mock_page.url = "https://example.com"
        mock_page.inner_text = AsyncMock(return_value="Page text content")
        mock_context.pages = [mock_page]
        mock_context.set_default_navigation_timeout = MagicMock()
        mock_pw.chromium.launch_persistent_context = AsyncMock(
            return_value=mock_context
        )

        yield {"pw": mock_pw, "context": mock_context, "page": mock_page}


@pytest.fixture
async def manager(tmp_profiles, tmp_traces, mock_playwright):
    """BrowserManager with mocked Playwright."""
    mgr = BrowserManager(
        profiles_root=tmp_profiles, slow_mo=0, nav_timeout=5000, traces_root=tmp_traces
    )
    yield mgr
    await mgr.close_all()


# --- Happy-path tests ---


async def test_start_session_creates_profile_dir(manager, tmp_profiles):
    """start_session creates the profile directory and returns session info."""
    result = await manager.start_session("clientA", "jira")
    assert result.session_id
    assert result.client_id == "clientA"
    assert result.app == "jira"
    assert Path(result.profile_path).exists()


async def test_start_session_uses_absolute_paths(manager, mock_playwright):
    """start_session always passes absolute path to launch_persistent_context."""
    await manager.start_session("clientA", "jira")
    call_args = mock_playwright["pw"].chromium.launch_persistent_context.call_args
    user_data_dir = call_args.kwargs["user_data_dir"]
    assert Path(user_data_dir).is_absolute()


async def test_do_action_allowlisted_succeeds(manager, mock_playwright):
    """do_action executes an allowlisted action (jira_open)."""
    session = await manager.start_session("clientA", "jira")
    result = await manager.do_action(
        session.session_id, ActionSpec(action="jira_open", params={"url": "https://example.com"})
    )
    assert result.success is True
    assert result.action_id


async def test_screenshot_saves_png(manager, mock_playwright, tmp_traces):
    """screenshot captures current page viewport."""
    session = await manager.start_session("clientA", "jira")
    result = await manager.screenshot(session.session_id)
    assert result.path.endswith(".png")
    assert result.timestamp
    # Verify page.screenshot was called
    mock_playwright["page"].screenshot.assert_called()


async def test_stop_session_closes_context(manager, mock_playwright):
    """stop_session calls context.close() to persist profile state."""
    session = await manager.start_session("clientA", "jira")
    result = await manager.stop_session(session.session_id)
    assert result.success is True
    mock_playwright["context"].close.assert_called_once()


# --- Error-path tests ---


async def test_do_action_rejects_non_allowlisted(manager, mock_playwright):
    """do_action refuses unimplemented actions (write actions are not in ACTION_REGISTRY)."""
    session = await manager.start_session("clientA", "jira")
    result = await manager.do_action(
        session.session_id,
        ActionSpec(action="jira_add_comment", params={"body": "test"}),
    )
    assert result.success is False
    assert "not implemented" in result.error


async def test_start_session_rejects_duplicate_profile(manager, mock_playwright):
    """start_session raises ValueError if profile is already in use."""
    await manager.start_session("clientA", "jira")
    with pytest.raises(ValueError, match="already in use"):
        await manager.start_session("clientA", "jira")


async def test_do_action_keeps_browser_open_on_failure(manager, mock_playwright):
    """On action failure, browser stays open for manual inspection."""
    mock_playwright["page"].goto = AsyncMock(side_effect=Exception("Network error"))
    session = await manager.start_session("clientA", "jira")
    result = await manager.do_action(
        session.session_id,
        ActionSpec(action="jira_open", params={"url": "https://example.com"}),
    )
    assert result.success is False
    # Session should still exist (browser not closed)
    assert session.session_id in manager._sessions


# ---------------------------------------------------------------------------
# P5 — Selector-based waiting and scoped text extraction
# ---------------------------------------------------------------------------


async def test_jira_capture_waits_for_selector(manager, mock_playwright):
    """jira_capture calls wait_for_selector instead of wait_for_timeout."""
    session = await manager.start_session("clientA", "jira")
    page = mock_playwright["page"]
    page.wait_for_selector = AsyncMock()
    page.inner_text = AsyncMock(return_value="main content")

    await manager.do_action(
        session.session_id,
        ActionSpec(action="jira_capture", params={}),
    )

    page.wait_for_selector.assert_called()
    # Must NOT have called wait_for_timeout (old sleep-based approach)
    page.wait_for_timeout.assert_not_called()


async def test_ado_capture_waits_for_selector(manager, mock_playwright):
    """ado_capture calls wait_for_selector instead of wait_for_timeout."""
    session = await manager.start_session("clientA", "ado")
    page = mock_playwright["page"]
    page.wait_for_selector = AsyncMock()
    page.inner_text = AsyncMock(return_value="main content")

    await manager.do_action(
        session.session_id,
        ActionSpec(action="ado_capture", params={}),
    )

    page.wait_for_selector.assert_called()
    page.wait_for_timeout.assert_not_called()


async def test_jira_capture_scoped_text(manager, mock_playwright):
    """jira_capture calls inner_text with a scoped selector, not 'body'."""
    session = await manager.start_session("clientA", "jira")
    page = mock_playwright["page"]
    page.wait_for_selector = AsyncMock()
    page.inner_text = AsyncMock(return_value="scoped content")

    await manager.do_action(
        session.session_id,
        ActionSpec(action="jira_capture", params={}),
    )

    calls = [str(c) for c in page.inner_text.call_args_list]
    assert any("body" not in c for c in calls), "inner_text should not use 'body'"
    # Should use role=main or similar scoped selector
    assert page.inner_text.call_args[0][0] != "body"


async def test_ado_capture_scoped_text(manager, mock_playwright):
    """ado_capture calls inner_text with a scoped selector, not 'body'."""
    session = await manager.start_session("clientA", "ado")
    page = mock_playwright["page"]
    page.wait_for_selector = AsyncMock()
    page.inner_text = AsyncMock(return_value="scoped content")

    await manager.do_action(
        session.session_id,
        ActionSpec(action="ado_capture", params={}),
    )

    assert page.inner_text.call_args[0][0] != "body"


# ---------------------------------------------------------------------------
# P5 — Retry logic
# ---------------------------------------------------------------------------


async def test_retry_succeeds_on_second_attempt():
    """with_retry retries on PlaywrightTimeoutError and succeeds on retry."""
    call_count = 0

    async def flaky():
        nonlocal call_count
        call_count += 1
        if call_count == 1:
            raise PlaywrightTimeoutError("timeout")
        return "ok"

    result = await with_retry(flaky, retries=2, backoff=0.0)
    assert result == "ok"
    assert call_count == 2


async def test_retry_exhausted_raises():
    """with_retry re-raises original error after all retries are exhausted."""
    call_count = 0

    async def always_fails():
        nonlocal call_count
        call_count += 1
        raise PlaywrightTimeoutError("always fails")

    with pytest.raises(PlaywrightTimeoutError):
        await with_retry(always_fails, retries=2, backoff=0.0)
    assert call_count == 3  # 1 initial + 2 retries


# ---------------------------------------------------------------------------
# P5 — Safe profile lock handling
# ---------------------------------------------------------------------------


async def test_lock_check_dead_process_cleans_up(tmp_profiles, tmp_traces, mock_playwright):
    """If SingletonLock contains a PID of a dead process, lock is removed and session starts."""
    mgr = BrowserManager(
        profiles_root=tmp_profiles, slow_mo=0, nav_timeout=5000, traces_root=tmp_traces
    )
    # Pre-create profile dir and write a lock file with a non-existent PID
    profile_dir = Path(tmp_profiles) / "clientA" / "jira"
    profile_dir.mkdir(parents=True, exist_ok=True)
    lock_file = profile_dir / "SingletonLock"
    lock_file.write_text("99999999")  # unlikely-to-exist PID

    result = await mgr.start_session("clientA", "jira")
    assert result.session_id
    assert not lock_file.exists(), "Stale lock file should have been removed"
    await mgr.close_all()


async def test_lock_check_live_process_raises(tmp_profiles, tmp_traces, mock_playwright):
    """If SingletonLock contains a PID of a running process, a clear error is raised."""
    import os

    mgr = BrowserManager(
        profiles_root=tmp_profiles, slow_mo=0, nav_timeout=5000, traces_root=tmp_traces
    )
    profile_dir = Path(tmp_profiles) / "clientB" / "jira"
    profile_dir.mkdir(parents=True, exist_ok=True)
    lock_file = profile_dir / "SingletonLock"
    # Use the current process PID — guaranteed to be alive
    lock_file.write_text(str(os.getpid()))

    with pytest.raises(ValueError, match="in use by PID"):
        await mgr.start_session("clientB", "jira")
