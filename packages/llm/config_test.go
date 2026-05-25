package llm

import (
	"os"
	"reflect"
	"testing"
	"time"
)

func setEnv(key, value string) func() {
	old := os.Getenv(key)
	os.Setenv(key, value)
	return func() { os.Setenv(key, old) }
}

func TestConfigFallbackModels(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()

	tests := []struct {
		name     string
		envValue string
		unset    bool
		want     []string
	}{
		{
			name:  "Default",
			unset: true,
			want:  []string{"openai/gpt-4o-mini", "gemini/gemini-2.0-flash-001"},
		},
		{
			name:     "Custom",
			envValue: " gemini-2.0-flash , gpt-4o-mini ",
			want:     []string{"gemini-2.0-flash", "gpt-4o-mini"},
		},
		{
			name:     "Empty",
			envValue: "",
			want:     []string{"openai/gpt-4o-mini", "gemini/gemini-2.0-flash-001"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.unset {
				os.Unsetenv("LLM_FALLBACK_MODELS")
			} else {
				os.Setenv("LLM_FALLBACK_MODELS", tt.envValue)
			}

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error: %v", err)
			}
			if !reflect.DeepEqual(cfg.FallbackModels, tt.want) {
				t.Errorf("FallbackModels = %v, want %v", cfg.FallbackModels, tt.want)
			}
		})
	}
}

func TestConfigReasoningDefaults(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	defer setEnv("LLM_REASONING_EFFORT", "high")()
	defer setEnv("LLM_THINKING_TYPE", "enabled")()
	defer setEnv("LLM_THINKING_BUDGET", "4000")()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.ReasoningEffort != "high" {
		t.Errorf("ReasoningEffort = %q, want 'high'", cfg.ReasoningEffort)
	}
	if cfg.ThinkingType != "enabled" {
		t.Errorf("ThinkingType = %q, want 'enabled'", cfg.ThinkingType)
	}
	if cfg.ThinkingBudget != 4000 {
		t.Errorf("ThinkingBudget = %d, want 4000", cfg.ThinkingBudget)
	}
}

func TestConfigReasoningDefaultsEmpty(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	os.Unsetenv("LLM_REASONING_EFFORT")
	os.Unsetenv("LLM_THINKING_TYPE")
	os.Unsetenv("LLM_THINKING_BUDGET")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}

	if cfg.ReasoningEffort != "" {
		t.Errorf("expected empty ReasoningEffort, got %q", cfg.ReasoningEffort)
	}
	if cfg.ThinkingType != "" {
		t.Errorf("expected empty ThinkingType, got %q", cfg.ThinkingType)
	}
	if cfg.ThinkingBudget != 0 {
		t.Errorf("expected ThinkingBudget 0, got %d", cfg.ThinkingBudget)
	}
}

func TestConfigMaxRetriesExceedsLimit(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	defer setEnv("LLM_MAX_RETRIES", "21")()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for MaxRetries=21 (exceeds max 20)")
	}
}

func TestConfigMaxRetriesNegative(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	defer setEnv("LLM_MAX_RETRIES", "-1")()

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for negative MaxRetries")
	}
}

func TestConfigMaxRetriesAtLimit(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	defer setEnv("LLM_MAX_RETRIES", "20")()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.MaxRetries != 20 {
		t.Errorf("MaxRetries = %d, want 20", cfg.MaxRetries)
	}
}

func TestConfigValidateMaxRetriesExceedsLimit(t *testing.T) {
	cfg := &Config{
		APIKey:     "test-key",
		BaseURL:    "https://example.com",
		Model:      "test-model",
		Timeout:    30 * time.Second,
		MaxRetries: 25,
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for MaxRetries=25 in Validate")
	}
}
