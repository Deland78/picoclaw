package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildLearningsStatus reads .learnings/ files and returns a brief status
// summary for injection into the system prompt. Returns empty string if
// self-improvement is disabled or no learnings exist.
func (cb *ContextBuilder) buildLearningsStatus() string {
	if cb.selfImprove == nil || !cb.selfImprove.Enabled {
		return ""
	}

	logDir := cb.selfImprove.LogDir
	if logDir == "" {
		logDir = ".learnings"
	}
	learningsDir := filepath.Join(cb.workspace, logDir)

	// Check if directory exists
	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		return ""
	}

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

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Self-improvement mode: **%s** (promotion threshold: %d recurrences)\n\n",
		cb.selfImprove.Mode, cb.selfImprove.PromotionThreshold))
	result.WriteString(fmt.Sprintf("Learnings directory: `%s/`\n\n", logDir))
	result.WriteString(sb.String())

	if totalPending > 0 {
		result.WriteString(fmt.Sprintf("\n%d pending entries to review. ", totalPending))
		if cb.selfImprove.Mode == "promote" {
			result.WriteString("Auto-promotion is enabled for entries meeting the recurrence threshold.")
		} else {
			result.WriteString("Mode is `log` — log only, no auto-promotion.")
		}
	}

	return result.String()
}
