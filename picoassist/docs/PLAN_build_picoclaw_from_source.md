# Plan: Build PicoClaw from Source

## Context

We're running PicoClaw as a native Go binary on Windows. Currently `picoclaw/setup.md` says "download the binary." We want to build from source instead, so we can modify PicoClaw's behavior and stay on the latest code. The source lives as a separate clone at `C:\Users\david\picoclaw` (sibling to the PicoAssist repo).

## Tasks

- [x] **Task 1**: Clone PicoClaw source to `C:\Users\david\picoclaw` — already exists
- [ ] **Task 2**: Build the binary (`make deps && make build`) — **BLOCKED: Go and make not installed**
- [ ] **Task 3**: Place `picoclaw.exe` on PATH and verify with `picoclaw --version`
- [ ] **Task 4**: Update `picoclaw/setup.md` — rewrite Steps 1-2 for clone+build workflow
- [ ] **Task 5**: Update `docs/LESSONS_LEARNED.md` — add build-from-source lesson
- [ ] **Task 6**: Verify (`pytest -v`, `ruff check .`, setup.md accuracy)

## Details

### Task 1 — Clone PicoClaw source
```bash
cd C:\Users\david
git clone https://github.com/sipeed/picoclaw.git
```
Independent sibling repo, not a submodule.

### Task 2 — Build the binary
Requires Go 1.21+ and `make` (Git Bash or MSYS2).
```bash
cd C:\Users\david\picoclaw
make deps
make build
```

### Task 3 — Place on PATH
Either `make install` or copy `picoclaw.exe` to `C:\Users\david\bin\`.

### Task 4 — Update `picoclaw/setup.md`
- Add prerequisite: Go 1.21+, `make`
- Step 1: clone sipeed/picoclaw to `C:\Users\david\picoclaw`
- Step 2: `make deps && make build`, copy to PATH
- Add update workflow: `git pull && make build`
- Keep remaining steps unchanged (config, skill, services, test)

### Task 5 — Update `docs/LESSONS_LEARNED.md`
Add lesson: Go build on Windows requires `make` via Git Bash or MSYS2. Always rebuild after `git pull`.

### Task 6 — Verify
- `picoclaw --version`
- `pytest -v` passes
- `ruff check .`
- setup.md matches actual workflow
