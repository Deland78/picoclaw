# Client config template (client_config.yaml)

This file tells the assistant:
- which clients exist
- which Jira/ADO URLs to use
- what "views" to open for digests
- where to write outputs

> Safety and action policy settings have moved to `policy.yaml` (v2).
> See the [policy.yaml section](#policyyaml-v2) below for details.

> Tip: keep this file in your repo, but keep secrets (tokens, cookies) out of it.

---

## Schema (YAML)

```yaml
version: 1

paths:
  profiles_root: "./profiles"       # overridden by PROFILES_ROOT env var (must be absolute)
  runs_root: "./data/runs"
  traces_root: "./data/traces"

defaults:
  browser:
    slow_mo_ms: 50
    navigation_timeout_ms: 45000
    action_timeout_ms: 30000

mail:
  provider: "microsoft_graph"
  folders:
    quarantine: "Quarantine"
    action_required: "ActionRequired"
    archive: "Archive"

clients:
  - id: "clientA"
    display_name: "Client A"
    notes: "Anything helpful (timezone, naming conventions, etc.)"
    jira:
      base_url: "https://clientA.atlassian.net"
      digest:
        # v1/v2 uses UI URLs (web-first). JQL queries are for v3 API-based access.
        ui_urls:
          - name: "My issues"
            url: "https://clientA.atlassian.net/issues/?jql=assignee%20%3D%20currentUser()%20AND%20statusCategory%20!%3D%20Done"
    ado:
      org_url: "https://dev.azure.com/clientAOrg"
      project: "ClientAProject"
      team: "ClientATeam"
      digest:
        ui_urls:
          - name: "Boards"
            url: "https://dev.azure.com/clientAOrg/ClientAProject/_boards/board/t/ClientATeam/Backlog%20items"
          - name: "Queries"
            url: "https://dev.azure.com/clientAOrg/ClientAProject/_queries"
    reporting:
      output_subdir: "clientA"
```

---

## policy.yaml (v2)

In v2, safety settings moved from `client_config.yaml` to a dedicated `policy.yaml` file.
`policy.yaml` is the source of truth for what actions are allowed at runtime.

### Global policy schema

```yaml
version: 1

global_policy:
  actions:
    read:
      - mail_list_unread
      - mail_get_thread
      - jira_open
      - jira_search
      - jira_capture
      - ado_open
      - ado_search
      - ado_capture
    write:
      - mail_move
      - mail_draft_reply
    blocked:
      - mail_send
      - mail_delete
  overnight:
    start_hour: 0    # 00:00 — write actions blocked from here...
    end_hour: 6      # 06:00 — ...until here (reads always allowed)
  approval:
    auto_approve_reads: true    # reads execute immediately, no approval prompt
    token_ttl_minutes: 30       # approval tokens expire after 30 minutes
```

### Per-client policy overrides

Clients can override global policy in `policy.yaml`:

```yaml
client_policies:
  - client_id: "clientA"
    actions:
      # Extend the global read list with client-specific actions
      read: [jira_open, jira_search, jira_capture]
      # Override blocked list (e.g. allow mail_move for this client)
      write: [mail_move]
```

Per-client overrides in `policy.yaml` take precedence over global policy for that client.

### How policy interacts with client_config.yaml

- `client_config.yaml` defines **what** clients exist and their Jira/ADO URLs.
- `policy.yaml` defines **what actions are allowed** and under what conditions.
- They are independent — you can add a client in `client_config.yaml` without changing `policy.yaml`.

---

## v2 action list

### Mail (read)
- `mail_list_unread` — list unread messages in a folder
- `mail_get_thread` — fetch thread summary for a message ID

### Mail (write — require approval by default)
- `mail_move` — move message to a folder (Archive, ActionRequired, Quarantine, Newsletters)
- `mail_draft_reply` — create a draft reply

### Mail (permanently blocked)
- `mail_send` — **never implemented**
- `mail_delete` — **never implemented**

### Jira (read-only)
- `jira_open` — navigate to a URL
- `jira_search` — navigate to JQL search URL, extract results text
- `jira_capture` — screenshot + optional page text extraction

### Azure DevOps (read-only)
- `ado_open` — navigate to a URL
- `ado_search` — navigate to queries page, extract results text
- `ado_capture` — screenshot + optional page text extraction

### v3 write actions (designed, not yet implemented)

```yaml
# Future v3 per-client policy additions:
    policies:
      allowed_actions:
        jira:
          write: [jira_add_comment, jira_transition]
        ado:
          write: [ado_update_field]
```

---

## Recommended client onboarding checklist

For each client:
1. Add the client stanza to `client_config.yaml`
2. Run `pwsh scripts/run_interactive.ps1 -ClientId <id> -App jira`
3. Log in manually in the browser; press Enter when done; type `quit`
4. Repeat for ADO: `pwsh scripts/run_interactive.ps1 -ClientId <id> -App ado`
5. Run a single digest to validate: `python digest_runner.py`
6. Check output in `data/runs/<today>/<client_id>/digest.md`
