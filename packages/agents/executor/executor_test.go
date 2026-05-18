package executor

import (
	"testing"

	"qa-orchestrator/packages/agents/types"
)

func TestMockToolRegistryRegisterAndExecute(t *testing.T) {
	registry := NewMockToolRegistry()

	result, err := registry.Execute("log", map[string]any{"message": "test"})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if result != "logged: test" {
		t.Errorf("result = %s, want 'logged: test'", result)
	}
}

func TestMockToolRegistryUnknownTool(t *testing.T) {
	registry := NewMockToolRegistry()

	_, err := registry.Execute("unknown_tool", nil)
	if err == nil {
		t.Error("Expected error for unknown tool")
	}
}

func TestExecutorExecuteStep(t *testing.T) {
	registry := NewMockToolRegistry()
	executor := NewExecutor(registry)

	step := &types.PlanStep{
		StepIndex: 0,
		StepID:    "test-step",
		Tool:      "echo",
		Params:    map[string]any{"value": "hello world"},
	}

	result := executor.ExecuteStep(step)

	if !result.Success {
		t.Errorf("Success = false, want true")
	}

	if result.Output != "hello world" {
		t.Errorf("Output = %v, want 'hello world'", result.Output)
	}
}

func TestExecutorExecuteStepFailure(t *testing.T) {
	registry := NewMockToolRegistry()
	executor := NewExecutor(registry)

	step := &types.PlanStep{
		StepIndex: 0,
		StepID:    "test-step",
		Tool:      "assert_true",
		Params:    map[string]any{"condition": false},
	}

	result := executor.ExecuteStep(step)

	if result.Success {
		t.Errorf("Success = true, want false")
	}

	if result.Error == nil {
		t.Error("Error should not be nil for failed step")
	}
}

func TestExecutorExecutePlan(t *testing.T) {
	registry := NewMockToolRegistry()
	executor := NewExecutor(registry)

	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log", Params: map[string]any{"message": "first"}},
			{StepIndex: 1, StepID: "step2", Tool: "log", Params: map[string]any{"message": "second"}},
		},
	}

	results := executor.ExecutePlan(plan)

	if len(results) != 2 {
		t.Errorf("len(results) = %d, want 2", len(results))
	}

	if plan.CurrentIdx != 2 {
		t.Errorf("plan.CurrentIdx = %d, want 2", plan.CurrentIdx)
	}
}
