# PicoAssist API Reference

## Mail Worker — port 8001

### GET /health
```json
{"status": "ok", "provider": "graph|gmail", "auth": "ready|not_configured"}
```

---

### POST /mail/list_unread
Request:
```json
{"folder": "Inbox", "max_results": 25}
```
Response:
```json
{
  "emails": [
    {
      "message_id": "AAMkAG...",
      "subject": "Re: Project update",
      "sender": "alice@example.com",
      "received_at": "2026-02-20T09:15:00Z",
      "preview": "First ~200 chars of body..."
    }
  ],
  "count": 1
}
```

---

### POST /mail/get_thread_summary
Request:
```json
{"message_id": "AAMkAG..."}
```
Response:
```json
{
  "subject": "Re: Project update",
  "messages": [
    {"message_id": "...", "sender": "alice@example.com", "sent_at": "...", "body_preview": "..."}
  ],
  "participant_count": 2
}
```

---

### POST /mail/move
Request:
```json
{"message_id": "AAMkAG...", "folder_name": "Archive", "client_id": "clientA"}
```
Response (immediate — policy allows):
```json
{"success": true, "new_folder": "Archive", "action_id": "uuid4", "approval_required": false}
```
Response (pending approval):
```json
{
  "success": false,
  "action_id": "uuid4",
  "approval_required": true,
  "description": "Move message 'AAMkAG...' to folder 'Archive'"
}
```
Response (blocked — 403):
```json
{"detail": "Action 'mail_move' is blocked by policy"}
```

Valid folder names: `Archive`, `ActionRequired`, `Quarantine`, `Newsletters`, `Junk`.

---

### POST /mail/draft_reply
Request:
```json
{
  "message_id": "AAMkAG...",
  "tone": "professional",
  "bullets": ["Confirm the timeline", "Ask for latest spec"],
  "client_id": "clientA"
}
```
Response (immediate):
```json
{
  "draft_id": "AAMkAG...",
  "subject": "Re: Project update",
  "body_preview": "Thank you for reaching out...",
  "action_id": "uuid4",
  "approval_required": false
}
```
Response (pending approval):
```json
{
  "action_id": "uuid4",
  "approval_required": true,
  "description": "Draft reply to message 'AAMkAG...' (tone: professional)"
}
```

Valid tones: `professional`, `casual`, `brief`.

---

## Browser Worker — port 8002

### GET /health
```json
{"status": "ok", "active_sessions": 0}
```

---

### POST /browser/start_session
Request:
```json
{"client_id": "clientA", "app": "jira"}
```
Response:
```json
{
  "session_id": "uuid4",
  "client_id": "clientA",
  "app": "jira",
  "profile_path": "C:/path/to/profiles/clientA/jira"
}
```
`app` must be `"jira"` or `"ado"`. Returns 409 if the profile is already in use.

---

### POST /browser/do
Request:
```json
{
  "session_id": "uuid4",
  "action_spec": {
    "action": "jira_capture",
    "params": {"url": "https://your.atlassian.net/issues/PA-42"}
  }
}
```
Response:
```json
{
  "success": true,
  "action_id": "uuid4",
  "result": {"url": "https://your.atlassian.net/issues/PA-42"},
  "artifacts": [
    {"type": "screenshot", "path": "C:/path/to/traces/session_id/jira_capture_20260220_091500.png"},
    {"type": "text", "path": "C:/path/to/traces/session_id/jira_capture_20260220_091500.txt"}
  ],
  "error": null
}
```

Available actions:

| Action | Required params | Description |
|--------|----------------|-------------|
| `jira_open` | `url` | Navigate to URL, wait for network idle |
| `jira_search` | `url` | Navigate, wait for `[role="main"]`, extract text |
| `jira_capture` | `url` (optional) | Screenshot + scoped text extraction |
| `ado_open` | `url` | Navigate to ADO URL |
| `ado_search` | `url` | Navigate, extract work items text |
| `ado_capture` | `url` (optional) | Screenshot + scoped text extraction |

Text artifacts are truncated to 4000 characters.

---

### POST /browser/screenshot
Request:
```json
{"session_id": "uuid4"}
```
Response:
```json
{"path": "C:/path/to/traces/session_id/screenshot_20260220_091500.png", "timestamp": "2026-02-20T09:15:00"}
```

---

### POST /browser/stop_session
Request:
```json
{"session_id": "uuid4"}
```
Response:
```json
{"success": true, "session_id": "uuid4"}
```

Always stop sessions when done. Unstopped sessions keep the browser process alive.

---

## Approval Endpoints (both ports)

### GET /approval/pending
Response:
```json
{
  "items": [
    {
      "action_id": "uuid4",
      "action_type": "mail_move",
      "client_id": "clientA",
      "description": "Move message '...' to folder 'Archive'",
      "params": {"message_id": "...", "folder_name": "Archive"},
      "created_at": "2026-02-20T09:00:00+00:00",
      "expires_at": "2026-02-20T09:30:00+00:00",
      "status": "pending_approval"
    }
  ],
  "count": 1
}
```

---

### POST /approval/approve
Request:
```json
{"action_id": "uuid4"}
```
Response (success):
```json
{"success": true, "action_id": "uuid4", "message": "Action approved and executed", "result": {...}}
```
Response (expired — 410):
```json
{"detail": "Approval window has expired"}
```
Response (not found — 404):
```json
{"detail": "Action not found: uuid4"}
```

---

### POST /approval/reject
Request:
```json
{"action_id": "uuid4"}
```
Response:
```json
{"success": true, "action_id": "uuid4", "message": "Action rejected"}
```

---

### POST /approval/expire
Expires all pending approvals that have passed their TTL. Call on-demand or periodically.

Response:
```json
{"expired_count": 2}
```

---

## Error codes summary

| Code | Meaning |
|------|---------|
| 200 | Success |
| 403 | Action blocked by policy (mail_send, mail_delete, overnight writes) |
| 404 | Session or action_id not found |
| 409 | Conflict (browser profile already in use, action not pending) |
| 410 | Approval window expired |
| 500 | Execution error (check `detail` field) |
| 503 | Service not initialised (approval engine unavailable) |
