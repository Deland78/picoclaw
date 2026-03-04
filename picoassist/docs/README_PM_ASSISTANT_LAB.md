# PM Assistant Lab (Windows) — Microsoft Email + Watched Web Actions (Jira Cloud + Azure DevOps Services)

This package is a **complete build plan** you can hand to a coding agent (Claude Code, OpenAI Codex, Gemini CLI) to implement a solo-consultant personal assistant focused on:
- **Microsoft 365 email** (Graph API)
- **Watched web actions** (Playwright headed browser you can see)
- **Client-owned Jira Cloud + Azure DevOps Services (SaaS)** with persistent browser profiles

> Design intent: **web-first** for client-owned systems (lowest friction), **API-first** for
> your own M365 email, and strict safety gates. **v1: Jira/ADO read-only; email allows
> non-destructive triage (move, draft — no send, no delete).**

---

## Quick links your coding agent should read first (context)

### Microsoft Email (Graph)
- Python email tutorial (Graph): https://learn.microsoft.com/en-us/graph/tutorials/python-email
- MSAL Python token acquisition: https://learn.microsoft.com/en-us/entra/msal/python/getting-started/acquiring-tokens

### Jira Cloud + ADO (auth realities & APIs for later)
- Jira OAuth 2.0 (3LO) apps (for the "API upgrade path"): https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/
- Jira REST API v3 reference: https://developer.atlassian.com/cloud/jira/platform/rest/v3/
- Azure DevOps "no new OAuth apps" note (use Entra ID OAuth): https://devblogs.microsoft.com/devops/no-new-azure-devops-oauth-apps/

### Playwright (watched automation + persistent profiles)
- Persistent context overview: https://www.browserstack.com/guide/playwright-persistent-context
- Playwright issue about relative `userDataDir` resolution (#34700): https://github.com/microsoft/playwright/issues/34700

---

## What you are building (high-level architecture)

### v1 components
1. **mail_worker** — Python module + optional FastAPI wrapper. Microsoft Graph read/triage (list unread, thread summaries, move to folder, draft replies).
2. **browser_worker** — Python module + FastAPI wrapper. Playwright **headed** browser automation with per-client persistent profiles. Read-only in v1 (navigate, screenshot, extract text).
3. **digest_runner.py** — Python script that imports both workers, reads `client_config.yaml`, and produces daily markdown digests.
4. **Local storage** — Run artifacts (markdown files in `data/runs/`) + per-client browser profiles.

### Why this architecture
- **No Docker for v1.** Everything runs as native Python on the Windows host. Simpler to set up, debug, and operate for a solo user. Docker can be added in v2.
- **No orchestrator for v1.** `digest_runner.py` replaces what an orchestrator (PicoClaw, MCP) would do. Orchestrator integration is deferred to v2 when tool registration requirements are clearer.
- **Workers are modules first, APIs second.** `digest_runner.py` imports workers directly. The FastAPI HTTP wrappers exist for interactive use (profile onboarding) and future orchestrator integration.
- **Headed browser is mandatory.** You must be able to see what the browser is doing. Mixed auth + MFA/SSO are much easier with a visible browser window.

---

## v1 scope — digest generation + email triage

### What v1 does
- **Email**: list unread, summarize threads, move to folder, draft replies (**no send, no delete**)
- **Jira**: open configured URLs, screenshot pages, extract visible text (**read-only**)
- **ADO**: open configured URLs, screenshot pages, extract visible text (**read-only**)
- **Digest**: combined markdown report per client in `data/runs/YYYY-MM-DD/<client_id>/digest.md`
- **Scheduled**: Windows Task Scheduler runs digest overnight

### What v1 does NOT do (deferred to v2)
- Jira write actions (comments, transitions)
- ADO write actions (field updates)
- Email send or delete
- Approval workflows (typed tokens)
- Interactive triage mode
- Orchestrator integration (PicoClaw, MCP)
- Docker containerization
- Separate `policy.yaml` (v1 uses `client_config.yaml` for safety settings)

---

## Safety model (tight now; loosen later)
- Default to **read-only** / **non-destructive** actions.
- Email: allow *move to folder*, allow *draft replies*, **no delete**, **no send**.
- Web actions (Jira/ADO): v1 allows navigation, screenshots, and text extraction only. No writes.
- All actions produce logs + a deterministic `action_id`.
- Overnight: Jira/ADO read-only (navigate, screenshot, extract text). Email triage (move, draft) allowed. **Send is never allowed.**

---

## Repository layout

> **Authoritative structure is in `AGENTS.md`** at the repo root. The listing below
> is a summary — if they conflict, `AGENTS.md` wins.

```
PicoAssist/
├── AGENTS.md               # shared agent context
├── CLAUDE.md               # Claude Code-specific instructions
├── .env.example
├── .gitignore
├── client_config.yaml      # per-client settings + safety (see CLIENT_CONFIG_TEMPLATE.md)
├── digest_runner.py        # v1 orchestrator
│
├── data/
│   ├── runs/               # ✅ commit for audit
│   ├── tokens/             # 🚫 never commit
│   └── traces/             # 🚫 never commit
│
├── profiles/               # 🚫 never commit — per-client browser state
│
├── services/
│   ├── mail_worker/        # importable module + optional HTTP API
│   └── browser_worker/     # importable module + HTTP API
│
└── scripts/                # PowerShell automation
```

---

## Authentication flows (launch recommendation)
- **Microsoft Graph (mail_worker):** use MSAL device code flow (single-user local tool, low setup friction). Cache tokens locally. Structure auth module to support client_secret flow later.
- **Jira/ADO (browser_worker):** manual login in headed browser with persistent Playwright profiles. Sessions survive browser restart via `user_data_dir`.
- **Write/API upgrades later (v2):** add OAuth app-based API auth only when web-first flows are stable and audited.

## Tool APIs (stable contracts)

### mail_worker

Importable as:
```python
from services.mail_worker import GraphMailClient, MSALAuth
```

Methods:
- `list_unread(folder, max_results)` -> `ListUnreadResponse`
- `get_thread_summary(message_id)` -> `ThreadSummaryResponse`
- `move(message_id, folder_name)` -> `MoveResponse`
- `draft_reply(message_id, tone, bullets)` -> `DraftReplyResponse`

Optional HTTP API (same operations):
```
GET  /health
POST /mail/list_unread
POST /mail/get_thread_summary
POST /mail/move
POST /mail/draft_reply
```

**Notes**
- Use MSAL; cache token in `./data/tokens/` (never commit).
- Recommend creating folders in Outlook: `Quarantine`, `ActionRequired`, `Archive`.

### browser_worker

Importable as:
```python
from services.browser_worker import BrowserManager
```

Methods:
- `start_session(client_id, app)` -> `StartSessionResponse`
- `do_action(session_id, action_spec)` -> `DoActionResponse`
- `screenshot(session_id)` -> `ScreenshotResponse`
- `stop_session(session_id)` -> `StopSessionResponse`
- `close_all()` -> None

HTTP API (same operations — needed for interactive profile onboarding):
```
GET  /health
POST /browser/start_session
POST /browser/do
POST /browser/screenshot
POST /browser/stop_session
```

**Important**
- Always launch Playwright **headed** (headless=false).
- Always use **absolute paths** for `userDataDir` to avoid Playwright path resolution issues on Windows.
- v1 enforces a read-only allowlist: `jira_open`, `jira_search`, `jira_capture`, `ado_open`, `ado_search`, `ado_capture`.

---

## Windows setup checklist (human steps)
1) Install **Python 3.11+** on Windows.
2) Install **PowerShell 7+** (`pwsh`).
3) Create an **Entra app registration** with `Mail.Read` + `Mail.ReadWrite` delegated permissions.
4) `cp .env.example .env` and fill in credentials + `PROFILES_ROOT` (absolute path).
5) Install dependencies:
   ```
   pip install -e ".[dev]"
   playwright install chromium
   ```
6) Edit `client_config.yaml` — add your clients.
7) Onboard browser profiles per client (see below).
8) Authenticate Graph API via device code flow.

---

## First-time onboarding per client (web profiles)
For each client and each app (Jira, ADO):
1) Run: `pwsh scripts/run_interactive.ps1 -ClientId clientA -App jira`
2) Browser opens; **you log in manually** (SSO/MFA).
3) Press Enter when logged in. Type `quit` to stop.
4) Profile is now saved in `./profiles/clientA/jira/`.
5) Repeat for ADO: `pwsh scripts/run_interactive.ps1 -ClientId clientA -App ado`

---

## Overnight runs (local artifacts only)
Use Windows Task Scheduler to run:
- `scripts/run_daily_digest.ps1` at e.g. 6:00 AM.

Overnight job:
- Opens Jira/ADO pages (read-only)
- Collects info (screenshots, extracted text)
- Produces markdown outputs under `./data/runs/YYYY-MM-DD/<client>/`
- **Never** posts comments, transitions, or field updates

`data/runs/` is intended to be committed for change history and auditability. Do not commit `profiles/`, `data/tokens/`, or `.env`.

---

## Deliverables the coding agent must produce
- Working mail_worker (importable module + optional HTTP API)
- Working browser_worker (importable module + HTTP API)
- Working `digest_runner.py` that produces daily digests
- PowerShell scripts for interactive and scheduled runs
- Unit tests for both workers (mocked external services)
- Documentation: README.md with first-run walkthrough

---

## v2 roadmap (designed but not built)

These features are specified in the docs for future implementation:
- **Write actions**: Jira comments/transitions, ADO field updates, email send
- **Approval workflow**: typed tokens (`APPROVE <action_id>`), TTL, pending queue
- **policy.yaml**: Separate global safety policy file with per-action permissions
- **Orchestrator**: PicoClaw (if tool registration matures), MCP server, or similar
- **Docker**: Containerize mail_worker; browser_worker stays on host for headed browser
- **Interactive triage**: Real-time issue review with proposed actions and approval gates
- **SQLite**: Local database for action log, email cache, digest metadata (v1 uses flat markdown files)
- **Jira/ADO API auth**: OAuth 2.0 (3LO) for Jira, Entra ID OAuth for ADO — replacing browser-based access

---

## Next: client_config.yaml
See `CLIENT_CONFIG_TEMPLATE.md` in this package for the required schema and examples.
