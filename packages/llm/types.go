package llm

type GenerateRequest struct {
	Model       string            `json:"model"`
	Messages    []Message         `json:"messages"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	TopP        float64           `json:"top_p,omitempty"`
	Stop        []string          `json:"stop,omitempty"`
	Provider    *ProviderSettings `json:"provider,omitempty"`
	Timeout     int               `json:"-"`
}

type ProviderSettings struct {
	Order             []string `json:"order,omitempty"`
	AllowFallbacks    *bool    `json:"allow_fallbacks,omitempty"`
	RequireParameters *bool    `json:"require_parameters,omitempty"`
	DataCollection    string   `json:"data_collection,omitempty"` // "allow" or "deny"
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
	Content string   `json:"-"` // Populated after parsing for convenience
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Code    string `json:"code,omitempty"`
}

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)
