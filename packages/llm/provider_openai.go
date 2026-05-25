package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type OpenAIProvider struct {
	BaseProvider
	BaseURL string
}

func NewOpenAIProvider() *OpenAIProvider {
	return &OpenAIProvider{
		BaseProvider: BaseProvider{name: "openai"},
	}
}

func (p *OpenAIProvider) Endpoint(model string) string {
	return endpoint(p.BaseURL, "https://api.openai.com/v1", "/chat/completions")
}

func (p *OpenAIProvider) AuthHeaders(apiKey string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	}
}

func (p *OpenAIProvider) BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error) {
	if err := p.CheckContext(ctx); err != nil {
		return nil, fmt.Errorf("openai check context: %w", err)
	}

	systemPrompt, messages := splitSystemMessage(req.Messages)
	allMessages := make([]Message, 0, len(messages)+1)
	if systemPrompt != "" {
		allMessages = append(allMessages, Message{Role: RoleSystem, Content: systemPrompt})
	}
	allMessages = append(allMessages, messages...)

	openReq := openAIRequest{
		Model:               req.Model,
		Messages:            allMessages,
		MaxCompletionTokens: req.EffectiveMaxTokens(),
		TopP:                req.TopP,
		Stop:                req.Stop,
		ReasoningEffort:     req.ReasoningEffort,
	}

	if req.Thinking != nil && openReq.ReasoningEffort == "" {
		if req.Thinking.Type == "enabled" || req.Thinking.Type == "max" {
			openReq.ReasoningEffort = "high"
		}
	}

	if !isReasoningModel(req.Model) {
		openReq.Temperature = req.Temperature
	}

	data, err := json.Marshal(openReq)
	if err != nil {
		return nil, fmt.Errorf("openai marshal request: %w", err)
	}
	return data, nil
}

func (p *OpenAIProvider) ParseResponse(body []byte) (*GenerateResponse, error) {
	resp, err := parseOpenAIResponse(body)
	if err != nil {
		return nil, fmt.Errorf("openai parse response: %w", err)
	}
	return resp, nil
}

func (p *OpenAIProvider) ParseError(statusCode int, body []byte) error {
	return parseOpenAIError(statusCode, body)
}

func (p *OpenAIProvider) ApplyConfig(cfg *Config) {
	p.BaseProvider.ApplyConfig(cfg)
	p.BaseURL = cfg.BaseURL
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

func parseOpenAIError(statusCode int, body []byte) error {
	var errResp openAIErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		codeStr := ""
		if errResp.Error.Code != nil {
			codeStr = fmt.Sprintf("%v", errResp.Error.Code)
		}
		apiErr := &APIError{
			StatusCode: statusCode,
			Message:    errResp.Error.Message,
			Type:       errResp.Error.Type,
			Code:       codeStr,
		}
		return NewRetryableError(apiErr, IsRetryableStatusCode(statusCode))
	}

	return NewRetryableError(
		fmt.Errorf("request failed with status %d", statusCode),
		IsRetryableStatusCode(statusCode),
	)
}

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
