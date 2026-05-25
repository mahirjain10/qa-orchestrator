package types

// ParameterInfo describes a single parameter for a tool.
type ParameterInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

// ToolInfo describes a tool for LLM consumption.
type ToolInfo struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Parameters  map[string]ParameterInfo `json:"parameters"`
	Hidden      bool                     `json:"-"`
}
