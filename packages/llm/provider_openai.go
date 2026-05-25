package llm

import (
	"context"
	"encoding/json"
)

type OpenAIProvider struct {
	BaseURL string
}

func (p *OpenAIProvider) Name() string {
	return "openai"
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
	if err := checkContext(ctx); err != nil {
		return nil, err
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

	return json.Marshal(openReq)
}

func (p *OpenAIProvider) ParseResponse(body []byte) (*GenerateResponse, error) {
	return parseOpenAIResponse(body)
}

func (p *OpenAIProvider) ParseError(statusCode int, body []byte) error {
	return parseOpenAIError(statusCode, body)
}

func (p *OpenAIProvider) ValidateModel(model string) error {
	return validateModel(model, "openai")
}

func (p *OpenAIProvider) ApplyConfig(cfg *Config) {
	p.BaseURL = cfg.BaseURL
}
