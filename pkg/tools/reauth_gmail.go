package tools

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ReauthGmailTool handles Gmail OAuth reauthorization when tokens expire.
// It deletes the stale token, restarts the mail worker, and triggers the
// OAuth browser flow so the user can re-consent.
type ReauthGmailTool struct{}

func NewReauthGmailTool() *ReauthGmailTool {
	return &ReauthGmailTool{}
}

func (t *ReauthGmailTool) Name() string {
	return "reauth_gmail"
}

func (t *ReauthGmailTool) Description() string {
	return "Reauthorize Gmail when OAuth tokens have expired (invalidgrant errors). " +
		"Deletes the stale token, restarts the mail worker, and opens the browser " +
		"for the user to complete OAuth consent. Takes ~30-120 seconds."
}

func (t *ReauthGmailTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *ReauthGmailTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	picoassistDir := resolvePicoAssistDir()

	// Step 1: Delete stale token
	if err := deleteGmailToken(picoassistDir); err != nil {
		return ErrorResult(fmt.Sprintf("Failed to delete Gmail token: %v", err))
	}

	// Step 2: Kill existing mail worker on port 8001
	killScript := `
$conn = Get-NetTCPConnection -LocalPort 8001 -State Listen -ErrorAction SilentlyContinue
if ($conn) {
    $pids = $conn | Select-Object -ExpandProperty OwningProcess -Unique
    foreach ($pid in $pids) {
        Stop-Process -Id $pid -Force -ErrorAction SilentlyContinue
        Write-Output "Killed PID $pid"
    }
} else {
    Write-Output "No process on port 8001"
}
`
	killOutput, err := runPowerShellScript(picoassistDir, killScript)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to kill mail worker: %v\nOutput: %s", err, killOutput))
	}

	// Give it a moment to release the port
	time.Sleep(2 * time.Second)

	// Step 3: Load .env and start mail worker
	envVars, err := loadDotEnv(filepath.Join(picoassistDir, ".env"))
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to load .env: %v", err))
	}

	startScript := buildStartScript(picoassistDir, envVars)
	startOutput, err := runPowerShellScript(picoassistDir, startScript)
	if err != nil {
		return ErrorResult(fmt.Sprintf("Failed to start mail worker: %v\nOutput: %s", err, startOutput))
	}

	// Step 4: Poll health endpoint until ready
	if err := pollHealth(ctx, "http://localhost:8001/health", 10*time.Second); err != nil {
		return ErrorResult(fmt.Sprintf("Mail worker failed to start: %v", err))
	}

	// Step 5: Trigger OAuth flow via list_unread (long timeout for user interaction)
	result, err := triggerOAuth(ctx)
	if err != nil {
		return ErrorResult(fmt.Sprintf(
			"OAuth flow failed or timed out. The browser may have opened — "+
				"please complete the consent flow and try again. Error: %v", err))
	}

	return &ToolResult{
		ForLLM:  fmt.Sprintf("Gmail reauthorization successful. Mail worker restarted.\n\n%s", result),
		ForUser: "Gmail reauthorized successfully — mail worker is running.",
	}
}

// deleteGmailToken removes the stale Gmail OAuth token file.
// Returns nil if the file doesn't exist (non-fatal).
func deleteGmailToken(picoassistDir string) error {
	tokenPath := filepath.Join(picoassistDir, "data", "tokens", "gmail_token.json")
	err := os.Remove(tokenPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// loadDotEnv reads a .env file and returns key=value pairs.
func loadDotEnv(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vars := make(map[string]string)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip surrounding quotes
			val = strings.Trim(val, `"'`)
			vars[key] = val
		}
	}
	return vars, scanner.Err()
}

// buildStartScript creates a PowerShell script that sets env vars and starts the mail worker.
func buildStartScript(picoassistDir string, envVars map[string]string) string {
	var sb strings.Builder
	sb.WriteString("# Set environment variables\n")
	for k, v := range envVars {
		// Escape single quotes in values
		escaped := strings.ReplaceAll(v, "'", "''")
		sb.WriteString(fmt.Sprintf("$env:%s = '%s'\n", k, escaped))
	}
	sb.WriteString(fmt.Sprintf("\n# Start mail worker\n"))
	sb.WriteString(fmt.Sprintf("Start-Process python -ArgumentList '-m','services.mail_worker.app' -WorkingDirectory '%s' -WindowStyle Hidden\n", picoassistDir))
	sb.WriteString("Write-Output 'Mail worker started'\n")
	return sb.String()
}

// runPowerShellScript writes a temp .ps1 file and executes it.
// Uses the MSYS_NO_PATHCONV pattern from MEMORY.md for Git Bash compatibility.
func runPowerShellScript(workDir, script string) (string, error) {
	tmpFile, err := os.CreateTemp("", "picoclaw-*.ps1")
	if err != nil {
		return "", fmt.Errorf("failed to create temp script: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.WriteString(script); err != nil {
		tmpFile.Close()
		return "", fmt.Errorf("failed to write script: %w", err)
	}
	tmpFile.Close()

	cmd := execCommand("powershell", "-ExecutionPolicy", "Bypass", "-File", tmpPath)
	cmd.Dir = workDir

	// Unset CLAUDECODE to avoid nested session protection
	env := os.Environ()
	filtered := make([]string, 0, len(env))
	for _, e := range env {
		if !strings.HasPrefix(e, "CLAUDECODE=") {
			filtered = append(filtered, e)
		}
	}
	cmd.Env = filtered

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// pollHealth polls a health endpoint until it returns 200 or times out.
func pollHealth(ctx context.Context, url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(1 * time.Second)
	}
	return fmt.Errorf("health check timed out after %v", timeout)
}

// triggerOAuth makes a request to list_unread which triggers the OAuth browser flow.
// Uses a long timeout since the user needs to interact with the browser.
func triggerOAuth(ctx context.Context) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Get("http://localhost:8001/mail/list_unread")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mail worker returned status %d: %s", resp.StatusCode, string(body))
	}

	return string(body), nil
}

// execCommand is a variable for testing (allows mocking exec.Command).
var execCommand = execCommandImpl

func execCommandImpl(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}
