package llm

import (
	"os"
	"testing"
)

func setEnv(key, value string) func() {
	old := os.Getenv(key)
	os.Setenv(key, value)
	return func() { os.Setenv(key, old) }
}

func TestConfigFallbackModelsDefault(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	os.Unsetenv("LLM_FALLBACK_MODELS")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.FallbackModels) == 0 {
		t.Error("expected non-empty fallback models")
	}
	if cfg.FallbackModels[0] != "openai/gpt-4o-mini" {
		t.Errorf("expected first fallback model 'openai/gpt-4o-mini', got %q", cfg.FallbackModels[0])
	}
}

func TestConfigFallbackModelsCustom(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	defer setEnv("LLM_FALLBACK_MODELS", " gemini-2.0-flash , gpt-4o-mini ")()

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.FallbackModels) != 2 {
		t.Fatalf("expected 2 fallback models, got %d: %v", len(cfg.FallbackModels), cfg.FallbackModels)
	}
	if cfg.FallbackModels[0] != "gemini-2.0-flash" {
		t.Errorf("expected first fallback model 'gemini-2.0-flash', got %q", cfg.FallbackModels[0])
	}
	if cfg.FallbackModels[1] != "gpt-4o-mini" {
		t.Errorf("expected second fallback model 'gpt-4o-mini', got %q", cfg.FallbackModels[1])
	}
}

func TestConfigFallbackModelsEmpty(t *testing.T) {
	defer setEnv("LLM_API_KEY", "test-key")()
	os.Setenv("LLM_FALLBACK_MODELS", "")
	defer os.Unsetenv("LLM_FALLBACK_MODELS")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error: %v", err)
	}
	if len(cfg.FallbackModels) == 0 {
		t.Error("expected fallback models to fall back to default when env var is empty")
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
