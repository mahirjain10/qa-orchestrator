package executor

import (
	"time"

	"qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/browser-runtime/tools"
)

type ToolRegistry interface {
	Execute(tool string, params map[string]any) (any, error)
}

type MockToolRegistry = tools.MockToolRegistry

func NewMockToolRegistry() *MockToolRegistry {
	return tools.NewMockToolRegistry()
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
