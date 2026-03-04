# PicoClaw Tools Configuration (Reference)

> Copied from https://github.com/sipeed/picoclaw/blob/main/docs/tools_configuration.md
> for skill authoring reference. Do not modify — update from upstream as needed.

PicoClaw's tools configuration is located in the `tools` field of `config.json`.

## Directory Structure

```json
{
  "tools": {
    "web": { ... },
    "exec": { ... },
    "approval": { ... },
    "cron": { ... }
  }
}
```

## Web Tools

Web tools are used for web search and fetching.

### Brave

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable Brave search |
| `api_key` | string | - | Brave Search API key |
| `max_results` | int | 5 | Maximum number of results |

### DuckDuckGo

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable DuckDuckGo search |
| `max_results` | int | 5 | Maximum number of results |

### Perplexity

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | false | Enable Perplexity search |
| `api_key` | string | - | Perplexity API key |
| `max_results` | int | 5 | Maximum number of results |

## Exec Tool

The exec tool is used to execute shell commands.

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enable_deny_patterns` | bool | true | Enable default dangerous command blocking |
| `custom_deny_patterns` | array | [] | Custom deny patterns (regular expressions) |

### Default Blocked Command Patterns

- Delete commands: `rm -rf`, `del /f/q`, `rmdir /s`
- Disk operations: `format`, `mkfs`, `diskpart`, `dd if=`, writing to `/dev/sd*`
- System operations: `shutdown`, `reboot`, `poweroff`
- Command substitution: `$()`, `${}`, backticks
- Pipe to shell: `| sh`, `| bash`
- Privilege escalation: `sudo`, `chmod`, `chown`
- Process control: `pkill`, `killall`, `kill -9`
- Remote operations: `curl | sh`, `wget | sh`, `ssh`
- Package management: `apt`, `yum`, `dnf`, `npm install -g`, `pip install --user`
- Containers: `docker run`, `docker exec`
- Git: `git push`, `git force`
- Other: `eval`, `source *.sh`

## Approval Tool

The approval tool controls permissions for dangerous operations.

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `enabled` | bool | true | Enable approval functionality |
| `write_file` | bool | true | Require approval for file writes |
| `edit_file` | bool | true | Require approval for file edits |
| `append_file` | bool | true | Require approval for file appends |
| `exec` | bool | true | Require approval for command execution |
| `timeout_minutes` | int | 5 | Approval timeout in minutes |

## Cron Tool

| Config | Type | Default | Description |
|--------|------|---------|-------------|
| `exec_timeout_minutes` | int | 5 | Execution timeout in minutes, 0 means no limit |

## Environment Variables

All configuration options can be overridden via environment variables with the format `PICOCLAW_TOOLS_<SECTION>_<KEY>`:

- `PICOCLAW_TOOLS_WEB_BRAVE_ENABLED=true`
- `PICOCLAW_TOOLS_EXEC_ENABLE_DENY_PATTERNS=false`
- `PICOCLAW_TOOLS_CRON_EXEC_TIMEOUT_MINUTES=10`
