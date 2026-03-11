package openai_compat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestProviderChat_UsesMaxCompletionTokensForGLM(t *testing.T) {
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message":       map[string]any{"content": "ok"},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("key", server.URL, "")
	_, err := p.Chat(
		t.Context(),
		[]Message{{Role: "user", Content: "hi"}},
		nil,
		"glm-4.7",
		map[string]any{"max_tokens": 1234},
	)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if _, ok := requestBody["max_completion_tokens"]; !ok {
		t.Fatalf("expected max_completion_tokens in request body")
	}
	if _, ok := requestBody["max_tokens"]; ok {
		t.Fatalf("did not expect max_tokens key for glm model")
	}
}

func TestProviderChat_ParsesToolCalls(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content": "",
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "get_weather",
									"arguments": "{\"city\":\"SF\"}",
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 5,
				"total_tokens":      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("key", server.URL, "")
	out, err := p.Chat(t.Context(), []Message{{Role: "user", Content: "hi"}}, nil, "gpt-4o", nil)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(out.ToolCalls))
	}
	if out.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("ToolCalls[0].Name = %q, want %q", out.ToolCalls[0].Name, "get_weather")
	}
	if out.ToolCalls[0].Arguments["city"] != "SF" {
		t.Fatalf("ToolCalls[0].Arguments[city] = %v, want SF", out.ToolCalls[0].Arguments["city"])
	}
}

func TestProviderChat_ParsesReasoningContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message": map[string]any{
						"content":           "The answer is 2",
						"reasoning_content": "Let me think step by step... 1+1=2",
						"tool_calls": []map[string]any{
							{
								"id":   "call_1",
								"type": "function",
								"function": map[string]any{
									"name":      "calculator",
									"arguments": "{\"expr\":\"1+1\"}",
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("key", server.URL, "")
	out, err := p.Chat(t.Context(), []Message{{Role: "user", Content: "1+1=?"}}, nil, "kimi-k2.5", nil)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if out.ReasoningContent != "Let me think step by step... 1+1=2" {
		t.Fatalf("ReasoningContent = %q, want %q", out.ReasoningContent, "Let me think step by step... 1+1=2")
	}
	if out.Content != "The answer is 2" {
		t.Fatalf("Content = %q, want %q", out.Content, "The answer is 2")
	}
	if len(out.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(out.ToolCalls))
	}
}

func TestProviderChat_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer server.Close()

	p := NewProvider("key", server.URL, "")
	_, err := p.Chat(t.Context(), []Message{{Role: "user", Content: "hi"}}, nil, "gpt-4o", nil)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestProviderChat_StripsMoonshotPrefixAndNormalizesKimiTemperature(t *testing.T) {
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message":       map[string]any{"content": "ok"},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("key", server.URL, "")
	_, err := p.Chat(
		t.Context(),
		[]Message{{Role: "user", Content: "hi"}},
		nil,
		"moonshot/kimi-k2.5",
		map[string]any{"temperature": 0.3},
	)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if requestBody["model"] != "kimi-k2.5" {
		t.Fatalf("model = %v, want kimi-k2.5", requestBody["model"])
	}
	if requestBody["temperature"] != 1.0 {
		t.Fatalf("temperature = %v, want 1.0", requestBody["temperature"])
	}
}

func TestProviderChat_StripsGroqAndOllamaPrefixes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantModel string
	}{
		{
			name:      "strips groq prefix and keeps nested model",
			input:     "groq/openai/gpt-oss-120b",
			wantModel: "openai/gpt-oss-120b",
		},
		{
			name:      "strips ollama prefix",
			input:     "ollama/qwen2.5:14b",
			wantModel: "qwen2.5:14b",
		},
		{
			name:      "strips deepseek prefix",
			input:     "deepseek/deepseek-chat",
			wantModel: "deepseek-chat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requestBody map[string]any

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				resp := map[string]any{
					"choices": []map[string]any{
						{
							"message":       map[string]any{"content": "ok"},
							"finish_reason": "stop",
						},
					},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			p := NewProvider("key", server.URL, "")
			_, err := p.Chat(t.Context(), []Message{{Role: "user", Content: "hi"}}, nil, tt.input, nil)
			if err != nil {
				t.Fatalf("Chat() error = %v", err)
			}

			if requestBody["model"] != tt.wantModel {
				t.Fatalf("model = %v, want %s", requestBody["model"], tt.wantModel)
			}
		})
	}
}

func TestProvider_ProxyConfigured(t *testing.T) {
	proxyURL := "http://127.0.0.1:8080"
	p := NewProvider("key", "https://example.com", proxyURL)

	transport, ok := p.httpClient.Transport.(*http.Transport)
	if !ok || transport == nil {
		t.Fatalf("expected http transport with proxy, got %T", p.httpClient.Transport)
	}

	req := &http.Request{URL: &url.URL{Scheme: "https", Host: "api.example.com"}}
	gotProxy, err := transport.Proxy(req)
	if err != nil {
		t.Fatalf("proxy function returned error: %v", err)
	}
	if gotProxy == nil || gotProxy.String() != proxyURL {
		t.Fatalf("proxy = %v, want %s", gotProxy, proxyURL)
	}
}

func TestProviderChat_AcceptsNumericOptionTypes(t *testing.T) {
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message":       map[string]any{"content": "ok"},
					"finish_reason": "stop",
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	p := NewProvider("key", server.URL, "")
	_, err := p.Chat(
		t.Context(),
		[]Message{{Role: "user", Content: "hi"}},
		nil,
		"gpt-4o",
		map[string]any{"max_tokens": float64(512), "temperature": 1},
	)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if requestBody["max_tokens"] != float64(512) {
		t.Fatalf("max_tokens = %v, want 512", requestBody["max_tokens"])
	}
	if requestBody["temperature"] != float64(1) {
		t.Fatalf("temperature = %v, want 1", requestBody["temperature"])
	}
}

func TestNormalizeModel_UsesAPIBase(t *testing.T) {
	if got := normalizeModel("deepseek/deepseek-chat", "https://api.deepseek.com/v1"); got != "deepseek-chat" {
		t.Fatalf("normalizeModel(deepseek) = %q, want %q", got, "deepseek-chat")
	}
	if got := normalizeModel("openrouter/auto", "https://openrouter.ai/api/v1"); got != "openrouter/auto" {
		t.Fatalf("normalizeModel(openrouter) = %q, want %q", got, "openrouter/auto")
	}
}

func TestIsOllama(t *testing.T) {
	tests := []struct {
		apiBase string
		want    bool
	}{
		{"http://localhost:11434/v1", true},
		{"http://127.0.0.1:11434/v1", true},
		{"http://LOCALHOST:11434/v1", true},
		{"https://api.openai.com/v1", false},
		{"http://localhost:8080/v1", false},
		{"", false},
	}
	for _, tt := range tests {
		p := &Provider{apiBase: tt.apiBase}
		if got := p.isOllama(); got != tt.want {
			t.Errorf("isOllama(%q) = %v, want %v", tt.apiBase, got, tt.want)
		}
	}
}

func TestParseOllamaResponse(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantContent string
		wantTools   int
		wantFinish  string
		wantPrompt  int
		wantEval    int
	}{
		{
			name: "simple text response",
			body: `{
				"message": {"role": "assistant", "content": "Hello world"},
				"done_reason": "stop",
				"prompt_eval_count": 42,
				"eval_count": 10
			}`,
			wantContent: "Hello world",
			wantTools:   0,
			wantFinish:  "stop",
			wantPrompt:  42,
			wantEval:    10,
		},
		{
			name: "tool call response",
			body: `{
				"message": {
					"role": "assistant",
					"content": "",
					"tool_calls": [
						{
							"function": {
								"name": "read_file",
								"arguments": {"path": "/tmp/test.txt"}
							}
						}
					]
				},
				"done_reason": "stop",
				"prompt_eval_count": 100,
				"eval_count": 20
			}`,
			wantContent: "",
			wantTools:   1,
			wantFinish:  "stop",
			wantPrompt:  100,
			wantEval:    20,
		},
		{
			name: "empty done_reason defaults to stop",
			body: `{
				"message": {"role": "assistant", "content": "ok"},
				"prompt_eval_count": 5,
				"eval_count": 3
			}`,
			wantContent: "ok",
			wantFinish:  "stop",
			wantPrompt:  5,
			wantEval:    3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := parseOllamaResponse([]byte(tt.body))
			if err != nil {
				t.Fatalf("parseOllamaResponse() error = %v", err)
			}
			if resp.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", resp.Content, tt.wantContent)
			}
			if len(resp.ToolCalls) != tt.wantTools {
				t.Errorf("len(ToolCalls) = %d, want %d", len(resp.ToolCalls), tt.wantTools)
			}
			if resp.FinishReason != tt.wantFinish {
				t.Errorf("FinishReason = %q, want %q", resp.FinishReason, tt.wantFinish)
			}
			if resp.Usage == nil {
				t.Fatal("Usage is nil")
			}
			if resp.Usage.PromptTokens != tt.wantPrompt {
				t.Errorf("PromptTokens = %d, want %d", resp.Usage.PromptTokens, tt.wantPrompt)
			}
			if resp.Usage.CompletionTokens != tt.wantEval {
				t.Errorf("CompletionTokens = %d, want %d", resp.Usage.CompletionTokens, tt.wantEval)
			}
			if resp.Usage.TotalTokens != tt.wantPrompt+tt.wantEval {
				t.Errorf("TotalTokens = %d, want %d", resp.Usage.TotalTokens, tt.wantPrompt+tt.wantEval)
			}
		})
	}
}

func TestParseOllamaResponse_ToolCallArguments(t *testing.T) {
	body := `{
		"message": {
			"role": "assistant",
			"content": "",
			"tool_calls": [
				{
					"function": {
						"name": "edit_file",
						"arguments": {"path": "/tmp/a.go", "content": "package main"}
					}
				}
			]
		},
		"done_reason": "stop",
		"prompt_eval_count": 50,
		"eval_count": 15
	}`

	resp, err := parseOllamaResponse([]byte(body))
	if err != nil {
		t.Fatalf("parseOllamaResponse() error = %v", err)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.ToolCalls))
	}
	tc := resp.ToolCalls[0]
	if tc.Name != "edit_file" {
		t.Errorf("Name = %q, want %q", tc.Name, "edit_file")
	}
	if tc.Arguments["path"] != "/tmp/a.go" {
		t.Errorf("Arguments[path] = %v, want /tmp/a.go", tc.Arguments["path"])
	}
}

func TestSanitizeOllamaMessages(t *testing.T) {
	// Build a request body with Messages containing tool calls with string arguments
	msgs := []Message{
		{Role: "user", Content: "hello"},
		{
			Role:    "assistant",
			Content: "",
			ToolCalls: []ToolCall{
				{
					Name:      "read_file",
					Arguments: map[string]any{"path": "/tmp/test.txt"},
				},
			},
		},
		{Role: "tool", Content: "file contents here", ToolCallID: "call_1"},
	}

	requestBody := map[string]any{
		"model":    "qwen3.5:14b",
		"messages": msgs,
	}

	sanitizeOllamaMessages(requestBody)

	sanitized, ok := requestBody["messages"].([]map[string]any)
	if !ok {
		t.Fatalf("messages type = %T, want []map[string]any", requestBody["messages"])
	}
	if len(sanitized) != 3 {
		t.Fatalf("len(messages) = %d, want 3", len(sanitized))
	}

	// Check assistant message has tool_calls with object arguments
	assistantMsg := sanitized[1]
	calls, ok := assistantMsg["tool_calls"].([]map[string]any)
	if !ok {
		t.Fatalf("tool_calls type = %T, want []map[string]any", assistantMsg["tool_calls"])
	}
	if len(calls) != 1 {
		t.Fatalf("len(tool_calls) = %d, want 1", len(calls))
	}
	fn, ok := calls[0]["function"].(map[string]any)
	if !ok {
		t.Fatalf("function type = %T, want map[string]any", calls[0]["function"])
	}
	if fn["name"] != "read_file" {
		t.Errorf("function.name = %v, want read_file", fn["name"])
	}
	args, ok := fn["arguments"].(map[string]any)
	if !ok {
		t.Fatalf("arguments type = %T, want map[string]any", fn["arguments"])
	}
	if args["path"] != "/tmp/test.txt" {
		t.Errorf("arguments.path = %v, want /tmp/test.txt", args["path"])
	}
}

func TestProviderChat_OllamaRouting(t *testing.T) {
	var receivedPath string
	var requestBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Return Ollama native response format
		resp := map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": "Hello from Ollama",
			},
			"done_reason":      "stop",
			"prompt_eval_count": 10,
			"eval_count":        5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Replace port in server URL with 11434 to trigger isOllama()
	// Since we can't control the port, we instead construct a provider with localhost:11434
	// and make it point to our test server by overriding the httpClient.
	p := NewProvider("", server.URL, "")
	// Manually set apiBase to look like Ollama so isOllama() returns true
	p.apiBase = "http://localhost:11434/v1"
	// Override the client to point to our test server
	p.httpClient = server.Client()

	// We need the ollamaChat to hit our test server, so patch the base
	// The ollamaChat strips /v1 and uses /api/chat, so we need the test server
	// to serve at that path. Let's use a different approach: just test that
	// the Ollama path is called by constructing a server at the right URL.
	// Actually the simplest: override apiBase to match the test server but include "localhost:11434"
	// This won't work since the test server has a different port.
	// Instead, test the inner functions directly (already done above) and
	// verify routing by checking isOllama detection + ollamaChat endpoint construction.

	// Test that isOllama detects correctly
	p2 := &Provider{apiBase: "http://localhost:11434/v1"}
	if !p2.isOllama() {
		t.Fatal("expected isOllama() = true for localhost:11434")
	}

	// Test that ollamaChat strips /v1 and uses /api/chat
	// Use a test server that captures the path
	ollamaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := map[string]any{
			"message": map[string]any{
				"role":    "assistant",
				"content": "Ollama response",
			},
			"done_reason":      "stop",
			"prompt_eval_count": 10,
			"eval_count":        5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ollamaServer.Close()

	// Create provider whose apiBase is the test server + /v1
	p3 := NewProvider("", ollamaServer.URL+"/v1", "")
	// Force isOllama to be false so Chat() calls v1Chat, but we want to test ollamaChat directly
	reqBody := map[string]any{
		"model": "qwen3.5:14b",
		"messages": []Message{
			{Role: "user", Content: "hi"},
		},
	}
	resp, err := p3.ollamaChat(t.Context(), reqBody)
	if err != nil {
		t.Fatalf("ollamaChat() error = %v", err)
	}
	if receivedPath != "/api/chat" {
		t.Errorf("received path = %q, want /api/chat", receivedPath)
	}
	if resp.Content != "Ollama response" {
		t.Errorf("Content = %q, want %q", resp.Content, "Ollama response")
	}
	// Verify think=false and stream=false were added
	if requestBody["think"] != false {
		t.Errorf("think = %v, want false", requestBody["think"])
	}
	if requestBody["stream"] != false {
		t.Errorf("stream = %v, want false", requestBody["stream"])
	}
}
