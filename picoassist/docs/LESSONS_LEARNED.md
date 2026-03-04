# Lessons Learned

Operational knowledge captured during PicoClaw + PicoAssist integration.

---

## PicoClaw session persistence

Session files in `~/.picoclaw/workspace/sessions/` cache old SKILL.md content.
After updating SKILL.md, delete the session file or the model uses stale
instructions.

## Windows curl quoting in PicoClaw

PicoClaw's `exec` runs through PowerShell. Inline `-d "{...}"` always fails due
to quote escaping. Use the `write_file` tool to create a JSON file, then
`exec curl.exe -d @file.json ...`.

## Always use curl.exe not curl on Windows

In PowerShell, `curl` is an alias for `Invoke-WebRequest` which has a completely
different syntax. Always use `curl.exe` explicitly in `exec` commands to invoke
the real curl binary.

## PicoClaw display behavior

Only the final assistant message (one without `tool_calls`) is shown to the user.
If the model combines results with a cleanup tool call in the same response, the
user sees nothing. SKILL.md must instruct the model to never make tool calls
after getting results — present results in a standalone final message.

## Port 8001 binding / zombie processes

`start_services.ps1` launches processes with `-NoNewWindow` and (previously) no
log capture. Zombie processes are invisible and hard to find from bash. Debug
with PowerShell:

```powershell
Get-NetTCPConnection -LocalPort 8001 | Select-Object OwningProcess
Stop-Process -Id <pid>
```

## Mail provider config

`.env` defaults to `MAIL_PROVIDER=graph`. If you want Gmail, you must explicitly
set `MAIL_PROVIDER=gmail` and `GMAIL_CLIENT_SECRETS_FILE`. The `/health` endpoint
says "ready" even when auth will block on the first real request.

## Gmail OAuth testing mode

Google Cloud apps in "Testing" status require your email in the test users list.
The "ineligible account" warning during OAuth can be ignored if the email appears
in the list.

## PicoClaw config locations and model_list

The live config is `~/.picoclaw/config.json`. The repo copy at
`picoclaw/config/config.json` is reference only.

PicoClaw supports two config formats:
- **`model_list`** (preferred) — define named aliases; `agents.defaults.model`
  references an alias. Enables easy model switching by changing one short name.
- **`providers`** (legacy) — PicoClaw auto-migrates this to `model_list` on load.

Use `model_list`. The earlier note that it was "a different tool's format" was wrong.

## start_services.ps1 logging

The script now writes logs to `logs/mail_worker.log` and
`logs/browser_worker.log`. When debugging interactively, you can also start
services manually:

```bash
python -m uvicorn services.mail_worker.app:app --host 0.0.0.0 --port 8001
```

## Model instruction-following varies

`qwen3-coder-next` ignored SKILL.md CRITICAL sections entirely.
`anthropic/claude-sonnet-4` followed them. Consider model quality when
debugging unexpected PicoClaw behavior.

## Building PicoClaw from source on Windows

The Go build uses `make`. On Windows, run `make deps && make build` from Git Bash
(not PowerShell or cmd). After `git pull`, always `make build` and copy the new
`picoclaw.exe` to your PATH before testing changes.

## Suppress PicoClaw log noise

Use `2>$null` in PowerShell to hide INFO-level lines and only see the response:

```powershell
picoclaw agent -m "Check PicoAssist health" 2>$null
```
