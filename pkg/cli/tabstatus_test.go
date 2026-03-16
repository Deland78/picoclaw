package cli

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestTabStatus_New(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatusWithWriter(&buf, "myproject")
	ts.New()
	want := "\033]0;picoclaw - myproject:\U0001F7E2 new\007"
	if got := buf.String(); got != want {
		t.Errorf("New() wrote %q, want %q", got, want)
	}
}

func TestTabStatus_Running(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatusWithWriter(&buf, "myproject")
	ts.Running()
	want := "\033]0;picoclaw - myproject:\U0001F535 running...\007"
	if got := buf.String(); got != want {
		t.Errorf("Running() wrote %q, want %q", got, want)
	}
}

func TestTabStatus_DoneWithCommit(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatusWithWriter(&buf, "myproject")
	ts.DoneWithCommit()
	want := "\033]0;picoclaw - myproject:\u2705 done\007"
	if got := buf.String(); got != want {
		t.Errorf("DoneWithCommit() wrote %q, want %q", got, want)
	}
}

func TestTabStatus_DoneNoCommit(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatusWithWriter(&buf, "myproject")
	ts.DoneNoCommit()
	want := "\033]0;picoclaw - myproject:\U0001F6A7 done\007"
	if got := buf.String(); got != want {
		t.Errorf("DoneNoCommit() wrote %q, want %q", got, want)
	}
}

func TestTabStatus_Error(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatusWithWriter(&buf, "myproject")
	ts.Error()
	want := "\033]0;picoclaw - myproject:\U0001F6D1 error\007"
	if got := buf.String(); got != want {
		t.Errorf("Error() wrote %q, want %q", got, want)
	}
}

func TestTabStatus_Reset(t *testing.T) {
	var buf bytes.Buffer
	ts := NewTabStatusWithWriter(&buf, "myproject")
	ts.Reset()
	want := "\033]0;\007"
	if got := buf.String(); got != want {
		t.Errorf("Reset() wrote %q, want %q", got, want)
	}
}

func TestDetectProject_WithWorkspace(t *testing.T) {
	got := DetectProject("/home/user/projects/myapp")
	want := "myapp"
	if got != want {
		t.Errorf("DetectProject() = %q, want %q", got, want)
	}
}

func TestDetectProject_WithWindowsPath(t *testing.T) {
	got := DetectProject(`C:\Users\david\picoclaw`)
	want := "picoclaw"
	if got != want {
		t.Errorf("DetectProject() = %q, want %q", got, want)
	}
}

func TestDetectProject_Empty(t *testing.T) {
	got := DetectProject("")
	// Should return cwd basename, not empty
	if got == "" || got == "." {
		t.Errorf("DetectProject(\"\") = %q, want non-empty cwd basename", got)
	}
}

func TestDetectProject_TrailingSlash(t *testing.T) {
	got := DetectProject("/home/user/projects/myapp/")
	// filepath.Base handles trailing slash
	want := filepath.Base("/home/user/projects/myapp/")
	if got != want {
		t.Errorf("DetectProject() = %q, want %q", got, want)
	}
}

func TestGitHeadCommit_CurrentRepo(t *testing.T) {
	// This test runs inside the picoclaw git repo, so HEAD should return a hash
	hash := GitHeadCommit(".")
	if len(hash) < 7 {
		t.Errorf("GitHeadCommit(\".\") = %q, want a git hash", hash)
	}
}

func TestGitHeadCommit_InvalidDir(t *testing.T) {
	hash := GitHeadCommit(t.TempDir())
	if hash != "" {
		t.Errorf("GitHeadCommit(tempdir) = %q, want empty", hash)
	}
}
