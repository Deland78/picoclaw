# Plan: Mapping obra/superpowers into PicoClaw + Claude Code Tool Inventory

**Date:** 2026-03-01
**Author:** David (with Claude)
**Status:** Draft
**Confidence:** Moderate (>79%) — PicoClaw source access was indirect; some tool names inferred from docs/DeepWiki rather than reading Go source directly.

---

## Part 1: Tool Registry Comparison

### PicoClaw Native Tools

Source: `pkg/tools/`, README sandbox docs, DeepWiki "Tool System Architecture"

| PicoClaw Tool | Category | Description | Sandbox Restricted |
|---|---|---|---|
| `read_file` | File I/O | Read file contents | Yes — workspace only |
| `write_file` | File I/O | Write/create files | Yes — workspace only |
| `append_file` | File I/O | Append to existing file | Yes — workspace only |
| `edit_file` | File I/O | Edit existing file content | Yes — workspace only |
| `list_dir` | File I/O | List directory contents | Yes — workspace only |
| `exec` | Shell | Execute shell commands | Yes — workspace paths + deny patterns |
| `web_search` | Web | Search via Brave/Tavily/DuckDuckGo | No |
| `web_fetch` | Web | Fetch URL content | No |
| `message` | Communication | Send message to user via channel | No |
| `spawn` | Agent | Create async subagent with independent context | No |
| `cron` | Scheduling | Schedule one-time or recurring tasks | No |
| MCP tools | Extensible | Any tool exposed via MCP server connection | Depends on server |

### obra/superpowers Tool Dependencies

Source: SKILL.md files, README, release notes, blog posts

| Superpowers Tool Reference | Used By Skills | Purpose |
|---|---|---|
| `Read` (file) | All skills | Read SKILL.md files, plan docs, code |
| `Write` (file) | writing-plans, brainstorming, finishing-a-development-branch | Save plans, design docs |
| `Edit` (file) | executing-plans, TDD | Modify code and test files |
| `Bash` | TDD, systematic-debugging, using-git-worktrees | Run tests, git commands, builds |
| `TodoWrite` / `TodoRead` | executing-plans, writing-plans | Track task progress within a plan |
| `Task` (subagent dispatch) | subagent-driven-development, remembering-conversations | Fork independent subagent with custom prompt |
| `Skill` (skill invocation) | using-superpowers, brainstorming→writing-plans chain | Load and invoke another skill by name |
| `git` (via Bash) | using-git-worktrees, finishing-a-development-branch | Branch, worktree, merge, PR |
| `gh` (GitHub CLI via Bash) | finishing-a-development-branch | Create pull requests |

### Claude Code Native Tools (for developing PicoClaw enhancements)

When using Claude Code as your IDE agent to write Go code for PicoClaw:

| Claude Code Tool | Category | Description |
|---|---|---|
| `Read` | File I/O | Read any file (no sandbox) |
| `Write` | File I/O | Create/overwrite files |
| `Edit` | File I/O | Surgical string replacement in files |
| `MultiEdit` | File I/O | Multiple edits to same file in one call |
| `Bash` | Shell | Execute any shell command (git, go build, go test, make, etc.) |
| `TodoWrite` | Task tracking | Create/update structured task lists |
| `TodoRead` | Task tracking | Read current task state |
| `Task` | Subagent | Dispatch subagent with independent context window |
| `Skill` | Skill system | Invoke a registered skill by name |
| `WebSearch` | Web | Search the web |
| `WebFetch` | Web | Fetch URL content |
| `mcp__*` | MCP | Any connected MCP server tools |

---

## Part 2: Tool Mapping — Superpowers → PicoClaw

### Direct Mappings (Works As-Is)

These superpowers tool references map 1:1 to PicoClaw tools:

| Superpowers Reference | PicoClaw Equivalent | Notes |
|---|---|---|
| `Read` file | `read_file` | Identical function. Skills say "read its SKILL.md file using the read_file tool" — PicoClaw's context.go uses this exact phrasing. |
| `Write` file | `write_file` | Direct match. Plans save to `docs/plans/`. |
| `Edit` file | `edit_file` | Direct match. |
| `Bash` (general) | `exec` | Functional match but **sandbox restricted**. See adaptation notes below. |
| Web search | `web_search` | Direct match. |
| Web fetch | `web_fetch` | Direct match. |

### Adaptations Required

#### 1. `TodoWrite` / `TodoRead` → File-based workaround

**Problem:** PicoClaw has no native todo/task tracking tool. Superpowers' `executing-plans` skill relies heavily on `TodoWrite` to track which tasks in a plan are complete, in-progress, or blocked.

**Solution — Option A (Recommended):** Adapt skills to use file-based tracking.
- Create `~/.picoclaw/workspace/todos/` directory
- Skills write task state to `todos/current-plan.md` using `write_file`
- Skills read state from same file using `read_file`
- Add instructions to adapted SKILL.md: "Since TodoWrite is not available, maintain task state in `todos/current-plan.md` using this format: `- [x] Task 1 (complete)`, `- [ ] Task 2 (pending)`, `- [!] Task 3 (blocked: reason)`"

**Solution — Option B (Go enhancement):** Implement `todo_write` and `todo_read` tools in PicoClaw's `pkg/tools/`. This is a straightforward PR — two new tool handlers that read/write a JSON file in the workspace.

**Effort:** Option A = 30 min (skill text edits). Option B = 2-4 hours (Go implementation + tests).

#### 2. `Task` (subagent dispatch) → `spawn`

**Problem:** Superpowers' `subagent-driven-development` skill dispatches subagents using Claude Code's `Task` tool, which accepts a `description`, `prompt`, and optional `subagent_type`. PicoClaw's `spawn` tool creates async subagents but the interface may differ.

**Key differences to investigate:**
- Does `spawn` accept a custom system prompt / instructions?
- Does `spawn` return results to the parent agent, or only communicate via `message` tool to user?
- Can the parent agent wait for spawn results, or is it fire-and-forget only?

**From PicoClaw docs:** "Subagent works independently... Subagent uses 'message' tool... User receives result directly." This suggests fire-and-forget with user-directed output, which differs from Claude Code's `Task` which returns results to the calling agent.

**Solution — Adapt skills to PicoClaw's spawn model:**
- In `subagent-driven-development`, change "dispatch Task for each engineering task" to "spawn subagent for each task; subagent should write results to `workspace/results/task-N.md` and message user on completion"
- Parent agent checks `workspace/results/` for completion rather than receiving inline results
- This is architecturally different but functionally workable

**Effort:** Medium. Requires rewriting the core loop in `subagent-driven-development` and `executing-plans` skills. ~2-4 hours of skill adaptation.

#### 3. `Skill` (skill invocation) → `read_file` on SKILL.md

**Problem:** Claude Code's `Skill` tool loads and invokes a skill by name in one step. PicoClaw's context.go says: "To use a skill, read its SKILL.md file using the read_file tool."

**Solution:** This is already handled by PicoClaw's design. The skill text just needs to say `read_file` instead of "invoke skill." Superpowers' skill-chaining (brainstorming → writing-plans → executing-plans) becomes:
```
"Read the writing-plans SKILL.md using read_file and follow its instructions."
```
instead of:
```
"Invoke the superpowers:writing-plans skill."
```

**Effort:** Low. Find-and-replace across adapted SKILL.md files. ~30 min.

#### 4. `Bash` with git/gh → `exec` with sandbox considerations

**Problem:** Superpowers' `using-git-worktrees` and `finishing-a-development-branch` skills execute git commands freely. PicoClaw's `exec` is sandboxed to workspace by default, and git operations typically need to run in the project root (outside workspace).

**Solutions:**
- **Option A:** Set `restrict_to_workspace: false` for development use cases. Appropriate when PicoClaw is being used as a coding agent on a real codebase.
- **Option B:** Configure PicoClaw's workspace to be the project root (e.g., `"workspace": "/path/to/picoclaw-project"`).
- **Option C:** Skip git-worktree skills entirely. Not all Superpowers skills need to be ported — brainstorming, writing-plans, executing-plans, TDD, and systematic-debugging work without git integration.

**Recommendation:** Option C for initial port. Add git skills later if needed.

**Effort:** Zero for Option C. Option A/B = configuration change only.

---

## Part 3: Implementation Plan

### Phase 1: Core Skills Port (Day 1-2)

**Goal:** Get the brainstorm → plan → execute pipeline running in PicoClaw.

#### Step 1.1: Set up skill directories
```bash
mkdir -p ~/.picoclaw/workspace/skills/superpowers/
mkdir -p ~/.picoclaw/workspace/todos/
mkdir -p ~/.picoclaw/workspace/docs/plans/
```

#### Step 1.2: Copy and adapt core skills

Port these skills in order (each depends on the previous):

1. **using-superpowers** — Bootstrap skill. Adapt tool references.
2. **brainstorming** — Minimal changes needed (mostly uses read/write).
3. **writing-plans** — Replace `TodoWrite` with file-based tracking.
4. **executing-plans** — Replace `TodoWrite` + adapt `Task` → `spawn` or sequential execution.

**For each skill:**
- Copy SKILL.md from obra/superpowers repo
- Find-replace: `"invoke the ... skill"` → `"read the SKILL.md file at skills/superpowers/.../SKILL.md using read_file and follow its instructions"`
- Find-replace: `TodoWrite` → file-based tracking instructions
- Find-replace: `Task` subagent references → `spawn` or sequential execution
- Remove Claude Code-specific references (plugin marketplace, /commands, hooks.json)
- Test with a simple task

#### Step 1.3: Test the pipeline
```bash
picoclaw agent -m "I want to build a feature that adds a /weather command to PicoClaw. Let's brainstorm."
```

Expected behavior: PicoClaw reads brainstorming SKILL.md, asks clarifying questions, produces design doc, chains to writing-plans.

### Phase 2: Quality Skills (Day 3-4)

5. **test-driven-development** — Port as-is (uses `exec` for test runner, `read_file`/`write_file` for code). Adapt test commands from JavaScript/Python conventions to Go: `go test ./...` instead of `npm test`.
6. **systematic-debugging** — Minimal adaptation needed. Mostly methodology, not tool-dependent.
7. **verification-before-completion** — Minimal adaptation. Uses read_file to review code.

### Phase 3: Advanced Skills (Day 5+, optional)

8. **subagent-driven-development** — Requires spawn adaptation (see §2.2 above).
9. **requesting-code-review** / **receiving-code-review** — Port if doing PR-based workflow.
10. **remembering-conversations** (from obra/episodic-memory) — Requires MCP server setup alongside PicoClaw. Heavier lift.

### Phase 4: PicoClaw-Native Enhancements (using Claude Code)

These are Go-level enhancements to PicoClaw itself, developed using Claude Code with its full tool set:

| Enhancement | Claude Code Tools Used | Priority |
|---|---|---|
| Implement `todo_write` / `todo_read` tools in `pkg/tools/` | Bash (go build, go test), Edit, Write, Task (for TDD subagent) | High |
| Add skill-chaining support (skill can invoke another skill programmatically) | Read (existing skill loader code), Edit (loader.go), Bash (tests) | Medium |
| Add `Task`-style synchronous subagent (spawn that returns result to caller) | Read (spawn.go, loop.go), Edit, Bash (go test), Task (for review subagent) | Medium |
| Fix race condition in session history (Issue #704) | Read (session/manager.go, agent/loop.go), Edit, Bash (go test -race) | High |
| Add structured output / JSON mode for tool calls across providers | Read (providers/), Edit, Bash | Medium |

**Claude Code development workflow for each enhancement:**
1. Use `/superpowers:brainstorm` to design the feature
2. Use `/superpowers:write-plan` to create implementation plan
3. Use `/superpowers:execute-plan` with TDD — write Go tests first, then implementation
4. Use `systematic-debugging` if tests fail
5. Use `verification-before-completion` before committing
6. Use `finishing-a-development-branch` to create PR to sipeed/picoclaw

---

## Part 4: Tool Availability Summary

### Matrix: What's available where

| Tool Capability | Claude Code (developing PicoClaw) | PicoClaw (running skills) | Adaptation Needed |
|---|---|---|---|
| Read files | ✅ `Read` | ✅ `read_file` | Name only |
| Write files | ✅ `Write` | ✅ `write_file` | Name only |
| Edit files | ✅ `Edit` / `MultiEdit` | ✅ `edit_file` | Name only |
| Append to files | ✅ via `Edit` | ✅ `append_file` | PicoClaw has dedicated tool |
| List directories | ✅ via `Bash` (`ls`) | ✅ `list_dir` | — |
| Shell execution | ✅ `Bash` (unrestricted) | ⚠️ `exec` (sandboxed) | Sandbox config |
| Web search | ✅ `WebSearch` | ✅ `web_search` | Name only |
| Web fetch | ✅ `WebFetch` | ✅ `web_fetch` | Name only |
| Task tracking (todos) | ✅ `TodoWrite` / `TodoRead` | ❌ Not native | File-based workaround or Go PR |
| Subagent (sync, returns result) | ✅ `Task` | ❌ `spawn` is async/fire-and-forget | Architectural adaptation |
| Subagent (async) | ✅ `Task` | ✅ `spawn` | Direct match |
| User messaging | ✅ (implicit in conversation) | ✅ `message` | — |
| Scheduling | ❌ | ✅ `cron` + `HEARTBEAT.md` | PicoClaw advantage |
| Skill invocation | ✅ `Skill` tool (by name) | ⚠️ `read_file` on SKILL.md | Phrasing change |
| Git operations | ✅ `Bash` (`git ...`) | ⚠️ `exec` (sandbox) | Config or skip |
| MCP servers | ✅ `mcp__*` tools | ✅ MCP client in `pkg/mcp/` | Server must run alongside |
| Memory (long-term) | ✅ via episodic-memory plugin | ⚠️ `MEMORY.md` (manual, not semantic) | Gap — episodic-memory MCP |
| Voice transcription | ❌ | ✅ Groq Whisper integration | PicoClaw advantage |
| Multi-channel I/O | ❌ (terminal only) | ✅ Telegram, Discord, QQ, LINE, etc. | PicoClaw advantage |

### Key Gaps to Close

1. **Todo tracking** — Highest priority. File-based workaround is quick; native Go tool is better long-term.
2. **Synchronous subagent** — Important for subagent-driven-development. Without it, the reviewing/inspection loop breaks. Consider this a Phase 4 Go enhancement.
3. **Semantic memory** — PicoClaw's `MEMORY.md` is a flat file. obra/episodic-memory provides vector-indexed semantic search. Running the episodic-memory MCP server alongside PicoClaw would close this gap, but adds RAM overhead beyond the 10MB target. Appropriate for beefier hardware (Raspberry Pi 4+, old Android phone, etc.).

---

## Appendix: openskills as Bridge Layer

The [numman-ali/openskills](https://github.com/numman-ali/openskills) project generates the same `<available_skills>` XML block that Claude Code uses, and writes it to AGENTS.md. PicoClaw loads AGENTS.md on every message. This could provide automatic skill discovery without manually copying SKILL.md files.

**Investigation needed:**
- Does PicoClaw's context builder parse `<available_skills>` XML in AGENTS.md?
- Or does it just inject AGENTS.md as raw text into the system prompt?
- If raw text injection, the LLM would see the skill list and could follow the read_file instructions — this may "just work."

**Recommendation:** Try `npx openskills install obra/superpowers` with output directed to PicoClaw's workspace, then test if the LLM picks up the skill references from AGENTS.md.
