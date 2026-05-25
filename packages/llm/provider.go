package llm

import (
	"context"
	"fmt"
)

type Provider interface {
	Name() string
	Endpoint(model string) string
	AuthHeaders(apiKey string) map[string]string
	BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error)
	ParseResponse(body []byte) (*GenerateResponse, error)
	ParseError(statusCode int, body []byte) error
	ValidateModel(model string) error
	ApplyConfig(cfg *Config)
}

func GetProvider(name string) (Provider, error) {
	switch name {
	case "openai":
		return NewOpenAIProvider(), nil
	case "openrouter":
		return NewOpenRouterProvider(), nil
	case "gemini":
		return NewGeminiProvider(), nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func DetectProvider(model string) Provider {
	providerName := detectProviderName(model)
	p, _ := GetProvider(providerName)
	return p
}
