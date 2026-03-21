package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestBuildLearningsStatus_Disabled(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir)
	// selfImprove is nil by default
	result := cb.buildLearningsStatus()
	if result != "" {
		t.Fatalf("expected empty string when disabled, got %q", result)
	}

	// Explicitly disabled
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{Enabled: false})
	result = cb.buildLearningsStatus()
	if result != "" {
		t.Fatalf("expected empty string when explicitly disabled, got %q", result)
	}
}

func TestBuildLearningsStatus_EnabledNoEntries(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled:            true,
		Mode:               "log",
		LogDir:             ".learnings",
		PromotionThreshold: 3,
	})

	result := cb.buildLearningsStatus()

	// Should return instructions even with no .learnings/ directory
	if result == "" {
		t.Fatal("expected non-empty result when enabled, got empty string")
	}

	// Must contain write-side instructions
	if !strings.Contains(result, "Active Logging Instructions") {
		t.Error("expected write instructions, not found")
	}
	if !strings.Contains(result, "ERRORS.md") {
		t.Error("expected ERRORS.md reference in instructions")
	}
	if !strings.Contains(result, "LEARNINGS.md") {
		t.Error("expected LEARNINGS.md reference in instructions")
	}
	if !strings.Contains(result, "FEATURE_REQUESTS.md") {
		t.Error("expected FEATURE_REQUESTS.md reference in instructions")
	}
	if !strings.Contains(result, "correction") {
		t.Error("expected detection trigger 'correction' in instructions")
	}

	// Must contain mode info
	if !strings.Contains(result, "log") {
		t.Error("expected mode 'log' in output")
	}

	// Should NOT contain status counts (no entries exist)
	if strings.Contains(result, "pending, ") {
		t.Error("should not contain entry counts when no entries exist")
	}
}

func TestBuildLearningsStatus_EnabledWithEntries(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Write a LEARNINGS.md with some entries
	learningsContent := `# Learnings

## [LRN-20260316-001] correction

**Logged**: 2026-03-16T10:00:00Z
**Priority**: medium
**Status**: pending

### Summary
Test learning entry

---

## [LRN-20260316-002] best_practice

**Logged**: 2026-03-16T11:00:00Z
**Priority**: low
**Status**: resolved

### Summary
Another entry

---
`
	if err := os.WriteFile(filepath.Join(learningsDir, "LEARNINGS.md"), []byte(learningsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Write an ERRORS.md with an entry
	errorsContent := `# Errors

## [ERR-20260316-001] exec

**Logged**: 2026-03-16T12:00:00Z
**Priority**: high
**Status**: pending

### Summary
Command failed

---
`
	if err := os.WriteFile(filepath.Join(learningsDir, "ERRORS.md"), []byte(errorsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled:            true,
		Mode:               "log",
		LogDir:             ".learnings",
		PromotionThreshold: 3,
	})

	result := cb.buildLearningsStatus()

	// Must contain both instructions AND status
	if !strings.Contains(result, "Active Logging Instructions") {
		t.Error("expected write instructions, not found")
	}

	// Must contain entry counts
	if !strings.Contains(result, "1 pending") {
		t.Error("expected '1 pending' in Learnings status")
	}
	if !strings.Contains(result, "1 resolved") {
		t.Error("expected '1 resolved' in Learnings status")
	}
	if !strings.Contains(result, "2 pending entries to review") {
		t.Error("expected '2 pending entries to review' summary")
	}

	// Mode should be mentioned
	if !strings.Contains(result, "log only, no auto-promotion") {
		t.Error("expected log mode description")
	}
}

func TestBuildLearningsStatus_PromoteMode(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `# Learnings

## [LRN-20260316-001] correction

**Status**: pending

---
`
	if err := os.WriteFile(filepath.Join(learningsDir, "LEARNINGS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled:            true,
		Mode:               "promote",
		LogDir:             ".learnings",
		PromotionThreshold: 3,
	})

	result := cb.buildLearningsStatus()

	if !strings.Contains(result, "promote") {
		t.Error("expected 'promote' mode in output")
	}
	if !strings.Contains(result, "Auto-promotion is enabled") {
		t.Error("expected auto-promotion message for promote mode")
	}
}

func TestBuildLearningsStatus_CustomLogDir(t *testing.T) {
	dir := t.TempDir()
	customDir := filepath.Join(dir, "my-logs")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `# Learnings

## [LRN-001] correction

**Status**: pending

---
`
	if err := os.WriteFile(filepath.Join(customDir, "LEARNINGS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled:            true,
		Mode:               "log",
		LogDir:             "my-logs",
		PromotionThreshold: 5,
	})

	result := cb.buildLearningsStatus()

	// Config should show the custom dir
	if !strings.Contains(result, "`my-logs/`") {
		t.Error("expected custom log dir 'my-logs/' in output")
	}
	// Write instructions should reference the custom dir
	if !strings.Contains(result, "my-logs/ERRORS.md") {
		t.Error("expected custom dir in write instructions file references")
	}
	// Status summary should still work
	if !strings.Contains(result, "1 pending") {
		t.Error("expected entry counts with custom log dir")
	}
}

func TestBuildLearningsStatus_DefaultLogDir(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled:            true,
		Mode:               "log",
		LogDir:             "", // empty → should default to ".learnings"
		PromotionThreshold: 3,
	})

	result := cb.buildLearningsStatus()

	if !strings.Contains(result, "`.learnings/`") {
		t.Error("expected default log dir '.learnings/' when LogDir is empty")
	}
}

func TestBuildLearningsStatus_DirExistsButHeadersOnly(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// File exists with only a header — no status entries
	if err := os.WriteFile(filepath.Join(learningsDir, "LEARNINGS.md"), []byte("# Learnings\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled:            true,
		Mode:               "log",
		LogDir:             ".learnings",
		PromotionThreshold: 3,
	})

	result := cb.buildLearningsStatus()

	// Should still return instructions (the write-side fix)
	if result == "" {
		t.Fatal("expected non-empty result when dir exists but has no entries")
	}
	if !strings.Contains(result, "Active Logging Instructions") {
		t.Error("expected write instructions even with header-only files")
	}
	// Should NOT have entry counts
	if strings.Contains(result, "pending entries to review") {
		t.Error("should not mention pending entries when file has only headers")
	}
}

func TestBuildEntryStatusSummary_AllStatuses(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// LEARNINGS.md with all three statuses
	learnings := `# Learnings

## [LRN-001]
**Status**: pending
---
## [LRN-002]
**Status**: pending
---
## [LRN-003]
**Status**: resolved
---
## [LRN-004]
**Status**: promoted
---
`
	// FEATURE_REQUESTS.md with one entry
	features := `# Feature Requests

## [FR-001]
**Status**: pending
---
`
	if err := os.WriteFile(filepath.Join(learningsDir, "LEARNINGS.md"), []byte(learnings), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(learningsDir, "FEATURE_REQUESTS.md"), []byte(features), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled: true,
		Mode:    "log",
	})

	summary := cb.buildEntryStatusSummary(learningsDir)

	if !strings.Contains(summary, "**Learnings**: 2 pending, 1 resolved, 1 promoted") {
		t.Errorf("unexpected learnings line in summary: %s", summary)
	}
	if !strings.Contains(summary, "**Feature Requests**: 1 pending, 0 resolved, 0 promoted") {
		t.Errorf("unexpected feature requests line in summary: %s", summary)
	}
	// 3 total pending (2 learnings + 1 feature request)
	if !strings.Contains(summary, "3 pending entries to review") {
		t.Errorf("expected '3 pending entries to review', got: %s", summary)
	}
}

func TestBuildEntryStatusSummary_NoPending(t *testing.T) {
	dir := t.TempDir()
	learningsDir := filepath.Join(dir, ".learnings")
	if err := os.MkdirAll(learningsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := `# Learnings

## [LRN-001]
**Status**: resolved
---
## [LRN-002]
**Status**: promoted
---
`
	if err := os.WriteFile(filepath.Join(learningsDir, "LEARNINGS.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cb := NewContextBuilder(dir)
	cb.SetSelfImproveConfig(&config.SelfImproveConfig{
		Enabled: true,
		Mode:    "promote",
	})

	summary := cb.buildEntryStatusSummary(learningsDir)

	// Should have counts but no "pending entries to review" message
	if !strings.Contains(summary, "0 pending") {
		t.Error("expected '0 pending' in summary")
	}
	if strings.Contains(summary, "pending entries to review") {
		t.Error("should not mention 'pending entries to review' when totalPending is 0")
	}
}

func TestBuildWriteInstructions(t *testing.T) {
	dir := t.TempDir()
	cb := NewContextBuilder(dir)

	instructions := cb.buildWriteInstructions(".learnings")

	// Check all file references appear
	for _, file := range []string{"ERRORS.md", "LEARNINGS.md", "FEATURE_REQUESTS.md"} {
		if !strings.Contains(instructions, file) {
			t.Errorf("expected %s reference in write instructions", file)
		}
	}

	// Check all categories appear
	for _, cat := range []string{"correction", "knowledge_gap", "best_practice"} {
		if !strings.Contains(instructions, cat) {
			t.Errorf("expected category %q in write instructions", cat)
		}
	}

	// Check skill reference
	if !strings.Contains(instructions, "self-improvement/SKILL.md") {
		t.Error("expected skill reference in write instructions")
	}
}
