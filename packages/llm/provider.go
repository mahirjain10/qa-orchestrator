package llm

import (
	"encoding/json"
	"fmt"
)

type Provider interface {
	Name() string
	Endpoint(model string) string
	AuthHeaders(apiKey string) map[string]string
	BuildRequest(messages []Message, systemPrompt string, model string, temperature float64, maxTokens int) ([]byte, error)
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
	Model               string    `json:"model"`
	Messages            []Message `json:"messages"`
	Temperature         float64   `json:"temperature,omitempty"`
	MaxTokens           int       `json:"max_tokens,omitempty"`
	MaxCompletionTokens int       `json:"max_completion_tokens,omitempty"`
	TopP                float64   `json:"top_p,omitempty"`
	Stop                []string  `json:"stop,omitempty"`
}

type openAIErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    any    `json:"code,omitempty"`
	} `json:"error"`
}

func parseOpenAIResponse(body []byte) (*GenerateResponse, error) {
	var resp GenerateResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}
	resp.Content = content

	return &resp, nil
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
