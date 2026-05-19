package engine

import (
	"testing"

	"qa-orchestrator/packages/agents/types"
)

func TestAgentEngineSimulation(t *testing.T) {
	engine := NewAgentEngine()

	flow := types.Flow{
		ID:   "simulation-flow",
		Name: "Simulation Flow",
		Goal: "Test simulation of browser tools",
		Steps: []types.Step{
			{ID: "step1", Tool: "navigate", Params: map[string]any{"url": "https://example.com"}},
			{ID: "step2", Tool: "click", Params: map[string]any{"selector": "button"}},
		},
	}

	result := engine.RunFlow("run_sim", flow)

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
