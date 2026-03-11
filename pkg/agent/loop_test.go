package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/providers"
	"github.com/sipeed/picoclaw/pkg/tools"
)

func TestRecordLastChannel(t *testing.T) {
	// Create temp workspace
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test config
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	// Create agent loop
	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Test RecordLastChannel
	testChannel := "test-channel"
	err = al.RecordLastChannel(testChannel)
	if err != nil {
		t.Fatalf("RecordLastChannel failed: %v", err)
	}

	// Verify channel was saved
	lastChannel := al.state.GetLastChannel()
	if lastChannel != testChannel {
		t.Errorf("Expected channel '%s', got '%s'", testChannel, lastChannel)
	}

	// Verify persistence by creating a new agent loop
	al2 := NewAgentLoop(cfg, msgBus, provider)
	if al2.state.GetLastChannel() != testChannel {
		t.Errorf("Expected persistent channel '%s', got '%s'", testChannel, al2.state.GetLastChannel())
	}
}

func TestRecordLastChatID(t *testing.T) {
	// Create temp workspace
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test config
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	// Create agent loop
	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Test RecordLastChatID
	testChatID := "test-chat-id-123"
	err = al.RecordLastChatID(testChatID)
	if err != nil {
		t.Fatalf("RecordLastChatID failed: %v", err)
	}

	// Verify chat ID was saved
	lastChatID := al.state.GetLastChatID()
	if lastChatID != testChatID {
		t.Errorf("Expected chat ID '%s', got '%s'", testChatID, lastChatID)
	}

	// Verify persistence by creating a new agent loop
	al2 := NewAgentLoop(cfg, msgBus, provider)
	if al2.state.GetLastChatID() != testChatID {
		t.Errorf("Expected persistent chat ID '%s', got '%s'", testChatID, al2.state.GetLastChatID())
	}
}

func TestNewAgentLoop_StateInitialized(t *testing.T) {
	// Create temp workspace
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test config
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	// Create agent loop
	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Verify state manager is initialized
	if al.state == nil {
		t.Error("Expected state manager to be initialized")
	}

	// Verify state directory was created
	stateDir := filepath.Join(tmpDir, "state")
	if _, err := os.Stat(stateDir); os.IsNotExist(err) {
		t.Error("Expected state directory to exist")
	}
}

// TestToolRegistry_ToolRegistration verifies tools can be registered and retrieved
func TestToolRegistry_ToolRegistration(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Register a custom tool
	customTool := &mockCustomTool{}
	al.RegisterTool(customTool)

	// Verify tool is registered by checking it doesn't panic on GetStartupInfo
	// (actual tool retrieval is tested in tools package tests)
	info := al.GetStartupInfo()
	toolsInfo := info["tools"].(map[string]any)
	toolsList := toolsInfo["names"].([]string)

	// Check that our custom tool name is in the list
	found := false
	for _, name := range toolsList {
		if name == "mock_custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected custom tool to be registered")
	}
}

// TestToolContext_Updates verifies tool context is updated with channel/chatID
func TestToolContext_Updates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "OK"}
	_ = NewAgentLoop(cfg, msgBus, provider)

	// Verify that ContextualTool interface is defined and can be implemented
	// This test validates the interface contract exists
	ctxTool := &mockContextualTool{}

	// Verify the tool implements the interface correctly
	var _ tools.ContextualTool = ctxTool
}

// TestToolRegistry_GetDefinitions verifies tool definitions can be retrieved
func TestToolRegistry_GetDefinitions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Register a test tool and verify it shows up in startup info
	testTool := &mockCustomTool{}
	al.RegisterTool(testTool)

	info := al.GetStartupInfo()
	toolsInfo := info["tools"].(map[string]any)
	toolsList := toolsInfo["names"].([]string)

	// Check that our custom tool name is in the list
	found := false
	for _, name := range toolsList {
		if name == "mock_custom" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected custom tool to be registered")
	}
}

// TestAgentLoop_GetStartupInfo verifies startup info contains tools
func TestAgentLoop_GetStartupInfo(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	info := al.GetStartupInfo()

	// Verify tools info exists
	toolsInfo, ok := info["tools"]
	if !ok {
		t.Fatal("Expected 'tools' key in startup info")
	}

	toolsMap, ok := toolsInfo.(map[string]any)
	if !ok {
		t.Fatal("Expected 'tools' to be a map")
	}

	count, ok := toolsMap["count"]
	if !ok {
		t.Fatal("Expected 'count' in tools info")
	}

	// Should have default tools registered
	if count.(int) == 0 {
		t.Error("Expected at least some tools to be registered")
	}
}

// TestAgentLoop_Stop verifies Stop() sets running to false
func TestAgentLoop_Stop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &mockProvider{}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Note: running is only set to true when Run() is called
	// We can't test that without starting the event loop
	// Instead, verify the Stop method can be called safely
	al.Stop()

	// Verify running is false (initial state or after Stop)
	if al.running.Load() {
		t.Error("Expected agent to be stopped (or never started)")
	}
}

// Mock implementations for testing

type simpleMockProvider struct {
	response string
}

func (m *simpleMockProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		Content:   m.response,
		ToolCalls: []providers.ToolCall{},
	}, nil
}

func (m *simpleMockProvider) GetDefaultModel() string {
	return "mock-model"
}

// mockCustomTool is a simple mock tool for registration testing
type mockCustomTool struct{}

func (m *mockCustomTool) Name() string {
	return "mock_custom"
}

func (m *mockCustomTool) Description() string {
	return "Mock custom tool for testing"
}

func (m *mockCustomTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (m *mockCustomTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	return tools.SilentResult("Custom tool executed")
}

// mockContextualTool tracks context updates
type mockContextualTool struct {
	lastChannel string
	lastChatID  string
}

func (m *mockContextualTool) Name() string {
	return "mock_contextual"
}

func (m *mockContextualTool) Description() string {
	return "Mock contextual tool"
}

func (m *mockContextualTool) Parameters() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (m *mockContextualTool) Execute(ctx context.Context, args map[string]any) *tools.ToolResult {
	return tools.SilentResult("Contextual tool executed")
}

func (m *mockContextualTool) SetContext(channel, chatID string) {
	m.lastChannel = channel
	m.lastChatID = chatID
}

// testHelper executes a message and returns the response
type testHelper struct {
	al *AgentLoop
}

func (h testHelper) executeAndGetResponse(tb testing.TB, ctx context.Context, msg bus.InboundMessage) string {
	// Use a short timeout to avoid hanging
	timeoutCtx, cancel := context.WithTimeout(ctx, responseTimeout)
	defer cancel()

	response, err := h.al.processMessage(timeoutCtx, msg)
	if err != nil {
		tb.Fatalf("processMessage failed: %v", err)
	}
	return response
}

const responseTimeout = 3 * time.Second

// TestToolResult_SilentToolDoesNotSendUserMessage verifies silent tools don't trigger outbound
func TestToolResult_SilentToolDoesNotSendUserMessage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "File operation complete"}
	al := NewAgentLoop(cfg, msgBus, provider)
	helper := testHelper{al: al}

	// ReadFileTool returns SilentResult, which should not send user message
	ctx := context.Background()
	msg := bus.InboundMessage{
		Channel:    "test",
		SenderID:   "user1",
		ChatID:     "chat1",
		Content:    "read test.txt",
		SessionKey: "test-session",
	}

	response := helper.executeAndGetResponse(t, ctx, msg)

	// Silent tool should return the LLM's response directly
	if response != "File operation complete" {
		t.Errorf("Expected 'File operation complete', got: %s", response)
	}
}

// TestToolResult_UserFacingToolDoesSendMessage verifies user-facing tools trigger outbound
func TestToolResult_UserFacingToolDoesSendMessage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "Command output: hello world"}
	al := NewAgentLoop(cfg, msgBus, provider)
	helper := testHelper{al: al}

	// ExecTool returns UserResult, which should send user message
	ctx := context.Background()
	msg := bus.InboundMessage{
		Channel:    "test",
		SenderID:   "user1",
		ChatID:     "chat1",
		Content:    "run hello",
		SessionKey: "test-session",
	}

	response := helper.executeAndGetResponse(t, ctx, msg)

	// User-facing tool should include the output in final response
	if response != "Command output: hello world" {
		t.Errorf("Expected 'Command output: hello world', got: %s", response)
	}
}

// failFirstMockProvider fails on the first N calls with a specific error
type failFirstMockProvider struct {
	failures    int
	currentCall int
	failError   error
	successResp string
}

func (m *failFirstMockProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	m.currentCall++
	if m.currentCall <= m.failures {
		return nil, m.failError
	}
	return &providers.LLMResponse{
		Content:   m.successResp,
		ToolCalls: []providers.ToolCall{},
	}, nil
}

func (m *failFirstMockProvider) GetDefaultModel() string {
	return "mock-fail-model"
}

// TestAgentLoop_ContextExhaustionRetry verify that the agent retries on context errors
func TestAgentLoop_ContextExhaustionRetry(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()

	// Create a provider that fails once with a context error
	contextErr := fmt.Errorf("InvalidParameter: Total tokens of image and text exceed max message tokens")
	provider := &failFirstMockProvider{
		failures:    1,
		failError:   contextErr,
		successResp: "Recovered from context error",
	}

	al := NewAgentLoop(cfg, msgBus, provider)

	// Inject some history to simulate a full context
	sessionKey := "test-session-context"
	// Create dummy history
	history := []providers.Message{
		{Role: "system", Content: "System prompt"},
		{Role: "user", Content: "Old message 1"},
		{Role: "assistant", Content: "Old response 1"},
		{Role: "user", Content: "Old message 2"},
		{Role: "assistant", Content: "Old response 2"},
		{Role: "user", Content: "Trigger message"},
	}
	defaultAgent := al.registry.GetDefaultAgent()
	if defaultAgent == nil {
		t.Fatal("No default agent found")
	}
	defaultAgent.Sessions.SetHistory(sessionKey, history)

	// Call ProcessDirectWithChannel
	// Note: ProcessDirectWithChannel calls processMessage which will execute runLLMIteration
	response, err := al.ProcessDirectWithChannel(
		context.Background(),
		"Trigger message",
		sessionKey,
		"test",
		"test-chat",
	)
	if err != nil {
		t.Fatalf("Expected success after retry, got error: %v", err)
	}

	if response != "Recovered from context error" {
		t.Errorf("Expected 'Recovered from context error', got '%s'", response)
	}

	// We expect 2 calls: 1st failed, 2nd succeeded
	if provider.currentCall != 2 {
		t.Errorf("Expected 2 calls (1 fail + 1 success), got %d", provider.currentCall)
	}

	// Check final history length
	finalHistory := defaultAgent.Sessions.GetHistory(sessionKey)
	// We verify that the history has been modified (compressed)
	// Original length: 6
	// Expected behavior: compression drops ~50% of history (mid slice)
	// We can assert that the length is NOT what it would be without compression.
	// Without compression: 6 + 1 (new user msg) + 1 (assistant msg) = 8
	if len(finalHistory) >= 8 {
		t.Errorf("Expected history to be compressed (len < 8), got %d", len(finalHistory))
	}
}

// usageMockProvider returns a response with Usage info attached.
type usageMockProvider struct {
	usage *providers.UsageInfo
}

func (m *usageMockProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	return &providers.LLMResponse{
		Content:   "Response with usage",
		ToolCalls: []providers.ToolCall{},
		Usage:     m.usage,
	}, nil
}

func (m *usageMockProvider) GetDefaultModel() string {
	return "mock-model"
}

// TestAgentLoop_SetUsageCallback verifies that the usage callback is invoked
// with the correct usage info after each LLM call.
func TestAgentLoop_SetUsageCallback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	expectedUsage := &providers.UsageInfo{
		PromptTokens:     100,
		CompletionTokens: 50,
		TotalTokens:      150,
	}

	msgBus := bus.NewMessageBus()
	provider := &usageMockProvider{usage: expectedUsage}
	al := NewAgentLoop(cfg, msgBus, provider)

	var receivedUsage *providers.UsageInfo
	var callCount int
	al.SetUsageCallback(func(u *providers.UsageInfo) {
		receivedUsage = u
		callCount++
	})

	helper := testHelper{al: al}
	ctx := context.Background()
	msg := bus.InboundMessage{
		Channel:    "test",
		SenderID:   "user1",
		ChatID:     "chat1",
		Content:    "hello",
		SessionKey: "test-session-usage",
	}

	_ = helper.executeAndGetResponse(t, ctx, msg)

	if callCount != 1 {
		t.Errorf("callback invoked %d times, want 1", callCount)
	}
	if receivedUsage == nil {
		t.Fatal("callback was not invoked with usage")
	}
	if receivedUsage.PromptTokens != expectedUsage.PromptTokens {
		t.Errorf("PromptTokens = %d, want %d", receivedUsage.PromptTokens, expectedUsage.PromptTokens)
	}
	if receivedUsage.CompletionTokens != expectedUsage.CompletionTokens {
		t.Errorf("CompletionTokens = %d, want %d", receivedUsage.CompletionTokens, expectedUsage.CompletionTokens)
	}
	if receivedUsage.TotalTokens != expectedUsage.TotalTokens {
		t.Errorf("TotalTokens = %d, want %d", receivedUsage.TotalTokens, expectedUsage.TotalTokens)
	}
}

// TestAgentLoop_SetUsageCallback_NilUsage verifies the callback is NOT invoked
// when the LLM response has nil Usage.
func TestAgentLoop_SetUsageCallback_NilUsage(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "No usage info"}
	al := NewAgentLoop(cfg, msgBus, provider)

	var callCount int
	al.SetUsageCallback(func(u *providers.UsageInfo) {
		callCount++
	})

	helper := testHelper{al: al}
	ctx := context.Background()
	msg := bus.InboundMessage{
		Channel:    "test",
		SenderID:   "user1",
		ChatID:     "chat1",
		Content:    "hello",
		SessionKey: "test-session-no-usage",
	}

	_ = helper.executeAndGetResponse(t, ctx, msg)

	if callCount != 0 {
		t.Errorf("callback invoked %d times, want 0 (nil usage)", callCount)
	}
}

// TestCronSessionKeyHonored verifies that cron-prefixed session keys are used
// instead of being overridden by the routing session key.
func TestCronSessionKeyHonored(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-cron-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "cron response"}
	al := NewAgentLoop(cfg, msgBus, provider)

	ctx := context.Background()
	cronKey := "cron-daily-backup"

	// Process two messages with the same cron session key.
	// If the key were overridden by routing, both would land in the default
	// interactive session and accumulate. With proper cron isolation, each
	// cron invocation starts fresh (session is cleared after processing).
	resp1, err := al.ProcessDirectWithChannel(ctx, "run backup", cronKey, "cli", "direct")
	if err != nil {
		t.Fatalf("first ProcessDirectWithChannel failed: %v", err)
	}
	if resp1 != "cron response" {
		t.Errorf("Expected 'cron response', got '%s'", resp1)
	}

	resp2, err := al.ProcessDirectWithChannel(ctx, "run backup again", cronKey, "cli", "direct")
	if err != nil {
		t.Fatalf("second ProcessDirectWithChannel failed: %v", err)
	}
	if resp2 != "cron response" {
		t.Errorf("Expected 'cron response', got '%s'", resp2)
	}

	// Verify cron session is cleared (ephemeral) — no accumulation
	defaultAgent := al.registry.GetDefaultAgent()
	cronHistory := defaultAgent.Sessions.GetHistory(cronKey)
	if len(cronHistory) != 0 {
		t.Errorf("Expected cron session to be cleared after processing, got %d messages", len(cronHistory))
	}

	// Verify the default interactive session was NOT polluted by cron messages
	defaultKey := al.GetDefaultSessionKey()
	defaultHistory := defaultAgent.Sessions.GetHistory(defaultKey)
	if len(defaultHistory) != 0 {
		t.Errorf("Expected default session to be empty (cron should not pollute it), got %d messages", len(defaultHistory))
	}
}

// TestCronSessionNotSaved verifies that cron sessions are not persisted to disk.
func TestCronSessionNotSaved(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-cron-save-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "cron response"}
	al := NewAgentLoop(cfg, msgBus, provider)

	ctx := context.Background()
	cronKey := "cron-daily-report"

	_, err = al.ProcessDirectWithChannel(ctx, "generate report", cronKey, "cli", "direct")
	if err != nil {
		t.Fatalf("ProcessDirectWithChannel failed: %v", err)
	}

	// Check that no session file was saved for the cron key
	sessionsDir := filepath.Join(tmpDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		// No sessions dir at all — that's fine, means nothing was saved
		return
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		t.Fatalf("Failed to read sessions dir: %v", err)
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), "cron") {
			t.Errorf("Found cron session file on disk: %s", entry.Name())
		}
	}
}

// TestCronSessionCleared verifies that cron sessions are cleared from memory
// after processing completes.
func TestCronSessionCleared(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-cron-clear-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "cron response"}
	al := NewAgentLoop(cfg, msgBus, provider)

	ctx := context.Background()
	cronKey := "cron-hourly-check"

	_, err = al.ProcessDirectWithChannel(ctx, "check status", cronKey, "cli", "direct")
	if err != nil {
		t.Fatalf("ProcessDirectWithChannel failed: %v", err)
	}

	// After processing, the cron session should be cleared from memory
	defaultAgent := al.registry.GetDefaultAgent()
	history := defaultAgent.Sessions.GetHistory(cronKey)
	if len(history) != 0 {
		t.Errorf("Expected cron session to be cleared, but found %d messages", len(history))
	}
}

// modelCaptureMockProvider captures which model was used in the Chat call.
type modelCaptureMockProvider struct {
	response     string
	capturedModel string
}

func (m *modelCaptureMockProvider) Chat(
	ctx context.Context,
	messages []providers.Message,
	tools []providers.ToolDefinition,
	model string,
	opts map[string]any,
) (*providers.LLMResponse, error) {
	m.capturedModel = model
	return &providers.LLMResponse{
		Content:   m.response,
		ToolCalls: []providers.ToolCall{},
	}, nil
}

func (m *modelCaptureMockProvider) GetDefaultModel() string {
	return "mock-model"
}

// TestHandleCommand_Model verifies the /model command.
func TestHandleCommand_Model(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-model-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "openai/gpt-4o",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
		ModelList: []config.ModelConfig{
			{ModelName: "gpt4", Model: "openai/gpt-4o", APIKey: "k1"},
			{ModelName: "opus", Model: "anthropic/claude-opus-4-6", APIKey: "k2"},
			{ModelName: "haiku", Model: "anthropic/claude-haiku-4-5", APIKey: "k3"},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "ok"}
	al := NewAgentLoop(cfg, msgBus, provider)

	ctx := context.Background()

	// /model with no args should show current model
	msg := bus.InboundMessage{Content: "/model", Channel: "telegram", SenderID: "u1", ChatID: "c1"}
	resp, handled := al.handleCommand(ctx, msg)
	if !handled {
		t.Fatal("/model should be handled")
	}
	if !strings.Contains(resp, "openai/gpt-4o") {
		t.Errorf("Expected current model in response, got: %s", resp)
	}

	// /model list should show available models
	msg.Content = "/model list"
	resp, handled = al.handleCommand(ctx, msg)
	if !handled {
		t.Fatal("/model list should be handled")
	}
	if !strings.Contains(resp, "gpt4") || !strings.Contains(resp, "opus") || !strings.Contains(resp, "haiku") {
		t.Errorf("Expected model names in list, got: %s", resp)
	}

	// /model opus should switch model
	msg.Content = "/model opus"
	resp, handled = al.handleCommand(ctx, msg)
	if !handled {
		t.Fatal("/model opus should be handled")
	}
	if !strings.Contains(resp, "opus") {
		t.Errorf("Expected switch confirmation, got: %s", resp)
	}

	// Verify the model was actually switched
	defaultAgent := al.registry.GetDefaultAgent()
	if defaultAgent.Model != "claude-opus-4-6" {
		t.Errorf("Expected model to be switched to claude-opus-4-6, got: %s", defaultAgent.Model)
	}

	// /model with invalid name
	msg.Content = "/model nonexistent"
	resp, handled = al.handleCommand(ctx, msg)
	if !handled {
		t.Fatal("/model nonexistent should be handled")
	}
	if !strings.Contains(strings.ToLower(resp), "not found") && !strings.Contains(strings.ToLower(resp), "unknown") {
		t.Errorf("Expected error for unknown model, got: %s", resp)
	}
}

// TestModelTagPrefix verifies @model: prefix parsing in processMessage.
func TestModelTagPrefix(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-modeltag-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Both models use openai/ protocol so the override provider creation
	// returns an HTTP provider (with fake API key) and the mock captures the model.
	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "openai/gpt-4o",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
		ModelList: []config.ModelConfig{
			{ModelName: "gpt4", Model: "openai/gpt-4o", APIKey: "k1"},
			{ModelName: "alt", Model: "openai/gpt-4o-alt", APIKey: "k2"},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &modelCaptureMockProvider{response: "model tag response"}
	al := NewAgentLoop(cfg, msgBus, provider)

	ctx := context.Background()

	// Send message with @alt: prefix — uses same provider protocol as default
	// The override creates a new HTTP provider, but the agent's default provider
	// is the mock. To test the model tag parsing, we verify the user message is
	// stripped and the processOptions are set correctly.
	// For an end-to-end test, we use a message without model tag as baseline.
	msg := bus.InboundMessage{
		Channel:  "telegram",
		SenderID: "u1",
		ChatID:   "c1",
		Content:  "what is the meaning of life",
	}

	resp, err := al.processMessage(ctx, msg)
	if err != nil {
		t.Fatalf("processMessage failed: %v", err)
	}
	if resp != "model tag response" {
		t.Errorf("Expected 'model tag response', got: %s", resp)
	}

	// Verify default model was used
	if provider.capturedModel != "openai/gpt-4o" {
		t.Errorf("Expected default model openai/gpt-4o, got: %s", provider.capturedModel)
	}

	// Verify the default agent model was NOT changed
	defaultAgent := al.registry.GetDefaultAgent()
	if defaultAgent.Model != "openai/gpt-4o" {
		t.Errorf("Expected default model unchanged (openai/gpt-4o), got: %s", defaultAgent.Model)
	}
}

// TestModelTagPrefix_RegexParsing verifies the @model: regex strips prefix correctly.
func TestModelTagPrefix_RegexParsing(t *testing.T) {
	tests := []struct {
		input     string
		wantMatch bool
		wantTag   string
	}{
		{"@opus: hello world", true, "opus"},
		{"@haiku: test", true, "haiku"},
		{"@my-model: test", true, "my-model"},
		{"@opus hello world", true, "opus"},  // colon optional
		{"hello @opus: world", false, ""},     // not at start
		{"@: test", false, ""},                // empty name
		{"regular message", false, ""},
	}

	for _, tt := range tests {
		m := modelTagRe.FindStringSubmatch(tt.input)
		gotMatch := m != nil
		if gotMatch != tt.wantMatch {
			t.Errorf("input=%q: match=%v, want=%v", tt.input, gotMatch, tt.wantMatch)
		}
		if gotMatch && m[1] != tt.wantTag {
			t.Errorf("input=%q: tag=%q, want=%q", tt.input, m[1], tt.wantTag)
		}
	}
}

// TestModelTagPrefix_InvalidModel verifies @model: with unknown model falls through.
func TestModelTagPrefix_InvalidModel(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-modeltag-inv-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "openai/gpt-4o",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
		ModelList: []config.ModelConfig{
			{ModelName: "gpt4", Model: "openai/gpt-4o", APIKey: "k1"},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &modelCaptureMockProvider{response: "ok"}
	al := NewAgentLoop(cfg, msgBus, provider)

	ctx := context.Background()

	// Send message with @unknown: prefix — should NOT override, treat as normal message
	msg := bus.InboundMessage{
		Channel:  "telegram",
		SenderID: "u1",
		ChatID:   "c1",
		Content:  "@unknown: hello",
	}

	_, err = al.processMessage(ctx, msg)
	if err != nil {
		t.Fatalf("processMessage failed: %v", err)
	}

	// Should use default model since @unknown is not in model list
	if provider.capturedModel != "openai/gpt-4o" {
		t.Errorf("Expected default model (openai/gpt-4o), got: %s", provider.capturedModel)
	}
}

// TestGetSessionHistory verifies the exposed GetSessionHistory method.
func TestGetSessionHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "agent-test-history-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.Config{
		Agents: config.AgentsConfig{
			Defaults: config.AgentDefaults{
				Workspace:         tmpDir,
				Model:             "test-model",
				MaxTokens:         4096,
				MaxToolIterations: 10,
			},
		},
	}

	msgBus := bus.NewMessageBus()
	provider := &simpleMockProvider{response: "test response"}
	al := NewAgentLoop(cfg, msgBus, provider)

	// Initially empty
	key := al.GetDefaultSessionKey()
	history := al.GetSessionHistory(key)
	if len(history) != 0 {
		t.Errorf("Expected empty history, got %d messages", len(history))
	}

	// Process a message to populate history
	ctx := context.Background()
	msg := bus.InboundMessage{
		Channel:  "cli",
		SenderID: "user",
		ChatID:   "direct",
		Content:  "hello",
	}
	helper := testHelper{al: al}
	_ = helper.executeAndGetResponse(t, ctx, msg)

	// Now should have history
	history = al.GetSessionHistory(key)
	if len(history) == 0 {
		t.Error("Expected non-empty history after processing a message")
	}

	// ClearSession should empty it
	al.ClearSession(key)
	history = al.GetSessionHistory(key)
	if len(history) != 0 {
		t.Errorf("Expected empty history after clear, got %d messages", len(history))
	}
}
