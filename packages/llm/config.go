package llm

import (
	"fmt"
	"os"
	"strings"
	"time"
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
	envProviderAllow    = "LLM_PROVIDER_ALLOW"
	envProviderOnly     = "LLM_PROVIDER_ONLY"
	envProvider         = "LLM_PROVIDER"
	envGeminiAPIKey     = "GEMINI_API_KEY"
	envGeminiModel      = "GEMINI_MODEL"

	defaultBaseURL     = "https://openrouter.ai/api/v1"
	defaultModel       = "openai/gpt-4o-mini"
	defaultTimeout     = 30
	defaultMaxRetries  = 3
	defaultProvider    = "auto"
	defaultGeminiModel = "gemini-2.0-flash"
)

type Config struct {
	APIKey           string
	BaseURL          string
	Model            string
	Timeout          time.Duration
	MaxRetries       int
	HTTPReferer      string
	AppTitle         string
	ProviderPriority string
	ProviderAllow    string
	ProviderOnly     string
	Provider         string
	GeminiAPIKey     string
	GeminiModel      string
}

func LoadConfig() (*Config, error) {
	provider := os.Getenv(envProvider)
	if provider == "" {
		provider = defaultProvider
	}

	if strings.HasPrefix(provider, "gemini") || strings.HasPrefix(provider, "google/") {
		provider = "gemini"
	} else if strings.HasPrefix(provider, "gpt-") || strings.HasPrefix(provider, "o1") || strings.HasPrefix(provider, "o3") {
		provider = "openai"
	} else if strings.HasPrefix(provider, "openai/") || strings.HasPrefix(provider, "anthropic/") || strings.HasPrefix(provider, "meta/") {
		provider = "openrouter"
	}

	model := os.Getenv(envModel)
	geminiModel := os.Getenv(envGeminiModel)
	if geminiModel == "" {
		geminiModel = defaultGeminiModel
	}

	if provider == "auto" {
		if model == "" {
			model = defaultModel
		}
	} else if provider == "gemini" {
		if model == "" {
			model = geminiModel
		}
	}

	apiKey := os.Getenv(envAPIKey)
	geminiAPIKey := os.Getenv(envGeminiAPIKey)

	if provider == "gemini" {
		if geminiAPIKey == "" && apiKey == "" {
			return nil, fmt.Errorf("%s or %s environment variable is required for Gemini provider", envGeminiAPIKey, envAPIKey)
		}
		if geminiAPIKey == "" {
			geminiAPIKey = apiKey
		}
	} else {
		if apiKey == "" {
			return nil, fmt.Errorf("%s environment variable is required", envAPIKey)
		}
	}

	if model == "" {
		return nil, fmt.Errorf("%s environment variable is required", envModel)
	}

	baseURL := os.Getenv(envBaseURL)
	if baseURL == "" && provider != "gemini" {
		baseURL = defaultBaseURL
	}

	timeout := defaultTimeout
	if timeoutStr := os.Getenv(envTimeout); timeoutStr != "" {
		var err error
		timeout, err = parseTimeout(timeoutStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", envTimeout, err)
		}
	}

	maxRetries := defaultMaxRetries
	if retriesStr := os.Getenv(envMaxRetries); retriesStr != "" {
		var err error
		maxRetries, err = parseInt(retriesStr)
		if err != nil {
			return nil, fmt.Errorf("invalid %s: %w", envMaxRetries, err)
		}
		if maxRetries < 0 {
			return nil, fmt.Errorf("%s must be non-negative, got %d", envMaxRetries, maxRetries)
		}
	}

	return &Config{
		APIKey:           apiKey,
		BaseURL:          baseURL,
		Model:            model,
		Timeout:          time.Duration(timeout) * time.Second,
		MaxRetries:       maxRetries,
		HTTPReferer:      os.Getenv(envHTTPReferer),
		AppTitle:         os.Getenv(envAppTitle),
		ProviderPriority: os.Getenv(envProviderPriority),
		ProviderAllow:    os.Getenv(envProviderAllow),
		ProviderOnly:     os.Getenv(envProviderOnly),
		Provider:         provider,
		GeminiAPIKey:     geminiAPIKey,
		GeminiModel:      geminiModel,
	}, nil
}

func parseTimeout(s string) (int, error) {
	return parseInt(s)
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
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
		return fmt.Errorf("model is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive, got %v", c.Timeout)
	}
	if c.MaxRetries < 0 {
		return fmt.Errorf("max retries must be non-negative, got %d", c.MaxRetries)
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
		return nil, err
	}

	if orProvider, ok := provider.(*OpenRouterProvider); ok {
		orProvider.HTTPReferer = c.HTTPReferer
		orProvider.AppTitle = c.AppTitle
		orProvider.ApplyProviderSettings(c.ProviderPriority, c.ProviderOnly, c.ProviderAllow)
	}

	if geminiProvider, ok := provider.(*GeminiProvider); ok {
		if c.GeminiAPIKey != "" {
			geminiProvider.APIKey = c.GeminiAPIKey
		} else {
			geminiProvider.APIKey = c.APIKey
		}
	}

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
