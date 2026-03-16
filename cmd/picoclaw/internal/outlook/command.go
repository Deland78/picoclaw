package outlook

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/spf13/cobra"
)

func NewOutlookCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "outlook",
		Short: "Outlook integrations",
	}

	cmd.AddCommand(newDigestCommand())
	cmd.AddCommand(newLoginCommand())
	return cmd
}

func newDigestCommand() *cobra.Command {
	var prompt string
	var output string
	var headed bool

	cmd := &cobra.Command{
		Use:   "digest",
		Short: "Generate email digest via Outlook Copilot",
		Long:  "Opens Outlook Web, queries Copilot with a prompt, and returns a structured email digest.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDigest(prompt, output, headed)
		},
	}

	cmd.Flags().StringVar(&prompt, "prompt", "", "Custom prompt for Copilot (default: from config)")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output file path for markdown digest")
	cmd.Flags().BoolVar(&headed, "headed", false, "Run browser in headed (visible) mode")

	return cmd
}

func newLoginCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "login",
		Short: "Log in to Outlook Web (opens browser for manual authentication)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin()
		},
	}
}

func runDigest(prompt, output string, headed bool) error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if prompt == "" {
		prompt = cfg.Tools.Outlook.DefaultPrompt
	}
	if prompt == "" {
		return fmt.Errorf("no prompt provided and no default_prompt in config tools.outlook")
	}

	// Determine output path
	if output == "" {
		workspace := cfg.WorkspacePath()
		if workspace == "" {
			home, _ := os.UserHomeDir()
			workspace = filepath.Join(home, ".picoclaw", "workspace")
		}
		output = filepath.Join(workspace, "outlook_digest.md")
	}

	picoassistDir := resolvePicoAssistDir()

	profileDir := cfg.Tools.Outlook.ProfileDir
	if profileDir == "" {
		profileDir = filepath.Join(picoassistDir, "profiles", "picoclaw", "outlook")
	}

	cmdArgs := []string{
		"-m", "services.outlook_digest",
		"--prompt", prompt,
		"--output", output,
		"--profile-dir", profileDir,
	}
	if headed {
		cmdArgs = append(cmdArgs, "--headed")
	}

	return runPython(picoassistDir, cmdArgs)
}

func runLogin() error {
	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	picoassistDir := resolvePicoAssistDir()

	profileDir := cfg.Tools.Outlook.ProfileDir
	if profileDir == "" {
		profileDir = filepath.Join(picoassistDir, "profiles", "picoclaw", "outlook")
	}

	cmdArgs := []string{
		"-m", "services.outlook_digest",
		"--login",
		"--profile-dir", profileDir,
	}

	return runPython(picoassistDir, cmdArgs)
}

func runPython(workDir string, args []string) error {
	var pythonBin string
	if runtime.GOOS == "windows" {
		pythonBin = "python"
	} else {
		pythonBin = "python3"
	}

	execCmd := exec.Command(pythonBin, args...)
	execCmd.Dir = workDir
	execCmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	var stdout, stderr bytes.Buffer
	execCmd.Stdout = &stdout
	execCmd.Stderr = &stderr

	if err := execCmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("digest failed: %s", errMsg)
	}

	fmt.Print(stdout.String())
	return nil
}

func resolvePicoAssistDir() string {
	exePath, err := os.Executable()
	if err == nil {
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

	home, _ := os.UserHomeDir()
	return filepath.Join(home, "picoclaw", "picoassist")
}
