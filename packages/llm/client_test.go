package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// mockProvider implements Provider for testing with configurable behavior.
type mockProvider struct {
	name          string
	endpoint      string
	authHeadersFn func(apiKey string) map[string]string
	buildReqFn    func(ctx context.Context, req *GenerateRequest) ([]byte, error)
	parseRespFn   func(body []byte) (*GenerateResponse, error)
	parseErrFn    func(statusCode int, body []byte) error
	validateFn    func(model string) error
}

func (m *mockProvider) Name() string                          { return m.name }
func (m *mockProvider) Endpoint(model string) string          { return m.endpoint }
func (m *mockProvider) AuthHeaders(apiKey string) map[string]string { return m.authHeadersFn(apiKey) }
func (m *mockProvider) BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error) {
	return m.buildReqFn(ctx, req)
}
func (m *mockProvider) ParseResponse(body []byte) (*GenerateResponse, error) { return m.parseRespFn(body) }
func (m *mockProvider) ParseError(statusCode int, body []byte) error         { return m.parseErrFn(statusCode, body) }
func (m *mockProvider) ValidateModel(model string) error { return m.validateFn(model) }
func (m *mockProvider) ApplyConfig(cfg *Config)          {}

func TestLoadConfig_Success(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key-123")
	os.Setenv("LLM_BASE_URL", "https://custom.api.com/v1")
	os.Setenv("LLM_MODEL", "custom/model")
	os.Setenv("LLM_TIMEOUT", "60")
	os.Setenv("LLM_MAX_RETRIES", "5")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_BASE_URL")
	defer os.Unsetenv("LLM_MODEL")
	defer os.Unsetenv("LLM_TIMEOUT")
	defer os.Unsetenv("LLM_MAX_RETRIES")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.APIKey != "test-key-123" {
		t.Errorf("expected API key 'test-key-123', got %q", cfg.APIKey)
	}
	if cfg.BaseURL != "https://custom.api.com/v1" {
		t.Errorf("expected base URL 'https://custom.api.com/v1', got %q", cfg.BaseURL)
	}
	if cfg.Model != "custom/model" {
		t.Errorf("expected model 'custom/model', got %q", cfg.Model)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected timeout 60s, got %v", cfg.Timeout)
	}
	if cfg.MaxRetries != 5 {
		t.Errorf("expected max retries 5, got %d", cfg.MaxRetries)
	}
}

func TestLoadConfig_Defaults(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "test/model")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_MODEL")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.BaseURL != defaultBaseURL {
		t.Errorf("expected default base URL %q, got %q", defaultBaseURL, cfg.BaseURL)
	}
	if cfg.Model != "test/model" {
		t.Errorf("expected configured model %q, got %q", "test/model", cfg.Model)
	}
	if cfg.Timeout != defaultTimeout*time.Second {
		t.Errorf("expected default timeout %ds, got %v", defaultTimeout, cfg.Timeout)
	}
	if cfg.MaxRetries != defaultMaxRetries {
		t.Errorf("expected default max retries %d, got %d", defaultMaxRetries, cfg.MaxRetries)
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	os.Unsetenv("LLM_API_KEY")
	os.Setenv("LLM_MODEL", "test/model")
	defer os.Unsetenv("LLM_MODEL")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for missing API key")
	}
	expected := "validate API keys: LLM_API_KEY environment variable is required"
	if err.Error() != expected {
		t.Errorf("unexpected error message: %v, want %q", err, expected)
	}
}

func TestLoadConfig_MissingModel(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key")
	os.Unsetenv("LLM_MODEL")
	os.Unsetenv("LLM_PROVIDER")
	defer os.Unsetenv("LLM_API_KEY")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Model != defaultModel {
		t.Errorf("expected default model %q, got %q", defaultModel, cfg.Model)
	}
}

func TestLoadConfig_InvalidTimeout(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "test/model")
	os.Setenv("LLM_TIMEOUT", "invalid")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_MODEL")
	defer os.Unsetenv("LLM_TIMEOUT")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for invalid timeout")
	}
}

func TestLoadConfig_ProviderSettings(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "test/model")
	os.Setenv("LLM_PROVIDER_PRIORITY", "OpenAI, Anthropic")
	os.Setenv("LLM_ALLOW_FALLBACKS", "false")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_MODEL")
	defer os.Unsetenv("LLM_PROVIDER_PRIORITY")
	defer os.Unsetenv("LLM_ALLOW_FALLBACKS")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.ProviderPriority != "OpenAI, Anthropic" {
		t.Errorf("expected provider priority 'OpenAI, Anthropic', got %q", cfg.ProviderPriority)
	}
	if cfg.AllowFallbacks != "false" {
		t.Errorf("expected allow fallbacks 'false', got %q", cfg.AllowFallbacks)
	}
}

func TestLoadConfig_ProviderOnly(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key")
	os.Setenv("LLM_MODEL", "test/model")
	os.Setenv("LLM_PROVIDER_ONLY", "openai")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_MODEL")
	defer os.Unsetenv("LLM_PROVIDER_ONLY")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.ProviderOnly != "openai" {
		t.Errorf("expected provider only 'openai', got %q", cfg.ProviderOnly)
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				APIKey:     "key",
				BaseURL:    "https://api.com",
				Model:      "model",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: &Config{
				BaseURL:    "https://api.com",
				Model:      "model",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: true,
		},
		{
			name: "missing base URL",
			config: &Config{
				APIKey:     "key",
				Model:      "model",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: true,
		},
		{
			name: "missing model",
			config: &Config{
				APIKey:     "key",
				BaseURL:    "https://api.com",
				Timeout:    30 * time.Second,
				MaxRetries: 3,
			},
			wantErr: true,
		},
		{
			name: "zero timeout",
			config: &Config{
				APIKey:     "key",
				BaseURL:    "https://api.com",
				Model:      "model",
				Timeout:    0,
				MaxRetries: 3,
			},
			wantErr: true,
		},
		{
			name: "negative max retries",
			config: &Config{
				APIKey:     "key",
				BaseURL:    "https://api.com",
				Model:      "model",
				Timeout:    30 * time.Second,
				MaxRetries: -1,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRetryConfig_CalculateDelay(t *testing.T) {
	config := RetryConfig{
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
	}

	tests := []struct {
		attempt     int
		expectedMin time.Duration
		expectedMax time.Duration
	}{
		{0, 400 * time.Millisecond, 600 * time.Millisecond},
		{1, 900 * time.Millisecond, 1100 * time.Millisecond},
		{2, 1900 * time.Millisecond, 2100 * time.Millisecond},
		{3, 3900 * time.Millisecond, 4100 * time.Millisecond},
		{10, 9 * time.Second, 11 * time.Second},
	}

	for _, tt := range tests {
		delay := config.CalculateDelay(tt.attempt)
		if delay < tt.expectedMin || delay > tt.expectedMax {
			t.Errorf("attempt %d: expected delay between %v and %v, got %v",
				tt.attempt, tt.expectedMin, tt.expectedMax, delay)
		}
	}
}

func TestShouldRetry(t *testing.T) {
	maxRetries := 3

	tests := []struct {
		err       error
		attempt   int
		wantRetry bool
	}{
		{nil, 0, false},
		{NewNonRetryableError(fmt.Errorf("error")), 0, false},
		{NewRetryableError(fmt.Errorf("error"), true), 0, true},
		{NewRetryableError(fmt.Errorf("error"), true), 2, true},
		{NewRetryableError(fmt.Errorf("error"), true), 3, false},
		{NewRetryableError(fmt.Errorf("error"), false), 0, false},
	}

	for _, tt := range tests {
		retry, _ := ShouldRetry(tt.err, tt.attempt, maxRetries)
		if retry != tt.wantRetry {
			t.Errorf("attempt %d: expected retry=%v, got %v", tt.attempt, tt.wantRetry, retry)
		}
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []int{429, 500, 502, 503, 504, 200, 400, 404}
	expected := []bool{true, true, true, true, true, false, false, false}

	for i, code := range tests {
		if got := IsRetryableStatusCode(code); got != expected[i] {
			t.Errorf("code %d: expected %v, got %v", code, expected[i], got)
		}
	}
}

func TestParseStepsFromResponse(t *testing.T) {
	response := `[{"tool": "navigate", "params": {"url": "https://example.com"}, "reason": "Go to login page"}]`

	steps, err := ParseStepsFromResponse(response)
	if err != nil {
		t.Fatalf("ParseStepsFromResponse failed: %v", err)
	}

	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}

	if steps[0]["tool"] != "navigate" {
		t.Errorf("expected tool 'navigate', got %v", steps[0]["tool"])
	}
}

func TestParseStepsFromResponse_InvalidJSON(t *testing.T) {
	response := `not valid json`

	_, err := ParseStepsFromResponse(response)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseStepsFromResponse_WithCodeFence(t *testing.T) {
	response := "```json\n[{\"tool\":\"navigate\",\"params\":{\"url\":\"https://example.com\"},\"reason\":\"go\"}]\n```"
	_, err := ParseStepsFromResponse(response)
	if err == nil {
		t.Error("expected error for code-fence-wrapped response (prompt forbids markdown fences)")
	}
}

func TestBuildSystemPrompt(t *testing.T) {
	tools := []ToolInfo{
		{
			Name:        "navigate",
			Description: "Navigate to a URL",
			Parameters: map[string]ParameterInfo{
				"url": {Type: "string", Description: "The URL to navigate to", Required: true},
			},
		},
	}

	prompt := BuildSystemPrompt(tools, "")

	if len(prompt) == 0 {
		t.Error("expected non-empty prompt")
	}

	if !contains(prompt, "navigate") {
		t.Error("expected prompt to contain tool name")
	}

	if !contains(prompt, "observe_ui") {
		t.Error("expected prompt to contain observe_ui rule")
	}
}

func TestBuildUserPrompt(t *testing.T) {
	data := PlannerPromptData{
		Goal:        "Test login",
		History:     "Step 1: opened page",
		Observation: "Login form visible",
		Tools:       []ToolInfo{},
	}

	prompt := BuildUserPrompt(data)

	if !contains(prompt, "Test login") {
		t.Error("expected prompt to contain goal")
	}
	if !contains(prompt, "Step 1: opened page") {
		t.Error("expected prompt to contain history")
	}
	if !contains(prompt, "Login form visible") {
		t.Error("expected prompt to contain observation")
	}
}

func TestBuildSystemPrompt_No404Prompts(t *testing.T) {
	tools := []ToolInfo{
		{Name: "navigate", Description: "Navigate to a URL",
			Parameters: map[string]ParameterInfo{"url": {Type: "string", Required: true}}},
		{Name: "click", Description: "Click an element",
			Parameters: map[string]ParameterInfo{"selector": {Type: "string", Required: true}}},
	}
	prompt := BuildSystemPrompt(tools, "")

	if contains(prompt, "NEVER use navigate()") {
		t.Error("system prompt should NOT contain 404 URL ban — recovery is engine-level now")
	}
	if contains(prompt, "root domain") {
		t.Error("system prompt should NOT mention root domain — recovery is engine-level now")
	}
}

func TestBuildUserPrompt_No404NavBan(t *testing.T) {
	data := PlannerPromptData{
		Goal:        "Test login",
		History:     "Step 1: opened page",
		Observation: "Login form visible",
		Tools:       []ToolInfo{},
	}
	prompt := BuildUserPrompt(data)

	if contains(prompt, "NEVER use navigate()") {
		t.Error("user prompt should NOT contain 404 URL ban — recovery is engine-level now")
	}
	if contains(prompt, "extract the root domain") {
		t.Error("user prompt should NOT contain root domain extraction — recovery is engine-level now")
	}
}

func TestSanitizePromptField(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"null bytes removed", "abc\x00def", "abcdef"},
		{"code fences removed", "```code```", "code"},
		{"double braces escaped", "{{.History}}", "&#123;&#123;.History&#125;&#125;"},
		{"nested braces escaped", "{{.Observation}} test {{.Goal}}", "&#123;&#123;.Observation&#125;&#125; test &#123;&#123;.Goal&#125;&#125;"},
		{"normal text preserved", "Step 1: opened page", "Step 1: opened page"},
		{"mixed content", "before```{{.Tools}}```after", "before&#123;&#123;.Tools&#125;&#125;after"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizePromptField(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePromptField(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildUserPrompt_SanitizesAllFields(t *testing.T) {
	data := PlannerPromptData{
		Goal:        "Test login",
		History:     "Data: {{.Observation}} inject",
		Observation: "{{.Goal}} injected",
		Tools:       []ToolInfo{},
	}

	prompt := BuildUserPrompt(data)

	if contains(prompt, "{{.Observation}}") {
		t.Error("History should have {{.Observation}} escaped")
	}
	if contains(prompt, "{{.Goal}}") {
		t.Error("Observation should have {{.Goal}} escaped")
	}
	if !contains(prompt, "&#123;&#123;.Goal&#125;&#125;") {
		t.Error("expected Observation {{.Goal}} to be HTML-entity escaped")
	}
}

func TestSanitizeGoal_EscapesTags(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"closing tag escaped with entities", "test</script>", "test&lt;/script&gt;"},
		{"bare angle bracket escaped", "<script>alert(1)</script>", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{"ampersand escaped first", "a&b<c>d", "a&amp;b&lt;c&gt;d"},
		{"no special chars", "simple goal", "simple goal"},
		{"trimmed spaces", "  goal  ", "goal"},
		{"code fences removed", "```code```", "code"},
		{"null bytes removed", "abc\x00def", "abcdef"},
		{"mixed escaping", "goal</a>\x00```end", "goal&lt;/a&gt;end"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeGoal(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeGoal(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestEmptyResponseIsRetryable(t *testing.T) {
	// Start a test server that returns 200 OK with empty content.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"","object":"chat.completion","created":0,"model":"test","choices":[]}`))
	}))
	defer server.Close()

	provider := &mockProvider{
		name:     "test",
		endpoint: server.URL,
		authHeadersFn: func(apiKey string) map[string]string {
			return map[string]string{"Content-Type": "application/json"}
		},
		buildReqFn: func(ctx context.Context, req *GenerateRequest) ([]byte, error) {
			return json.Marshal(map[string]any{
				"model":    req.Model,
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			})
		},
		parseRespFn: func(body []byte) (*GenerateResponse, error) {
			return (&OpenAIProvider{}).ParseResponse(body)
		},
		parseErrFn: func(statusCode int, body []byte) error {
			return fmt.Errorf("API error: %s", string(body))
		},
		validateFn: func(model string) error { return nil },
	}

	client := &HTTPClient{
		config: &Config{
			APIKey:     "test-key",
			Model:      "test-model",
			MaxRetries: 2,
		},
		provider:   provider,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.Generate(context.Background(), &GenerateRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})

	if err == nil {
		t.Fatal("expected error from empty response, got nil")
	}
	if !IsRetryable(err) {
		t.Errorf("expected retryable error, got non-retryable: %v", err)
	}
	if !contains(err.Error(), "empty response") {
		t.Errorf("expected error to mention 'empty response', got: %v", err)
	}
}

func TestGenerate_MaxTokensNotDefaultedWithThinkingBudget(t *testing.T) {
	// When thinking budget is set, MaxTokens should NOT be defaulted to 1024.
	buildReqCalled := false
	provider := &mockProvider{
		name:     "test",
		endpoint: "http://localhost:9999/v1/chat/completions",
		authHeadersFn: func(apiKey string) map[string]string {
			return map[string]string{"Content-Type": "application/json"}
		},
		buildReqFn: func(ctx context.Context, req *GenerateRequest) ([]byte, error) {
			buildReqCalled = true
			if req.MaxTokens != 0 {
				t.Errorf("MaxTokens = %d, want 0 (should not be defaulted when thinking budget is set)", req.MaxTokens)
			}
			return json.Marshal(map[string]any{
				"model":    req.Model,
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			})
		},
		parseRespFn: func(body []byte) (*GenerateResponse, error) {
			return &GenerateResponse{
				Content: "response",
				Choices: []Choice{{Message: Message{Role: RoleAssistant, Content: "response"}}},
			}, nil
		},
		parseErrFn: func(statusCode int, body []byte) error {
			return fmt.Errorf("error")
		},
		validateFn: func(model string) error { return nil },
	}

	client := &HTTPClient{
		config: &Config{
			APIKey:     "test-key",
			Model:      "test-model",
			MaxRetries: 1,
		},
		provider:   provider,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.Generate(context.Background(), &GenerateRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "test"}},
		Thinking: &ThinkingConfig{Type: "enabled", BudgetTokens: 4000},
	})
	// Error is expected since the endpoint doesn't exist, but buildReqFn must be called.
	if !buildReqCalled {
		t.Error("buildReqFn was never called")
	}
	_ = err
}

func TestGenerate_MaxTokensNotDefaultedWithThinkingTypeNoBudget(t *testing.T) {
	// Thinking type "enabled" without budget should still suppress the 1024 default.
	buildReqCalled := false
	provider := &mockProvider{
		name:     "test",
		endpoint: "http://localhost:9999/v1/chat/completions",
		authHeadersFn: func(apiKey string) map[string]string {
			return map[string]string{"Content-Type": "application/json"}
		},
		buildReqFn: func(ctx context.Context, req *GenerateRequest) ([]byte, error) {
			buildReqCalled = true
			if req.MaxTokens != 0 {
				t.Errorf("MaxTokens = %d, want 0 (should not be defaulted when thinking is enabled)", req.MaxTokens)
			}
			return json.Marshal(map[string]any{
				"model":    req.Model,
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			})
		},
		parseRespFn: func(body []byte) (*GenerateResponse, error) {
			return &GenerateResponse{
				Content: "response",
				Choices: []Choice{{Message: Message{Role: RoleAssistant, Content: "response"}}},
			}, nil
		},
		parseErrFn: func(statusCode int, body []byte) error {
			return fmt.Errorf("error")
		},
		validateFn: func(model string) error { return nil },
	}

	client := &HTTPClient{
		config: &Config{
			APIKey:     "test-key",
			Model:      "test-model",
			MaxRetries: 1,
		},
		provider:   provider,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.Generate(context.Background(), &GenerateRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "test"}},
		Thinking: &ThinkingConfig{Type: "enabled"}, // no BudgetTokens
	})
	if !buildReqCalled {
		t.Error("buildReqFn was never called")
	}
	_ = err
}

func TestGenerate_MaxTokensDefaultedWithoutThinking(t *testing.T) {
	// Without thinking budget, MaxTokens should default to 1024.
	buildReqCalled := false
	provider := &mockProvider{
		name:     "test",
		endpoint: "http://localhost:9999/v1/chat/completions",
		authHeadersFn: func(apiKey string) map[string]string {
			return map[string]string{"Content-Type": "application/json"}
		},
		buildReqFn: func(ctx context.Context, req *GenerateRequest) ([]byte, error) {
			buildReqCalled = true
			if req.MaxTokens == 0 {
				t.Error("MaxTokens = 0, want 1024 (should be defaulted without thinking budget)")
			}
			if req.MaxTokens != 1024 {
				t.Errorf("MaxTokens = %d, want 1024", req.MaxTokens)
			}
			return json.Marshal(map[string]any{
				"model":    req.Model,
				"messages": []map[string]string{{"role": "user", "content": "test"}},
			})
		},
		parseRespFn: func(body []byte) (*GenerateResponse, error) {
			return &GenerateResponse{
				Content: "response",
				Choices: []Choice{{Message: Message{Role: RoleAssistant, Content: "response"}}},
			}, nil
		},
		parseErrFn: func(statusCode int, body []byte) error {
			return fmt.Errorf("error")
		},
		validateFn: func(model string) error { return nil },
	}

	client := &HTTPClient{
		config: &Config{
			APIKey:     "test-key",
			Model:      "test-model",
			MaxRetries: 1,
		},
		provider:   provider,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	_, err := client.Generate(context.Background(), &GenerateRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if !buildReqCalled {
		t.Error("buildReqFn was never called")
	}
	_ = err
}

func TestGenerate_DoesNotMutateCallerRequest(t *testing.T) {
	provider := &mockProvider{
		name:     "test",
		endpoint: "http://localhost:9999/v1/chat/completions",
		authHeadersFn: func(apiKey string) map[string]string {
			return map[string]string{"Content-Type": "application/json"}
		},
		buildReqFn: func(ctx context.Context, req *GenerateRequest) ([]byte, error) {
			if req.Model != "original-model" {
				t.Errorf("buildReq received Model=%q, want 'original-model'", req.Model)
			}
			return json.Marshal(map[string]any{"model": req.Model})
		},
		parseRespFn: func(body []byte) (*GenerateResponse, error) {
			return &GenerateResponse{Content: "response", Choices: []Choice{{Message: Message{Role: RoleAssistant, Content: "response"}}}}, nil
		},
		parseErrFn: func(statusCode int, body []byte) error {
			return fmt.Errorf("error")
		},
		validateFn: func(model string) error { return nil },
	}
	client := &HTTPClient{
		config: &Config{
			APIKey:     "test-key",
			Model:      "test-model",
			MaxRetries: 0,
		},
		provider:   provider,
		httpClient: &http.Client{Timeout: 5 * time.Second},
	}

	original := &GenerateRequest{
		Model:    "original-model",
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	}
	_, err := client.Generate(context.Background(), original)
	_ = err

	if original.Model != "original-model" {
		t.Errorf("caller's Model was mutated to %q", original.Model)
	}
}

func TestIsNetworkError_WithWrappedRetryableError(t *testing.T) {
	inner := NetworkError("connection refused")
	wrapped := NewRetryableError(inner, true)

	if !IsNetworkError(wrapped) {
		t.Error("IsNetworkError should unwrap RetryableError and detect NetworkError")
	}

	nonNetwork := NewRetryableError(fmt.Errorf("some other error"), true)
	if IsNetworkError(nonNetwork) {
		t.Error("IsNetworkError should return false for non-network errors wrapped in RetryableError")
	}
}

func TestIsAPIError_WithWrappedRetryableError(t *testing.T) {
	inner := &APIError{StatusCode: 429, Message: "rate limited"}
	wrapped := NewRetryableError(inner, true)

	if !IsAPIError(wrapped) {
		t.Error("IsAPIError should unwrap RetryableError and detect APIError")
	}

	nonAPI := NewRetryableError(fmt.Errorf("some other error"), true)
	if IsAPIError(nonAPI) {
		t.Error("IsAPIError should return false for non-API errors wrapped in RetryableError")
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
