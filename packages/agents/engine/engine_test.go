package engine

import (
	"context"
	"testing"
	"time"

	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/agents/types"
	browsertools "qa-orchestrator/packages/browser-runtime/tools"
	"qa-orchestrator/packages/runtime"
	sharedtypes "qa-orchestrator/packages/shared/types"
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
				ID:     "step1",
				Tool:   "echo",
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
				ID:     "step1",
				Tool:   "echo",
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

type mockBrowserTools struct {
	docs []browsertools.ToolInfo
}

func (m *mockBrowserTools) ListToolsWithDocs() []browsertools.ToolInfo {
	return m.docs
}

func TestConvertToLLMTools_UsesRegistryDocs(t *testing.T) {
	tools := &mockBrowserTools{
		docs: []browsertools.ToolInfo{
			{
				Name:        "custom_tool",
				Description: "custom",
				Parameters: map[string]browsertools.ParameterInfo{
					"arg": {Type: "string", Description: "arg", Required: true},
				},
			},
		},
	}

	result := convertToLLMTools(tools)
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}
	if result[0].Name != "custom_tool" {
		t.Fatalf("expected custom_tool, got %s", result[0].Name)
	}
}

type cancelAwareLLMClient struct {
	started chan struct{}
}

func (m *cancelAwareLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	<-ctx.Done()
	return "", ctx.Err()
}

func (m *cancelAwareLLMClient) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	close(m.started)
	<-ctx.Done()
	return "", ctx.Err()
}

func TestAutonomousFlow_CancelsDuringGeneration(t *testing.T) {
	registry := executor.NewMockToolRegistry()
	llmClient := &cancelAwareLLMClient{started: make(chan struct{})}
	engine := NewAgentEngineWithLLM(registry, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})
	lifecycle := runtime.NewLifecycleController("run_test")
	engine.SetLifecycleController(lifecycle)

	flow := types.Flow{
		ID:   "auto-flow",
		Mode: sharedtypes.FlowModeAutonomous,
		Goal: "Generate one step",
	}

	done := make(chan *ExecutionResult, 1)
	go func() {
		done <- engine.RunFlow("run_test", flow)
	}()

	select {
	case <-llmClient.started:
	case <-time.After(2 * time.Second):
		t.Fatal("LLM generation did not start")
	}

	lifecycle.RequestCancel()

	select {
	case result := <-done:
		if result.Outcome != OutcomeSkip {
			t.Fatalf("expected OutcomeSkip, got %s with errors %v", result.Outcome, result.Errors)
		}
		if len(result.Errors) == 0 {
			t.Fatal("expected cancellation error details")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("RunFlow did not return after cancellation")
	}
}
