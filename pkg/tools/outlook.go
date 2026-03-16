package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/sipeed/picoclaw/pkg/config"
)

// OutlookDigestTool generates an email digest via Outlook Copilot browser automation.
type OutlookDigestTool struct {
	defaultPrompt string
	profileDir    string
	workspace     string
	picoassistDir string
}

// NewOutlookDigestTool creates a new OutlookDigestTool.
func NewOutlookDigestTool(cfg *config.Config, workspace string) *OutlookDigestTool {
	// Resolve picoassist directory relative to the binary or known location
	picoassistDir := resolvePicoAssistDir()

	profileDir := cfg.Tools.Outlook.ProfileDir
	if profileDir == "" {
		profileDir = filepath.Join(picoassistDir, "profiles", "picoclaw", "outlook")
	}

	return &OutlookDigestTool{
		defaultPrompt: cfg.Tools.Outlook.DefaultPrompt,
		profileDir:    profileDir,
		workspace:     workspace,
		picoassistDir: picoassistDir,
	}
}

func (t *OutlookDigestTool) Name() string {
	return "outlook_digest"
}

func (t *OutlookDigestTool) Description() string {
	return "Generate an email digest by querying Copilot in Outlook Web. " +
		"Uses browser automation with a persistent login session. " +
		"Returns a markdown-formatted summary of unread emails grouped by project."
}

func (t *OutlookDigestTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"prompt": map[string]any{
				"type":        "string",
				"description": "Custom prompt to send to Copilot. If omitted, uses the configured default prompt.",
			},
		},
	}
}

func (t *OutlookDigestTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	prompt := t.defaultPrompt
	if p, ok := args["prompt"].(string); ok && p != "" {
		prompt = p
	}
	if prompt == "" {
		return ErrorResult("No prompt provided and no default_prompt configured in tools.outlook")
	}

	// Determine output path
	now := time.Now().Format("2006-01-02")
	outputPath := filepath.Join(t.workspace, fmt.Sprintf("outlook_digest_%s.md", now))

	// Build python command
	cmdArgs := []string{
		"-m", "services.outlook_digest",
		"--prompt", prompt,
		"--output", outputPath,
		"--profile-dir", t.profileDir,
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 150*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(timeoutCtx, "python", cmdArgs...)
	} else {
		cmd = exec.CommandContext(timeoutCtx, "python3", cmdArgs...)
	}
	cmd.Dir = t.picoassistDir
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return ErrorResult(fmt.Sprintf("Outlook digest failed: %s", strings.TrimSpace(errMsg)))
	}

	output := stdout.String()
	if output == "" {
		return ErrorResult("Outlook digest produced no output")
	}

	return &ToolResult{
		ForLLM:  output,
		ForUser: output,
	}
}

// resolvePicoAssistDir finds the picoassist directory.
func resolvePicoAssistDir() string {
	// Try relative to the binary
	exePath, err := os.Executable()
	if err == nil {
		// Binary is at <project>/cmd/picoclaw/ or <project>/bin/
		// PicoAssist is at <project>/picoassist/
		dir := filepath.Dir(exePath)
		candidates := []string{
			filepath.Join(dir, "..", "..", "picoassist"),
			filepath.Join(dir, "..", "picoassist"),
			filepath.Join(dir, "picoassist"),
		}
		for _, c := range candidates {
			abs, _ := filepath.Abs(c)
			if info, err := os.Stat(abs); err == nil && info.IsDir() {
				return abs
			}
		}
	}

	// Fallback: well-known location
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "picoclaw", "picoassist")
}
