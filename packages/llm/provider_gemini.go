package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type GeminiProvider struct {
	APIKey string
}

func (p *GeminiProvider) Name() string {
	return "gemini"
}

func (p *GeminiProvider) Endpoint(model string) string {
	if model == "" {
		model = "gemini-2.0-flash"
	}
	model = strings.TrimPrefix(model, "gemini/")
	model = strings.TrimPrefix(model, "google/")
	baseURL := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	if p.APIKey != "" {
		return fmt.Sprintf("%s?key=%s", baseURL, p.APIKey)
	}
	return baseURL
}

func (p *GeminiProvider) AuthHeaders(apiKey string) map[string]string {
	if p.APIKey != "" {
		return map[string]string{
			"Content-Type": "application/json",
		}
	}
	return map[string]string{
		"x-goog-api-key": apiKey,
		"Content-Type":   "application/json",
	}
}

type geminiPart struct {
	Text    string `json:"text,omitempty"`
	Thought bool   `json:"thought,omitempty"`
}

type geminiThinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget,omitempty"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     float64              `json:"temperature,omitempty"`
	MaxOutputTokens int                  `json:"maxOutputTokens,omitempty"`
	TopP            float64              `json:"topP,omitempty"`
	StopSequences   []string             `json:"stopSequences,omitempty"`
	ThinkingConfig  *geminiThinkingConfig `json:"thinkingConfig,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
		Index        int    `json:"index"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	ModelVersion string `json:"modelVersion"`
}

type geminiErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}

func (p *GeminiProvider) BuildRequest(ctx context.Context, req *GenerateRequest) ([]byte, error) {
	messages := req.Messages
	systemPrompt := ""

	if len(messages) > 0 && messages[0].Role == RoleSystem {
		systemPrompt = messages[0].Content
		messages = messages[1:]
	}

	geminiReq := geminiRequest{
		Contents: make([]geminiContent, 0, len(messages)),
	}

	if systemPrompt != "" {
		geminiReq.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		}
	}

	for _, msg := range messages {
		if msg.Role == RoleSystem {
			if geminiReq.SystemInstruction == nil {
				geminiReq.SystemInstruction = &geminiContent{
					Parts: []geminiPart{},
				}
			}
			geminiReq.SystemInstruction.Parts = append(geminiReq.SystemInstruction.Parts, geminiPart{Text: msg.Content})
			continue
		}

		geminiRole := "user"
		if msg.Role == RoleAssistant {
			geminiRole = "model"
		}

		geminiReq.Contents = append(geminiReq.Contents, geminiContent{
			Role:  geminiRole,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	if req.Temperature > 0 || req.MaxTokens > 0 || req.MaxCompletionTokens > 0 || req.Thinking != nil {
		maxTokens := req.MaxTokens
		if req.MaxCompletionTokens > 0 {
			maxTokens = req.MaxCompletionTokens
		}
		cfg := &geminiGenerationConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: maxTokens,
		}
		if req.Thinking != nil && req.Thinking.BudgetTokens > 0 {
			cfg.ThinkingConfig = &geminiThinkingConfig{
				ThinkingBudget: req.Thinking.BudgetTokens,
			}
		}
		geminiReq.GenerationConfig = cfg
	}

	return json.Marshal(geminiReq)
}

func (p *GeminiProvider) ParseResponse(body []byte) (*GenerateResponse, error) {
	var resp geminiResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decoding Gemini response: %w", err)
	}

	if len(resp.Candidates) == 0 {
		return nil, NewNonRetryableError(fmt.Errorf("Gemini returned no candidates"))
	}

	content := ""
	reasoning := ""
	finishReason := ""
	for _, part := range resp.Candidates[0].Content.Parts {
		if part.Thought {
			if reasoning == "" {
				reasoning = part.Text
			} else {
				reasoning += "\n" + part.Text
			}
		} else {
			content += part.Text
		}
	}
	if resp.Candidates[0].FinishReason != "" {
		finishReason = resp.Candidates[0].FinishReason
	}

	genResp := &GenerateResponse{
		ID:      resp.ModelVersion,
		Object:  "generateContent.response",
		Model:   resp.ModelVersion,
		Content: content,
		Reasoning: reasoning,
		Choices: []Choice{
			{
				Index:        0,
				Message:      Message{Role: RoleAssistant, Content: content},
				FinishReason: finishReason,
			},
		},
		Usage: Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		},
	}

	return genResp, nil
}

func (p *GeminiProvider) ParseError(statusCode int, body []byte) error {
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		apiErr := &APIError{
			StatusCode: statusCode,
			Message:    errResp.Error.Message,
			Type:       errResp.Error.Status,
		}
		return NewRetryableError(apiErr, IsRetryableStatusCode(statusCode))
	}

	return NewRetryableError(
		fmt.Errorf("Gemini request failed with status %d", statusCode),
		IsRetryableStatusCode(statusCode),
	)
}

func (p *GeminiProvider) ValidateModel(model string) error {
	if model == "" {
		return fmt.Errorf("model is required for Gemini provider")
	}
	return nil
}
