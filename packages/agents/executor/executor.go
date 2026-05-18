package executor

import (
	"fmt"
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
		StepID: planStep.StepID,
		Tool:   planStep.Tool,
		Params: planStep.Params,
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
				StepID: step.StepID,
				Tool:   step.Tool,
				Params: step.Params,
				Output: "skipped",
				Success: true,
			})
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
