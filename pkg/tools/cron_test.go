package tools

import (
	"context"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/cron"
)

// TestCronTool_ZeroValueScheduleParams verifies that when LLMs fill unused optional
// parameters with default values (0 for int, "" for string), the zero values don't
// shadow the intended schedule type.
func TestCronTool_ZeroValueScheduleParams(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := tmpDir + "/cron_store.json"
	cronService := cron.NewCronService(storePath, func(job *cron.CronJob) (string, error) { return "", nil })
	if err := cronService.Start(); err != nil {
		t.Fatalf("failed to start cron service: %v", err)
	}
	defer cronService.Stop()

	cfg := config.DefaultConfig()
	tool := NewCronTool(cronService, nil, nil, tmpDir, false, 0, cfg)
	tool.SetContext("test-channel", "test-chat")

	// Simulate an LLM that sends all three schedule params,
	// with the intended one being every_seconds=30 but also at_seconds=0 and cron_expr=""
	result := tool.Execute(context.Background(), map[string]any{
		"action":        "add",
		"message":       "test recurring job",
		"at_seconds":    float64(0), // LLM default - should be ignored
		"every_seconds": float64(30),
		"cron_expr":     "",
	})

	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.ForLLM)
	}

	// The job should be "every" type, not "at" type
	// If zero-value at_seconds takes priority, it would be an "at" job firing immediately
	if result.ForLLM == "" {
		t.Fatal("expected non-empty ForLLM result")
	}

	// Verify by listing jobs - should show an "every" job
	listResult := tool.Execute(context.Background(), map[string]any{
		"action": "list",
	})
	if listResult.IsError {
		t.Fatalf("list failed: %s", listResult.ForLLM)
	}
	// The result should mention "every" scheduling, not "at"
	if !containsSubstring(listResult.ForLLM, "every") {
		t.Errorf("expected job to be 'every' type, got: %s", listResult.ForLLM)
	}
}

func containsSubstring(s, sub string) bool {
	return len(s) >= len(sub) && searchSubstring(s, sub)
}

func searchSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
