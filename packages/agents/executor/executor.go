package executor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"qa-orchestrator/packages/agents/types"
)

type ToolRegistry interface {
	Execute(tool string, params map[string]any) (any, error)
}

type MockToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]func(params map[string]any) (any, error)
}

func NewMockToolRegistry() *MockToolRegistry {
	registry := &MockToolRegistry{
		tools: make(map[string]func(params map[string]any) (any, error)),
	}
	registry.registerDefaultTools()
	return registry
}

func (r *MockToolRegistry) registerDefaultTools() {
	r.Register("log", func(params map[string]any) (any, error) {
		msg, _ := params["message"].(string)
		return fmt.Sprintf("logged: %s", msg), nil
	})

	r.Register("delay", func(params map[string]any) (any, error) {
		ms, ok := params["ms"].(float64)
		if !ok {
			ms = 100
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return fmt.Sprintf("delayed %dms", int(ms)), nil
	})

	r.Register("assert_true", func(params map[string]any) (any, error) {
		condition, ok := params["condition"].(bool)
		if !ok {
			return nil, fmt.Errorf("condition must be boolean")
		}
		if !condition {
			return nil, fmt.Errorf("assertion failed: condition is false")
		}
		return true, nil
	})

	r.Register("echo", func(params map[string]any) (any, error) {
		return params["value"], nil
	})

	// Browser tool mocks for simulation
	r.Register("navigate", func(params map[string]any) (any, error) {
		url, _ := params["url"].(string)
		lower := strings.ToLower(url)
		if strings.Contains(lower, "does-not-exist") || strings.Contains(lower, "fail-test") || strings.Contains(lower, "error-test") {
			return nil, fmt.Errorf("navigate failed: connection refused for %s", url)
		}
		if strings.Contains(lower, "timeout") {
			return nil, fmt.Errorf("navigate failed: timeout exceeded waiting for %s", url)
		}
		return fmt.Sprintf("simulated: navigated to %s", url), nil
	})

	r.Register("click", func(params map[string]any) (any, error) {
		selector, _ := params["selector"].(string)
		lower := strings.ToLower(selector)
		if strings.Contains(lower, "nonexistent") || strings.Contains(lower, "missing") {
			return nil, fmt.Errorf("selector '%s' not found on page — selector does not exist in current DOM", selector)
		}
		return fmt.Sprintf("simulated: clicked %s", selector), nil
	})

	r.Register("type_text", func(params map[string]any) (any, error) {
		selector, _ := params["selector"].(string)
		value, _ := params["value"].(string)
		return fmt.Sprintf("simulated: typed '%s' into %s", value, selector), nil
	})

	r.Register("wait_for", func(params map[string]any) (any, error) {
		selector, _ := params["selector"].(string)
		lower := strings.ToLower(selector)
		if strings.Contains(lower, "nonexistent") || strings.Contains(lower, "missing") {
			return nil, fmt.Errorf("selector '%s' not found on page — selector does not exist in current DOM", selector)
		}
		return fmt.Sprintf("simulated: waited for %s", selector), nil
	})

	r.Register("get_text", func(params map[string]any) (any, error) {
		selector, _ := params["selector"].(string)
		lower := strings.ToLower(selector)
		if strings.Contains(lower, "nonexistent") || strings.Contains(lower, "missing") {
			return nil, fmt.Errorf("selector '%s' not found on page — selector does not exist in current DOM", selector)
		}
		return fmt.Sprintf("simulated text from %s", selector), nil
	})

	r.Register("screenshot", func(params map[string]any) (any, error) {
		return "simulated: screenshot captured", nil
	})

	r.Register("assert_text_visible", func(params map[string]any) (any, error) {
		text, _ := params["text"].(string)
		lower := strings.ToLower(text)
		if strings.Contains(lower, "nonexistent") || strings.Contains(lower, "missing-text") || strings.Contains(lower, "not-on-page") {
			return nil, fmt.Errorf("assertion failed: text '%s' is not visible on the page", text)
		}
		return fmt.Sprintf("simulated: verified '%s' is visible", text), nil
	})

	r.Register("observe_ui", func(params map[string]any) (any, error) {
		return map[string]any{
			"page_state": "loaded",
			"interactive": []any{
				map[string]any{"tag": "input", "selector": "#username", "text": "Username"},
				map[string]any{"tag": "input", "selector": "#password", "text": ""},
				map[string]any{"tag": "button", "selector": "#login-btn", "text": "Login"},
				map[string]any{"tag": "a", "selector": "a[href='/register']", "text": "Register"},
				map[string]any{"tag": "h1", "selector": "h1", "text": "Welcome"},
			},
		}, nil
	})
}

func (r *MockToolRegistry) Register(name string, fn func(params map[string]any) (any, error)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools[name] = fn
}

func (r *MockToolRegistry) Execute(tool string, params map[string]any) (any, error) {
	r.mu.RLock()
	fn, exists := r.tools[tool]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}

	return fn(params)
}

type Executor struct {
	registry ToolRegistry
}

func NewExecutor(registry ToolRegistry) *Executor {
	return &Executor{
		registry: registry,
	}
}

func (e *Executor) ExecuteStep(planStep *types.PlanStep) *types.StepResult {
	start := time.Now()

	result := &types.StepResult{
		StepID:  planStep.StepID,
		Tool:    planStep.Tool,
		Params:  planStep.Params,
		Success: false,
	}

	output, err := e.registry.Execute(planStep.Tool, planStep.Params)
	result.Output = output
	result.Error = err

	if err != nil {
		result.Success = false
	} else {
		result.Success = true
	}

	result.DurationMs = time.Since(start).Milliseconds()
	return result
}

func (e *Executor) ExecutePlan(plan *types.Plan) []*types.StepResult {
	var results []*types.StepResult

	for i, step := range plan.Steps {
		if step.Skip {
			results = append(results, &types.StepResult{
				StepID:  step.StepID,
				Tool:    step.Tool,
				Params:  step.Params,
				Output:  "skipped",
				Success: true,
			})
			plan.CurrentIdx = i + 1
			continue
		}

		result := e.ExecuteStep(&step)
		results = append(results, result)

		if !result.Success {
			break
		}

		plan.CurrentIdx = i + 1
	}

	return results
}

func (e *Executor) GetRegistry() ToolRegistry {
	return e.registry
}
