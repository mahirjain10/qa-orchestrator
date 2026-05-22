package llm

import (
	"encoding/json"
	"fmt"
	"strings"
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

func (p *OpenAIProvider) BuildRequest(messages []Message, systemPrompt string, model string, temperature float64, maxTokens int) ([]byte, error) {
	allMessages := make([]Message, 0, len(messages)+1)
	if systemPrompt != "" {
		allMessages = append(allMessages, Message{Role: RoleSystem, Content: systemPrompt})
	}
	allMessages = append(allMessages, messages...)

	req := openAIRequest{
		Model:               model,
		Messages:            allMessages,
		MaxCompletionTokens: maxTokens,
	}

	// Reasoning models (o1, o3, gpt-5+) do not support temperature.
	if supportsTemperature(model) {
		req.Temperature = temperature
	}

	return json.Marshal(req)
}

// supportsTemperature returns false for reasoning models that reject the temperature parameter.
func supportsTemperature(model string) bool {
	lower := strings.ToLower(model)
	return !strings.HasPrefix(lower, "o1") &&
		!strings.HasPrefix(lower, "o3") &&
		!strings.HasPrefix(lower, "gpt-5")
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
