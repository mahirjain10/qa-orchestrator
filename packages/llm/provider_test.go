package llm

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestGetProvider_OpenAI(t *testing.T) {
	p, err := GetProvider("openai")
	if err != nil {
		t.Fatalf("GetProvider(openai) failed: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected name 'openai', got %q", p.Name())
	}
}

func TestGetProvider_OpenRouter(t *testing.T) {
	p, err := GetProvider("openrouter")
	if err != nil {
		t.Fatalf("GetProvider(openrouter) failed: %v", err)
	}
	if p.Name() != "openrouter" {
		t.Errorf("expected name 'openrouter', got %q", p.Name())
	}
}

func TestGetProvider_Gemini(t *testing.T) {
	p, err := GetProvider("gemini")
	if err != nil {
		t.Fatalf("GetProvider(gemini) failed: %v", err)
	}
	if p.Name() != "gemini" {
		t.Errorf("expected name 'gemini', got %q", p.Name())
	}
}

func TestGetProvider_Unknown(t *testing.T) {
	_, err := GetProvider("unknown")
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
	if !strings.Contains(err.Error(), "unknown provider") {
		t.Errorf("expected 'unknown provider' error, got %v", err)
	}
}

func TestDetectProvider_GeminiModels(t *testing.T) {
	models := []string{"gemini-2.0-flash", "gemini-2.5-pro", "google/gemini-pro"}
	for _, model := range models {
		p := DetectProvider(model)
		if p.Name() != "gemini" {
			t.Errorf("DetectProvider(%s) = %q, want 'gemini'", model, p.Name())
		}
	}
}

func TestDetectProvider_OpenRouterModels(t *testing.T) {
	models := []string{"openai/gpt-4o-mini", "anthropic/claude-3.5-sonnet", "meta/llama-3"}
	for _, model := range models {
		p := DetectProvider(model)
		if p.Name() != "openrouter" {
			t.Errorf("DetectProvider(%s) = %q, want 'openrouter'", model, p.Name())
		}
	}
}

func TestDetectProvider_Empty(t *testing.T) {
	p := DetectProvider("")
	if p.Name() != "openrouter" {
		t.Errorf("DetectProvider('') = %q, want 'openrouter'", p.Name())
	}
}

func TestOpenAIProvider_Endpoint(t *testing.T) {
	p := &OpenAIProvider{}
	endpoint := p.Endpoint("gpt-4o-mini")
	expected := "https://api.openai.com/v1/chat/completions"
	if endpoint != expected {
		t.Errorf("endpoint = %q, want %q", endpoint, expected)
	}
}

func TestOpenAIProvider_AuthHeaders(t *testing.T) {
	p := &OpenAIProvider{}
	headers := p.AuthHeaders("test-key")
	if headers["Authorization"] != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want 'Bearer test-key'", headers["Authorization"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header = %q, want 'application/json'", headers["Content-Type"])
	}
}

func TestOpenAIProvider_BuildRequest_WithTemperature(t *testing.T) {
	p := &OpenAIProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "You are helpful"},
			{Role: RoleUser, Content: "Hello"},
		},
		Model:       "gpt-4o-mini",
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if req["model"] != "gpt-4o-mini" {
		t.Errorf("model = %v, want 'gpt-4o-mini'", req["model"])
	}

	msgs := req["messages"].([]any)
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	firstMsg := msgs[0].(map[string]any)
	if firstMsg["role"] != "system" {
		t.Errorf("first message role = %q, want 'system'", firstMsg["role"])
	}

	_, hasMaxTokens := req["max_tokens"]
	if hasMaxTokens {
		t.Error("OpenAI request must NOT include 'max_tokens' (newer models reject it)")
	}

	val, hasMaxCompletionTokens := req["max_completion_tokens"]
	if !hasMaxCompletionTokens {
		t.Error("OpenAI request must include 'max_completion_tokens'")
	} else if int(val.(float64)) != 100 {
		t.Errorf("max_completion_tokens = %v, want 100", val)
	}

	temp, hasTemp := req["temperature"]
	if !hasTemp {
		t.Error("non-reasoning model request must include 'temperature'")
	} else if temp.(float64) != 0.7 {
		t.Errorf("temperature = %v, want 0.7", temp)
	}
}

func TestOpenAIProvider_BuildRequest_ReasoningModelOmitsTemperature(t *testing.T) {
	p := &OpenAIProvider{}

	reasoningModels := []string{"o1-mini", "o3-mini", "gpt-5-mini"}
	for _, model := range reasoningModels {
		body, err := p.BuildRequest(context.Background(), &GenerateRequest{
			Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
			Model:       model,
			Temperature: 0.7,
			MaxTokens:   100,
		})
		if err != nil {
			t.Fatalf("BuildRequest(%s) failed: %v", model, err)
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		_, hasTemp := req["temperature"]
		if hasTemp {
			t.Errorf("%s: must NOT include 'temperature' (reasoning model rejects it)", model)
		}

		if req["model"] != model {
			t.Errorf("model = %v, want %q", req["model"], model)
		}
	}
}

func TestOpenAIProvider_ParseResponse(t *testing.T) {
	p := &OpenAIProvider{}
	jsonBody := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"model": "gpt-4o-mini",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Hello there!"},
			"finish_reason": "stop"
		}],
		"usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
	}`

	resp, err := p.ParseResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Content != "Hello there!" {
		t.Errorf("content = %q, want 'Hello there!'", resp.Content)
	}
	if resp.Model != "gpt-4o-mini" {
		t.Errorf("model = %q, want 'gpt-4o-mini'", resp.Model)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("total_tokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

func TestOpenRouterProvider_Endpoint(t *testing.T) {
	p := &OpenRouterProvider{}
	endpoint := p.Endpoint("openai/gpt-4o-mini")
	expected := "https://openrouter.ai/api/v1/chat/completions"
	if endpoint != expected {
		t.Errorf("endpoint = %q, want %q", endpoint, expected)
	}
}

func TestOpenRouterProvider_AuthHeaders(t *testing.T) {
	p := &OpenRouterProvider{
		HTTPReferer: "https://example.com",
		AppTitle:    "Test App",
	}
	headers := p.AuthHeaders("test-key")
	if headers["Authorization"] != "Bearer test-key" {
		t.Errorf("Authorization header = %q, want 'Bearer test-key'", headers["Authorization"])
	}
	if headers["HTTP-Referer"] != "https://example.com" {
		t.Errorf("HTTP-Referer header = %q, want 'https://example.com'", headers["HTTP-Referer"])
	}
	if headers["X-Title"] != "Test App" {
		t.Errorf("X-Title header = %q, want 'Test App'", headers["X-Title"])
	}
}

func TestOpenRouterProvider_BuildRequest(t *testing.T) {
	p := &OpenRouterProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
		Model:       "openai/gpt-4o-mini",
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	_, hasMaxCompletionTokens := req["max_completion_tokens"]
	if hasMaxCompletionTokens {
		t.Error("OpenRouter request must NOT include 'max_completion_tokens' (some backends reject it)")
	}

	val, hasMaxTokens := req["max_tokens"]
	if !hasMaxTokens {
		t.Error("OpenRouter request must include 'max_tokens'")
	} else if int(val.(float64)) != 100 {
		t.Errorf("max_tokens = %v, want 100", val)
	}

	if req["model"] != "openai/gpt-4o-mini" {
		t.Errorf("model = %v, want 'openai/gpt-4o-mini'", req["model"])
	}
}

func TestOpenRouterProvider_BuildRequest_WithProviderSettings(t *testing.T) {
	p := &OpenRouterProvider{
		Provider: &ProviderSettings{
			Only: []string{"OpenAI"},
		},
	}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
		Model:       "openai/gpt-4o-mini",
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	provider, ok := req["provider"]
	if !ok {
		t.Fatal("expected 'provider' field in request")
	}

	providerMap := provider.(map[string]any)
	only := providerMap["only"].([]any)
	if len(only) != 1 || only[0] != "OpenAI" {
		t.Errorf("provider.only = %v, want ['OpenAI']", only)
	}
}

func TestOpenRouterProvider_ApplyProviderSettings(t *testing.T) {
	p := &OpenRouterProvider{}
	p.ApplyProviderSettings("OpenAI,Anthropic", "openai", "false")

	if p.Provider == nil {
		t.Fatal("expected provider to be set")
	}
	if len(p.Provider.Order) != 2 {
		t.Errorf("expected 2 order items, got %d", len(p.Provider.Order))
	}
	if len(p.Provider.Only) != 1 || p.Provider.Only[0] != "openai" {
		t.Errorf("expected only=['openai'], got %v", p.Provider.Only)
	}
	if p.Provider.AllowFallbacks == nil || *p.Provider.AllowFallbacks != false {
		t.Errorf("expected allow_fallbacks=false")
	}
}

func TestGeminiProvider_Endpoint(t *testing.T) {
	p := &GeminiProvider{}

	tests := []struct {
		model    string
		expected string
	}{
		{
			model:    "gemini-2.0-flash",
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		},
		{
			model:    "gemini/gemini-2.5-pro",
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-pro:generateContent",
		},
		{
			model:    "google/gemini-pro",
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-pro:generateContent",
		},
		{
			model:    "",
			expected: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent",
		},
	}

	for _, tt := range tests {
		endpoint := p.Endpoint(tt.model)
		if endpoint != tt.expected {
			t.Errorf("Endpoint(%q) = %q, want %q", tt.model, endpoint, tt.expected)
		}
	}
}

func TestGeminiProvider_AuthHeaders(t *testing.T) {
	p := &GeminiProvider{}
	headers := p.AuthHeaders("gemini-key")
	if headers["x-goog-api-key"] != "gemini-key" {
		t.Errorf("x-goog-api-key header = %q, want 'gemini-key'", headers["x-goog-api-key"])
	}
	if headers["Content-Type"] != "application/json" {
		t.Errorf("Content-Type header = %q, want 'application/json'", headers["Content-Type"])
	}
}

func TestGeminiProvider_BuildRequest(t *testing.T) {
	p := &GeminiProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages: []Message{
			{Role: RoleSystem, Content: "You are helpful"},
			{Role: RoleUser, Content: "Hello"},
			{Role: RoleAssistant, Content: "Hi there!"},
		},
		Model:       "gemini-2.0-flash",
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	sysInstr, ok := req["systemInstruction"]
	if !ok {
		t.Fatal("expected 'systemInstruction' field")
	}
	sysMap := sysInstr.(map[string]any)
	sysParts := sysMap["parts"].([]any)
	if len(sysParts) != 1 {
		t.Errorf("expected 1 system instruction part, got %d", len(sysParts))
	}

	contents := req["contents"].([]any)
	if len(contents) != 2 {
		t.Fatalf("expected 2 content items, got %d", len(contents))
	}

	firstContent := contents[0].(map[string]any)
	if firstContent["role"] != "user" {
		t.Errorf("first content role = %q, want 'user'", firstContent["role"])
	}

	secondContent := contents[1].(map[string]any)
	if secondContent["role"] != "model" {
		t.Errorf("second content role = %q, want 'model'", secondContent["role"])
	}

	config := req["generationConfig"].(map[string]any)
	if config["temperature"].(float64) != 0.7 {
		t.Errorf("temperature = %v, want 0.7", config["temperature"])
	}
}

func TestGeminiProvider_ParseResponse(t *testing.T) {
	p := &GeminiProvider{}
	jsonBody := `{
		"candidates": [{
			"content": {
				"parts": [{"text": "Hello from Gemini!"}],
				"role": "model"
			},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {
			"promptTokenCount": 10,
			"candidatesTokenCount": 5,
			"totalTokenCount": 15
		},
		"modelVersion": "gemini-2.0-flash"
	}`

	resp, err := p.ParseResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Content != "Hello from Gemini!" {
		t.Errorf("content = %q, want 'Hello from Gemini!'", resp.Content)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("total_tokens = %d, want 15", resp.Usage.TotalTokens)
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("prompt_tokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].FinishReason != "STOP" {
		t.Errorf("finish_reason = %q, want 'STOP'", resp.Choices[0].FinishReason)
	}
}

func TestGeminiProvider_ParseResponse_EmptyCandidates(t *testing.T) {
	p := &GeminiProvider{}
	jsonBody := `{"candidates": []}`

	_, err := p.ParseResponse([]byte(jsonBody))
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
	if !strings.Contains(err.Error(), "no candidates") {
		t.Errorf("expected 'no candidates' error, got %v", err)
	}
}

func TestGeminiProvider_ParseError(t *testing.T) {
	p := &GeminiProvider{}
	jsonBody := `{
		"error": {
			"code": 400,
			"message": "API key not valid",
			"status": "INVALID_ARGUMENT"
		}
	}`

	err := p.ParseError(400, []byte(jsonBody))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "API key not valid") {
		t.Errorf("expected 'API key not valid' in error, got %v", err)
	}
}

func TestOpenAIProvider_ValidateModel(t *testing.T) {
	p := &OpenAIProvider{}
	if err := p.ValidateModel(""); err == nil {
		t.Error("expected error for empty model")
	}
	if err := p.ValidateModel("gpt-4o-mini"); err != nil {
		t.Errorf("unexpected error for valid model: %v", err)
	}
}

func TestGeminiProvider_ValidateModel(t *testing.T) {
	p := &GeminiProvider{}
	if err := p.ValidateModel(""); err == nil {
		t.Error("expected error for empty model")
	}
	if err := p.ValidateModel("gemini-2.0-flash"); err != nil {
		t.Errorf("unexpected error for valid model: %v", err)
	}
}

func TestOpenAIProvider_BuildRequest_WithReasoningEffort(t *testing.T) {
	p := &OpenAIProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:        []Message{{Role: RoleUser, Content: "Hello"}},
		Model:           "gpt-4o-mini",
		Temperature:     0.7,
		MaxTokens:       100,
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	effort, ok := req["reasoning_effort"]
	if !ok {
		t.Fatal("expected 'reasoning_effort' in request")
	}
	if effort != "high" {
		t.Errorf("reasoning_effort = %v, want 'high'", effort)
	}
}

func TestOpenAIProvider_BuildRequest_WithThinkingMapping(t *testing.T) {
	p := &OpenAIProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
		Model:    "gpt-4o-mini",
		Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 2000},
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Thinking should be mapped to reasoning_effort="high" since no explicit ReasoningEffort was set.
	effort, ok := req["reasoning_effort"]
	if !ok {
		t.Fatal("expected 'reasoning_effort' in request (mapped from Thinking)")
	}
	if effort != "high" {
		t.Errorf("reasoning_effort = %v, want 'high' (mapped from Thinking)", effort)
	}
}

func TestOpenAIProvider_BuildRequest_WithThinkingMappingRespectsExplicitEffort(t *testing.T) {
	p := &OpenAIProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:        []Message{{Role: RoleUser, Content: "Hello"}},
		Model:           "gpt-4o-mini",
		ReasoningEffort: "low",
		Thinking:        &ThinkingConfig{Type: "enabled"},
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	// Explicit ReasoningEffort should take precedence over Thinking mapping.
	effort, ok := req["reasoning_effort"]
	if !ok {
		t.Fatal("expected 'reasoning_effort' in request")
	}
	if effort != "low" {
		t.Errorf("reasoning_effort = %v, want 'low'", effort)
	}
}

func TestOpenRouterProvider_BuildRequest_WithReasoningEffort(t *testing.T) {
	p := &OpenRouterProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:        []Message{{Role: RoleUser, Content: "Hello"}},
		Model:           "openai/gpt-4o-mini",
		Temperature:     0.7,
		MaxTokens:       100,
		ReasoningEffort: "high",
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	effort, ok := req["reasoning_effort"]
	if !ok {
		t.Fatal("expected 'reasoning_effort' in request")
	}
	if effort != "high" {
		t.Errorf("reasoning_effort = %v, want 'high'", effort)
	}
}

func TestOpenRouterProvider_BuildRequest_WithThinking(t *testing.T) {
	p := &OpenRouterProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages: []Message{{Role: RoleUser, Content: "Hello"}},
		Model:    "deepseek/deepseek-v4-pro",
		MaxTokens: 4000,
		Thinking:  &ThinkingConfig{Type: "enabled", BudgetTokens: 2000},
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	thinking, ok := req["thinking"]
	if !ok {
		t.Fatal("expected 'thinking' in request")
	}
	thinkingMap := thinking.(map[string]any)
	if thinkingMap["type"] != "enabled" {
		t.Errorf("thinking.type = %v, want 'enabled'", thinkingMap["type"])
	}
	if int(thinkingMap["budget_tokens"].(float64)) != 2000 {
		t.Errorf("thinking.budget_tokens = %v, want 2000", thinkingMap["budget_tokens"])
	}
}

func TestOpenRouterProvider_BuildRequest_ReasoningModelOmitsTemperature(t *testing.T) {
	p := &OpenRouterProvider{}

	models := []string{"openai/o1-mini", "openai/o3-mini", "openai/gpt-5-mini"}
	for _, model := range models {
		body, err := p.BuildRequest(context.Background(), &GenerateRequest{
			Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
			Model:       model,
			Temperature: 0.7,
			MaxTokens:   100,
		})
		if err != nil {
			t.Fatalf("BuildRequest(%s) failed: %v", model, err)
		}

		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		_, hasTemp := req["temperature"]
		if hasTemp {
			t.Errorf("%s: must NOT include 'temperature' (reasoning model rejects it)", model)
		}
	}
}

func TestOpenRouterProvider_BuildRequest_NonReasoningModelKeepsTemperature(t *testing.T) {
	p := &OpenRouterProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
		Model:       "openai/gpt-4o-mini",
		Temperature: 0.7,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	temp, hasTemp := req["temperature"]
	if !hasTemp {
		t.Fatal("non-reasoning model must include 'temperature'")
	}
	if temp.(float64) != 0.7 {
		t.Errorf("temperature = %v, want 0.7", temp)
	}
}

func TestIsReasoningModel(t *testing.T) {
	tests := []struct {
		model    string
		expected bool
	}{
		// Direct model names
		{"o1-mini", true},
		{"o3-mini", true},
		{"o4-mini", true},
		{"gpt-5-mini", true},
		{"gpt-4o-mini", false},
		{"gpt-4o", false},
		// Provider-prefixed (as used with OpenRouter)
		{"openai/o1-mini", true},
		{"openai/o3-mini", true},
		{"openai/o4", true},
		{"openai/gpt-5-mini", true},
		{"openai/gpt-4o-mini", false},
		// Other providers
		{"deepseek/deepseek-v4-pro", false},
		{"anthropic/claude-sonnet-4", false},
		// Edge cases
		{"", false},
		{"o1", true},
		{"o1-xyz", true},
	}
	for _, tt := range tests {
		result := isReasoningModel(tt.model)
		if result != tt.expected {
			t.Errorf("isReasoningModel(%q) = %v, want %v", tt.model, result, tt.expected)
		}
	}
}

func TestParseOpenAIResponse_WithTopLevelReasoning(t *testing.T) {
	jsonBody := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"model": "openai/gpt-5-mini",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Final answer"},
			"finish_reason": "stop"
		}],
		"reasoning": "The model's chain of thought here..."
	}`

	resp, err := parseOpenAIResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Content != "Final answer" {
		t.Errorf("content = %q, want 'Final answer'", resp.Content)
	}
	if resp.Reasoning != "The model's chain of thought here..." {
		t.Errorf("reasoning = %q, want 'The model\\'s chain of thought here...'", resp.Reasoning)
	}
}

func TestParseOpenAIResponse_WithReasoningContent(t *testing.T) {
	jsonBody := `{
		"id": "chatcmpl-456",
		"object": "chat.completion",
		"model": "deepseek/deepseek-v4-pro",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Final answer",
				"reasoning_content": "DeepSeek chain of thought"
			},
			"finish_reason": "stop"
		}]
	}`

	resp, err := parseOpenAIResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Content != "Final answer" {
		t.Errorf("content = %q, want 'Final answer'", resp.Content)
	}
	if resp.Reasoning != "DeepSeek chain of thought" {
		t.Errorf("reasoning = %q, want 'DeepSeek chain of thought'", resp.Reasoning)
	}
}

func TestParseOpenAIResponse_PrefersTopLevelReasoning(t *testing.T) {
	// When both top-level "reasoning" and choice-level "reasoning_content" are present,
	// top-level reasoning takes precedence (OpenRouter returns both in some cases).
	jsonBody := `{
		"id": "chatcmpl-789",
		"model": "openai/o3-mini",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Answer",
				"reasoning_content": "choice-level reasoning"
			},
			"finish_reason": "stop"
		}],
		"reasoning": "top-level reasoning"
	}`

	resp, err := parseOpenAIResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Reasoning != "top-level reasoning" {
		t.Errorf("reasoning = %q, want 'top-level reasoning'", resp.Reasoning)
	}
}

func TestParseOpenAIResponse_NoReasoning(t *testing.T) {
	jsonBody := `{
		"id": "chatcmpl-123",
		"model": "gpt-4o-mini",
		"choices": [{
			"index": 0,
			"message": {"role": "assistant", "content": "Hello"},
			"finish_reason": "stop"
		}]
	}`

	resp, err := parseOpenAIResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Reasoning != "" {
		t.Errorf("expected empty reasoning for non-reasoning model, got %q", resp.Reasoning)
	}
}

func TestGeminiProvider_BuildRequest_WithThinking(t *testing.T) {
	p := &GeminiProvider{}
	body, err := p.BuildRequest(context.Background(), &GenerateRequest{
		Messages:    []Message{{Role: RoleUser, Content: "Hello"}},
		Model:       "gemini-2.5-pro",
		Temperature: 0.7,
		MaxTokens:   4000,
		Thinking:    &ThinkingConfig{Type: "enabled", BudgetTokens: 2000},
	})
	if err != nil {
		t.Fatalf("BuildRequest failed: %v", err)
	}

	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	config, ok := req["generationConfig"].(map[string]any)
	if !ok {
		t.Fatal("expected 'generationConfig' in request")
	}

	tc, ok := config["thinkingConfig"].(map[string]any)
	if !ok {
		t.Fatal("expected 'thinkingConfig' in generationConfig")
	}

	budget := int(tc["thinkingBudget"].(float64))
	if budget != 2000 {
		t.Errorf("thinkingBudget = %d, want 2000", budget)
	}

	if int(config["maxOutputTokens"].(float64)) != 4000 {
		t.Errorf("maxOutputTokens = %v, want 4000", config["maxOutputTokens"])
	}
}

func TestGeminiProvider_ParseResponse_WithThoughtParts(t *testing.T) {
	p := &GeminiProvider{}
	jsonBody := `{
		"candidates": [{
			"content": {
				"parts": [
					{"text": "Let me think step by step...", "thought": true},
					{"text": "The final answer is 42.", "thought": false}
				],
				"role": "model"
			},
			"finishReason": "STOP",
			"index": 0
		}],
		"usageMetadata": {
			"promptTokenCount": 20,
			"candidatesTokenCount": 15,
			"totalTokenCount": 35
		},
		"modelVersion": "gemini-2.5-pro"
	}`

	resp, err := p.ParseResponse([]byte(jsonBody))
	if err != nil {
		t.Fatalf("ParseResponse failed: %v", err)
	}

	if resp.Content != "The final answer is 42." {
		t.Errorf("content = %q, want 'The final answer is 42.'", resp.Content)
	}
	if resp.Reasoning != "Let me think step by step..." {
		t.Errorf("reasoning = %q, want 'Let me think step by step...'", resp.Reasoning)
	}
}

func TestDetectProviderName(t *testing.T) {
	tests := []struct {
		model    string
		expected string
	}{
		{"gpt-4o-mini", "openai"},
		{"gpt-4o", "openai"},
		{"o1-mini", "openai"},
		{"o3-mini", "openai"},
		{"gemini-2.0-flash", "gemini"},
		{"google/gemini-pro", "gemini"},
		{"openai/gpt-4o-mini", "openrouter"},
		{"anthropic/claude-3.5-sonnet", "openrouter"},
		{"", "openrouter"},
	}

	for _, tt := range tests {
		result := detectProviderName(tt.model)
		if result != tt.expected {
			t.Errorf("detectProviderName(%q) = %q, want %q", tt.model, result, tt.expected)
		}
	}
}
