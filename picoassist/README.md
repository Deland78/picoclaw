# PicoAssist

Personal assistant for a solo PM consultant. Triages Microsoft 365 email via Graph API (or Gmail), monitors Jira Cloud and Azure DevOps via Playwright browser automation, and produces daily markdown digest reports.

In v2, **PicoClaw** is the primary interface — send natural-language commands like "list my unread email" or "show my Jira issues" and PicoClaw calls PicoAssist on your behalf.

## Architecture

```
You
 │
 │  picoclaw agent -m "list my unread email"
 ▼
PicoClaw (Go binary, runs locally)
 │  picoassist skill — knows the API
 │
 ├──► mail_worker  (FastAPI, port 8001)  — email triage via Microsoft Graph / Gmail
 └──► browser_worker (FastAPI, port 8002) — Jira/ADO capture via Playwright
```

Both workers run as native Python processes on Windows. No cloud, no Docker required.

## Prerequisites

- **Python 3.11+** on Windows
- **PowerShell 7+** (`pwsh`) or Windows PowerShell (`powershell`)
- A **Microsoft Entra app registration** — or a **Gmail OAuth client** (see below)
- **PicoClaw binary** (v2 interface) — see [PicoClaw Setup](#picoclaw-setup-v2-interface)

## Entra App Registration (one-time setup)

PicoAssist uses Microsoft Graph API to read and triage your email. You need to register an app in Entra ID (formerly Azure AD):

1. Go to [Azure Portal > Entra ID > App registrations](https://portal.azure.com/#view/Microsoft_AAD_RegisteredApps/ApplicationsListBlade)
2. Click **New registration**
   - Name: `PicoAssist` (or any name you prefer)
   - Supported account types: **Accounts in this organizational directory only** (single tenant)
   - Redirect URI: leave blank (device code flow doesn't need one)
3. Click **Register**
4. On the app's **Overview** page, copy:
   - **Application (client) ID** — this is your `GRAPH_CLIENT_ID`
   - **Directory (tenant) ID** — this is your `GRAPH_TENANT_ID`
5. Go to **API permissions** > **Add a permission** > **Microsoft Graph** > **Delegated permissions**
   - Add: `Mail.Read`
   - Add: `Mail.ReadWrite`
6. Click **Grant admin consent for [your org]** (if you're the admin) or ask your admin to consent
7. Go to **Authentication** > under **Advanced settings**, set **Allow public client flows** to **Yes** (required for device code flow)
8. Click **Save**

## Gmail Setup (alternative to Graph)

If Microsoft Graph credentials aren't available, you can use Gmail instead:

1. Go to [Google Cloud Console > APIs & Services > Credentials](https://console.cloud.google.com/apis/credentials)
2. Create a project (or select existing), then click **Create Credentials** > **OAuth client ID**
3. Application type: **Desktop app**, Name: `PicoAssist`
4. Click **Create**, then **Download JSON** — save as `gmail_client_secrets.json` in the repo root
5. Go to **APIs & Services > Library**, enable the **Gmail API**
6. In your `.env`, set:
   ```
   MAIL_PROVIDER=gmail
   GMAIL_CLIENT_SECRETS_FILE=./gmail_client_secrets.json
   ```
7. On first run, a browser window opens for Google consent. The refresh token is cached in `data/tokens/gmail_token.json`.

## First Run

1. Clone the repo and set up:
   ```
   git clone https://github.com/Deland78/picoclaw.git
   cd picoclaw/picoassist
   cp .env.example .env
   ```

2. Edit `.env` — fill in your values:
   ```
   GRAPH_CLIENT_ID=<from step 4 above>
   GRAPH_TENANT_ID=<from step 4 above>
   PROFILES_ROOT=./profiles
   ```

3. Install dependencies:
   ```
   pip install -e ".[dev]"
   playwright install chromium
   ```

4. Edit `client_config.yaml` — add your first client with Jira/ADO URLs

5. Onboard client browser profiles (one-time per client):
   ```
   pwsh scripts/run_interactive.ps1 -ClientId clientA -App jira
   # Browser opens -> log in manually -> press Enter -> type 'quit'

   pwsh scripts/run_interactive.ps1 -ClientId clientA -App ado
   # Same process for ADO
   ```

6. Authenticate Graph API (device code flow):
   ```
   python -c "
   import asyncio
   from services.mail_worker import MSALAuth
   auth = MSALAuth('YOUR_CLIENT_ID', 'YOUR_TENANT_ID', ['Mail.Read', 'Mail.ReadWrite'], './data/tokens')
   asyncio.run(auth.get_token())
   "
   ```
   Follow the device code prompt to sign in at https://microsoft.com/devicelogin

7. Start services and run your first command via PicoClaw:
   ```
   pwsh scripts/start_services.ps1
   picoclaw agent -m "Check PicoAssist health"
   picoclaw agent -m "List my unread email"
   ```

   Or run the digest directly (no PicoClaw required):
   ```
   python digest_runner.py
   ```

8. Check digest output:
   ```powershell
   Get-ChildItem data/runs/$(Get-Date -Format 'yyyy-MM-dd')/
   ```

9. (Optional) Schedule daily runs:
   ```
   pwsh scripts/schedule_task.ps1 -Time "06:00"
   ```

## PicoClaw Setup (v2 interface)

PicoClaw is a lightweight AI agent that translates natural-language requests into PicoAssist API calls. See `docs/picoclaw-setup.md` for full setup instructions.

**Quick steps:**

1. Download the PicoClaw binary and place it on your PATH (e.g. `C:\Users\<you>\bin\`),
   or build from source: `pwsh scripts/build_picoclaw.ps1`

2. Configure PicoClaw with your LLM API key (edit `~/.picoclaw/config.json`):
   ```json
   {
     "agents": { "defaults": { "model": "your-model-name" } },
     "providers": {
       "openrouter": {
         "api_key": "sk-or-...",
         "api_base": "https://openrouter.ai/api/v1"
       }
     }
   }
   ```

3. Install the PicoAssist skill:
   ```
   pwsh scripts/install_skill.ps1
   ```

4. Start services and test:
   ```
   pwsh scripts/start_services.ps1
   picoclaw agent -m "Check PicoAssist health"
   ```

## policy.yaml Configuration

`policy.yaml` controls what actions are allowed, overnight restrictions, and approval requirements. Example:

```yaml
version: 1

global_policy:
  actions:
    read:  [mail_list_unread, mail_get_thread, jira_open, jira_search, jira_capture, ado_open, ado_search, ado_capture]
    write: [mail_move, mail_draft_reply]
    blocked: [mail_send, mail_delete]
  overnight:
    start_hour: 0
    end_hour: 6
  approval:
    auto_approve_reads: true
    token_ttl_minutes: 30
```

See `docs/CLIENT_CONFIG_TEMPLATE.md` for per-client policy overrides.

## Running Tests

```
pytest -v
```

All tests use mocked external services (Graph API, Playwright) and run without credentials.

## Current Scope (v2)

- **Email**: list unread, summarize threads, move to folder, draft replies (no send, no delete)
- **Jira**: open configured URLs, screenshot, extract text (read-only)
- **ADO**: open configured URLs, screenshot, extract text (read-only)
- **Digest**: combined markdown report per client in `data/runs/`
- **Policy engine**: `policy.yaml` controls action rules and overnight restrictions
- **Approval workflow**: write actions queue for user confirmation
- **Action log**: SQLite database (`data/picoassist.db`) for all actions
- **PicoClaw**: natural-language interface via skill

See `docs/README_PM_ASSISTANT_LAB.md` for full architecture details.
