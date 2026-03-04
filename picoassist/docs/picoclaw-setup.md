# PicoClaw + PicoAssist Setup

This guide sets up PicoClaw as a native Go binary on Windows alongside PicoAssist
Python services.

## Architecture overview

```
Windows host
├── PicoAssist services  (native Python)
│   ├── mail_worker      port 8001
│   └── browser_worker   port 8002
└── PicoClaw             (native Go binary)
    └── reaches services at localhost
```

PicoClaw and PicoAssist services both run natively on the Windows host.
No Docker required.

---

## Prerequisites

- PicoAssist repo cloned and dependencies installed (`pip install -e ".[dev]"`)
- `.env` file configured (copy `.env.example`, fill in credentials)
- Playwright browser installed: `playwright install chromium`
- **Go 1.21+** installed (`go version` to check)
- **make** available — on Windows use Git Bash (bundled with Git for Windows) or install MSYS2

---

## Step 1 — Clone and build PicoClaw from source

Clone the PicoClaw repo as a sibling to PicoAssist (one-time step):

```bash
cd C:\Users\david
git clone https://github.com/sipeed/picoclaw.git
```

Build the binary:

```bash
cd C:\Users\david\picoclaw
make deps
make build
```

This produces `picoclaw.exe`. Copy it to a directory on your PATH:

```powershell
mkdir -Force "$env:USERPROFILE\bin"
Copy-Item "C:\Users\david\picoclaw\picoclaw.exe" "$env:USERPROFILE\bin\picoclaw.exe"
# Add to PATH if not already there
$p = [Environment]::GetEnvironmentVariable("Path", "User")
if ($p -notlike "*$env:USERPROFILE\bin*") {
    [Environment]::SetEnvironmentVariable("Path", "$p;$env:USERPROFILE\bin", "User")
}
```

Verify:

```powershell
picoclaw --version
```

> **To update or rebuild PicoClaw:** run `pwsh scripts\build_picoclaw.ps1` from
> the PicoAssist repo. It pulls the latest source, copies the workspace embed,
> builds, and installs in one step.
>
> **To modify PicoClaw behavior:** edit the Go source in `C:\Users\david\picoclaw`,
> then run `pwsh scripts\build_picoclaw.ps1`.

---

## Step 2 — Configure PicoClaw

Create the config directory and file:

```powershell
mkdir -Force "$env:USERPROFILE\.picoclaw"
```

Create `~/.picoclaw/config.json` with your settings:

```json
{
  "agents": {
    "defaults": {
      "workspace": "C:\\Users\\<you>\\.picoclaw\\workspace",
      "model": "sonnet",
      "max_tokens": 8192,
      "restrict_to_workspace": false
    }
  },
  "model_list": [
    {
      "model_name": "sonnet",
      "model": "anthropic/claude-sonnet-4.6",
      "api_key": "sk-or-v1-YOUR_KEY",
      "api_base": "https://openrouter.ai/api/v1"
    },
    {
      "model_name": "opus",
      "model": "anthropic/claude-opus-4.6",
      "api_key": "sk-or-v1-YOUR_KEY",
      "api_base": "https://openrouter.ai/api/v1"
    },
    {
      "model_name": "qwen",
      "model": "qwen/qwen3-coder-next",
      "api_key": "sk-or-v1-YOUR_KEY",
      "api_base": "https://openrouter.ai/api/v1"
    }
  ],
  "heartbeat": {
    "enabled": true,
    "interval": 30
  },
  "tools": {
    "web": {
      "brave": {
        "enabled": true,
        "api_key": "YOUR_BRAVE_API_KEY",
        "max_results": 5
      },
      "duckduckgo": {
        "enabled": true,
        "max_results": 5
      }
    }
  }
}
```

> **Important:** The live config is `~/.picoclaw/config.json`. The repo copy at
> `picoclaw/config/config.json` is reference only.
>
> Use `model_list` to define named aliases. `agents.defaults.model` references
> an alias (e.g. `"sonnet"`), not a raw OpenRouter model ID. To switch models,
> change `agents.defaults.model` to a different alias and restart PicoClaw.
> The legacy `providers` format is still accepted but auto-migrated.
>
> **Web search:** PicoClaw has built-in web search. With a Brave API key
> (`brave.enabled: true`), it uses the Brave Search API (2,000 free queries/month).
> Without a key, set `brave.enabled: false` and it falls back to DuckDuckGo
> automatically. Get a free key at <https://brave.com/search/api>.

---

## Step 3 — Install the PicoAssist skill

```powershell
$dest = "$env:USERPROFILE\.picoclaw\workspace\skills\picoassist"
New-Item -ItemType Directory -Force $dest | Out-Null
Copy-Item -Recurse -Force "picoclaw\skills\picoassist\*" $dest
```

```bash
# Linux / macOS
mkdir -p ~/.picoclaw/workspace/skills/picoassist
cp -r picoclaw/skills/picoassist/. ~/.picoclaw/workspace/skills/picoassist/
```

---

## Step 4 — Install the HEARTBEAT and USER.md

```powershell
Copy-Item "picoclaw\HEARTBEAT.md" "$env:USERPROFILE\.picoclaw\workspace\HEARTBEAT.md"
Copy-Item "picoclaw\USER.md" "$env:USERPROFILE\.picoclaw\workspace\USER.md"
```

> `USER.md` is a bootstrap file — PicoClaw loads it fully into the system prompt
> every session. It contains always-available instructions (e.g. model switching).
> `HEARTBEAT.md` drives periodic background tasks.

---

## Step 5 — Configure Telegram (optional)

To chat with PicoClaw over Telegram:

1. Message [@BotFather](https://t.me/BotFather) → `/newbot` → copy the token
2. Message [@userinfobot](https://t.me/userinfobot) → note your numeric user ID
3. Add to `~/.picoclaw/config.json`:

```json
"channels": {
  "telegram": {
    "enabled": true,
    "token": "7957497658:AAFjkJ9Q777bzsXtIR9g3RcMeqJb1JyHI6M",
    "allow_from": ["6956894395"]
  }
}
```

The `allow_from` list restricts who can message the bot — always set this.

---

## Step 6 — Start PicoAssist services

```powershell
pwsh scripts\start_services.ps1
```

This starts `mail_worker` (port 8001), `browser_worker` (port 8002), and the
PicoClaw gateway (Telegram + other channels). Verify services are up:

```powershell
curl http://localhost:8001/health
curl http://localhost:8002/health
```

---

## Step 7 — Test

```powershell
picoclaw agent -m "Check PicoAssist health"
```

Expected: PicoClaw calls both `/health` endpoints and reports `status: ok`.

```powershell
picoclaw agent -m "List my unread emails"
```

Expected: PicoClaw calls `/mail/list_unread` and summarises the results.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `Connection refused localhost:8001` | Run `pwsh scripts\start_services.ps1` — check logs in `logs/` |
| `not_configured` in health response | Check `.env` credentials |
| Browser profile locked | Close any open Chromium windows for that profile |
| Approval window expired (410) | The 30-minute window passed — re-issue the original request |
| PicoClaw doesn't recognise the skill | Verify skill files at `~/.picoclaw/workspace/skills/picoassist/` |
| Stale SKILL.md after update | Delete session files in `~/.picoclaw/workspace/sessions/` |
| Port already in use | `Get-NetTCPConnection -LocalPort 8001` then `Stop-Process -Id <pid>` |
| PicoClaw shows no output | Model may have combined response with a tool call — see `docs/LESSONS_LEARNED.md` |
| `exec curl` with inline JSON fails | Use `write_file` + `-d @file` pattern — see `docs/LESSONS_LEARNED.md` |
