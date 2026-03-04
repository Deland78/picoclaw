"""Integration tests for digest_runner — end-to-end with mocked workers."""

import re
from pathlib import Path
from unittest.mock import AsyncMock, MagicMock, patch

import pytest
import yaml

from config import load_config
from services.browser_worker.models import (
    ActionArtifact,
    DoActionResponse,
    StartSessionResponse,
    StopSessionResponse,
)
from services.mail_worker.models import EmailSummary, ListUnreadResponse

_SKILL_PATH = Path("skill/SKILL.md")
_API_REF_PATH = Path("skill/references/api-reference.md")

# --- Fixtures ---


@pytest.fixture
def test_config(tmp_path):
    """Write a minimal client_config.yaml and return its path."""
    config = {
        "version": 1,
        "clients": [
            {
                "id": "test-client",
                "display_name": "Test Client",
                "jira": {
                    "base_url": "https://test.atlassian.net",
                    "digest": {
                        "ui_urls": [
                            {
                                "name": "My Issues",
                                "url": "https://test.atlassian.net/issues/?jql=assignee%3DcurrentUser()",
                            }
                        ]
                    },
                },
                "ado": {
                    "org_url": "https://dev.azure.com/testOrg",
                    "project": "TestProject",
                    "team": "TestTeam",
                    "digest": {
                        "ui_urls": [
                            {
                                "name": "Boards",
                                "url": "https://dev.azure.com/testOrg/TestProject/_boards",
                            }
                        ]
                    },
                },
            }
        ],
    }
    config_path = tmp_path / "client_config.yaml"
    config_path.write_text(yaml.dump(config), encoding="utf-8")
    return config_path


@pytest.fixture
def empty_config(tmp_path):
    """Config with no clients."""
    config = {"version": 1, "clients": []}
    config_path = tmp_path / "client_config.yaml"
    config_path.write_text(yaml.dump(config), encoding="utf-8")
    return config_path


@pytest.fixture
def mock_mail():
    """Mocked GraphMailClient returning canned data."""
    client = AsyncMock()
    client.list_unread.return_value = ListUnreadResponse(
        emails=[
            EmailSummary(
                message_id="msg-1",
                subject="Test Email",
                sender="sender@example.com",
                received_at="2026-02-15T10:00:00Z",
                preview="This is a test email preview.",
            )
        ],
        count=1,
    )
    client.close = AsyncMock()
    return client


@pytest.fixture
def mock_browser(tmp_path):
    """Mocked BrowserManager returning canned screenshot artifacts."""
    mgr = AsyncMock()

    # Create a fake screenshot file for artifacts
    traces_dir = tmp_path / "traces"
    traces_dir.mkdir(exist_ok=True)
    screenshot_path = traces_dir / "screenshot.png"
    screenshot_path.write_bytes(b"fake-png-data")

    # Create a fake text extraction file
    text_path = traces_dir / "page_text.txt"
    text_path.write_text("Extracted page text content", encoding="utf-8")

    mgr.start_session.return_value = StartSessionResponse(
        session_id="sess-1",
        client_id="test-client",
        app="jira",
        profile_path=str(tmp_path / "profiles" / "test-client" / "jira"),
    )
    mgr.do_action.return_value = DoActionResponse(
        success=True,
        action_id="action-1",
        artifacts=[
            ActionArtifact(type="screenshot", path=str(screenshot_path)),
            ActionArtifact(type="text", path=str(text_path)),
        ],
    )
    mgr.stop_session.return_value = StopSessionResponse(success=True, session_id="sess-1")
    mgr.close_all = AsyncMock()
    return mgr


# --- Helper to run digest with mocks ---


async def _run_digest(
    config_path,
    mock_mail_client,
    mock_browser_mgr,
    runs_dir,
    monkeypatch,
):
    """Import and invoke run_digest with all externals mocked."""
    monkeypatch.setenv("GRAPH_CLIENT_ID", "test-id")
    monkeypatch.setenv("GRAPH_TENANT_ID", "test-tenant")
    monkeypatch.setenv("PROFILES_ROOT", "C:/tmp/test-profiles")
    monkeypatch.setenv("RUNS_DIR", str(runs_dir))

    # Use load_config() to build an AppConfig from the test config file,
    # then patch digest_runner.load_config to return it without file I/O.
    app_config = load_config(str(config_path))

    # Mock ActionLogDB to avoid creating real SQLite files during tests.
    mock_action_log = AsyncMock()
    mock_action_log_class = MagicMock(return_value=mock_action_log)

    with (
        patch("digest_runner.load_config", return_value=app_config),
        patch("digest_runner.create_mail_provider", return_value=mock_mail_client),
        patch("digest_runner.BrowserManager", return_value=mock_browser_mgr),
        patch("digest_runner.ActionLogDB", mock_action_log_class),
        patch("digest_runner.load_dotenv"),
    ):
        from digest_runner import run_digest

        await run_digest()


# --- Tests ---


async def test_digest_produces_markdown(
    test_config,
    mock_mail,
    mock_browser,
    runs_dir,
    monkeypatch,
):
    """End-to-end: digest_runner produces a digest.md file."""
    await _run_digest(test_config, mock_mail, mock_browser, runs_dir, monkeypatch)

    # Find the digest output
    digests = list(runs_dir.rglob("digest.md"))
    assert len(digests) == 1, f"Expected 1 digest, found {len(digests)}: {digests}"

    content = digests[0].read_text(encoding="utf-8")
    assert "Daily Digest" in content
    assert "Test Client" in content


async def test_digest_contains_email_section(
    test_config,
    mock_mail,
    mock_browser,
    runs_dir,
    monkeypatch,
):
    """Digest contains an Email Summary section with unread count."""
    await _run_digest(test_config, mock_mail, mock_browser, runs_dir, monkeypatch)

    content = list(runs_dir.rglob("digest.md"))[0].read_text(encoding="utf-8")
    assert "## Email Summary" in content
    assert "Unread: 1" in content
    assert "Test Email" in content


async def test_digest_contains_jira_section(
    test_config,
    mock_mail,
    mock_browser,
    runs_dir,
    monkeypatch,
):
    """Digest contains a Jira section with captured views."""
    await _run_digest(test_config, mock_mail, mock_browser, runs_dir, monkeypatch)

    content = list(runs_dir.rglob("digest.md"))[0].read_text(encoding="utf-8")
    assert "## Jira" in content
    assert "### My Issues" in content


async def test_digest_contains_ado_section(
    test_config,
    mock_mail,
    mock_browser,
    runs_dir,
    monkeypatch,
):
    """Digest contains an Azure DevOps section."""
    await _run_digest(test_config, mock_mail, mock_browser, runs_dir, monkeypatch)

    content = list(runs_dir.rglob("digest.md"))[0].read_text(encoding="utf-8")
    assert "## Azure DevOps" in content
    assert "### Boards" in content


async def test_digest_handles_no_clients(
    empty_config,
    mock_mail,
    mock_browser,
    runs_dir,
    monkeypatch,
):
    """Empty client list produces no digest files without error."""
    await _run_digest(empty_config, mock_mail, mock_browser, runs_dir, monkeypatch)

    digests = list(runs_dir.rglob("digest.md"))
    assert len(digests) == 0


# ---------------------------------------------------------------------------
# P6 — PicoClaw skill validation
# ---------------------------------------------------------------------------


def test_skill_frontmatter_valid():
    """SKILL.md has valid YAML frontmatter with required name and description fields."""
    assert _SKILL_PATH.exists(), f"Skill file not found: {_SKILL_PATH}"
    content = _SKILL_PATH.read_text(encoding="utf-8")

    # Extract content between the first two '---' delimiters
    parts = content.split("---")
    assert len(parts) >= 3, "SKILL.md must have YAML frontmatter between --- delimiters"

    meta = yaml.safe_load(parts[1])
    assert meta.get("name") == "picoassist", "Skill name must be 'picoassist'"
    assert "description" in meta and meta["description"], "Skill must have a non-empty description"
    assert len(meta["description"]) <= 1024, "Description must be ≤ 1024 characters"


def test_api_reference_matches_endpoints():
    """Documented endpoints in api-reference.md match actual FastAPI routes."""
    assert _API_REF_PATH.exists(), f"API reference not found: {_API_REF_PATH}"
    content = _API_REF_PATH.read_text(encoding="utf-8")

    from services.browser_worker.app import app as browser_app
    from services.mail_worker.app import app as mail_app

    # Collect all HTTP routes from both apps (excluding docs/openapi)
    _docs = {"/openapi.json", "/docs", "/docs/oauth2-redirect", "/redoc"}

    def app_routes(app):
        return {
            route.path
            for route in app.routes
            if hasattr(route, "methods") and route.path not in _docs
        }

    mail_routes = app_routes(mail_app)
    browser_routes = app_routes(browser_app)

    # Every actual route should appear as a heading in the reference doc
    for path in mail_routes | browser_routes:
        # Match "### GET /health" or "### POST /mail/move" etc.
        assert re.search(
            rf"#+\s+(?:GET|POST|PUT|DELETE|PATCH)\s+{re.escape(path)}", content
        ), f"Route '{path}' not documented in api-reference.md"


@pytest.mark.live
async def test_services_health_check():
    """Both services respond to /health with status=ok (requires running services)."""
    import httpx

    async with httpx.AsyncClient() as client:
        r_mail = await client.get("http://localhost:8001/health", timeout=5.0)
        r_browser = await client.get("http://localhost:8002/health", timeout=5.0)

    assert r_mail.status_code == 200
    assert r_mail.json()["status"] == "ok"
    assert r_browser.status_code == 200
    assert r_browser.json()["status"] == "ok"
