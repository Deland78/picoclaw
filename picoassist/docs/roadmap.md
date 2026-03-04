# PicoAssist Roadmap

> Living document tracking completed, in-progress, and future versions.
> Updated: 2026-02-20

---

## Architecture

PicoAssist is a **tool backend** for a PicoClaw AI agent. Users interact with
PicoClaw (via CLI, Telegram, Discord, etc.) and PicoClaw calls PicoAssist's
FastAPI HTTP endpoints to fulfill requests.

```
User (Telegram / Discord / CLI)
    ↓
PicoClaw agent (LLM + built-in tools)
    ↓ exec curl http://localhost:800x/...
PicoAssist FastAPI endpoints
    ├── mail_worker   :8001  (Graph API / Gmail)
    ├── browser_worker :8002  (Playwright headed browser)
    └── (future services)
    ↓
Policy engine → Action log (SQLite) → Workers
```

**Why this architecture:**
- **PicoClaw is the UI.** It handles user interaction, conversation, scheduling
  (cron/heartbeat), and multi-channel delivery. PicoAssist doesn't build any of that.
- **PicoAssist is the tool layer.** It owns the domain logic: email triage, browser
  automation, safety policy, audit logging. Exposed as HTTP APIs.
- **PicoClaw skills** teach the LLM agent how to call PicoAssist's APIs. A skill is
  a markdown file — no MCP server, no custom protocol.
- **Dual-layer safety.** PicoClaw's sandbox protects the OS (workspace restriction,
  exec guards). PicoAssist's policy engine protects the data (no unauthorized Jira
  writes, no email delete).

---

## v1 — Digest Generation + Email Triage (COMPLETE)

**Status:** Shipped

**What it does:**
- Email triage via Microsoft Graph API or Gmail (list, summarize, move, draft — no send/delete)
- Jira Cloud monitoring via Playwright headed browser (read-only: navigate, screenshot, extract text)
- Azure DevOps monitoring via Playwright headed browser (read-only: same)
- Daily markdown digest per client in `data/runs/`
- Scheduled overnight runs via Windows Task Scheduler
- Gmail provider as alternative to Microsoft Graph

**Architecture:** Python modules importable directly + optional FastAPI HTTP wrappers.
`digest_runner.py` as standalone orchestrator. No Docker, no external orchestrator.

**Docs:** `docs/V1-IMPLEMENTATION_TASKS.md`

---

## v2 — Infrastructure, Safety, PicoClaw Integration (IN PROGRESS)

**Status:** Planning complete — see `docs/V2-Implementation-Plan.md`

**Goal:** Build the infrastructure for safe write actions (V3) and connect PicoClaw
as the user-facing agent. V2 adds no new write capabilities to Jira/ADO — it builds
the safety and audit machinery so V3 can add them safely.

**Phases:**
- **P1:** Pydantic config validation — fail fast on bad YAML with clear errors
- **P2:** SQLite action log — every action persisted for audit, replay, and approval queue
- **P3:** Policy engine (`policy.yaml`) — dedicated safety rules, per-client per-action
- **P4:** Approval workflow — approval endpoints that PicoClaw brokers to the user
- **P5:** Playwright stability — smart selectors, scoped text extraction, retry logic
- **P6:** PicoClaw skill + setup — teach PicoClaw to use PicoAssist's APIs
- **P7:** Documentation updates

**Key decisions:**
- **No MCP server.** PicoClaw calls PicoAssist via `exec curl` to existing FastAPI
  endpoints, following PicoClaw's native skill pattern (same as github, weather skills).
- **No custom triage CLI.** PicoClaw IS the triage interface — the user converses
  with PicoClaw, which calls PicoAssist's APIs to list items, propose actions, etc.
- **No custom approval CLI.** PicoClaw surfaces approval requests in conversation.
  PicoAssist exposes `/approval/*` HTTP endpoints that PicoClaw calls.
- **PicoClaw heartbeat/cron replaces Windows Task Scheduler** as the primary
  scheduling mechanism. `digest_runner.py` and PowerShell scripts remain as fallbacks.

---

## v3 — Write Actions, Pagination, API Auth (FUTURE)

**Status:** Designed, not planned in detail

### Jira/ADO write actions (browser-based, approval-gated)
- `jira_add_comment` — add a comment to a Jira issue via browser UI
- `jira_transition` — transition a Jira issue status via browser UI
- `ado_update_field` — update a work item field in ADO via browser UI
- All gated by V2's policy engine + approval workflow
- Overnight runs remain strictly read-only

### Email pagination
- Graph API: follow `@odata.nextLink` for large inboxes
- Gmail API: follow `nextPageToken`
- Configurable max cap and date boundary

### Jira/ADO API authentication
- Replace browser-based access with native API calls where possible
- Jira: OAuth 2.0 (3LO) — https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/
- ADO: Entra ID OAuth — https://devblogs.microsoft.com/devops/no-new-azure-devops-oauth-apps/
- Browser fallback for SSO/MFA-heavy environments

### Docker containerization
- Containerize `mail_worker` (stateless, API-only)
- `browser_worker` stays on Windows host (requires headed browser + display)
- `docker-compose.yml` for multi-container setup

### Email send (approval-gated)
- Add `mail_send` action to mail_worker
- Requires explicit approval — never auto-approved, never overnight
- Draft-then-send workflow: draft created (v1), user approves sending (v3)
- Requires `Mail.Send` permission scope in Entra app registration

### MCP server (optional)
- If other orchestrators (Claude Desktop, etc.) need to call PicoAssist
- Wraps existing FastAPI endpoints in MCP protocol
- Not needed for PicoClaw (uses skill + exec pattern)

---

## v4 — Multi-User, Notifications, Intelligence (FUTURE)

**Status:** Ideas only — scope may change significantly

### Multi-user support
- User accounts with role-based access (admin, approver, viewer)
- Per-user approval permissions
- Shared action log with user attribution

### Notification channels
- Slack/Teams integration for digest delivery
- Real-time alerts for high-priority email or Jira status changes
- Webhook support for custom integrations

### LLM-powered intelligence
- Email classification and priority scoring
- Automated draft generation for common reply patterns
- Jira/ADO status summarization with trend analysis
- Natural language queries against the action log

### Additional integrations
- Confluence (read pages, search)
- SharePoint / OneDrive (file access)
- Calendar (meeting summaries, scheduling conflicts)

### SQLite upgrades
- Email cache (reduce Graph API calls)
- Digest metadata and search
- Full-text search across action log and digests

---

## Version comparison

| Feature | v1 | v2 | v3 | v4 |
|---------|:--:|:--:|:--:|:--:|
| Email list/summarize/move/draft | Y | Y | Y | Y |
| Email send | - | - | Y | Y |
| Email pagination | - | - | Y | Y |
| Jira read (browser) | Y | Y | Y | Y |
| Jira write (browser) | - | - | Y | Y |
| Jira API auth | - | - | Y | Y |
| ADO read (browser) | Y | Y | Y | Y |
| ADO write (browser) | - | - | Y | Y |
| ADO API auth | - | - | Y | Y |
| Markdown digest | Y | Y | Y | Y |
| HTML digest | - | - | Y | Y |
| Pydantic config | - | Y | Y | Y |
| SQLite action log | - | Y | Y | Y |
| Approval workflow | - | Y | Y | Y |
| policy.yaml | - | Y | Y | Y |
| Playwright stability | - | Y | Y | Y |
| PicoClaw skill | - | Y | Y | Y |
| PicoClaw scheduling | - | Y | Y | Y |
| Docker | - | - | Y | Y |
| MCP server | - | - | Y | Y |
| Multi-user | - | - | - | Y |
| Notifications | - | - | - | Y |
| LLM intelligence | - | - | - | Y |
