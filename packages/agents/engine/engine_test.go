package engine

import (
	"testing"

	"qa-orchestrator/packages/agents/types"
)

func TestAgentEngineRunFlowSuccess(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Goal: "Test the agent loop",
		Steps: []types.Step{
			{ID: "step1", Tool: "log", Params: map[string]any{"message": "starting"}},
			{ID: "step2", Tool: "echo", Params: map[string]any{"value": "done"}},
		},
	}

	result := engine.RunFlow("run_test", flow)

	if result.Outcome != OutcomePass {
		t.Errorf("Outcome = %s, want PASSED", result.Outcome)
	}

	if len(result.Steps) != 2 {
		t.Errorf("len(Steps) = %d, want 2", len(result.Steps))
	}

	for _, step := range result.Steps {
		if !step.Success {
			t.Errorf("Step %s failed: %v", step.StepID, step.Error)
		}
	}
}

func TestAgentEngineRunFlowWithFailure(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Goal: "Test failure handling",
		Steps: []types.Step{
			{ID: "step1", Tool: "log", Params: map[string]any{"message": "starting"}},
			{ID: "step2", Tool: "unknown_tool", Params: nil},
		},
	}

	result := engine.RunFlow("run_test", flow)

	if result.Outcome == OutcomePass {
		t.Error("Outcome should not be PASSED with unknown tool")
	}

	if len(result.Steps) < 2 {
		t.Errorf("Should have at least 2 steps, got %d", len(result.Steps))
	}
}

func TestAgentEngineRunFlowWithAssertion(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Goal: "Test assertions",
		Steps: []types.Step{
			{
				ID:   "step1",
				Tool: "echo",
				Params: map[string]any{"value": "hello"},
				Assertions: []types.Assertion{
					{Type: "equals", Value: "hello"},
				},
			},
		},
	}

	result := engine.RunFlow("run_test", flow)

	if result.Outcome != OutcomePass {
		t.Errorf("Outcome = %s, want PASSED", result.Outcome)
	}
}

func TestAgentEngineRunFlowWithAssertionFailure(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Goal: "Test assertion failure",
		Steps: []types.Step{
			{
				ID:   "step1",
				Tool: "echo",
				Params: map[string]any{"value": "hello"},
				Assertions: []types.Assertion{
					{Type: "equals", Value: "world"},
				},
			},
		},
	}

	result := engine.RunFlow("run_test", flow)

	if result.Outcome == OutcomePass {
		t.Error("Outcome should be FAILED with wrong assertion")
	}
}

func TestAgentEngineRunFlowWithRetry(t *testing.T) {
	engine := NewAgentEngine()

	engine.RegisterTool("flaky", func(params map[string]any) (any, error) {
		return "flaky success", nil
	})

	flow := types.Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Goal: "Test with retry",
		Steps: []types.Step{
			{ID: "step1", Tool: "flaky", Params: nil},
		},
	}

	result := engine.RunFlowWithRetry("run_test", flow, 2)

	if result.Outcome != OutcomePass {
		t.Errorf("Outcome = %s, want PASSED", result.Outcome)
	}
}

func TestAgentEngineGetAgents(t *testing.T) {
	engine := NewAgentEngine()

	if engine.GetPlanner() == nil {
		t.Error("Planner should not be nil")
	}

	if engine.GetExecutor() == nil {
		t.Error("Executor should not be nil")
	}

	if engine.GetValidator() == nil {
		t.Error("Validator should not be nil")
	}

	if engine.GetRecovery() == nil {
		t.Error("Recovery should not be nil")
	}
}

func TestAgentEngineEmptyFlow(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:    "empty-flow",
		Name:  "Empty Flow",
		Goal:  "No steps",
		Steps: []types.Step{},
	}

	result := engine.RunFlow("run_test", flow)

	if result.Outcome != OutcomePass {
		t.Errorf("Outcome = %s, want PASSED for empty flow", result.Outcome)
	}
}

func TestAgentEngineRunFlowWithDelay(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:   "test-flow",
		Name: "Test Flow",
		Goal: "Test delay tool",
		Steps: []types.Step{
			{ID: "step1", Tool: "delay", Params: map[string]any{"ms": 10.0}},
		},
	}

	result := engine.RunFlow("run_test", flow)

	if result.Outcome != OutcomePass {
		t.Errorf("Outcome = %s, want PASSED", result.Outcome)
	}

	if result.DurationMs < 10 {
		t.Errorf("DurationMs = %d, should be at least 10", result.DurationMs)
	}
}
