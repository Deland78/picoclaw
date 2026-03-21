package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestLogToolError_Disabled(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.SelfImproveConfig{Enabled: false}

	LogToolError(dir, cfg, "exec", "some error")

	errFile := filepath.Join(dir, ".learnings", "ERRORS.md")
	if _, err := os.Stat(errFile); err == nil {
		t.Fatal("expected no file created when disabled")
	}
}

func TestLogToolError_NilConfig(t *testing.T) {
	dir := t.TempDir()

	LogToolError(dir, nil, "exec", "some error")

	errFile := filepath.Join(dir, ".learnings", "ERRORS.md")
	if _, err := os.Stat(errFile); err == nil {
		t.Fatal("expected no file created when config is nil")
	}
}

func TestLogToolError_CreatesFirstEntry(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.SelfImproveConfig{Enabled: true}

	LogToolError(dir, cfg, "exec", "command not found: foobar")

	errFile := filepath.Join(dir, ".learnings", "ERRORS.md")
	data, err := os.ReadFile(errFile)
	if err != nil {
		t.Fatalf("expected ERRORS.md to be created: %v", err)
	}
	content := string(data)

	today := time.Now().UTC().Format("20060102")
	expectedID := "ERR-" + today + "-001"

	if !strings.Contains(content, "## ["+expectedID+"] exec") {
		t.Errorf("expected entry header with ID %s, got:\n%s", expectedID, content)
	}
	if !strings.Contains(content, "**Priority**: high") {
		t.Error("expected Priority: high")
	}
	if !strings.Contains(content, "**Status**: pending") {
		t.Error("expected Status: pending")
	}
	if !strings.Contains(content, "command not found: foobar") {
		t.Error("expected full error message in Error section")
	}
	if !strings.Contains(content, "---") {
		t.Error("expected entry separator ---")
	}
}

func TestLogToolError_SequentialIDs(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.SelfImproveConfig{Enabled: true}

	LogToolError(dir, cfg, "exec", "error one")
	LogToolError(dir, cfg, "web_fetch", "error two")
	LogToolError(dir, cfg, "exec", "error three")

	errFile := filepath.Join(dir, ".learnings", "ERRORS.md")
	data, err := os.ReadFile(errFile)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	today := time.Now().UTC().Format("20060102")

	if !strings.Contains(content, "ERR-"+today+"-001") {
		t.Error("expected first entry ID -001")
	}
	if !strings.Contains(content, "ERR-"+today+"-002") {
		t.Error("expected second entry ID -002")
	}
	if !strings.Contains(content, "ERR-"+today+"-003") {
		t.Error("expected third entry ID -003")
	}
}

func TestLogToolError_CustomLogDir(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.SelfImproveConfig{Enabled: true, LogDir: "custom-logs"}

	LogToolError(dir, cfg, "exec", "some error")

	// Should NOT be in default .learnings
	defaultFile := filepath.Join(dir, ".learnings", "ERRORS.md")
	if _, err := os.Stat(defaultFile); err == nil {
		t.Fatal("should not create in default .learnings when LogDir is set")
	}

	// Should be in custom dir
	customFile := filepath.Join(dir, "custom-logs", "ERRORS.md")
	data, err := os.ReadFile(customFile)
	if err != nil {
		t.Fatalf("expected file in custom-logs: %v", err)
	}
	if !strings.Contains(string(data), "exec") {
		t.Error("expected entry in custom log dir")
	}
}

func TestLogToolError_TruncatedSummary(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.SelfImproveConfig{Enabled: true}

	longError := strings.Repeat("x", 300)
	LogToolError(dir, cfg, "exec", longError)

	data, err := os.ReadFile(filepath.Join(dir, ".learnings", "ERRORS.md"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Summary should be truncated
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "exec tool failed:") {
			if len(line) > 160 {
				t.Errorf("summary line too long (%d chars): %s", len(line), line[:80]+"...")
			}
			if !strings.HasSuffix(line, "...") {
				t.Error("truncated summary should end with ...")
			}
			break
		}
	}

	// Full error should still be present
	if !strings.Contains(content, longError) {
		t.Error("full error should be in Error section")
	}
}

func TestLogToolError_EmptyErrorMsg(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.SelfImproveConfig{Enabled: true}

	LogToolError(dir, cfg, "exec", "")

	errFile := filepath.Join(dir, ".learnings", "ERRORS.md")
	data, err := os.ReadFile(errFile)
	if err != nil {
		t.Fatalf("expected file created even with empty error: %v", err)
	}
	if !strings.Contains(string(data), "exec tool failed") {
		t.Error("expected entry even with empty error message")
	}
}
