// PicoClaw - Self-improvement CLI commands
// License: MIT

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func improveCmd() {
	if len(os.Args) < 3 {
		improveHelp()
		return
	}

	subcommand := os.Args[2]

	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	workspace := cfg.WorkspacePath()
	logDir := cfg.Tools.SelfImprove.LogDir
	if logDir == "" {
		logDir = ".learnings"
	}
	learningsDir := filepath.Join(workspace, logDir)

	switch subcommand {
	case "status":
		improveStatusCmd(learningsDir, cfg.Tools.SelfImprove.Mode, cfg.Tools.SelfImprove.PromotionThreshold)
	case "show":
		if len(os.Args) < 4 {
			fmt.Println("Usage: picoclaw improve show <entry-id>")
			return
		}
		improveShowCmd(learningsDir, os.Args[3])
	default:
		fmt.Printf("Unknown improve command: %s\n", subcommand)
		improveHelp()
	}
}

func improveHelp() {
	fmt.Println("\nSelf-improvement commands:")
	fmt.Println("  status            Show learnings status (pending, resolved, promoted)")
	fmt.Println("  show <id>         Show a specific learning entry by ID")
}

func improveStatusCmd(learningsDir string, mode string, threshold int) {
	fmt.Printf("\n\U0001f9e0 Self-Improvement Status\n\n")
	fmt.Printf("  Mode: %s\n", mode)
	fmt.Printf("  Promotion threshold: %d recurrences\n", threshold)
	fmt.Printf("  Learnings directory: %s\n\n", learningsDir)

	if _, err := os.Stat(learningsDir); os.IsNotExist(err) {
		fmt.Println("  No .learnings/ directory found. The agent will create entries as it learns.")
		return
	}

	files := []struct {
		name  string
		label string
		tag   string
	}{
		{"LEARNINGS.md", "Learnings", "LRN"},
		{"ERRORS.md", "Errors", "ERR"},
		{"FEATURE_REQUESTS.md", "Feature Requests", "FEAT"},
	}

	totalPending := 0
	totalResolved := 0
	totalPromoted := 0

	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(learningsDir, f.name))
		if err != nil {
			continue
		}
		content := string(data)

		pending := strings.Count(content, "**Status**: pending")
		resolved := strings.Count(content, "**Status**: resolved")
		promoted := strings.Count(content, "**Status**: promoted")
		wontfix := strings.Count(content, "**Status**: wont_fix")

		total := pending + resolved + promoted + wontfix
		if total == 0 {
			fmt.Printf("  %s: (empty)\n", f.label)
			continue
		}

		totalPending += pending
		totalResolved += resolved
		totalPromoted += promoted

		fmt.Printf("  %s: %d pending, %d resolved, %d promoted", f.label, pending, resolved, promoted)
		if wontfix > 0 {
			fmt.Printf(", %d wont_fix", wontfix)
		}
		fmt.Println()

		// Show recent pending entries (last 3)
		entries := extractEntryHeaders(content, f.tag, "pending")
		shown := 0
		for _, entry := range entries {
			if shown >= 3 {
				remaining := len(entries) - shown
				if remaining > 0 {
					fmt.Printf("      ... and %d more\n", remaining)
				}
				break
			}
			fmt.Printf("    \u2022 %s\n", entry)
			shown++
		}
	}

	fmt.Printf("\n  Total: %d pending, %d resolved, %d promoted\n",
		totalPending, totalResolved, totalPromoted)

	if totalPending > 0 && mode == "promote" {
		fmt.Printf("\n  Auto-promotion is enabled (threshold: %d recurrences).\n", threshold)
	} else if totalPending > 0 {
		fmt.Printf("\n  Mode is 'log' - entries are captured but not auto-promoted.\n")
		fmt.Printf("  Set tools.self_improve.mode to 'promote' in config to enable auto-promotion.\n")
	}
}

// extractEntryHeaders finds entry headers like "## [LRN-20250115-001] category" with a given status.
func extractEntryHeaders(content, tag, status string) []string {
	var results []string

	pattern := regexp.MustCompile(`## \[` + tag + `-\d{8}-\w+\] .+`)
	matches := pattern.FindAllStringIndex(content, -1)

	for _, match := range matches {
		// Check if this entry has the target status
		headerEnd := match[1]
		nextEntry := len(content)
		nextMatch := pattern.FindStringIndex(content[headerEnd:])
		if nextMatch != nil {
			nextEntry = headerEnd + nextMatch[0]
		}
		block := content[headerEnd:nextEntry]

		if strings.Contains(block, "**Status**: "+status) {
			header := content[match[0]:match[1]]
			// Extract just the ID and category from "## [LRN-20250115-001] category"
			header = strings.TrimPrefix(header, "## ")
			results = append(results, header)
		}
	}

	return results
}

func improveShowCmd(learningsDir, entryID string) {
	// Determine which file to search based on the ID prefix
	var filename string
	upper := strings.ToUpper(entryID)
	switch {
	case strings.HasPrefix(upper, "LRN"):
		filename = "LEARNINGS.md"
	case strings.HasPrefix(upper, "ERR"):
		filename = "ERRORS.md"
	case strings.HasPrefix(upper, "FEAT"):
		filename = "FEATURE_REQUESTS.md"
	default:
		// Search all files
		for _, f := range []string{"LEARNINGS.md", "ERRORS.md", "FEATURE_REQUESTS.md"} {
			data, err := os.ReadFile(filepath.Join(learningsDir, f))
			if err != nil {
				continue
			}
			if entry := extractEntry(string(data), entryID); entry != "" {
				fmt.Println(entry)
				return
			}
		}
		fmt.Printf("Entry '%s' not found.\n", entryID)
		return
	}

	data, err := os.ReadFile(filepath.Join(learningsDir, filename))
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", filename, err)
		return
	}

	entry := extractEntry(string(data), entryID)
	if entry == "" {
		fmt.Printf("Entry '%s' not found in %s.\n", entryID, filename)
		return
	}

	fmt.Println(entry)
}

// extractEntry finds a complete entry block by its ID.
func extractEntry(content, entryID string) string {
	upper := strings.ToUpper(entryID)
	marker := "[" + upper + "]"

	idx := strings.Index(strings.ToUpper(content), marker)
	if idx < 0 {
		return ""
	}

	// Find the start of the ## header line
	start := strings.LastIndex(content[:idx], "## ")
	if start < 0 {
		start = idx
	}

	// Find the next entry (next ## [ or end of content)
	rest := content[idx+len(marker):]
	nextEntry := strings.Index(rest, "\n## [")
	if nextEntry < 0 {
		return strings.TrimSpace(content[start:])
	}

	end := idx + len(marker) + nextEntry
	return strings.TrimSpace(content[start:end])
}
