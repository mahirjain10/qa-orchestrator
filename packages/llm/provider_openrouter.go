package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type OpenRouterProvider struct {
	HTTPReferer string
	AppTitle    string
	Provider    *ProviderSettings
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) Endpoint(model string) string {
	return "https://openrouter.ai/api/v1/chat/completions"
}

func (p *OpenRouterProvider) AuthHeaders(apiKey string) map[string]string {
	headers := map[string]string{
		"Authorization": "Bearer " + apiKey,
		"Content-Type":  "application/json",
	}
	if p.HTTPReferer != "" {
		headers["HTTP-Referer"] = p.HTTPReferer
	}
	if p.AppTitle != "" {
		headers["X-Title"] = p.AppTitle
	}
	return headers
}

func (p *OpenRouterProvider) BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error) {
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

	openReq := openAIRequest{
		Model:           req.Model,
		Messages:        allMessages,
		MaxTokens:       req.MaxTokens,
		ReasoningEffort: req.ReasoningEffort,
		Thinking:        req.Thinking,
	}

	// Reasoning models (o1, o3, o4, gpt-5+) reject the temperature parameter.
	if !isReasoningModel(req.Model) {
		openReq.Temperature = req.Temperature
	}

	type openRouterRequest struct {
		openAIRequest
		Provider *ProviderSettings `json:"provider,omitempty"`
	}

	orReq := openRouterRequest{
		openAIRequest: openReq,
		Provider:      p.Provider,
	}

	return json.Marshal(orReq)
}

func (p *OpenRouterProvider) ParseResponse(body []byte) (*GenerateResponse, error) {
	return parseOpenAIResponse(body)
}

func (p *OpenRouterProvider) ParseError(statusCode int, body []byte) error {
	return parseOpenAIError(statusCode, body)
}

func (p *OpenRouterProvider) ValidateModel(model string) error {
	if model == "" {
		return fmt.Errorf("model is required for OpenRouter provider")
	}
	return nil
}

func (p *OpenRouterProvider) ApplyProviderSettings(priority, only, allow string) {
	if priority == "" && only == "" && allow == "" {
		return
	}

	p.Provider = &ProviderSettings{}

	if priority != "" {
		p.Provider.Order = strings.Split(priority, ",")
		for i := range p.Provider.Order {
			p.Provider.Order[i] = strings.TrimSpace(p.Provider.Order[i])
		}
	}

	if only != "" {
		p.Provider.Only = strings.Split(only, ",")
		for i := range p.Provider.Only {
			p.Provider.Only[i] = strings.TrimSpace(p.Provider.Only[i])
		}
	}

	if allow != "" {
		val := allow == "true"
		p.Provider.AllowFallbacks = &val
	}
}
