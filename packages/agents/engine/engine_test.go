package engine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/agents/types"
	browsertools "qa-orchestrator/packages/browser-runtime/tools"
	"qa-orchestrator/packages/runtime"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/session"
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

type sequenceLLMClient struct {
	responses []string
	idx       int
}

func (m *sequenceLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return "", nil
}

func (m *sequenceLLMClient) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	if m.idx >= len(m.responses) {
		return `[{"tool":"finish","params":{},"reason":"done"}]`, nil
	}
	resp := m.responses[m.idx]
	m.idx++
	return resp, nil
}

func TestAutonomousFlow_CancelsDuringGeneration(t *testing.T) {
	registry := executor.NewMockToolRegistry()
	llmClient := &cancelAwareLLMClient{started: make(chan struct{})}
	engine := NewAgentEngineWithLLM(registry, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})
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

func TestAutonomousFlow_MultiTurnUntilFinish(t *testing.T) {
	registry := executor.NewMockToolRegistry()
	llmClient := &sequenceLLMClient{
		responses: []string{
			`[{"tool":"echo","params":{"value":"step1"},"reason":"first"}]`,
			`[{"tool":"finish","params":{},"reason":"goal achieved"}]`,
		},
	}
	engine := NewAgentEngineWithLLM(registry, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	flow := types.Flow{
		ID:   "auto-flow",
		Mode: sharedtypes.FlowModeAutonomous,
		Goal: "Do two turns then finish",
	}

	result := engine.RunFlow("run_test", flow)
	if result.Outcome != OutcomePass {
		t.Fatalf("expected pass, got %s (%v)", result.Outcome, result.Errors)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected one executed step before finish, got %d", len(result.Steps))
	}
	if llmClient.idx < 2 {
		t.Fatalf("expected at least 2 LLM turns, got %d", llmClient.idx)
	}
}

func TestGuidedFlow_ReplanFallsBackToRetry(t *testing.T) {
	engine := NewAgentEngine()
	attempts := 0
	engine.RegisterTool("flaky_locator", func(params map[string]any) (any, error) {
		attempts++
		if attempts == 1 {
			return nil, fmt.Errorf("locator error: element not found")
		}
		return "ok", nil
	})

	flow := types.Flow{
		ID:   "guided-flow",
		Mode: sharedtypes.FlowModeGuided,
		Steps: []types.Step{
			{ID: "s1", Tool: "flaky_locator", Params: map[string]any{}},
		},
	}

	result := engine.RunFlow("run_test", flow)
	if result.Outcome != OutcomePass {
		t.Fatalf("expected pass, got %s (%v)", result.Outcome, result.Errors)
	}
	if attempts < 2 {
		t.Fatalf("expected retry attempt, got %d attempts", attempts)
	}
}

func TestRunFlow_CancelBeforeExecution_FinalizesSkippedState(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	campaign := &sharedtypes.Campaign{
		Name: "cancel-test",
		Flows: []sharedtypes.Flow{
			{ID: "flow-cancel", Name: "Cancel Flow", Mode: sharedtypes.FlowModeGuided, Priority: sharedtypes.FlowPriorityMedium},
		},
	}
	sess, err := store.Create(campaign)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	engine := NewAgentEngineWithStores(executor.NewMockToolRegistry(), store, nil, nil)
	lc := runtime.NewLifecycleController(sess.RunID)
	lc.RequestCancel()
	engine.SetLifecycleController(lc)

	flow := types.Flow{
		ID:   "flow-cancel",
		Mode: sharedtypes.FlowModeGuided,
		Steps: []types.Step{
			{ID: "s1", Tool: "echo", Params: map[string]any{"value": "x"}},
		},
	}

	result := engine.RunFlow(sess.RunID, flow)
	if result.Outcome != OutcomeSkip {
		t.Fatalf("expected SKIPPED outcome, got %s", result.Outcome)
	}

	updated, err := store.Get(sess.RunID)
	if err != nil {
		t.Fatalf("failed to load updated session: %v", err)
	}
	if len(updated.Flows) != 1 {
		t.Fatalf("expected 1 flow state, got %d", len(updated.Flows))
	}
	if updated.Flows[0].Status != sharedtypes.FlowStateSkippedUser {
		t.Fatalf("expected SKIPPED_USER, got %s", updated.Flows[0].Status)
	}
}
