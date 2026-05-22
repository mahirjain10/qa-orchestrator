package llm

import (
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
	Text string `json:"text"`
}

type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

type geminiGenerationConfig struct {
	Temperature     float64  `json:"temperature,omitempty"`
	MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
	TopP            float64  `json:"topP,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type geminiRequest struct {
	Contents          []geminiContent         `json:"contents"`
	SystemInstruction *geminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *geminiGenerationConfig `json:"generationConfig,omitempty"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
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

func (p *GeminiProvider) BuildRequest(messages []Message, systemPrompt string, model string, temperature float64, maxTokens int) ([]byte, error) {
	req := geminiRequest{
		Contents: make([]geminiContent, 0, len(messages)),
	}

	if systemPrompt != "" {
		req.SystemInstruction = &geminiContent{
			Parts: []geminiPart{{Text: systemPrompt}},
		}
	}

	for _, msg := range messages {
		if msg.Role == RoleSystem {
			if req.SystemInstruction == nil {
				req.SystemInstruction = &geminiContent{
					Parts: []geminiPart{},
				}
			}
			req.SystemInstruction.Parts = append(req.SystemInstruction.Parts, geminiPart{Text: msg.Content})
			continue
		}

		geminiRole := "user"
		if msg.Role == RoleAssistant {
			geminiRole = "model"
		}

		req.Contents = append(req.Contents, geminiContent{
			Role:  geminiRole,
			Parts: []geminiPart{{Text: msg.Content}},
		})
	}

	if temperature > 0 || maxTokens > 0 {
		req.GenerationConfig = &geminiGenerationConfig{
			Temperature:     temperature,
			MaxOutputTokens: maxTokens,
		}
	}

	return json.Marshal(req)
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
	finishReason := ""
	if len(resp.Candidates[0].Content.Parts) > 0 {
		content = resp.Candidates[0].Content.Parts[0].Text
	}
	if resp.Candidates[0].FinishReason != "" {
		finishReason = resp.Candidates[0].FinishReason
	}

	genResp := &GenerateResponse{
		ID:      resp.ModelVersion,
		Object:  "generateContent.response",
		Model:   resp.ModelVersion,
		Content: content,
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
