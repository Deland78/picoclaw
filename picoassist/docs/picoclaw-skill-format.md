# PicoClaw Skill Format (Reference)

> Summarized from the `skill-creator` builtin skill at
> https://github.com/sipeed/picoclaw/blob/main/workspace/skills/skill-creator/SKILL.md
> for authoring reference. Do not modify — update from upstream as needed.

## What is a skill?

A skill is a folder containing a `SKILL.md` file that teaches PicoClaw's LLM agent
how to accomplish specific tasks using PicoClaw's built-in tools (`exec`, `read_file`,
`write_file`, `web_search`, `cron`, `spawn`, `message`, etc.).

Skills are **not executable code** — they are prompt instructions. The LLM reads them
and uses built-in tools accordingly.

## Skill structure

```
skill-name/
├── SKILL.md              # required — frontmatter + instructions
├── scripts/              # optional — executable helper scripts
├── references/           # optional — docs loaded into context on demand
└── assets/               # optional — files used in output (templates, etc.)
```

## SKILL.md format

### Frontmatter (required)

```yaml
---
name: my-skill-name
description: "What this skill does and when to use it. This is the primary
  triggering mechanism — be clear and comprehensive."
---
```

- `name`: lowercase, digits, hyphens only. Max 64 chars.
- `description`: max 1024 chars. Include both what it does AND when to use it.

### Body (markdown)

Instructions the agent follows when the skill is activated. Keep under 500 lines.
Use progressive disclosure — reference files in `references/` for detailed content.

## Skill loading hierarchy

1. **Workspace skills** (`~/.picoclaw/workspace/skills/`) — project-level, highest priority
2. **Global skills** (`~/.picoclaw/skills/`) — user-level
3. **Builtin skills** — shipped with PicoClaw

Workspace overrides global overrides builtin (by name).

## Example: weather skill

```yaml
---
name: weather
description: Get current weather and forecasts (no API key required).
---
```

```markdown
# Weather

## wttr.in (primary)

Quick one-liner:
\```bash
curl -s "wttr.in/London?format=3"
\```

Full forecast:
\```bash
curl -s "wttr.in/London?T"
\```
```

The agent reads this and uses `exec` to run `curl` commands.

## Example: github skill

```yaml
---
name: github
description: "Interact with GitHub using the gh CLI."
---
```

The body teaches the agent `gh pr`, `gh issue`, `gh api` patterns.

## Key principles

- **Skills share the context window.** Keep them concise.
- **The agent is already smart.** Only add knowledge it doesn't have.
- **Progressive disclosure:** Frontmatter is always loaded (~100 words).
  Body loads when triggered. References load on demand.
- **Use exec for HTTP calls:** `exec curl` is how skills call external APIs.

## Skill registries

Skills can be installed from ClawHub registry:
```json
"tools": {
  "skills": {
    "registries": {
      "clawhub": {
        "enabled": true,
        "base_url": "https://clawhub.ai"
      }
    }
  }
}
```

Or placed directly in the workspace `skills/` directory.
