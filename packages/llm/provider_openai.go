package llm

import (
	"context"
	"encoding/json"
	"fmt"
)

type OpenAIProvider struct{}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Endpoint(model string) string {
	return "https://api.openai.com/v1/chat/completions"
}

func (p *OpenAIProvider) AuthHeaders(apiKey string) map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	}
}

func (p *OpenAIProvider) BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error) {
	messages := req.Messages
	systemPrompt := ""

	if len(messages) > 0 && messages[0].Role == RoleSystem {
		systemPrompt = messages[0].Content
		messages = messages[1:]
	}

	allMessages := make([]Message, 0, len(messages)+1)
	if systemPrompt != "" {
		allMessages = append(allMessages, Message{Role: RoleSystem, Content: systemPrompt})
	}
	allMessages = append(allMessages, messages...)

	maxTokens := req.MaxCompletionTokens
	if maxTokens == 0 {
		maxTokens = req.MaxTokens
	}

	openReq := openAIRequest{
		Model:               req.Model,
		Messages:            allMessages,
		MaxCompletionTokens: maxTokens,
		ReasoningEffort:     req.ReasoningEffort,
	}


	// OpenAI doesn't natively use ThinkingConfig, but it uses reasoning_effort.
	// We map Thinking if provided for compatibility with DeepSeek via generic requests.
	if req.Thinking != nil && openReq.ReasoningEffort == "" {
		if req.Thinking.Type == "enabled" || req.Thinking.Type == "max" {
			openReq.ReasoningEffort = "high"
		}
	}

	// Reasoning models (o1, o3, o4, gpt-5+) do not support temperature.
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
	if model == "" {
		return fmt.Errorf("model is required for OpenAI provider")
	}
	return nil
}
