package llm

import (
	"context"
	"encoding/json"

	"qa-orchestrator/packages/shared"
)

type OpenRouterProvider struct {
	HTTPReferer string
	AppTitle    string
	BaseURL     string
	Provider    *ProviderSettings
}

func (p *OpenRouterProvider) Name() string {
	return "openrouter"
}

func (p *OpenRouterProvider) Endpoint(model string) string {
	return endpoint(p.BaseURL, "https://openrouter.ai/api/v1", "/chat/completions")
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
		Model:           req.Model,
		Messages:        allMessages,
		MaxTokens:       req.MaxTokens,
		TopP:            req.TopP,
		Stop:            req.Stop,
		ReasoningEffort: req.ReasoningEffort,
		Thinking:        req.Thinking,
	}

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
	return validateModel(model, "openrouter")
}

func (p *OpenRouterProvider) ApplyConfig(cfg *Config) {
	p.HTTPReferer = cfg.HTTPReferer
	p.AppTitle = cfg.AppTitle
	p.BaseURL = cfg.BaseURL
	p.ApplyProviderSettings(cfg.ProviderPriority, cfg.ProviderOnly, cfg.AllowFallbacks)
}

func (p *OpenRouterProvider) ApplyProviderSettings(priority, only, allow string) {
	if priority == "" && only == "" && allow == "" {
		return
	}

	if p.Provider == nil {
		p.Provider = &ProviderSettings{}
	}

	if priority != "" {
		p.Provider.Order = shared.SplitAndTrim(priority, ",")
	}

	if only != "" {
		p.Provider.Only = shared.SplitAndTrim(only, ",")
	}

	if allow != "" {
		val := allow == "true"
		p.Provider.AllowFallbacks = &val
	}
}
