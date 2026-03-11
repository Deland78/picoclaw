package agent

import (
	"fmt"
	"os"

	"github.com/chzyer/readline"

	"github.com/sipeed/picoclaw/pkg/config"
)

// pickerKey represents a key event from the picker input reader.
type pickerKey int

const (
	pickerKeyNone pickerKey = iota
	pickerKeyUp
	pickerKeyDown
	pickerKeyEnter
	pickerKeyEscape
	pickerKeyCtrlC
)

// showModelPicker displays an interactive arrow-key picker for model selection.
// Returns the selected model name and true, or empty string and false on cancel.
func showModelPicker(_ *readline.Instance, currentModel string, models []config.ModelConfig) (string, bool) {
	// Write directly to stdout for ANSI rendering. On Windows, stdout has
	// VT processing enabled (readline sets it up). os.Stderr may not.
	w := os.Stdout

	if len(models) == 0 {
		fmt.Fprintln(w, "No models configured in model_list")
		return "", false
	}

	// Deduplicate model names (multiple entries = load-balanced, show once)
	type entry struct {
		name  string
		model string
	}
	seen := make(map[string]bool)
	var entries []entry
	cursor := 0
	for _, mc := range models {
		if seen[mc.ModelName] {
			continue
		}
		seen[mc.ModelName] = true
		if mc.ModelName == currentModel || mc.Model == currentModel {
			cursor = len(entries)
		}
		entries = append(entries, entry{name: mc.ModelName, model: mc.Model})
	}

	isCurrent := func(e entry) bool {
		return e.name == currentModel || e.model == currentModel
	}

	total := len(entries)

	render := func(first bool) {
		if !first {
			for i := 0; i < total+1; i++ {
				fmt.Fprint(w, "\033[A")
			}
		}
		fmt.Fprintf(w, "\033[K\033[1mSelect model:\033[0m \033[90m(arrows navigate, enter select, esc cancel)\033[0m\n")
		for i, e := range entries {
			fmt.Fprint(w, "\033[K")
			if i == cursor {
				fmt.Fprintf(w, "  \033[7m %s \033[0m", e.name)
			} else {
				fmt.Fprintf(w, "   %s", e.name)
			}
			if isCurrent(e) {
				fmt.Fprint(w, " \033[90m(current)\033[0m")
			}
			fmt.Fprintln(w)
		}
	}

	render(true)

	// Platform-specific key reader (uses ReadConsoleInput on Windows,
	// raw mode + ANSI parsing on Unix)
	reader, err := newPickerReader()
	if err != nil {
		fmt.Fprintf(w, "Error initializing input: %v\n", err)
		return "", false
	}
	defer reader.Close()

	cleanup := func() {
		for i := 0; i < total+1; i++ {
			fmt.Fprint(w, "\033[A\033[K")
		}
	}

	for {
		key := reader.ReadKey()

		switch key {
		case pickerKeyCtrlC, pickerKeyEscape:
			cleanup()
			return "", false

		case pickerKeyEnter:
			selected := entries[cursor]
			cleanup()
			return selected.name, true

		case pickerKeyUp:
			if cursor > 0 {
				cursor--
				render(false)
			}

		case pickerKeyDown:
			if cursor < total-1 {
				cursor++
				render(false)
			}
		}
	}
}
