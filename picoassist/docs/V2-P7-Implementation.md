# V2 Phase 7 — Documentation

> **References:**
> - `docs/V2-Implementation-Plan.md` — all phases
> - `docs/roadmap.md` — version scope and architecture
> - All V2 phase plans (`V2-P1` through `V2-P6`)
> - `AGENTS.md` — shared agent context (needs V2 updates)
> - `README.md` — user-facing docs (needs PicoClaw section)
> - `docs/CLIENT_CONFIG_TEMPLATE.md` — config schema (needs policy.yaml reference)

## Goal

Update all project documentation to reflect V2 changes: PicoClaw as primary UI,
policy engine, approval workflow, action log, and updated repo structure.

## Dependencies

All V2 phases (P1–P6) should be complete before finalizing docs.

---

## Tasks

### 7.1 Update `AGENTS.md`

- [ ] Update "What this project is" to mention PicoClaw as the user interface
- [ ] Update repository structure to include V2 additions:
  - `config/` package
  - `services/action_log/`
  - `policy.yaml`
  - `picoclaw/` directory (skill, heartbeat, setup)
- [ ] Update "Build and run commands" to include:
  - `pwsh scripts/start_services.ps1` — start both FastAPI services
  - PicoClaw setup and skill installation commands
  - `picoclaw agent -m "..."` as the primary interaction method
- [ ] Add MCP server port placeholder (port 8003, V3)
- [ ] Update safety rules section to reference `policy.yaml`
- [ ] Update V2 roadmap section to mark completed items, add V3 items

### 7.2 Update `README.md`

- [ ] Add "Architecture" section explaining PicoClaw + PicoAssist relationship
- [ ] Add "PicoClaw Setup" section:
  1. Install PicoClaw
  2. Copy skill to workspace
  3. Configure heartbeat
  4. Start PicoAssist services
  5. Test with `picoclaw agent -m "check health"`
- [ ] Update "First Run" to mention PicoClaw as the recommended interface
- [ ] Keep standalone `python digest_runner.py` instructions as fallback
- [ ] Add `policy.yaml` configuration section
- [ ] Update "v1 Scope" section to "Current Scope (v2)"

### 7.3 Update `docs/CLIENT_CONFIG_TEMPLATE.md`

- [ ] Note that safety settings have moved to `policy.yaml` (P3)
- [ ] Keep `defaults.safety` in client_config for backward compatibility reference
- [ ] Document how `policy.yaml` client overrides interact with `client_config.yaml`

### 7.4 Update `.env.example`

- [ ] Add any new environment variables introduced in V2
- [ ] Document which vars are needed for PicoClaw integration

### 7.5 Verify all doc links are valid

- [ ] Check all internal doc references (`docs/*.md`) resolve to existing files
- [ ] Check all external URLs are still valid (Graph API, Playwright, PicoClaw)

### Run tests and verify

- [ ] Lint markdown for broken links (manual check or tool)
- [ ] All tests pass: `pytest -v`
- [ ] Full lint: `ruff check .`

---

## Verify — Phase 7

```bash
# README has PicoClaw section
python -c "
content = open('README.md').read()
assert 'PicoClaw' in content, 'README missing PicoClaw section'
print('PASS')
"

# AGENTS.md has updated structure
python -c "
content = open('AGENTS.md').read()
assert 'policy.yaml' in content, 'AGENTS.md missing policy.yaml'
assert 'picoclaw' in content.lower(), 'AGENTS.md missing PicoClaw'
print('PASS')
"

# Final full verification
pytest -v
ruff check .
ruff format --check .

# All V2 imports
python -c "
from config import load_config
from config.policy import PolicyEngine
from services.action_log import ActionLogDB
print('ALL V2 IMPORTS PASS')
"
```
