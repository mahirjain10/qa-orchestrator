package llm

import (
	"context"
	"encoding/json"
	"fmt"

	"qa-orchestrator/packages/shared"
)

type OpenRouterProvider struct {
	BaseProvider
	HTTPReferer string
	AppTitle    string
	BaseURL     string
	Provider    *ProviderSettings
}

func NewOpenRouterProvider() *OpenRouterProvider {
	return &OpenRouterProvider{
		BaseProvider: BaseProvider{name: "openrouter"},
	}
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
	if err := p.CheckContext(ctx); err != nil {
		return nil, fmt.Errorf("openrouter check context: %w", err)
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

	data, err := json.Marshal(orReq)
	if err != nil {
		return nil, fmt.Errorf("openrouter marshal request: %w", err)
	}
	return data, nil
}

func (p *OpenRouterProvider) ParseResponse(body []byte) (*GenerateResponse, error) {
	resp, err := parseOpenAIResponse(body)
	if err != nil {
		return nil, fmt.Errorf("openrouter parse response: %w", err)
	}
	return resp, nil
}

func (p *OpenRouterProvider) ParseError(statusCode int, body []byte) error {
	return parseOpenAIError(statusCode, body)
}

func (p *OpenRouterProvider) ApplyConfig(cfg *Config) {
	p.BaseProvider.ApplyConfig(cfg)
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
