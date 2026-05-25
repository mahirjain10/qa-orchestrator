package llm

type GenerateRequest struct {
	Model               string           `json:"model"`
	Messages            []Message        `json:"messages"`
	Temperature         float64          `json:"temperature,omitempty"`
	MaxTokens           int              `json:"max_tokens,omitempty"`
	MaxCompletionTokens int              `json:"max_completion_tokens,omitempty"`
	TopP                float64          `json:"top_p,omitempty"`
	Stop                []string         `json:"stop,omitempty"`
	ReasoningEffort     string           `json:"reasoning_effort,omitempty"`
	Thinking            *ThinkingConfig  `json:"thinking,omitempty"`
	Timeout             int              `json:"-"`
}

func (r *GenerateRequest) EffectiveMaxTokens() int {
	if r.MaxCompletionTokens > 0 {
		return r.MaxCompletionTokens
	}
	return r.MaxTokens
}

type ThinkingConfig struct {
	Type         string `json:"type"`                   // "enabled", "disabled", or "max"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // Optional token budget for thinking
}

type ProviderSettings struct {
	Order             []string `json:"order,omitempty"`
	Only              []string `json:"only,omitempty"`
	AllowFallbacks    *bool    `json:"allow_fallbacks,omitempty"`
	RequireParameters *bool    `json:"require_parameters,omitempty"`
	DataCollection    string   `json:"data_collection,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GenerateResponse struct {
	ID        string   `json:"id"`
	Object    string   `json:"object"`
	Created   int64    `json:"created"`
	Model     string   `json:"model"`
	Choices   []Choice `json:"choices"`
	Usage     Usage    `json:"usage"`
	Content   string   `json:"-"` // Populated after parsing for convenience
	Reasoning string   `json:"-"` // Model's chain-of-thought/reasoning, extracted from response
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

const (
	RoleSystem    = "system"
	RoleUser      = "user"
	RoleAssistant = "assistant"
)
