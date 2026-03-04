# Project: PicoAssist — PM Assistant for Solo Consultants

> **Multi-agent note:** This file is the shared source of truth for all coding agents
> (Claude Code, OpenAI Codex, Gemini CLI). Agent-specific files (`CLAUDE.md`, etc.)
> reference this file and add tool-specific instructions. **Do not duplicate content.**

## What this project is

A personal assistant for a solo PM consultant that:
- Triages **Microsoft 365 email** via Graph API (or Gmail)
- Monitors **Jira Cloud** and **Azure DevOps Services** via watched browser automation (Playwright)
- Produces **daily digest reports** (local markdown artifacts)
- Runs on **Windows 11** as native Python processes (no Docker required)
- Receives natural-language commands via **PicoClaw** (v2+)

Design philosophy: **web-first** for client-owned systems (Jira/ADO — lowest friction, handles SSO/MFA),
**API-first** for your own M365 email, strict safety gates for any write/destructive action.

## Current scope (v2)

v2 extends v1 with PicoClaw as the primary user interface, a policy engine, an approval
workflow, and an action log database:

- Email: list unread, summarize threads, move to folder, draft replies (**no send, no delete**)
- Jira: open configured URLs, screenshot, extract text (**read-only — no comments, no transitions**)
- ADO: open configured URLs, screenshot, extract text (**read-only — no field updates**)
- Digest: combined markdown report per client
- **Policy engine** (`policy.yaml`): global + per-client action rules, overnight restrictions
- **Approval workflow**: write actions queue for user confirmation before execution
- **Action log** (`data/picoassist.db`): SQLite audit trail for all actions
- **PicoClaw integration**: skill installed to `~/.picoclaw/workspace/skills/picoassist/`
- Scheduled: Windows Task Scheduler runs digest overnight

v2 explicitly **defers** to v3:
- MCP server (port 8003)
- Jira/ADO write actions (comment, transition, field update)
- Email send

## Key docs (read in order)

1. `AGENTS.md` (this file) — project context, structure, conventions, commands
2. `docs/README_PM_ASSISTANT_LAB.md` — full architecture, auth flows, component design
3. `docs/V2-Implementation-Plan.md` — phased build plan with verification steps
4. `docs/CLIENT_CONFIG_TEMPLATE.md` — per-client YAML schema and action names
5. `docs/picoclaw-setup.md` — PicoClaw installation and configuration guide

## Tech stack

| Component | Technology | Version | Runs on |
|-----------|-----------|---------|---------|
| mail_worker | Python + FastAPI + MSAL + httpx | Python 3.11, FastAPI 0.115.x, MSAL 1.31.x, httpx 0.28.x | Windows host |
| browser_worker | Python + FastAPI + Playwright | Python 3.11, FastAPI 0.115.x, Playwright 1.49.x | Windows host (headed) |
| digest_runner | Python script | Python 3.11 | Windows host |
| config / policy | Python + PyYAML | Python 3.11 | Windows host |
| action_log | Python + aiosqlite | Python 3.11 | Windows host |
| PicoClaw | Go binary (v0.1.x) | — | Windows host |
| Scripting | PowerShell 7+ | — | Windows host |

## Repository structure (authoritative)

PicoAssist lives inside the picoclaw Go repo as `picoassist/`.

```
picoclaw/                        # Go repo root (upstream: sipeed/picoclaw)
├── cmd/, pkg/, go.mod           # Go source code (managed separately)
├── workspace/skills/            # Default skills embedded in Go binary
│
└── picoassist/                  # ← Python project lives here
    ├── AGENTS.md                # ← you are here (shared agent context)
    ├── CLAUDE.md                # Claude Code-specific instructions
    ├── pyproject.toml           # project metadata, dependencies, ruff + pytest config
    ├── policy.yaml              # global + per-client safety policy (see docs/CLIENT_CONFIG_TEMPLATE.md)
    ├── .env.example             # template — copy to .env and fill in values
    ├── .gitignore               # picoassist-specific ignores (secrets, runtime data)
    ├── client_config.yaml       # per-client settings (Jira/ADO URLs, reporting paths)
    ├── conftest.py              # pytest root conftest
    ├── digest_runner.py         # v1/v2 orchestrator — reads config, calls workers, writes markdown
    │
    ├── config/                  # V2: config + policy loading
    │   ├── __init__.py          # Exports load_config, load_policy
    │   ├── client_config.py     # Client config loader (Pydantic)
    │   ├── policy.py            # PolicyEngine — evaluates action rules + overnight mode
    │   └── tests/
    │
    ├── docs/
    │   ├── README_PM_ASSISTANT_LAB.md
    │   ├── V2-Implementation-Plan.md
    │   ├── CLIENT_CONFIG_TEMPLATE.md
    │   ├── picoclaw-setup.md    # PicoClaw installation + config guide
    │   ├── HEARTBEAT.md         # Heartbeat configuration
    │   └── USER-template.md     # Template for runtime USER.md
    │
    ├── skill/                   # PicoClaw skill source
    │   ├── SKILL.md             # Skill prompt (install to ~/.picoclaw/workspace/skills/picoassist/)
    │   └── references/
    │       └── api-reference.md
    │
    ├── data/                    # runtime data — NOT committed
    │   ├── picoassist.db        # 🚫 NEVER COMMIT — SQLite action log (V2)
    │   ├── tokens/              # 🚫 NEVER COMMIT — MSAL token cache
    │   └── traces/              # 🚫 NEVER COMMIT — Playwright traces/screenshots
    │
    ├── profiles/                # 🚫 NEVER COMMIT — per-client browser state
    │   └── <client_id>/
    │       ├── jira/
    │       └── ado/
    │
    ├── services/
    │   ├── action_log/          # V2: SQLite action log
    │   │   ├── __init__.py      # Exports ActionLogDB
    │   │   ├── db.py            # Async SQLite wrapper
    │   │   ├── models.py        # ActionRecord Pydantic model
    │   │   └── tests/
    │   │
    │   ├── approval_engine/     # V2: approval workflow
    │   │   ├── __init__.py
    │   │   ├── engine.py        # ApprovalEngine — pending/approve/reject/expire
    │   │   ├── models.py
    │   │   └── tests/
    │   │
    │   ├── mail_worker/
    │   │   ├── __init__.py      # Exports GraphMailClient / GmailClient for direct import
    │   │   ├── app.py           # FastAPI wrapper (HTTP API)
    │   │   ├── auth_msal.py     # MSAL device-code + token cache
    │   │   ├── graph_client.py  # Graph API wrapper — core logic
    │   │   ├── gmail_client.py  # Gmail API wrapper (alternative provider)
    │   │   ├── models.py        # Pydantic request/response schemas
    │   │   └── tests/
    │   │       └── test_mail.py
    │   │
    │   └── browser_worker/
    │       ├── __init__.py      # Exports BrowserManager for direct import
    │       ├── app.py           # FastAPI wrapper (HTTP API for interactive use)
    │       ├── playwright_runner.py  # Browser lifecycle + persistent profiles — core logic
    │       ├── models.py        # Pydantic request/response schemas
    │       ├── actions/
    │       │   ├── __init__.py  # Action registry + policy enforcement
    │       │   ├── jira.py      # Jira read actions (open, capture, search)
    │       │   └── ado.py       # ADO read actions (open, capture, search)
    │       └── tests/
    │           └── test_browser.py
    │
    ├── scripts/
    │   ├── start_services.ps1   # Start mail_worker + browser_worker + picoclaw gateway
    │   ├── build_picoclaw.ps1   # Build Go binary from source
    │   ├── install_skill.ps1    # Copy skill/ to ~/.picoclaw runtime location
    │   ├── run_daily_digest.ps1
    │   ├── run_interactive.ps1
    │   └── schedule_task.ps1
    │
    └── tests/                   # Integration tests spanning services
        └── test_integration.py
```

### What NOT to commit

These patterns must be in `.gitignore`:
```
.env
.env.*
profiles/
data/tokens/
data/traces/
data/picoassist.db
__pycache__/
*.pyc
.pytest_cache/
```

`data/runs/` is intentionally committed for audit history.

## Build and run commands

```bash
# Install dependencies (one-time)
pip install -e ".[dev]"
playwright install chromium

# Start both FastAPI services (recommended — runs mail_worker + browser_worker)
pwsh scripts/start_services.ps1

# Or start services individually
python -m services.mail_worker.app
python -m services.browser_worker.app

# Health checks (when HTTP APIs are running)
curl http://localhost:8001/health    # mail_worker
curl http://localhost:8002/health    # browser_worker

# Run daily digest (standalone, no PicoClaw required)
python digest_runner.py

# Run tests
pytest -v

# Lint
ruff check .
ruff format --check .
```

### PicoClaw setup (V2 primary interface)

See `docs/picoclaw-setup.md` for full installation instructions. Quick reference:

```bash
# 1. Install PicoClaw binary to PATH (e.g. C:\Users\<you>\bin\)
#    Or build from source: pwsh scripts/build_picoclaw.ps1

# 2. Install the PicoAssist skill to the runtime location
pwsh scripts/install_skill.ps1

# 3. Start PicoAssist services
pwsh scripts/start_services.ps1

# 4. Test via PicoClaw
picoclaw agent -m "Check PicoAssist health"
picoclaw agent -m "List my unread email"
```

## Service ports

| Service | Port | Notes |
|---------|------|-------|
| mail_worker | 8001 | Primary mail triage API |
| browser_worker | 8002 | Browser automation + approval API |
| MCP server | 8003 | V3 — not yet implemented |

## Coding conventions

- **Python**: 4-space indent, `snake_case` for files/functions/variables, `PascalCase` for classes
- **Markdown/docs**: `kebab-case` or `UPPER_SNAKE` filenames
- **Formatting**: Use `ruff` for Python linting and formatting
- **Type hints**: Required on all public function signatures
- **Docstrings**: Only where logic is non-obvious; prefer self-documenting names
- **Error handling**: Return structured error responses from APIs; don't swallow exceptions silently
- **Module design**: Every service must be importable (`from services.mail_worker import GraphMailClient`)
  AND optionally runnable as an HTTP API (`python -m services.mail_worker.app`)

## Testing conventions

- Tests live next to the code they test: `services/mail_worker/tests/`
- Integration tests spanning services go in `tests/`
- Naming: `test_<module>.py` with functions `test_<behavior>()`
- At minimum: one happy-path and one error-path test per endpoint
- Keep tests deterministic — mock external services (Graph API, Playwright)
- Use `pytest` with `httpx.AsyncClient` for FastAPI endpoint testing

## Safety rules (non-negotiable)

Safety is enforced at two layers:

1. **Hardcoded in code**: browser_worker action registry always blocks unknown actions.
2. **`policy.yaml`**: configures per-action rules, overnight restrictions, and approval requirements.
   `policy.yaml` is the source of truth for what is allowed at runtime.

Specific rules:
1. **Jira/ADO are read-only in v1/v2.** No comments, transitions, or field updates.
   Email allows non-destructive triage: list, summarize, move to folder, draft replies.
   **No email send. No email delete.**
2. **All actions produce logs** with deterministic `action_id` for audit (SQLite in v2).
3. **Never commit secrets.** `.env`, `data/tokens/`, `profiles/`, `data/picoassist.db` are always gitignored.
4. **Browser is always headed.** `headless=False` — the user must be able to see what's happening.
5. **Playwright paths are always absolute.** Avoids Windows path resolution bugs.
6. **Overnight runs restrict write actions.** The policy engine blocks write/triage actions between
   00:00–06:00 by default; reads are always allowed overnight.

## Commit and PR guidelines

- Imperative commit messages: `Add auth token validator`, `Fix Playwright profile path`
- One logical change per commit
- PRs include: what changed, why, how to validate, linked issue if any

## V3 roadmap (deferred from v2)

- **MCP server** (port 8003): expose PicoAssist tools as MCP endpoints
- **Jira/ADO write actions**: comments, transitions, field updates (requires approval workflow already in v2)
- **Email send**: with strict approval gate
