package llm

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"qa-orchestrator/packages/shared"
)

const (
	envAPIKey           = "LLM_API_KEY"
	envBaseURL          = "LLM_BASE_URL"
	envModel            = "LLM_MODEL"
	envTimeout          = "LLM_TIMEOUT"
	envMaxRetries       = "LLM_MAX_RETRIES"
	envHTTPReferer      = "LLM_HTTP_REFERER"
	envAppTitle         = "LLM_APP_TITLE"
	envProviderPriority = "LLM_PROVIDER_PRIORITY"
	envAllowFallbacks    = "LLM_ALLOW_FALLBACKS"
	envProviderOnly     = "LLM_PROVIDER_ONLY"
	envProvider         = "LLM_PROVIDER"
	envGeminiAPIKey     = "GEMINI_API_KEY"
	envGeminiModel      = "GEMINI_MODEL"
	envFallbackModels   = "LLM_FALLBACK_MODELS"
	envReasoningEffort  = "LLM_REASONING_EFFORT"
	envThinkingType     = "LLM_THINKING_TYPE"
	envThinkingBudget   = "LLM_THINKING_BUDGET"

	defaultBaseURL        = "https://openrouter.ai/api/v1"
	defaultModel          = "openai/gpt-4o-mini"
	defaultTimeout        = 120
	defaultMaxRetries     = 3
	defaultProvider       = "auto"
	defaultGeminiModel    = "gemini-2.0-flash"
	defaultFallbackModels = "openai/gpt-4o-mini,gemini/gemini-2.0-flash-001"
	maxMaxRetries        = 20
)

type Config struct {
	APIKey           string
	BaseURL          string
	Model            string
	FallbackModels   []string
	Timeout          time.Duration
	MaxRetries       int
	HTTPReferer      string
	AppTitle         string
	ProviderPriority string
	AllowFallbacks string
	ProviderOnly     string
	Provider         string
	GeminiAPIKey     string
	GeminiModel      string
	ReasoningEffort  string
	ThinkingType     string
	ThinkingBudget   int
}

func LoadConfig() (*Config, error) {
	provider := resolveProvider(os.Getenv(envProvider))
	model, geminiModel := resolveModel(provider, os.Getenv(envModel), os.Getenv(envGeminiModel))
	apiKey, geminiAPIKey, err := validateAPIKeys(provider, os.Getenv(envAPIKey), os.Getenv(envGeminiAPIKey))
	if err != nil {
		return nil, fmt.Errorf("validate API keys: %w", err)
	}
	if model == "" {
		return nil, fmt.Errorf("%s environment variable is required", envModel)
	}
	baseURL := resolveBaseURL(provider, model, os.Getenv(envBaseURL))
	timeout, err := parseEnvInt(envTimeout, defaultTimeout)
	if err != nil {
		return nil, fmt.Errorf("parse timeout: %w", err)
	}
	maxRetries, err := parseEnvIntWithBounds(envMaxRetries, defaultMaxRetries, 0, maxMaxRetries)
	if err != nil {
		return nil, fmt.Errorf("parse max retries: %w", err)
	}
	fallbackModels := shared.SplitAndTrim(os.Getenv(envFallbackModels), ",")
	if len(fallbackModels) == 0 {
		fallbackModels = shared.SplitAndTrim(defaultFallbackModels, ",")
	}
	budget, err := parseEnvIntWithBounds(envThinkingBudget, 0, 0, -1)
	if err != nil {
		return nil, fmt.Errorf("parse thinking budget: %w", err)
	}
	return &Config{
		APIKey:           apiKey,
		BaseURL:          baseURL,
		Model:            model,
		FallbackModels:   fallbackModels,
		Timeout:          time.Duration(timeout) * time.Second,
		MaxRetries:       maxRetries,
		HTTPReferer:      os.Getenv(envHTTPReferer),
		AppTitle:         os.Getenv(envAppTitle),
		ProviderPriority: os.Getenv(envProviderPriority),
		AllowFallbacks:   os.Getenv(envAllowFallbacks),
		ProviderOnly:     os.Getenv(envProviderOnly),
		Provider:         provider,
		GeminiAPIKey:     geminiAPIKey,
		GeminiModel:      geminiModel,
		ReasoningEffort:  os.Getenv(envReasoningEffort),
		ThinkingType:     os.Getenv(envThinkingType),
		ThinkingBudget:   budget,
	}, nil
}

func resolveProvider(provider string) string {
	if provider == "" {
		provider = defaultProvider
	}
	switch {
	case strings.HasPrefix(provider, "gemini") || strings.HasPrefix(provider, "google/"):
		return "gemini"
	case strings.HasPrefix(provider, "gpt-") || strings.HasPrefix(provider, "o1") || strings.HasPrefix(provider, "o3"):
		return "openai"
	case strings.HasPrefix(provider, "openai/") || strings.HasPrefix(provider, "anthropic/") || strings.HasPrefix(provider, "meta/"):
		return "openrouter"
	}
	return provider
}

func resolveModel(provider, model, geminiModel string) (string, string) {
	if geminiModel == "" {
		geminiModel = defaultGeminiModel
	}
	if model != "" {
		return model, geminiModel
	}
	switch provider {
	case "auto":
		return defaultModel, geminiModel
	case "gemini":
		return geminiModel, geminiModel
	case "openai":
		return "gpt-4o-mini", geminiModel
	}
	return defaultModel, geminiModel
}

func validateAPIKeys(provider, apiKey, geminiAPIKey string) (string, string, error) {
	if provider == "gemini" {
		if geminiAPIKey == "" && apiKey == "" {
			return "", "", fmt.Errorf("%s or %s environment variable is required for Gemini provider", envGeminiAPIKey, envAPIKey)
		}
		if geminiAPIKey == "" {
			geminiAPIKey = apiKey
		}
	} else {
		if apiKey == "" {
			return "", "", fmt.Errorf("%s environment variable is required", envAPIKey)
		}
	}
	return apiKey, geminiAPIKey, nil
}

func resolveBaseURL(provider, model, baseURL string) string {
	if baseURL != "" {
		return baseURL
	}
	resolved := provider
	if resolved == "auto" {
		resolved = detectProviderName(model)
	}
	if resolved != "gemini" {
		return defaultBaseURL
	}
	return ""
}

func parseEnvInt(key string, defaultVal int) (int, error) {
	s := os.Getenv(key)
	if s == "" {
		return defaultVal, nil
	}
	v, err := parseInt(s)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return v, nil
}

func parseEnvIntWithBounds(key string, defaultVal, min, max int) (int, error) {
	v, err := parseEnvInt(key, defaultVal)
	if err != nil {
		return 0, err
	}
	if min >= 0 && v < min {
		return 0, fmt.Errorf("%s must be at least %d, got %d", key, min, v)
	}
	if max >= 0 && v > max {
		return 0, fmt.Errorf("%s must not exceed %d, got %d", key, max, v)
	}
	return v, nil
}

func parseInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("parse int: %w", err)
	}
	return n, nil
}

func (c *Config) Validate() error {
	if c.Provider == "gemini" {
		if c.GeminiAPIKey == "" && c.APIKey == "" {
			return fmt.Errorf("API key is required (set GEMINI_API_KEY or LLM_API_KEY for Gemini provider)")
		}
	} else {
		if c.APIKey == "" {
			return fmt.Errorf("API key is required")
		}
		if c.BaseURL == "" {
			return fmt.Errorf("base URL is required")
		}
	}
	if c.Model == "" {
		return fmt.Errorf("%w", shared.ErrModelRequired)
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", c.Timeout)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative, got %d", c.MaxRetries)
	}
	if c.MaxRetries > maxMaxRetries {
		return fmt.Errorf("max retries must not exceed %d, got %d", maxMaxRetries, c.MaxRetries)
	}
	return nil
}

func (c *Config) GetProvider() (Provider, error) {
	providerName := c.Provider
	if providerName == "auto" {
		providerName = detectProviderName(c.Model)
	}

	provider, err := GetProvider(providerName)
	if err != nil {
		return nil, fmt.Errorf("get provider %s: %w", providerName, err)
	}

	provider.ApplyConfig(c)
	return provider, nil
}

func (c *Config) GetAPIKey() string {
	providerName := c.Provider
	if providerName == "auto" {
		providerName = detectProviderName(c.Model)
	}

	if providerName == "gemini" {
		if c.GeminiAPIKey != "" {
			return c.GeminiAPIKey
		}
		return c.APIKey
	}

	return c.APIKey
}

func detectProviderName(model string) string {
	if model == "" {
		return "openrouter"
	}

	lower := strings.ToLower(model)
	switch {
	case strings.HasPrefix(lower, "gemini") || strings.HasPrefix(lower, "google/"):
		return "gemini"
	case strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "o1") || strings.HasPrefix(lower, "o3"):
		return "openai"
	default:
		return "openrouter"
	}
}
