package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// LogToolError appends a structured error entry to .learnings/ERRORS.md
// when self-improvement is enabled. Called automatically by the agent loop
// when a tool returns IsError: true.
func LogToolError(workspace string, selfImprove *config.SelfImproveConfig, toolName string, errorMsg string) {
	if selfImprove == nil || !selfImprove.Enabled {
		return
	}

	logDir := selfImprove.LogDir
	if logDir == "" {
		logDir = ".learnings"
	}
	learningsDir := filepath.Join(workspace, logDir)

	if err := os.MkdirAll(learningsDir, 0755); err != nil {
		return
	}

	errFile := filepath.Join(learningsDir, "ERRORS.md")

	now := time.Now().UTC()
	dateStr := now.Format("20060102")
	nextSeq := nextErrorSequence(errFile, dateStr)
	entryID := fmt.Sprintf("ERR-%s-%03d", dateStr, nextSeq)

	summary := truncateSummary(toolName, errorMsg, 120)

	entry := fmt.Sprintf(`## [%s] %s

**Logged**: %s
**Priority**: high
**Status**: pending

### Summary
%s

### Error
%s

---
`, entryID, toolName, now.Format(time.RFC3339), summary, errorMsg)

	f, err := os.OpenFile(errFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	f.WriteString(entry)
}

// nextErrorSequence scans ERRORS.md for existing ERR-{date}-NNN IDs and
// returns the next sequence number.
func nextErrorSequence(filePath string, dateStr string) int {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 1
	}

	pattern := regexp.MustCompile(`ERR-` + dateStr + `-(\d{3})`)
	matches := pattern.FindAllStringSubmatch(string(data), -1)

	maxSeq := 0
	for _, m := range matches {
		if n, err := strconv.Atoi(m[1]); err == nil && n > maxSeq {
			maxSeq = n
		}
	}
	return maxSeq + 1
}

// truncateSummary builds a summary line, truncating the error if too long.
func truncateSummary(toolName string, errorMsg string, maxLen int) string {
	prefix := toolName + " tool failed: "
	msg := strings.TrimSpace(errorMsg)
	if msg == "" {
		return prefix + "(no error message)"
	}

	// Replace newlines with spaces for summary
	msg = strings.ReplaceAll(msg, "\n", " ")
	msg = strings.ReplaceAll(msg, "\r", "")

	full := prefix + msg
	if len(full) <= maxLen {
		return full
	}
	return full[:maxLen-3] + "..."
}
