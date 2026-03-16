package agent

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/chzyer/readline"

	"github.com/sipeed/picoclaw/cmd/picoclaw/internal"
	"github.com/sipeed/picoclaw/pkg/agent"
	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/cli"
	"github.com/sipeed/picoclaw/pkg/logger"
	"github.com/sipeed/picoclaw/pkg/providers"
)

func agentCmd(message, sessionKey, model string, debug bool) error {
	if sessionKey == "" {
		sessionKey = "cli:default"
	}

	if debug {
		logger.SetLevel(logger.DEBUG)
		fmt.Println("🔍 Debug mode enabled")
	}

	cfg, err := internal.LoadConfig()
	if err != nil {
		return fmt.Errorf("error loading config: %w", err)
	}

	if model != "" {
		cfg.Agents.Defaults.ModelName = model
	}

	provider, modelID, err := providers.CreateProvider(cfg)
	if err != nil {
		return fmt.Errorf("error creating provider: %w", err)
	}

	// Use the resolved model ID from provider creation
	if modelID != "" {
		cfg.Agents.Defaults.ModelName = modelID
	}

	msgBus := bus.NewMessageBus()
	agentLoop := agent.NewAgentLoop(cfg, msgBus, provider)

	// Set up usage tracking
	tracker := cli.NewUsageTracker()
	agentLoop.SetUsageCallback(func(usage *providers.UsageInfo) {
		tracker.Record(usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
	})

	// Set up tab-status for terminal tab title
	project := cli.DetectProject(cfg.Agents.Defaults.Workspace)
	tabStatus := cli.NewTabStatus(project)
	defer tabStatus.Reset()

	// Print agent startup info (only for interactive mode)
	startupInfo := agentLoop.GetStartupInfo()
	logger.InfoCF("agent", "Agent initialized",
		map[string]any{
			"tools_count":      startupInfo["tools"].(map[string]any)["count"],
			"skills_total":     startupInfo["skills"].(map[string]any)["total"],
			"skills_available": startupInfo["skills"].(map[string]any)["available"],
		})

	if message != "" {
		tabStatus.Running()
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, message, sessionKey)
		if err != nil {
			tabStatus.Error()
			return fmt.Errorf("error processing message: %w", err)
		}
		tabStatus.DoneNoCommit()
		fmt.Printf("\n%s %s\n", internal.Logo, response)
		return nil
	}

	fmt.Printf("%s Interactive mode (Ctrl+C to exit)\n\n", internal.Logo)
	interactiveMode(agentLoop, sessionKey, cfg.Agents.Defaults.ModelName, tracker, tabStatus)

	return nil
}

// promptSessionResume checks for existing session history and asks the user
// whether to continue or start fresh. Returns true if session was cleared.
func promptSessionResume(agentLoop *agent.AgentLoop, sessionKey string, reader func() (string, error)) bool {
	resolvedKey := sessionKey
	if resolvedKey == "" || resolvedKey == "cli:default" {
		resolvedKey = agentLoop.GetDefaultSessionKey()
	}
	history := agentLoop.GetSessionHistory(resolvedKey)
	if len(history) == 0 {
		return false
	}

	fmt.Printf("Previous session found (%d messages). Continue? [Y/n] ", len(history))
	line, err := reader()
	if err != nil {
		return false
	}
	answer := strings.TrimSpace(line)
	if answer == "n" || answer == "N" {
		agentLoop.ClearSession(resolvedKey)
		fmt.Println("Session cleared.")
		return true
	}
	return false
}

func interactiveMode(agentLoop *agent.AgentLoop, sessionKey, model string, tracker *cli.UsageTracker, tabStatus *cli.TabStatus) {
	prompt := fmt.Sprintf("%s You: ", internal.Logo)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          prompt,
		HistoryFile:     filepath.Join(os.TempDir(), ".picoclaw_history"),
		HistoryLimit:    100,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
	if err != nil {
		fmt.Printf("Error initializing readline: %v\n", err)
		fmt.Println("Falling back to simple input mode...")
		simpleInteractiveMode(agentLoop, sessionKey, model, tracker, tabStatus)
		return
	}
	defer rl.Close()

	// Check for existing session and prompt user
	promptSessionResume(agentLoop, sessionKey, func() (string, error) {
		return rl.Readline()
	})

	spinner := cli.NewSpinner(rl.Stdout())
	tabStatus.New()

	workDir := agentLoop.GetConfig().WorkspacePath()

	for {
		line, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt || err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		// Intercept "/model" with no args for interactive picker
		if input == "/model" {
			cfg := agentLoop.GetConfig()
			if len(cfg.ModelList) > 0 {
				selected, ok := showModelPicker(rl, model, cfg.ModelList)
				if ok && selected != "" {
					input = "/model " + selected
				} else {
					continue
				}
			}
		}

		commitBefore := cli.GitHeadCommit(workDir)
		tabStatus.Running()
		spinner.Start("Thinking...")
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, sessionKey)
		spinner.Stop()

		if err != nil {
			tabStatus.Error()
			fmt.Printf("Error: %v\n", err)
			continue
		}

		// Check if a git commit happened during this turn
		commitAfter := cli.GitHeadCommit(workDir)
		if commitAfter != "" && commitAfter != commitBefore {
			tabStatus.DoneWithCommit()
		} else {
			tabStatus.DoneNoCommit()
		}

		// Update model name for status line after successful /model switch
		if strings.HasPrefix(input, "/model ") && strings.HasPrefix(response, "Switched model") {
			model = strings.TrimPrefix(input, "/model ")
		}

		fmt.Fprintf(rl.Stdout(), "\n%s %s\n", internal.Logo, response)
		fmt.Fprintf(rl.Stdout(), "%s\n\n", tracker.FormatStatusLine(model, spinner.Elapsed()))
	}
}

func simpleInteractiveMode(agentLoop *agent.AgentLoop, sessionKey, model string, tracker *cli.UsageTracker, tabStatus *cli.TabStatus) {
	reader := bufio.NewReader(os.Stdin)

	// Check for existing session and prompt user
	promptSessionResume(agentLoop, sessionKey, func() (string, error) {
		return reader.ReadString('\n')
	})

	spinner := cli.NewSpinner(os.Stderr)
	tabStatus.New()

	workDir := agentLoop.GetConfig().WorkspacePath()

	for {
		fmt.Print(fmt.Sprintf("%s You: ", internal.Logo))
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				fmt.Println("\nGoodbye!")
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input := strings.TrimSpace(line)
		if input == "" {
			continue
		}

		if input == "exit" || input == "quit" {
			fmt.Println("Goodbye!")
			return
		}

		commitBefore := cli.GitHeadCommit(workDir)
		tabStatus.Running()
		spinner.Start("Thinking...")
		ctx := context.Background()
		response, err := agentLoop.ProcessDirect(ctx, input, sessionKey)
		spinner.Stop()

		if err != nil {
			tabStatus.Error()
			fmt.Printf("Error: %v\n", err)
			continue
		}

		commitAfter := cli.GitHeadCommit(workDir)
		if commitAfter != "" && commitAfter != commitBefore {
			tabStatus.DoneWithCommit()
		} else {
			tabStatus.DoneNoCommit()
		}

		fmt.Printf("\n%s %s\n", internal.Logo, response)
		fmt.Printf("\033[90m%s\033[0m\n\n", tracker.FormatStatusLine(model, spinner.Elapsed()))
	}
}
