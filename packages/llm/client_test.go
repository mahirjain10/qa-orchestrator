package llm

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

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
	if err.Error() != "LLM_API_KEY environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoadConfig_MissingModel(t *testing.T) {
	os.Setenv("LLM_API_KEY", "test-key")
	os.Unsetenv("LLM_MODEL")
	defer os.Unsetenv("LLM_API_KEY")

	_, err := LoadConfig()
	if err == nil {
		t.Error("expected error for missing model")
	}
	if err.Error() != "LLM_MODEL environment variable is required" {
		t.Errorf("unexpected error message: %v", err)
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
	os.Setenv("LLM_PROVIDER_ALLOW", "false")
	defer os.Unsetenv("LLM_API_KEY")
	defer os.Unsetenv("LLM_MODEL")
	defer os.Unsetenv("LLM_PROVIDER_PRIORITY")
	defer os.Unsetenv("LLM_PROVIDER_ALLOW")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.ProviderPriority != "OpenAI, Anthropic" {
		t.Errorf("expected provider priority 'OpenAI, Anthropic', got %q", cfg.ProviderPriority)
	}
	if cfg.ProviderAllow != "false" {
		t.Errorf("expected provider allow 'false', got %q", cfg.ProviderAllow)
	}
}

func TestParseErrorResponse_Standard(t *testing.T) {
	json := `{"error": {"message": "Invalid API key", "type": "invalid_request_error", "code": "invalid_api_key"}}`
	resp, err := parseErrorResponse(strings.NewReader(json))
	if err != nil {
		t.Fatalf("parseErrorResponse failed: %v", err)
	}
	if resp.Error.Message != "Invalid API key" {
		t.Errorf("expected message 'Invalid API key', got %q", resp.Error.Message)
	}
	if resp.Error.Code != "invalid_api_key" {
		t.Errorf("expected code 'invalid_api_key', got %q", resp.Error.Code)
	}
}

func TestParseErrorResponse_Nested(t *testing.T) {
	json := `{"error": {"message": "OpenRouter error", "inner_error": {"message": "Provider failed"}}}`
	resp, err := parseErrorResponse(strings.NewReader(json))
	if err != nil {
		t.Fatalf("parseErrorResponse failed: %v", err)
	}
	// It should prioritize the top-level message if present in the first unmarshal
	if resp.Error.Message != "OpenRouter error" {
		t.Errorf("expected message 'OpenRouter error', got %q", resp.Error.Message)
	}
}

func TestParseErrorResponse_Inner(t *testing.T) {
	json := `{"error": {"inner_error": {"message": "Provider failed"}}}`
	resp, err := parseErrorResponse(strings.NewReader(json))
	if err != nil {
		t.Fatalf("parseErrorResponse failed: %v", err)
	}
	if resp.Error.Message != "Provider failed" {
		t.Errorf("expected message 'Provider failed', got %q", resp.Error.Message)
	}
	if resp.Error.Type != "provider_error" {
		t.Errorf("expected type 'provider_error', got %q", resp.Error.Type)
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

func TestConfig_Endpoint(t *testing.T) {
	cfg := &Config{
		BaseURL: "https://openrouter.ai/api/v1",
	}
	expected := "https://openrouter.ai/api/v1/chat/completions"
	if cfg.Endpoint() != expected {
		t.Errorf("expected endpoint %q, got %q", expected, cfg.Endpoint())
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
	steps, err := ParseStepsFromResponse(response)
	if err != nil {
		t.Fatalf("ParseStepsFromResponse failed: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
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

	prompt := BuildSystemPrompt(tools)

	if len(prompt) == 0 {
		t.Error("expected non-empty prompt")
	}

	if !contains(prompt, "navigate") {
		t.Error("expected prompt to contain tool name")
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

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
