package tools

import (
	"fmt"
	"sync"

	"qa-orchestrator/packages/browser-runtime"
)

type Tool func(params map[string]any) (any, error)

type ParameterInfo struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type ToolInfo struct {
	Name        string                   `json:"name"`
	Description string                   `json:"description"`
	Parameters  map[string]ParameterInfo `json:"parameters"`
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
	meta  map[string]ToolInfo
}

func NewToolRegistry(runtime *browserruntime.BrowserRuntime) *ToolRegistry {
	registry := &ToolRegistry{
		tools: make(map[string]Tool),
		meta:  make(map[string]ToolInfo),
	}
	registry.registerDefaultTools(runtime)
	return registry
}

func (r *ToolRegistry) registerDefaultTools(runtime *browserruntime.BrowserRuntime) {
	r.Register("navigate", map[string]ParameterInfo{
		"url": {Type: "string", Description: "The URL to navigate to", Required: true},
	}, func(params map[string]any) (any, error) {
		url, ok := params["url"].(string)
		if !ok {
			return nil, fmt.Errorf("url parameter required")
		}
		if err := runtime.Navigate(url); err != nil {
			return nil, fmt.Errorf("navigate failed: %w", err)
		}
		return fmt.Sprintf("navigated to %s", url), nil
	})

	r.Register("click", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element to click", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		if err := runtime.Click(selector); err != nil {
			return nil, fmt.Errorf("click failed: %w", err)
		}
		return fmt.Sprintf("clicked %s", selector), nil
	})

	r.Register("type_text", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the input field", Required: true},
		"value":    {Type: "string", Description: "Text to type into the field", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		value, ok := params["value"].(string)
		if !ok {
			return nil, fmt.Errorf("value parameter required")
		}
		if err := runtime.Fill(selector, value); err != nil {
			return nil, fmt.Errorf("type_text failed: %w", err)
		}
		return fmt.Sprintf("typed '%s' into %s", value, selector), nil
	})

	r.Register("wait_for", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element to wait for", Required: true},
		"state":    {Type: "string", Description: "Wait state: visible, hidden, attached (default: visible)", Required: false},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		state := "visible"
		if s, ok := params["state"].(string); ok {
			state = s
		}
		err := runtime.WaitForSelector(selector, &browserruntime.WaitForOptions{State: state})
		if err != nil {
			return nil, fmt.Errorf("wait_for failed: %w", err)
		}
		return fmt.Sprintf("waited for %s (%s)", selector, state), nil
	})

	r.Register("get_text", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		text, err := runtime.TextContent(selector)
		if err != nil {
			return nil, fmt.Errorf("get_text failed: %w", err)
		}
		return text, nil
	})

	r.Register("get_html", map[string]ParameterInfo{
		"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
	}, func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		html, err := runtime.InnerHTML(selector)
		if err != nil {
			return nil, fmt.Errorf("get_html failed: %w", err)
		}
		return html, nil
	})

	r.Register("evaluate", map[string]ParameterInfo{
		"expression": {Type: "string", Description: "JavaScript expression to evaluate", Required: true},
	}, func(params map[string]any) (any, error) {
		expr, ok := params["expression"].(string)
		if !ok {
			return nil, fmt.Errorf("expression parameter required")
		}
		result, err := runtime.Evaluate(expr)
		if err != nil {
			return nil, fmt.Errorf("evaluate failed: %w", err)
		}
		return result, nil
	})

	r.Register("screenshot", map[string]ParameterInfo{
		"path":      {Type: "string", Description: "File path to save the screenshot", Required: false},
		"full_page": {Type: "bool", Description: "Capture full page if true", Required: false},
	}, func(params map[string]any) (any, error) {
		path := ""
		if p, ok := params["path"].(string); ok {
			path = p
		}
		fullPage := false
		if fp, ok := params["full_page"].(bool); ok {
			fullPage = fp
		}
		screenshot, err := runtime.Screenshot(&browserruntime.ScreenshotOptions{
			Path:     path,
			FullPage: fullPage,
		})
		if err != nil {
			return nil, fmt.Errorf("screenshot failed: %w", err)
		}
		return fmt.Sprintf("screenshot captured (%d bytes)", len(screenshot)), nil
	})
}

func (r *ToolRegistry) Register(name string, params map[string]ParameterInfo, fn Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[name] = fn
	r.meta[name] = ToolInfo{
		Name:        name,
		Description: getToolDescription(name),
		Parameters:  params,
	}
}

func getToolDescription(name string) string {
	descriptions := map[string]string{
		"navigate":   "Navigate to a URL in the browser",
		"click":      "Click on an element identified by CSS selector",
		"type_text":  "Type text into an input field",
		"wait_for":   "Wait for an element to reach a specific state",
		"get_text":   "Get the text content of an element",
		"get_html":   "Get the inner HTML of an element",
		"evaluate":   "Evaluate a JavaScript expression in the browser context",
		"screenshot": "Take a screenshot of the page",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return fmt.Sprintf("Tool: %s", name)
}

func (r *ToolRegistry) Execute(name string, params map[string]any) (any, error) {
	r.mu.RLock()
	tool, exists := r.tools[name]
	meta, metaExists := r.meta[name]
	r.mu.RUnlock()
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	if !metaExists {
		return nil, fmt.Errorf("missing metadata for tool: %s", name)
	}
	if err := validateToolParams(meta, params); err != nil {
		return nil, err
	}
	return tool(params)
}

func (r *ToolRegistry) ListTools() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		tools = append(tools, name)
	}
	return tools
}

func (r *ToolRegistry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.tools[name]
	return exists
}

func (r *ToolRegistry) ListToolsWithDocs() []ToolInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]ToolInfo, 0, len(r.meta))
	for _, info := range r.meta {
		tools = append(tools, info)
	}
	return tools
}

func (r *ToolRegistry) GetToolInfo(name string) (ToolInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	info, exists := r.meta[name]
	return info, exists
}

func (r *ToolRegistry) ToLLMTools() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]map[string]any, 0, len(r.meta))
	for _, info := range r.meta {
		params := make(map[string]map[string]any)
		for name, p := range info.Parameters {
			params[name] = map[string]any{
				"type":        p.Type,
				"description": p.Description,
				"required":    p.Required,
			}
		}
		result = append(result, map[string]any{
			"name":        info.Name,
			"description": info.Description,
			"parameters":  params,
		})
	}
	return result
}

func validateToolParams(info ToolInfo, params map[string]any) error {
	if params == nil {
		params = map[string]any{}
	}
	for name, spec := range info.Parameters {
		value, exists := params[name]
		if spec.Required && !exists {
			return fmt.Errorf("%s parameter required", name)
		}
		if !exists {
			continue
		}
		if !matchesType(spec.Type, value) {
			return fmt.Errorf("%s must be %s", name, spec.Type)
		}
	}
	return nil
}

func matchesType(expected string, value any) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "bool":
		_, ok := value.(bool)
		return ok
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, float32, float64:
			return true
		default:
			return false
		}
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	default:
		return true
	}
}
