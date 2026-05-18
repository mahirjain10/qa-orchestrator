package llm

import (
	"fmt"
	"os"
	"time"
)

const (
	envAPIKey   = "LLM_API_KEY"
	envBaseURL  = "LLM_BASE_URL"
	envModel    = "LLM_MODEL"
	envTimeout  = "LLM_TIMEOUT"
	envMaxRetries = "LLM_MAX_RETRIES"

	defaultBaseURL  = "https://openrouter.ai/api/v1"
	defaultModel   = "openai/gpt-4o-mini"
	defaultTimeout = 30
	defaultMaxRetries = 3
)

type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
	Timeout    time.Duration
	MaxRetries int
}

func LoadConfig() (*Config, error) {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("%s environment variable is required", envAPIKey)
	}

	baseURL := os.Getenv(envBaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	model := os.Getenv(envModel)
	if model == "" {
		model = defaultModel
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
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Model:      model,
		Timeout:    time.Duration(timeout) * time.Second,
		MaxRetries: maxRetries,
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
	if c.APIKey == "" {
		return fmt.Errorf("API key is required")
	}
	if c.BaseURL == "" {
		return fmt.Errorf("base URL is required")
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

func (c *Config) Endpoint() string {
	return c.BaseURL + "/chat/completions"
}