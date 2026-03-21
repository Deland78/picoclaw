package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildLearningsStatus returns self-improvement instructions and status for
// the system prompt. Returns empty string only if self-improvement is disabled.
// When enabled, it ALWAYS returns write-side instructions so the agent knows
// to log entries — plus a status summary when entries already exist.
func (cb *ContextBuilder) buildLearningsStatus() string {
	if cb.selfImprove == nil || !cb.selfImprove.Enabled {
		return ""
	}

	logDir := cb.selfImprove.LogDir
	if logDir == "" {
		logDir = ".learnings"
	}
	learningsDir := filepath.Join(cb.workspace, logDir)

	var result strings.Builder

	// Always emit config info
	result.WriteString(fmt.Sprintf("Self-improvement mode: **%s** (promotion threshold: %d recurrences)\n\n",
		cb.selfImprove.Mode, cb.selfImprove.PromotionThreshold))
	result.WriteString(fmt.Sprintf("Learnings directory: `%s/`\n\n", logDir))

	// Emit status summary if entries exist
	if _, err := os.Stat(learningsDir); err == nil {
		statusSummary := cb.buildEntryStatusSummary(learningsDir)
		if statusSummary != "" {
			result.WriteString(statusSummary)
			result.WriteString("\n")
		}
	}

	// Always emit write-side instructions so the agent actively logs
	result.WriteString(cb.buildWriteInstructions(logDir))

	return result.String()
}

// buildEntryStatusSummary scans .learnings/ files and returns a summary of
// pending/resolved/promoted counts. Returns "" if no entries found.
func (cb *ContextBuilder) buildEntryStatusSummary(learningsDir string) string {
	files := []struct {
		name  string
		label string
	}{
		{"LEARNINGS.md", "Learnings"},
		{"ERRORS.md", "Errors"},
		{"FEATURE_REQUESTS.md", "Feature Requests"},
	}

	var sb strings.Builder
	totalPending := 0
	hasContent := false

	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(learningsDir, f.name))
		if err != nil {
			continue
		}
		content := string(data)

		pending := strings.Count(content, "**Status**: pending")
		resolved := strings.Count(content, "**Status**: resolved")
		promoted := strings.Count(content, "**Status**: promoted")

		total := pending + resolved + promoted
		if total == 0 {
			continue
		}

		hasContent = true
		totalPending += pending
		fmt.Fprintf(&sb, "- **%s**: %d pending, %d resolved, %d promoted\n", f.label, pending, resolved, promoted)
	}

	if !hasContent {
		return ""
	}

	status := sb.String()
	if totalPending > 0 {
		status += fmt.Sprintf("\n%d pending entries to review. ", totalPending)
		if cb.selfImprove.Mode == "promote" {
			status += "Auto-promotion is enabled for entries meeting the recurrence threshold."
		} else {
			status += "Mode is `log` — log only, no auto-promotion."
		}
	}

	return status
}

// buildWriteInstructions returns the system prompt text that tells the agent
// when and how to log entries to .learnings/ files. This is the critical
// "write-side" that was previously missing.
func (cb *ContextBuilder) buildWriteInstructions(logDir string) string {
	return fmt.Sprintf(`### Active Logging Instructions

You MUST actively log to %s/ files during conversations. Do NOT ask permission — log immediately when triggers are detected.

**When to log:**
- **Errors** → append to %s/ERRORS.md when a command, tool, or operation fails unexpectedly
- **Corrections** → append to %s/LEARNINGS.md (category: correction) when the user corrects you
- **Knowledge gaps** → append to %s/LEARNINGS.md (category: knowledge_gap) when your knowledge was wrong or outdated
- **Best practices** → append to %s/LEARNINGS.md (category: best_practice) when a better approach is discovered
- **Feature requests** → append to %s/FEATURE_REQUESTS.md when the user wants a capability that doesn't exist

**Detection triggers:**
- User says "No, that's wrong", "Actually, it should be...", "That's outdated" → correction
- Non-zero exit codes, exceptions, stack traces, timeouts → error
- "Can you also...", "I wish you could...", "Why can't you..." → feature request

**How to log:** Read skills/self-improvement/SKILL.md for the detailed entry format (ID generation, fields, metadata). Use write_file or the appropriate file tool to append entries.

**Before major tasks:** Read %s/ to check for relevant past learnings that might help.`, logDir, logDir, logDir, logDir, logDir, logDir, logDir)
}
