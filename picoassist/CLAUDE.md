# Claude Code Instructions

> Read `AGENTS.md` first — it contains the full project context, repo structure,
> tech stack, conventions, and safety rules shared across all coding agents.

## Claude Code-specific guidance

### Before implementing anything
1. Read `AGENTS.md` for project context and structure
2. Read `docs/V2-Implementation-Plan.md` for the phased build plan
3. Check which phase you're working on — follow the phase order, don't skip ahead
4. Run the **Verify** block at the end of each phase before moving to the next

### Implementation preferences
- Before starting any new phase (multi-file changes), plan your approach first
- Prefer editing existing files over creating new ones
- Run `pytest` after writing each module to catch issues early
- Use `ruff check` and `ruff format` before considering a phase complete

### File creation order within each phase
When building a service module, create files in this order:
1. `models.py` (Pydantic schemas — defines the contract)
2. Core logic module (e.g., `graph_client.py`, `playwright_runner.py`)
3. `__init__.py` (package exports)
4. `app.py` (FastAPI wrapper — wires models to logic)
5. Verify versions match `pyproject.toml` dependencies
6. `tests/test_*.py` (at least one happy-path, one error-path)

### Key constraints
- **Jira/ADO are read-only** — do not implement write actions (comments, transitions, field updates)
- **Email allows triage**: list, summarize, move, draft. Do NOT implement send or delete.
- Workers must be **importable modules** (`from services.mail_worker import GraphMailClient`)
  AND optionally runnable as HTTP APIs (`python -m services.mail_worker.app`)
- PicoClaw is a **native Go binary** on Windows — see `docs/picoclaw-setup.md`
- Policy engine and approval workflow exist — see `config/policy.py` and `services/approval_engine/`
- Services run as native Python on the Windows host (no Docker)
- Don't commit `.env`, `data/tokens/`, or `profiles/` under any circumstances

### What not to do
- Don't create CONTRIBUTING.md, TROUBLESHOOTING.md, or extra docs unless asked
- Don't refactor existing docs structure — only edit content within existing files
- Don't add features beyond what the current phase specifies
