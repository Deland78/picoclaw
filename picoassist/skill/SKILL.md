---
name: picoassist
description: "Personal PM assistant for email triage, Jira/ADO monitoring, daily
  digests, and model switching. Use when the user asks about email, Jira issues,
  ADO work items, daily digest reports, wants to switch the AI model, OR asks
  what model is currently active / in use. Calls PicoAssist HTTP APIs on
  localhost:8001 (email) and localhost:8002 (browser automation). Supports an
  approval workflow for write/triage actions."
---

# PicoAssist

PicoAssist is a local HTTP backend running two services:
- **mail_worker** on `http://localhost:8001` — email triage via Microsoft 365 or Gmail
- **browser_worker** on `http://localhost:8002` — Jira/ADO capture via persistent Playwright sessions

All calls use `exec curl.exe`. Full endpoint details are in `references/api-reference.md`.

## CRITICAL: Windows curl pattern — NEVER use inline JSON

**IMPORTANT: Do NOT use `-d "{...}"` or `-d '{...}'` — this ALWAYS fails on Windows due to shell quoting.**

You MUST write the JSON body to a file first, then pass the file path:

```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"key":"value"}
exec curl.exe -s -X POST http://... -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```

This is the ONLY working pattern. All examples below use it. Do not attempt any other approach.

## CRITICAL: Response rules

1. NEVER delete or clean up temp files. NEVER call Remove-Item. The file `req.json` is reused every time — cleanup is unnecessary and wasteful.
2. After the curl command returns data, IMMEDIATELY respond to the user with a formatted summary. Do NOT make any more tool calls. Your response after getting curl results must contain ONLY text, NO tool calls.
3. NEVER say "Cleaned up" or "temporary file". Summarize the actual data you retrieved.

## CRITICAL: Never fabricate API data

- ONLY report data that was actually returned by an API call. If a curl command fails or returns an error, report the error — do NOT invent or guess what the data might look like.
- If a service is down (connection refused, timeout, non-200 status), say "Service on port XXXX is not responding" — do NOT fabricate emails, approvals, or other data.
- NEVER generate fake names, subjects, amounts, or dates to fill in for missing data.
- If you are unsure whether data is real, show the raw API response.

## Model switching

The user can ask to switch models at any time (e.g. "switch to Sonnet", "use GPT-4o", "what model are you on?", "what model are you using?").

**IMPORTANT:** You cannot introspect your own model. You MUST read the config file to answer any question about the current model — do not guess or answer from general knowledge.

### Check current model
```bash
exec read_file C:\Users\david\.picoclaw\config.json
```
Report the value of `agents.defaults.model`.

### Switch model
Read the config, update `agents.defaults.model`, write it back:
```bash
exec read_file C:\Users\david\.picoclaw\config.json
exec write_file C:\Users\david\.picoclaw\config.json <full updated JSON>
```

**CRITICAL:** Write the complete config JSON — never a partial update. Preserve all other fields exactly.

After writing, tell the user: "Switched to `<model>`. Restart PicoClaw (close and reopen) for the change to take effect."

### Common OpenRouter model IDs

| Name | Model ID |
|------|----------|
| Claude Sonnet 4.6 | `anthropic/claude-sonnet-4-6` |
| Claude Opus 4.6 | `anthropic/claude-opus-4-6` |
| Claude Haiku 4.5 | `anthropic/claude-haiku-4-5-20251001` |
| GPT-4o | `openai/gpt-4o` |
| GPT-4o mini | `openai/gpt-4o-mini` |
| o3 mini | `openai/o3-mini` |
| Gemini 2.0 Flash | `google/gemini-2.0-flash-001` |
| Gemini 2.5 Pro | `google/gemini-2.5-pro-preview-03-25` |
| Qwen3 Coder | `qwen/qwen3-coder-next` |
| DeepSeek R1 | `deepseek/deepseek-r1` |
| Mistral Large | `mistralai/mistral-large-2411` |

If the user names a model not in this list, use your best judgement to construct the OpenRouter model ID (format: `provider/model-name`) and confirm with the user before writing.

---

## Prerequisites

Both services must be running. Check health first:

```bash
exec curl -s http://localhost:8001/health
exec curl -s http://localhost:8002/health
```

Expected: `{"status":"ok",...}`. If either fails, tell the user to run `scripts/start_services.ps1`.

## Email operations

### List unread email
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"folder":"Inbox","max_results":25}
exec curl.exe -s -X POST http://localhost:8001/mail/list_unread -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```

### Get thread detail
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"message_id":"MSG_ID"}
exec curl.exe -s -X POST http://localhost:8001/mail/get_thread_summary -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```

### Move a message (requires approval — see Approval Flow)
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"message_id":"MSG_ID","folder_name":"Archive","client_id":"CLIENT_ID"}
exec curl.exe -s -X POST http://localhost:8001/mail/move -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```
Valid folders: `Archive`, `ActionRequired`, `Quarantine`, `Newsletters`.

### Draft a reply (requires approval)
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"message_id":"MSG_ID","tone":"professional","bullets":["Point 1","Point 2"],"client_id":"CLIENT_ID"}
exec curl.exe -s -X POST http://localhost:8001/mail/draft_reply -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```
Valid tones: `professional`, `casual`, `brief`.

## Browser / Jira / ADO operations

### Start a session
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"client_id":"CLIENT_ID","app":"jira"}
exec curl.exe -s -X POST http://localhost:8002/browser/start_session -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```
`app` is `"jira"` or `"ado"`. Returns `session_id`.

### Capture a page
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"session_id":"SESSION_ID","action_spec":{"action":"jira_capture","params":{"url":"https://your.atlassian.net/issues"}}}
exec curl.exe -s -X POST http://localhost:8002/browser/do -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```
Available actions: `jira_open`, `jira_search`, `jira_capture`, `ado_open`, `ado_search`, `ado_capture`.

### Stop session (always stop when done)
```bash
exec write_file C:\Users\david\.picoclaw\workspace\req.json {"session_id":"SESSION_ID"}
exec curl.exe -s -X POST http://localhost:8002/browser/stop_session -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
```

## Approval flow

When a response contains `"approval_required": true`, the action was **not executed** — it is waiting for user confirmation.

1. **Show the user** the `description` field from the response.
2. **Ask** "Shall I go ahead?"
3. If yes → approve:
   ```bash
   exec write_file C:\Users\david\.picoclaw\workspace\req.json {"action_id":"ACTION_ID"}
   exec curl.exe -s -X POST http://localhost:8001/approval/approve -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
   ```
4. If no → reject:
   ```bash
   exec write_file C:\Users\david\.picoclaw\workspace\req.json {"action_id":"ACTION_ID"}
   exec curl.exe -s -X POST http://localhost:8001/approval/reject -H "Content-Type: application/json" -d @C:\Users\david\.picoclaw\workspace\req.json
   ```
5. List pending approvals at any time:
   ```bash
   exec curl -s http://localhost:8001/approval/pending
   ```

Use port `8002` for browser action approvals, `8001` for mail action approvals.

Approvals expire after **30 minutes** by default. A `410` response means the window has passed.

## Running the daily digest

```bash
exec python digest_runner.py
```

Or via the API — list unread for each client, capture Jira/ADO views, summarise.

## Safety rules (non-negotiable)

| Action | Policy |
|--------|--------|
| `mail_list_unread`, `mail_get_thread` | Always allowed (read) |
| `jira_*`, `ado_*` | Always allowed (read) |
| `mail_move`, `mail_draft_reply` | Require approval (default) |
| `mail_send`, `mail_delete` | **Permanently blocked** — never attempt |

During overnight hours (00:00–06:00) all write actions are blocked even with approval.

## More detail

Load `references/api-reference.md` for full request/response schemas.
