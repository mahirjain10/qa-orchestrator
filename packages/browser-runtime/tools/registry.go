package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"qa-orchestrator/packages/browser-runtime"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

const selectorExistsJS = `((selector) => {
	try {
		const el = document.querySelector(selector);
		if (!el) return JSON.stringify({exists: false});
		const rect = el.getBoundingClientRect();
		return JSON.stringify({exists: true, visible: rect.width > 0 && rect.height > 0});
	} catch(e) {
		return JSON.stringify({exists: false, error: e.message});
	}
})`

type Tool func(params map[string]any) (any, error)

type ParameterInfo = sharedtypes.ParameterInfo
type ToolInfo = sharedtypes.ToolInfo

type ToolRegistry struct {
	mu       sync.RWMutex
	tools    map[string]Tool
	meta     map[string]ToolInfo
	ctx      context.Context
	cancelFn context.CancelFunc
}

func NewToolRegistry(runtime browserruntime.BrowserRuntimeInterface) *ToolRegistry {
	return NewToolRegistryWithContext(runtime, context.Background())
}

func NewToolRegistryWithContext(runtime browserruntime.BrowserRuntimeInterface, ctx context.Context) *ToolRegistry {
	ctx, cancel := context.WithCancel(ctx)
	registry := &ToolRegistry{
		tools:    make(map[string]Tool),
		meta:     make(map[string]ToolInfo),
		ctx:      ctx,
		cancelFn: cancel,
	}
	registry.registerDefaultTools(runtime)
	return registry
}

func (r *ToolRegistry) Cancel() {
	if r.cancelFn != nil {
		r.cancelFn()
	}
}

func (r *ToolRegistry) checkSelectorExists(runtime browserruntime.BrowserRuntimeInterface, selector string) error {
	selectorJSON, _ := json.Marshal(selector)
	js := selectorExistsJS + `(` + string(selectorJSON) + `)`
	result, err := runtime.Evaluate(r.ctx, js)
	if err != nil {
		return fmt.Errorf("selector check eval failed: %w", err)
	}
	str, ok := result.(string)
	if !ok {
		return fmt.Errorf("selector check: unexpected result type %T", result)
	}
	var check struct {
		Exists  bool   `json:"exists"`
		Visible bool   `json:"visible"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal([]byte(str), &check); err != nil {
		return fmt.Errorf("selector check: failed to parse result: %w", err)
	}
	if !check.Exists {
		if check.Error != "" {
			return fmt.Errorf("invalid selector syntax for '%s': %s", selector, check.Error)
		}
		return fmt.Errorf("selector '%s' not found on page — selector does not exist in current DOM", selector)
	}
	return nil
}

func (r *ToolRegistry) registerDefaultTools(runtime browserruntime.BrowserRuntimeInterface) {
	r.registerNavigate(runtime)
	r.registerClick(runtime)
	r.registerTypeText(runtime)
	r.registerSelectOption(runtime)
	r.registerWaitFor(runtime)
	r.registerGetText(runtime)
	r.registerGetHTML(runtime)
	r.registerEvaluate(runtime)
	r.registerScreenshot(runtime)
	r.registerFinish(runtime)
	r.registerObserveUI(runtime)
	r.registerEcho()
}

func (r *ToolRegistry) registerFinish(runtime browserruntime.BrowserRuntimeInterface) {
	r.Register("finish", map[string]ParameterInfo{
		"status": {Type: "string", Description: "Set to 'success' if goal is achieved, or 'fail' if the goal is unachievable (e.g. elements not found).", Required: false},
	}, func(params map[string]any) (any, error) {
		status, ok := params["status"].(string)
		if ok && status == "fail" {
			return "goal unachievable, execution complete", nil
		}
		return "goal achieved, execution complete", nil
	})
}

func (r *ToolRegistry) registerEcho() {
	r.RegisterHidden("echo", map[string]ParameterInfo{
		"value": {Type: "string", Description: "The value to echo back", Required: true},
	}, func(params map[string]any) (any, error) {
		return params["value"], nil
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

func (r *ToolRegistry) RegisterHidden(name string, params map[string]ParameterInfo, fn Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.tools[name] = fn
	r.meta[name] = ToolInfo{
		Name:        name,
		Description: getToolDescription(name),
		Parameters:  params,
		Hidden:      true,
	}
}

func getToolDescription(name string) string {
	descriptions := map[string]string{
		"navigate":      "Navigate to a URL in the browser",
		"click":         "Click on an element identified by CSS selector",
		"type_text":     "Type text into an input field",
		"select_option": "Select an option in a <select> element by value, label, or index",
		"wait_for":      "Wait for an element to reach a specific state",
		"get_text":      "Get the text content of an element",
		"get_html":      "Get the inner HTML of an element",
		"evaluate":      "Evaluate a JavaScript expression in the browser context",
		"screenshot":    "Take a screenshot of the page",
		"finish":        "Signal that the goal has been achieved (or is unachievable) and no more steps are needed",
		"observe_ui":    "Inspect the current page and return a list of visible interactive elements with their selectors",
		"echo":          "Return the provided value as-is. Useful for testing tool integration and guided flow verification.",
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
		return nil, fmt.Errorf("execute: %w", err)
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
		if info.Hidden {
			continue
		}
		tools = append(tools, info)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})
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

	sorted := make([]ToolInfo, 0, len(r.meta))
	for _, info := range r.meta {
		sorted = append(sorted, info)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Name < sorted[j].Name
	})

	result := make([]map[string]any, 0, len(sorted))
	for _, info := range sorted {
		if info.Hidden {
			continue
		}
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
	for name := range params {
		if _, known := info.Parameters[name]; !known {
			return fmt.Errorf("unknown parameter: %s", name)
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
		case int, int8, int16, int32, int64, float32, float64, json.Number:
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
