package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// TabStatus updates the terminal tab title to reflect the agent's current state.
// Uses ANSI OSC escape sequences supported by Windows Terminal, iTerm2, etc.
type TabStatus struct {
	w       io.Writer
	project string
	mu      sync.Mutex
}

// NewTabStatus creates a TabStatus that writes to os.Stderr.
func NewTabStatus(project string) *TabStatus {
	return &TabStatus{w: os.Stderr, project: project}
}

// NewTabStatusWithWriter creates a TabStatus with a custom writer (for testing).
func NewTabStatusWithWriter(w io.Writer, project string) *TabStatus {
	return &TabStatus{w: w, project: project}
}

func (t *TabStatus) setTitle(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.w, "\033]0;picoclaw - %s:%s\007", t.project, status)
}

// New sets the tab title to indicate a new session.
func (t *TabStatus) New() {
	t.setTitle("\U0001F7E2 new")
}

// Running sets the tab title to indicate the agent is processing.
func (t *TabStatus) Running() {
	t.setTitle("\U0001F535 running...")
}

// DoneWithCommit sets the tab title to indicate success with a git commit.
func (t *TabStatus) DoneWithCommit() {
	t.setTitle("\u2705 done")
}

// DoneNoCommit sets the tab title to indicate success without a git commit.
func (t *TabStatus) DoneNoCommit() {
	t.setTitle("\U0001F6A7 done")
}

// Error sets the tab title to indicate an error occurred.
func (t *TabStatus) Error() {
	t.setTitle("\U0001F6D1 error")
}

// Reset clears the terminal tab title back to the default.
func (t *TabStatus) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintf(t.w, "\033]0;\007")
}

// DetectProject returns a short project name from the workspace path.
// Falls back to the current working directory basename.
func DetectProject(workspace string) string {
	if workspace != "" {
		return filepath.Base(workspace)
	}
	if cwd, err := os.Getwd(); err == nil {
		return filepath.Base(cwd)
	}
	return "unknown"
}

// GitHeadCommit returns the current HEAD commit hash in the given directory,
// or an empty string if git is unavailable or the directory is not a repo.
func GitHeadCommit(dir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
