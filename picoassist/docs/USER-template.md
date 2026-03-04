# User Preferences

## CRITICAL: Never fabricate data

When calling APIs (curl, web_fetch, etc.), ONLY report data actually returned.
If a call fails or a service is down, report the error — NEVER invent emails,
approvals, names, subjects, amounts, or dates. If unsure whether data is real,
show the raw response.

## CRITICAL: Validate URLs before presenting them

Before including ANY URL in a response to the user, you MUST verify it loads
successfully using `web_fetch`. If `web_fetch` fails (DNS error, timeout, 404,
etc.), do NOT present that URL. Only give the user URLs that returned a valid
page. Search result listings are NOT proof a URL works — always fetch to confirm.

## Channel routing

Only respond on the channel the message came from. If asked from the terminal,
reply to the terminal — do NOT also send to Telegram. If asked from Telegram,
reply to Telegram. Never forward responses to a different channel unless the
user explicitly asks.

## OpenRouter usage and costs

To check OpenRouter spending, call the auth/key endpoint:

```
exec curl.exe -s "https://openrouter.ai/api/v1/auth/key" -H "Authorization: Bearer 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"
```

The response contains:
- `usage_daily` — spend today (USD)
- `usage_weekly` — spend this week (USD)
- `usage_monthly` — spend this month (USD)
- `usage` — total lifetime spend (USD)
- `limit_remaining` — remaining balance on the account

Note: hourly breakdown is not available from the API.

## Shell environment

The `exec` tool runs commands in **PowerShell** on Windows. Do NOT use Unix
commands (`find`, `head`, `tail`, `grep`, `/dev/null`, `2>/dev/null`). Use
PowerShell equivalents:

| Unix | PowerShell |
|------|------------|
| `find . -name "*.ps1"` | `Get-ChildItem -Recurse -Filter *.ps1` |
| `head -10` | `Select-Object -First 10` |
| `grep pattern file` | `Select-String -Pattern pattern file` |
| `2>/dev/null` | `-ErrorAction SilentlyContinue` |
| `cat file` | `Get-Content file` |

When a service is unreachable, report the error to the user immediately —
do NOT spend iterations trying to discover scripts or debug infrastructure.

## Fetching JSON APIs

Use `exec curl.exe -s <url>` to call JSON APIs — NOT `web_fetch`. `web_fetch`
sends a browser User-Agent that Cloudflare blocks on many APIs. On Windows,
`curl` is an alias for Invoke-WebRequest — always use `curl.exe` explicitly.

Example — get approximate location from IP:
```
exec curl.exe -s https://ipapi.co/json/
```

## Model switching

Your active model is set by `agents.defaults.model` in
`C:\Users\david\.picoclaw\config.json`. The value must be a `model_name` alias
from the `model_list` in that file.

When asked "what model are you using" or similar: read the config file and
report the `agents.defaults.model` value — do not guess.

When asked to switch models: read the config, update `agents.defaults.model`
to the alias, write the full config back, and tell the user to restart PicoClaw.

Available aliases:

| Alias | Model |
|-------|-------|
| `sonnet` | Claude Sonnet 4.6 (`anthropic/claude-sonnet-4.6`) |
| `opus` | Claude Opus 4.6 (`anthropic/claude-opus-4.6`) |
| `haiku` | Claude Haiku 4.5 (`anthropic/claude-haiku-4.5`) |
| `gpt4o` | GPT-4o |
| `gpt4o-mini` | GPT-4o mini |
| `gemini-flash` | Gemini 2.0 Flash |
| `qwen` | Qwen3 Coder |
| `deepseek` | DeepSeek R1 |
| `minimax` | MiniMax M2.5 |
| `nemo3` | Nvidia Nemotron 3 Nano 30B (free) |

To add a new model, add an entry to `model_list` in the config with a new
`model_name` alias and the OpenRouter model ID, then use the alias.
