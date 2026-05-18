package tools

import (
	"fmt"
	"sync"

	"qa-orchestrator/packages/browser-runtime"
)

type Tool func(params map[string]any) (any, error)

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewToolRegistry(runtime *browserruntime.BrowserRuntime) *ToolRegistry {
	registry := &ToolRegistry{
		tools: make(map[string]Tool),
	}
	registry.registerDefaultTools(runtime)
	return registry
}

func (r *ToolRegistry) registerDefaultTools(runtime *browserruntime.BrowserRuntime) {
	r.Register("navigate", func(params map[string]any) (any, error) {
		url, ok := params["url"].(string)
		if !ok {
			return nil, fmt.Errorf("url parameter required")
		}
		if err := runtime.Navigate(url); err != nil {
			return nil, fmt.Errorf("navigate failed: %w", err)
		}
		return fmt.Sprintf("navigated to %s", url), nil
	})

	r.Register("click", func(params map[string]any) (any, error) {
		selector, ok := params["selector"].(string)
		if !ok {
			return nil, fmt.Errorf("selector parameter required")
		}
		if err := runtime.Click(selector); err != nil {
			return nil, fmt.Errorf("click failed: %w", err)
		}
		return fmt.Sprintf("clicked %s", selector), nil
	})

	r.Register("type_text", func(params map[string]any) (any, error) {
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

	r.Register("wait_for", func(params map[string]any) (any, error) {
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

	r.Register("get_text", func(params map[string]any) (any, error) {
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

	r.Register("get_html", func(params map[string]any) (any, error) {
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

	r.Register("evaluate", func(params map[string]any) (any, error) {
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

	r.Register("screenshot", func(params map[string]any) (any, error) {
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

func (r *ToolRegistry) Register(name string, fn Tool) {
	r.tools[name] = fn
}

func (r *ToolRegistry) Execute(name string, params map[string]any) (any, error) {
	tool, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
	return tool(params)
}

func (r *ToolRegistry) ListTools() []string {
	tools := make([]string, 0, len(r.tools))
	for name := range r.tools {
		tools = append(tools, name)
	}
	return tools
}

func (r *ToolRegistry) HasTool(name string) bool {
	_, exists := r.tools[name]
	return exists
}
