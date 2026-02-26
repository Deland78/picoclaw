---
name: self-improvement
description: "Captures learnings, errors, and corrections to enable continuous improvement. Use when: (1) A command or operation fails unexpectedly, (2) User corrects you, (3) User requests a capability that doesn't exist, (4) You discover your knowledge was wrong, (5) A better approach is found for a recurring task. Also review learnings before major tasks."
---

# Self-Improvement

Log learnings and errors to `.learnings/` files in your workspace. Proven patterns get promoted to workspace memory and instructions.

## Quick Reference

| Situation | Action |
|-----------|--------|
| Command/operation fails | Log to `.learnings/ERRORS.md` |
| User corrects you | Log to `.learnings/LEARNINGS.md` (category: `correction`) |
| User wants missing feature | Log to `.learnings/FEATURE_REQUESTS.md` |
| API/external tool fails | Log to `.learnings/ERRORS.md` with integration details |
| Knowledge was outdated | Log to `.learnings/LEARNINGS.md` (category: `knowledge_gap`) |
| Found better approach | Log to `.learnings/LEARNINGS.md` (category: `best_practice`) |
| Broadly applicable learning | Promote to `memory/MEMORY.md` |
| Behavioral patterns | Promote to `SOUL.md` |
| Workflow improvements | Promote to `AGENT.md` |

## When to Log

Log immediately when you notice:

**Corrections**: User says "No, that's wrong", "Actually, it should be...", "That's outdated"
**Errors**: Non-zero exit codes, exceptions, stack traces, timeouts, connection failures
**Knowledge gaps**: User provides information you didn't know, docs you referenced were outdated
**Feature requests**: "Can you also...", "I wish you could...", "Why can't you..."
**Best practices**: You discover a better way to do something recurring

## Logging Format

### Learning Entry

Append to `.learnings/LEARNINGS.md`:

```markdown
## [LRN-YYYYMMDD-NNN] category

**Logged**: YYYY-MM-DDTHH:MM:SSZ
**Priority**: low | medium | high | critical
**Status**: pending
**Area**: frontend | backend | infra | config | tools | skills

### Summary
One-line description

### Details
What happened, what was wrong, what's correct

### Suggested Action
Specific fix or improvement

### Metadata
- Source: conversation | error | user_feedback
- Related Files: path/to/file
- Tags: tag1, tag2

---
```

### Error Entry

Append to `.learnings/ERRORS.md`:

```markdown
## [ERR-YYYYMMDD-NNN] tool_or_command_name

**Logged**: YYYY-MM-DDTHH:MM:SSZ
**Priority**: high
**Status**: pending

### Summary
What failed

### Error
Actual error message or output

### Context
- Command attempted
- Input or parameters
- Environment details

### Suggested Fix
What might resolve this

### Metadata
- Reproducible: yes | no | unknown
- Related Files: path/to/file

---
```

### Feature Request Entry

Append to `.learnings/FEATURE_REQUESTS.md`:

```markdown
## [FEAT-YYYYMMDD-NNN] capability_name

**Logged**: YYYY-MM-DDTHH:MM:SSZ
**Priority**: medium
**Status**: pending

### Requested Capability
What the user wanted

### User Context
Why they needed it

### Suggested Implementation
How this could be built

### Metadata
- Frequency: first_time | recurring

---
```

## ID Generation

Format: `TYPE-YYYYMMDD-NNN`
- TYPE: `LRN`, `ERR`, or `FEAT`
- YYYYMMDD: Current date
- NNN: Sequential number (check existing entries to find next)

## Resolving Entries

When an issue is fixed, update the entry:

1. Change `**Status**: pending` to `**Status**: resolved`
2. Add resolution:

```markdown
### Resolution
- **Resolved**: YYYY-MM-DDTHH:MM:SSZ
- **Notes**: What was done
```

## Promoting to Workspace Memory

When a learning is broadly applicable (not a one-off fix), promote it.

### When to Promote

- Learning applies across multiple tasks
- Knowledge any future session should know
- Prevents recurring mistakes
- Documented a project-specific convention
- The same pattern has appeared 3+ times

### Promotion Targets

| Target | What Belongs There |
|--------|-------------------|
| `memory/MEMORY.md` | Persistent facts, conventions, project knowledge |
| `SOUL.md` | Behavioral guidelines, communication style |
| `AGENT.md` | Workflow improvements, tool usage patterns |

### How to Promote

1. Distill the learning into a concise rule or fact
2. Append to the appropriate target file
3. Update original entry: set `**Status**: promoted` and add `**Promoted**: target_file`

## Recurring Pattern Detection

Before logging, check if a similar entry exists:

1. Read `.learnings/LEARNINGS.md` and `.learnings/ERRORS.md`
2. If similar entry exists, add `See Also: ID` in Metadata of both entries
3. Bump priority if issue keeps recurring
4. If 3+ similar entries exist, consider promotion

## Periodic Review (Heartbeat)

During heartbeat checks, review `.learnings/`:

1. Count pending items
2. Check for entries that should be promoted (high priority, recurring, resolved)
3. Promote entries that meet the threshold
4. Mark stale entries as `wont_fix` if no longer relevant

## Configuration Modes

The self-improvement system has two modes, set via config key `tools.self_improve.mode`:

### Mode: log (default)

- Capture all learnings, errors, and feature requests to .learnings/
- Do NOT auto-promote entries to workspace files
- Promotion only happens when explicitly requested by the user or operator
- Use this mode when getting started or when you want full human control

### Mode: promote

- Capture all learnings (same as log mode)
- Additionally, auto-promote entries that meet the promotion threshold:
  - Entry has 3+ related See Also links (recurrence threshold from config)
  - Entry is resolved or pending with high/critical priority
  - Pattern has been seen across 2+ distinct sessions
- Promote to: memory/MEMORY.md (facts), AGENT.md (workflows), SOUL.md (behavior)
- Write promoted rules as short prevention rules, not verbose incident reports

### Checking Current Mode

The current mode and threshold are shown in the system prompt under the
Self-Improvement section. You can also check via the CLI.

## Priority Guidelines

| Priority | When |
|----------|------|
| `critical` | Blocks core functionality, data loss risk |
| `high` | Significant impact, affects common workflows, recurring |
| `medium` | Moderate impact, workaround exists |
| `low` | Minor inconvenience, edge case |
