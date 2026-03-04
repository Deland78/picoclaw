# Implementation tasks for coding agents (Codex / Claude Code / Gemini CLI)

This file is a "do this in order" build spec. Complete each phase and run its
**Verify** block before moving to the next phase.

**v1 scope: digest generation + email triage.** Jira/ADO read-only. Email allows move/draft (no send/delete). No orchestrator, no Docker.

---

## Phase 0 — Decisions (locked — do not revisit)

- v1: email triage (list, summarize, move, draft — **no send, no delete**), Jira/ADO **read-only** (screenshot+text extraction), markdown digests.
- No Docker. All services run as native Python on the Windows host.
- No PicoClaw. A simple `digest_runner.py` script orchestrates v1.
- No Jira/ADO write actions, no email send, no approval workflows — deferred to v2.
- Workers are **importable Python modules first**, with optional FastAPI HTTP wrappers.
- `action_id` format: `str(uuid.uuid4())` — generated per action for audit logging. No structured format in v1.
- Build directly in this repository (no second repo).
- Commit `data/runs/` for audit history.

---

## Test infrastructure (applies to all phases)

### Mock strategy
- **httpx calls** (mail_worker → Graph API): use `respx` to intercept and mock HTTP requests
- **Playwright** (browser_worker): use `unittest.mock.AsyncMock` to mock `async_playwright()`, `BrowserContext`, and `Page` objects. Never launch a real browser in unit tests.
- **MSALAuth**: mock with `AsyncMock(spec=MSALAuth)`, stub `get_token()` to return a fake token string

### Shared conftest (`conftest.py` at repo root)
Create during Phase 2 (first phase that needs tests):

```python
"""Shared pytest fixtures for PicoAssist."""

import os
import pytest

# Ensure tests never use real credentials
os.environ.setdefault("GRAPH_CLIENT_ID", "test-client-id")
os.environ.setdefault("GRAPH_TENANT_ID", "test-tenant-id")
os.environ.setdefault("PROFILES_ROOT", "C:/tmp/picoassist-test-profiles")


@pytest.fixture
def runs_dir(tmp_path):
    """Temporary data/runs directory for digest output."""
    d = tmp_path / "runs"
    d.mkdir()
    return d


@pytest.fixture
def traces_dir(tmp_path):
    """Temporary data/traces directory for screenshots."""
    d = tmp_path / "traces"
    d.mkdir()
    return d
```

### Test environment
- Tests must run without `.env`, live Graph API tokens, or browser profiles
- Use `os.environ.setdefault()` in conftest.py to provide safe defaults
- Tests requiring live services (manual only) should be marked with `@pytest.mark.live` and skipped by default:
  ```python
  live = pytest.mark.skipunless(os.environ.get("PICOASSIST_LIVE_TESTS"), reason="Set PICOASSIST_LIVE_TESTS=1")
  ```

---

## Phase 1 — Scaffold the repo

1) Create the folder structure defined in `AGENTS.md` § "Repository structure".

2) Create `.gitignore`:

```gitignore
# Secrets and auth state
.env
.env.*
data/tokens/
profiles/

# Playwright traces
data/traces/

# Python
__pycache__/
*.pyc
.pytest_cache/
*.egg-info/

# IDE
.vscode/
.idea/

# Do NOT ignore data/runs/ — committed for audit
```

3) Create `.env.example`:

```env
# === Microsoft Graph (mail_worker) ===
GRAPH_CLIENT_ID=<your-entra-app-client-id>
GRAPH_TENANT_ID=<your-entra-tenant-id>
# Space-separated scopes. Mail.ReadWrite needed for move/draft (allowed in v1).
GRAPH_SCOPES=Mail.Read Mail.ReadWrite

# === Service ports (for optional HTTP APIs) ===
MAIL_WORKER_PORT=8001
BROWSER_WORKER_PORT=8002

# === Playwright (browser_worker) ===
# MUST be absolute path — avoids Windows path resolution bug in Playwright
PROFILES_ROOT=C:/Users/david/PicoAssist/profiles
BROWSER_SLOW_MO_MS=50
BROWSER_NAV_TIMEOUT_MS=45000
BROWSER_ACTION_TIMEOUT_MS=30000

# === Paths ===
DATA_DIR=./data
RUNS_DIR=./data/runs
TOKENS_DIR=./data/tokens
```

4) Create placeholder `client_config.yaml` with the example from `docs/CLIENT_CONFIG_TEMPLATE.md`.

5) Create empty `__init__.py` files:
   - `services/__init__.py`
   - `services/mail_worker/__init__.py`
   - `services/browser_worker/__init__.py`
   - `services/browser_worker/actions/__init__.py`

6) Create empty placeholder directories with `.gitkeep`:
   - `data/runs/.gitkeep`
   - `data/tokens/.gitkeep`
   - `data/traces/.gitkeep`
   - `profiles/.gitkeep`

7) Create `pyproject.toml` (root packaging — replaces per-worker requirements.txt):

```toml
[project]
name = "picoassist"
version = "0.1.0"
requires-python = ">=3.11"
dependencies = [
    "fastapi==0.115.6",
    "uvicorn[standard]==0.34.0",
    "msal==1.31.1",
    "httpx==0.28.1",
    "playwright==1.49.1",
    "pydantic==2.10.4",
    "pyyaml==6.0.2",
    "python-dotenv==1.0.1",
]

[project.optional-dependencies]
dev = [
    "pytest==8.3.4",
    "pytest-asyncio==0.25.0",
    "respx==0.22.0",
    "ruff==0.8.6",
]

[tool.pytest.ini_options]
asyncio_mode = "auto"
testpaths = ["services", "tests"]

[tool.ruff]
line-length = 100
target-version = "py311"

[tool.ruff.lint]
select = ["E", "F", "I", "UP"]
```

### Verify — Phase 1
```powershell
# All expected directories exist
@("data/runs","data/tokens","data/traces","profiles","services/mail_worker","services/browser_worker","scripts","tests") |
  ForEach-Object { if (Test-Path $_) { Write-Host "PASS: $_" } else { Write-Host "FAIL: $_ missing" } }

# .gitignore works correctly
git init   # if not already a repo
git status  # profiles/, data/tokens/, data/traces/, .env should not appear

# .env.example exists and has content
if ((Test-Path .env.example) -and ((Get-Item .env.example).Length -gt 0)) { Write-Host "PASS: .env.example" } else { Write-Host "FAIL: .env.example" }

# pyproject.toml exists
if ((Test-Path pyproject.toml) -and (Select-String -Path pyproject.toml -Pattern "picoassist" -Quiet)) { Write-Host "PASS: pyproject.toml" } else { Write-Host "FAIL: pyproject.toml" }

# Install project in dev mode
pip install -e ".[dev]"
playwright install chromium

# Python packages are importable (empty but no errors)
python -c "import services.mail_worker; import services.browser_worker; print('PASS: imports')"
```

---

## Phase 2 — mail_worker module

> Dependencies are managed in `pyproject.toml` (installed in Phase 1). No per-worker `requirements.txt` needed.

### API schemas (implement in `services/mail_worker/models.py`)

```python
from pydantic import BaseModel, Field
from datetime import datetime

# --- list_unread ---
class ListUnreadRequest(BaseModel):
    folder: str = "Inbox"
    max_results: int = Field(default=25, le=100)

class EmailSummary(BaseModel):
    message_id: str
    subject: str
    sender: str
    received_at: datetime
    preview: str  # first ~200 chars of body

class ListUnreadResponse(BaseModel):
    emails: list[EmailSummary]
    count: int

# --- get_thread_summary ---
class ThreadSummaryRequest(BaseModel):
    message_id: str

class ThreadMessage(BaseModel):
    message_id: str
    sender: str
    sent_at: datetime
    body_preview: str

class ThreadSummaryResponse(BaseModel):
    subject: str
    messages: list[ThreadMessage]
    participant_count: int

# --- move ---
class MoveRequest(BaseModel):
    message_id: str
    folder_name: str  # e.g. "Quarantine", "ActionRequired", "Archive"

class MoveResponse(BaseModel):
    success: bool
    new_folder: str
    action_id: str  # deterministic audit ID

# --- draft_reply ---
class DraftReplyRequest(BaseModel):
    message_id: str
    tone: str = "professional"  # "professional" | "casual" | "brief"
    bullets: list[str]          # key points for the reply

class DraftReplyResponse(BaseModel):
    draft_id: str
    subject: str
    body_preview: str
    action_id: str
```

### Core module (`services/mail_worker/graph_client.py`)

Implement a `GraphMailClient` class:

```python
class GraphMailClient:
    """Async client for Microsoft Graph mail operations."""

    def __init__(self, auth: MSALAuth):
        self._auth = auth
        self._http = httpx.AsyncClient(base_url="https://graph.microsoft.com/v1.0")

    async def list_unread(self, folder="Inbox", max_results=25) -> ListUnreadResponse: ...
    async def get_thread_summary(self, message_id: str) -> ThreadSummaryResponse: ...
    async def move(self, message_id: str, folder_name: str) -> MoveResponse: ...
    async def draft_reply(self, message_id: str, tone: str, bullets: list[str]) -> DraftReplyResponse: ...
    async def close(self): ...
```

- All methods return Pydantic models.
- Always pass `Authorization: Bearer {token}` header (token from `self._auth`).
- Graph scopes needed: `Mail.Read` (list/get), `Mail.ReadWrite` (move/draft).

### Auth module (`services/mail_worker/auth_msal.py`)

Implement an `MSALAuth` class:

```python
class MSALAuth:
    """MSAL authentication with device-code flow and token caching."""

    def __init__(self, client_id: str, tenant_id: str, scopes: list[str], cache_dir: str):
        ...

    async def get_token(self) -> str:
        """Return valid access token. Uses cache first, falls back to device code."""
        ...
```

- Cache tokens to `{cache_dir}/msal_cache.json`.
- On startup, attempt silent token acquisition from cache.
- If cache miss, initiate device code flow and print the user code to stdout.
- Structure so `client_secret` flow can be added later (accept `auth_method` param:
  `"device_code"` (default) | `"client_secret"`).

### Package exports (`services/mail_worker/__init__.py`)
```python
from .graph_client import GraphMailClient
from .auth_msal import MSALAuth
```

### Optional HTTP API (`services/mail_worker/app.py`)

FastAPI wrapper around `GraphMailClient`:
```
GET  /health                  -> {"status": "ok", "auth": "ready"|"needs_device_code"}
POST /mail/list_unread        -> ListUnreadResponse
POST /mail/get_thread_summary -> ThreadSummaryResponse
POST /mail/move               -> MoveResponse
POST /mail/draft_reply        -> DraftReplyResponse
```

Include `if __name__ == "__main__"` block so it can be run standalone:
```python
if __name__ == "__main__":
    uvicorn.run("services.mail_worker.app:app", host="0.0.0.0", port=int(os.getenv("MAIL_WORKER_PORT", 8001)))
```

### Unit tests (`services/mail_worker/tests/test_mail.py`)

Use `respx` to mock httpx requests to Graph API. All tests are async (`asyncio_mode = "auto"` in pyproject.toml).

```python
import pytest
import respx
from httpx import Response

from services.mail_worker.auth_msal import MSALAuth
from services.mail_worker.graph_client import GraphMailClient


@pytest.fixture
def mock_auth(mocker):
    """MSALAuth that always returns a fake token."""
    auth = mocker.AsyncMock(spec=MSALAuth)
    auth.get_token.return_value = "fake-token-123"
    return auth


@pytest.fixture
async def mail_client(mock_auth):
    client = GraphMailClient(mock_auth)
    yield client
    await client.close()


# --- Happy-path tests ---

@respx.mock
async def test_list_unread_returns_emails(mail_client):
    """list_unread returns EmailSummary list from Graph /messages response."""
    ...

@respx.mock
async def test_get_thread_summary_returns_thread(mail_client):
    """get_thread_summary returns ThreadSummaryResponse for a conversation."""
    ...

@respx.mock
async def test_move_returns_success(mail_client):
    """move returns MoveResponse with new_folder and action_id."""
    ...

@respx.mock
async def test_draft_reply_creates_draft(mail_client):
    """draft_reply returns DraftReplyResponse with draft_id."""
    ...

# --- Error-path tests ---

@respx.mock
async def test_list_unread_handles_401(mail_client):
    """list_unread raises or returns error on expired token (401)."""
    ...

@respx.mock
async def test_move_handles_not_found(mail_client):
    """move returns error when message_id doesn't exist (404)."""
    ...
```

> These are **function signatures only.** The coding agent must implement the test bodies
> with appropriate `respx.route(...)` mocks matching the Graph API URLs in `graph_client.py`.

### Verify — Phase 2 (automated — agent must run)
```powershell
# Module is importable
python -c "from services.mail_worker import GraphMailClient, MSALAuth; print('PASS')"

# Unit tests pass (mocked Graph responses)
pytest services/mail_worker/tests/ -v

# Lint passes
ruff check services/mail_worker/
```

### Verify — Phase 2 (manual — requires live Graph API credentials)
```powershell
# Start HTTP API and check health
$proc = Start-Process -FilePath python -ArgumentList "-m services.mail_worker.app" -PassThru -NoNewWindow
Start-Sleep -Seconds 3
Invoke-RestMethod http://localhost:8001/health | ConvertTo-Json
Stop-Process -Id $proc.Id -Force
```

---

## Phase 3 — browser_worker module

> Dependencies are managed in `pyproject.toml` (installed in Phase 1). No per-worker `requirements.txt` needed.

### API schemas (implement in `services/browser_worker/models.py`)

```python
from pydantic import BaseModel
from datetime import datetime

# --- start_session ---
class StartSessionRequest(BaseModel):
    client_id: str
    app: str  # "jira" | "ado"

class StartSessionResponse(BaseModel):
    session_id: str
    client_id: str
    app: str
    profile_path: str  # absolute path used for userDataDir

# --- do action ---
class ActionSpec(BaseModel):
    action: str       # e.g. "jira_open", "ado_capture" — must be in allowlist
    params: dict = {} # action-specific parameters (url, issue_key, etc.)

class DoActionRequest(BaseModel):
    session_id: str
    action_spec: ActionSpec

class ActionArtifact(BaseModel):
    type: str   # "screenshot" | "text" | "html"
    path: str   # absolute file path to artifact

class DoActionResponse(BaseModel):
    success: bool
    action_id: str              # deterministic audit ID
    result: dict = {}           # action-specific return data
    artifacts: list[ActionArtifact] = []
    error: str | None = None

# --- screenshot ---
class ScreenshotRequest(BaseModel):
    session_id: str

class ScreenshotResponse(BaseModel):
    path: str
    timestamp: datetime

# --- stop_session ---
class StopSessionRequest(BaseModel):
    session_id: str

class StopSessionResponse(BaseModel):
    success: bool
    session_id: str
```

### Core module (`services/browser_worker/playwright_runner.py`)

Implement a `BrowserManager` class:

```python
class BrowserManager:
    """Manages Playwright browser contexts with persistent profiles."""

    def __init__(self, profiles_root: str, slow_mo: int = 50, nav_timeout: int = 45000):
        ...

    async def start_session(self, client_id: str, app: str) -> StartSessionResponse:
        """Launch headed browser with persistent profile for client+app."""
        ...

    async def do_action(self, session_id: str, action_spec: ActionSpec) -> DoActionResponse:
        """Execute an allowlisted read action. Refuses write actions in v1."""
        ...

    async def screenshot(self, session_id: str) -> ScreenshotResponse:
        """Capture current page screenshot."""
        ...

    async def stop_session(self, session_id: str) -> StopSessionResponse:
        """Close browser context and save profile state."""
        ...

    async def close_all(self):
        """Shut down all sessions and the Playwright instance."""
        ...
```

Key implementation rules:
- Always use `headless=False` (non-negotiable).
- Always use **absolute paths** for `user_data_dir` — build as `{profiles_root}/{client_id}/{app}`.
- Apply `slow_mo` from init parameter.
- Store sessions in a dict keyed by `session_id` (UUID generated on start).
- On action failure: return error in response, **keep browser open** for manual takeover.
- Record a screenshot after every action (save to `data/traces/{session_id}/`).

### Action registry (`services/browser_worker/actions/__init__.py`)

```python
# v1: read actions only
ACTION_REGISTRY: dict[str, Callable] = {}

def register(name: str):
    """Decorator to register an action handler."""
    def decorator(fn):
        ACTION_REGISTRY[name] = fn
        return fn
    return decorator

V1_ALLOWLIST = {
    "jira_open", "jira_search", "jira_capture",
    "ado_open", "ado_search", "ado_capture",
}

async def execute_action(page, action_spec: ActionSpec) -> DoActionResponse:
    if action_spec.action not in V1_ALLOWLIST:
        return DoActionResponse(success=False, action_id="", error=f"Action '{action_spec.action}' not allowed in v1")
    handler = ACTION_REGISTRY[action_spec.action]
    return await handler(page, action_spec.params)
```

### v1 action handlers

**`actions/jira.py`:**
- `jira_open(page, params)` — navigate to `params["url"]`, wait for network idle
- `jira_search(page, params)` — navigate to JQL search URL, extract results table text
- `jira_capture(page, params)` — screenshot + text extraction via `page.inner_text("body")`

**`actions/ado.py`:**
- `ado_open(page, params)` — navigate to `params["url"]`, wait for network idle
- `ado_search(page, params)` — navigate to queries page, extract results text
- `ado_capture(page, params)` — screenshot + text extraction

All handlers return `DoActionResponse` with artifacts list.

### Package exports (`services/browser_worker/__init__.py`)
```python
from .playwright_runner import BrowserManager
```

### HTTP API (`services/browser_worker/app.py`)

FastAPI wrapper around `BrowserManager`:
```
GET  /health                  -> {"status": "ok", "active_sessions": <int>}
POST /browser/start_session   -> StartSessionResponse
POST /browser/do              -> DoActionResponse
POST /browser/screenshot      -> ScreenshotResponse
POST /browser/stop_session    -> StopSessionResponse
```

This HTTP API is needed for:
- Interactive profile onboarding (start session, user logs in manually, stop session)
- Future orchestrator integration (v2)

Include `if __name__ == "__main__"` block.

### Unit tests (`services/browser_worker/tests/test_browser.py`)

Use `unittest.mock.AsyncMock` to mock Playwright objects. Never launch a real browser in unit tests.

```python
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from pathlib import Path

from services.browser_worker.playwright_runner import BrowserManager


@pytest.fixture
def tmp_profiles(tmp_path):
    """Temporary profiles directory."""
    return str(tmp_path / "profiles")


@pytest.fixture
def mock_playwright():
    """Mocked Playwright instance — no real browser launched."""
    with patch("services.browser_worker.playwright_runner.async_playwright") as mock_pw:
        mock_instance = AsyncMock()
        mock_pw.return_value.__aenter__.return_value = mock_instance

        # Mock persistent context
        mock_context = AsyncMock()
        mock_page = AsyncMock()
        mock_context.pages = [mock_page]
        mock_instance.chromium.launch_persistent_context.return_value = mock_context

        yield {"pw": mock_instance, "context": mock_context, "page": mock_page}


# --- Happy-path tests ---

async def test_start_session_creates_profile_dir(tmp_profiles, mock_playwright):
    """start_session creates the profile directory and returns session info."""
    ...

async def test_start_session_uses_absolute_paths(tmp_profiles, mock_playwright):
    """start_session always passes absolute path to launch_persistent_context."""
    ...

async def test_do_action_allowlisted_succeeds(tmp_profiles, mock_playwright):
    """do_action executes an allowlisted action (e.g. jira_open)."""
    ...

async def test_screenshot_saves_png(tmp_profiles, mock_playwright):
    """screenshot captures current page viewport."""
    ...

async def test_stop_session_closes_context(tmp_profiles, mock_playwright):
    """stop_session calls context.close() to persist profile state."""
    ...

# --- Error-path tests ---

async def test_do_action_rejects_non_allowlisted(tmp_profiles, mock_playwright):
    """do_action refuses actions not in V1_ALLOWLIST."""
    ...

async def test_start_session_rejects_duplicate_profile(tmp_profiles, mock_playwright):
    """start_session raises ValueError if profile is already in use."""
    ...

async def test_do_action_keeps_browser_open_on_failure(tmp_profiles, mock_playwright):
    """On action failure, browser stays open for manual inspection."""
    ...
```

> These are **function signatures only.** The coding agent must implement the test bodies
> using the mocked Playwright objects from the `mock_playwright` fixture.

### Verify — Phase 3 (automated — agent must run)
```powershell
# Module is importable
python -c "from services.browser_worker import BrowserManager; print('PASS')"

# Unit tests pass (mocked Playwright)
pytest services/browser_worker/tests/ -v

# Lint
ruff check services/browser_worker/
```

### Verify — Phase 3 (manual — requires browser + profile setup)
```powershell
# Start HTTP API and verify a session opens a visible browser
$proc = Start-Process -FilePath python -ArgumentList "-m services.browser_worker.app" -PassThru -NoNewWindow
Start-Sleep -Seconds 3
Invoke-RestMethod http://localhost:8002/health | ConvertTo-Json
# POST /browser/start_session with a test client_id — browser window should appear
Stop-Process -Id $proc.Id -Force
```

---

## Phase 4 — digest_runner.py

This is the v1 orchestrator. A single Python script that:
1. Loads `.env` and `client_config.yaml`
2. For each client: starts browser sessions, navigates to configured URLs, captures screenshots + text
3. Calls mail_worker to list unread email and summarize threads
4. Writes a combined markdown digest per client to `data/runs/YYYY-MM-DD/<client_id>/digest.md`
5. Cleans up all browser sessions

### Implementation sketch

```python
"""PicoAssist v1 digest runner — reads config, calls workers, writes markdown."""

import asyncio
import os
from datetime import date
from pathlib import Path

import yaml
from dotenv import load_dotenv

from services.mail_worker import GraphMailClient, MSALAuth
from services.browser_worker import BrowserManager
from services.browser_worker.models import ActionSpec


async def run_digest():
    load_dotenv()

    # Load config
    with open("client_config.yaml") as f:
        config = yaml.safe_load(f)

    # Init workers
    auth = MSALAuth(
        client_id=os.environ["GRAPH_CLIENT_ID"],
        tenant_id=os.environ["GRAPH_TENANT_ID"],
        scopes=os.environ.get("GRAPH_SCOPES", "Mail.Read Mail.ReadWrite").split(),
        cache_dir=os.environ.get("TOKENS_DIR", "./data/tokens"),
    )
    mail = GraphMailClient(auth)
    browser = BrowserManager(
        profiles_root=os.environ["PROFILES_ROOT"],
        slow_mo=int(os.environ.get("BROWSER_SLOW_MO_MS", "50")),
    )

    today = date.today().isoformat()

    try:
        # Email summary
        unread = await mail.list_unread()

        for client in config["clients"]:
            client_id = client["id"]
            output_dir = Path(os.environ.get("RUNS_DIR", "./data/runs")) / today / client_id
            output_dir.mkdir(parents=True, exist_ok=True)

            sections = []
            sections.append(f"# Daily Digest: {client['display_name']} — {today}\n")

            # Email section
            sections.append("## Email Summary\n")
            sections.append(f"Unread: {unread.count}\n")
            for email in unread.emails[:10]:
                sections.append(f"- **{email.subject}** from {email.sender} ({email.received_at})")
                sections.append(f"  {email.preview}\n")

            # Jira section
            if "jira" in client:
                sections.append("## Jira\n")
                session = await browser.start_session(client_id, "jira")
                for view in client["jira"].get("digest", {}).get("ui_urls", []):
                    result = await browser.do_action(session.session_id, ActionSpec(
                        action="jira_capture",
                        params={"url": view["url"]},
                    ))
                    sections.append(f"### {view['name']}\n")
                    if result.success:
                        for artifact in result.artifacts:
                            if artifact.type == "screenshot":
                                sections.append(f"![{view['name']}]({artifact.path})\n")
                            elif artifact.type == "text":
                                sections.append(f"```\n{Path(artifact.path).read_text()[:2000]}\n```\n")
                await browser.stop_session(session.session_id)

            # ADO section
            if "ado" in client:
                sections.append("## Azure DevOps\n")
                session = await browser.start_session(client_id, "ado")
                for view in client["ado"].get("digest", {}).get("ui_urls", []):
                    result = await browser.do_action(session.session_id, ActionSpec(
                        action="ado_capture",
                        params={"url": view["url"]},
                    ))
                    sections.append(f"### {view['name']}\n")
                    if result.success:
                        for artifact in result.artifacts:
                            if artifact.type == "screenshot":
                                sections.append(f"![{view['name']}]({artifact.path})\n")
                            elif artifact.type == "text":
                                sections.append(f"```\n{Path(artifact.path).read_text()[:2000]}\n```\n")
                await browser.stop_session(session.session_id)

            # Write digest
            digest_path = output_dir / "digest.md"
            digest_path.write_text("\n".join(sections), encoding="utf-8")
            print(f"Digest written: {digest_path}")

    finally:
        await browser.close_all()
        await mail.close()


if __name__ == "__main__":
    asyncio.run(run_digest())
```

> This is a **sketch, not final code.** The coding agent should implement this with
> proper error handling, logging, and action_id generation. The structure and import
> pattern are the contract.

### Verify — Phase 4 (automated — agent must run)
```powershell
# digest_runner.py exists and imports resolve
python -c "import ast; ast.parse(open('digest_runner.py').read()); print('PASS: syntax')"

# Lint
ruff check digest_runner.py
```

### Verify — Phase 4 (manual — requires .env, client_config.yaml, and browser profiles)
```powershell
# Dry-run with a test client
# The browser should open, navigate to configured URLs, take screenshots
python digest_runner.py

# Digest output exists
$today = Get-Date -Format "yyyy-MM-dd"
if (Get-ChildItem "data/runs/$today/*/digest.md" -ErrorAction SilentlyContinue) { Write-Host "PASS" } else { Write-Host "FAIL: no digest output" }
```

---

## Phase 5 — Scripts (PowerShell)

### `scripts/run_daily_digest.ps1`
```powershell
# Load environment from .env
$envFile = Join-Path $PSScriptRoot ".." ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
        }
    }
}

$date = Get-Date -Format "yyyy-MM-dd"
$runsDir = Join-Path $PSScriptRoot ".." "data" "runs" $date

# Ensure output directory exists
New-Item -ItemType Directory -Force -Path $runsDir | Out-Null

# Run digest
$repoRoot = Split-Path $PSScriptRoot -Parent
python (Join-Path $repoRoot "digest_runner.py")

Write-Host "Digest complete. Output: $runsDir"
```

### `scripts/run_interactive.ps1`
```powershell
param(
    [Parameter(Mandatory)][string]$ClientId,
    [Parameter(Mandatory)][ValidateSet("jira","ado")][string]$App
)

# Load environment
$envFile = Join-Path $PSScriptRoot ".." ".env"
if (Test-Path $envFile) {
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
        }
    }
}

$browserUrl = "http://localhost:$($env:BROWSER_WORKER_PORT ?? '8002')"

Write-Host "Starting browser_worker HTTP API..."
$worker = Start-Process -FilePath "python" -ArgumentList "-m services.browser_worker.app" `
    -WorkingDirectory (Split-Path $PSScriptRoot -Parent) -PassThru -NoNewWindow
Start-Sleep -Seconds 3

try {
    # Start session
    $session = Invoke-RestMethod -Uri "$browserUrl/browser/start_session" -Method POST `
        -ContentType "application/json" -Body (@{client_id=$ClientId; app=$App} | ConvertTo-Json)

    Write-Host "Session started: $($session.session_id)"
    Write-Host "Browser is open. Log in manually if needed, then press Enter..."
    Read-Host

    # Interactive loop
    while ($true) {
        $action = Read-Host "Action name (or 'quit')"
        if ($action -eq 'quit') { break }
        $url = Read-Host "URL to navigate"
        $body = @{
            session_id = $session.session_id
            action_spec = @{action = $action; params = @{url = $url}}
        } | ConvertTo-Json -Depth 5
        $result = Invoke-RestMethod -Uri "$browserUrl/browser/do" -Method POST `
            -ContentType "application/json" -Body $body
        $result | ConvertTo-Json -Depth 5
    }

    # Stop session
    Invoke-RestMethod -Uri "$browserUrl/browser/stop_session" -Method POST `
        -ContentType "application/json" -Body (@{session_id=$session.session_id} | ConvertTo-Json)
    Write-Host "Session stopped."
}
finally {
    Stop-Process -Id $worker.Id -Force -ErrorAction SilentlyContinue
}
```

### `scripts/schedule_task.ps1`
```powershell
param(
    [string]$Time = "06:00",
    [string]$TaskName = "PicoAssist-DailyDigest"
)

$scriptPath = Join-Path $PSScriptRoot "run_daily_digest.ps1"
$action = New-ScheduledTaskAction -Execute "pwsh.exe" `
    -Argument "-NonInteractive -File `"$scriptPath`""
$trigger = New-ScheduledTaskTrigger -Daily -At $Time
Register-ScheduledTask -TaskName $TaskName -Action $action -Trigger $trigger `
    -Description "PicoAssist daily digest" -Force
Write-Host "Scheduled task '$TaskName' registered to run daily at $Time"
```

### Verify — Phase 5
```powershell
# Scripts exist and have content
@("scripts/run_daily_digest.ps1","scripts/run_interactive.ps1","scripts/schedule_task.ps1") |
  ForEach-Object {
    if ((Test-Path $_) -and ((Get-Item $_).Length -gt 0)) { Write-Host "PASS: $_" } else { Write-Host "FAIL: $_" }
  }

# PowerShell syntax check (parse without executing)
@("scripts/run_daily_digest.ps1","scripts/run_interactive.ps1","scripts/schedule_task.ps1") |
  ForEach-Object {
    try { [System.Management.Automation.PSParser]::Tokenize((Get-Content $_ -Raw), [ref]$null) | Out-Null; Write-Host "PASS: $_ syntax OK" }
    catch { Write-Host "FAIL: $_ syntax error" }
  }
```

---

## Phase 6 — Documentation and first-run walkthrough

Add a "First Run" section to the project root `README.md` (create it):

### Prerequisites
- Python 3.11+ on Windows
- PowerShell 7+ (`pwsh`)
- A Microsoft Entra app registration with `Mail.Read` + `Mail.ReadWrite` delegated permissions

### First Run
1. `cp .env.example .env` — fill in `GRAPH_CLIENT_ID`, `GRAPH_TENANT_ID`, `PROFILES_ROOT`
2. `pip install -e ".[dev]"`
3. `playwright install chromium`
5. Edit `client_config.yaml` — add your first client with Jira/ADO URLs
6. Onboard client browser profiles:
   ```
   pwsh scripts/run_interactive.ps1 -ClientId clientA -App jira
   # Browser opens → log in manually → press Enter → type 'quit'
   pwsh scripts/run_interactive.ps1 -ClientId clientA -App ado
   # Same process
   ```
7. Authenticate Graph API (device code flow):
   ```
   python -c "
   import asyncio
   from services.mail_worker import MSALAuth
   auth = MSALAuth('YOUR_CLIENT_ID', 'YOUR_TENANT_ID', ['Mail.Read'], './data/tokens')
   asyncio.run(auth.get_token())
   "
   # Follow the device code prompt to sign in
   ```
8. Run first digest: `python digest_runner.py`
9. Check output: `Get-ChildItem data/runs/$(Get-Date -Format 'yyyy-MM-dd')/`
10. (Optional) Schedule: `pwsh scripts/schedule_task.ps1 -Time "06:00"`

### Verify — Phase 6
```powershell
# README.md exists with First Run section
if ((Test-Path README.md) -and (Select-String -Path README.md -Pattern "First Run" -Quiet)) {
    Write-Host "PASS: README.md has First Run section"
} else { Write-Host "FAIL: README.md missing or no First Run section" }

# Full smoke test (manual):
# 1. python digest_runner.py
# 2. Check data/runs/<today>/<client>/digest.md exists
# 3. Digest contains Email, Jira, and ADO sections
# 4. Screenshots exist in data/traces/
```

---

## Final acceptance tests

Run these after all phases are complete:

| Test | Command | Expected |
|------|---------|----------|
| mail_worker import | `python -c "from services.mail_worker import GraphMailClient"` | No error |
| browser_worker import | `python -c "from services.browser_worker import BrowserManager"` | No error |
| mail_worker unit tests | `pytest services/mail_worker/tests/ -v` | All pass |
| browser_worker unit tests | `pytest services/browser_worker/tests/ -v` | All pass |
| Digest run | `python digest_runner.py` | Markdown files in `data/runs/<date>/` |
| Screenshot artifacts | `ls data/traces/` | PNG files from browser captures |
| Lint | `ruff check .` | No errors |
| Optional: mail HTTP API | `python -m services.mail_worker.app` then `curl http://localhost:8001/health` | `{"status":"ok"}` |
| Optional: browser HTTP API | `python -m services.browser_worker.app` then `curl http://localhost:8002/health` | `{"status":"ok"}` |
