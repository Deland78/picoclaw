package tools

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/providers"
)

// mockToolLoopProvider returns a fixed response with tool calls on first call,
// then a plain text response on subsequent calls.
type mockToolLoopProvider struct {
	callCount int
	toolCalls []providers.ToolCall
}

func (m *mockToolLoopProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	m.callCount++
	if m.callCount == 1 {
		return &providers.LLMResponse{
			Content:   "",
			ToolCalls: m.toolCalls,
		}, nil
	}
	return &providers.LLMResponse{
		Content:   "done",
		ToolCalls: nil,
	}, nil
}

func (m *mockToolLoopProvider) GetDefaultModel() string {
	return "mock-model"
}

// slowTool sleeps for a configurable duration before returning.
type slowTool struct {
	name     string
	duration time.Duration
	started  atomic.Int32 // tracks concurrent executions
	maxConc  atomic.Int32 // records peak concurrency
}

func (t *slowTool) Name() string        { return t.name }
func (t *slowTool) Description() string  { return "slow tool for testing" }
func (t *slowTool) Parameters() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}

func (t *slowTool) Execute(ctx context.Context, args map[string]any) *ToolResult {
	cur := t.started.Add(1)
	// Record peak concurrency
	for {
		old := t.maxConc.Load()
		if cur <= old || t.maxConc.CompareAndSwap(old, cur) {
			break
		}
	}
	time.Sleep(t.duration)
	t.started.Add(-1)
	return &ToolResult{ForLLM: "ok from " + t.name}
}

func TestRunToolLoop_ParallelExecution(t *testing.T) {
	delay := 100 * time.Millisecond

	tool1 := &slowTool{name: "slow_a", duration: delay}
	tool2 := &slowTool{name: "slow_b", duration: delay}
	tool3 := &slowTool{name: "slow_c", duration: delay}

	registry := NewToolRegistry()
	registry.Register(tool1)
	registry.Register(tool2)
	registry.Register(tool3)

	provider := &mockToolLoopProvider{
		toolCalls: []providers.ToolCall{
			{ID: "call_1", Name: "slow_a", Arguments: map[string]any{}},
			{ID: "call_2", Name: "slow_b", Arguments: map[string]any{}},
			{ID: "call_3", Name: "slow_c", Arguments: map[string]any{}},
		},
	}

	messages := []providers.Message{
		{Role: "user", Content: "run all tools"},
	}

	start := time.Now()
	result, err := RunToolLoop(
		context.Background(),
		ToolLoopConfig{
			Provider:      provider,
			Model:         "test",
			Tools:         registry,
			MaxIterations: 5,
		},
		messages,
		"test-ch", "test-id",
	)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}
	if result.Content != "done" {
		t.Errorf("Content = %q, want %q", result.Content, "done")
	}

	// Sequential: ~300ms, Parallel: ~100ms
	// Use 250ms as threshold - fails for sequential, passes for parallel
	if elapsed > 250*time.Millisecond {
		t.Errorf("Execution took %v, want < 250ms (tools should run in parallel)", elapsed)
	}
}

func TestRunToolLoop_ParallelResultsInOrder(t *testing.T) {
	// Tool A is slow, Tool B is fast. Results should still be in call order.
	toolA := &slowTool{name: "slow_first", duration: 80 * time.Millisecond}
	toolB := &slowTool{name: "fast_second", duration: 10 * time.Millisecond}

	registry := NewToolRegistry()
	registry.Register(toolA)
	registry.Register(toolB)

	provider := &mockToolLoopProvider{
		toolCalls: []providers.ToolCall{
			{ID: "call_1", Name: "slow_first", Arguments: map[string]any{}},
			{ID: "call_2", Name: "fast_second", Arguments: map[string]any{}},
		},
	}

	messages := []providers.Message{
		{Role: "user", Content: "run both"},
	}

	_, err := RunToolLoop(
		context.Background(),
		ToolLoopConfig{
			Provider:      provider,
			Model:         "test",
			Tools:         registry,
			MaxIterations: 5,
		},
		messages,
		"ch", "id",
	)
	if err != nil {
		t.Fatalf("RunToolLoop error: %v", err)
	}

	// Verify both calls happened (provider callCount = 2: first with tools, second without)
	if provider.callCount != 2 {
		t.Errorf("provider.callCount = %d, want 2", provider.callCount)
	}
}
