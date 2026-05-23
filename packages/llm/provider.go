package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Provider interface {
	Name() string
	Endpoint(model string) string
	AuthHeaders(apiKey string) map[string]string
	BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error)
	ParseResponse(body []byte) (*GenerateResponse, error)
	ParseError(statusCode int, body []byte) error
	ValidateModel(model string) error
}

func GetProvider(name string) (Provider, error) {
	switch name {
	case "openai":
		return &OpenAIProvider{}, nil
	case "openrouter":
		return &OpenRouterProvider{}, nil
	case "gemini":
		return &GeminiProvider{}, nil
	default:
		return nil, fmt.Errorf("unknown provider: %s", name)
	}
}

func DetectProvider(model string) Provider {
	providerName := detectProviderName(model)
	p, _ := GetProvider(providerName)
	return p
}

type openAIRequest struct {
	Model               string          `json:"model"`
	Messages            []Message       `json:"messages"`
	Temperature         float64         `json:"temperature,omitempty"`
	MaxTokens           int             `json:"max_tokens,omitempty"`
	MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	TopP                float64         `json:"top_p,omitempty"`
	Stop                []string        `json:"stop,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Thinking            *ThinkingConfig `json:"thinking,omitempty"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code,omitempty"`
	} `json:"error"`
}

func parseOpenAIResponse(body []byte) (*GenerateResponse, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decoding raw response: %w", err)
	}

	var reasoning string

	// OpenRouter returns reasoning at the top level.
	if r, ok := raw["reasoning"].(string); ok {
		reasoning = r
	}

	// DeepSeek returns reasoning_content inside the first choice's message.
	if reasoning == "" {
		if choices, ok := raw["choices"].([]any); ok && len(choices) > 0 {
			if first, ok := choices[0].(map[string]any); ok {
				if msg, ok := first["message"].(map[string]any); ok {
					if rc, ok := msg["reasoning_content"].(string); ok {
						reasoning = rc
					}
				}
			}
		}
	}

	// Unmarshal into the typed struct for the standard fields.
	var resp GenerateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(resp.Choices) > 0 {
		resp.Content = resp.Choices[0].Message.Content
	}
	resp.Reasoning = reasoning

	return &resp, nil
}

// isReasoningModel returns true for models that reject the temperature parameter.
// Strips common provider prefixes (openai/, google/, etc.) before matching.
func isReasoningModel(model string) bool {
	lower := strings.ToLower(model)
	// Strip provider prefixes like "openai/", "google/", "deepseek/"
	if idx := strings.Index(lower, "/"); idx >= 0 {
		lower = lower[idx+1:]
	}
	return strings.HasPrefix(lower, "o1") ||
		strings.HasPrefix(lower, "o3") ||
		strings.HasPrefix(lower, "o4") ||
		strings.HasPrefix(lower, "gpt-5")
}

func parseOpenAIError(statusCode int, body []byte) error {
	var errResp openAIErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		apiErr := &APIError{
			StatusCode: statusCode,
			Message:    errResp.Error.Message,
			Type:       errResp.Error.Type,
		}
		return NewRetryableError(apiErr, IsRetryableStatusCode(statusCode))
	}

	return NewRetryableError(
		fmt.Errorf("request failed with status %d", statusCode),
		IsRetryableStatusCode(statusCode),
	)
}
