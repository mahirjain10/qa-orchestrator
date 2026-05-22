package engine

import (
	"context"
	"fmt"
	"strings"
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
	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})
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
	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

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

	_ = store.UpdateStatus(sess.RunID, sharedtypes.RunStateCancelling)

	engine := NewAgentEngineWithStores(executor.NewMockToolRegistry(), store, nil, nil)

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
		t.Fatalf("expected SKIPPED_USER (cancelled flow), got %s", updated.Flows[0].Status)
	}
}

func TestRunFlow_PauseResumeViaDBPolling(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	campaign := &sharedtypes.Campaign{
		Name: "pause-resume-test",
		Flows: []sharedtypes.Flow{
			{ID: "flow-pause", Name: "Pause Flow", Mode: sharedtypes.FlowModeGuided, Priority: sharedtypes.FlowPriorityMedium,
				Steps: []sharedtypes.Step{
					{ID: "s1", Tool: "echo", Params: map[string]any{"value": "step1"}},
					{ID: "s2", Tool: "echo", Params: map[string]any{"value": "step2"}},
					{ID: "s3", Tool: "echo", Params: map[string]any{"value": "step3"}},
				},
			},
		},
	}
	sess, err := store.Create(campaign)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	engine := NewAgentEngineWithStores(executor.NewMockToolRegistry(), store, nil, nil)

	go func() {
		time.Sleep(100 * time.Millisecond)
		_ = store.UpdateStatus(sess.RunID, sharedtypes.RunStatePausing)
		time.Sleep(200 * time.Millisecond)
		_ = store.UpdateStatus(sess.RunID, sharedtypes.RunStateResuming)
	}()

	flow := sharedtypes.Flow{
		ID:   "flow-pause",
		Mode: sharedtypes.FlowModeGuided,
		Steps: []sharedtypes.Step{
			{ID: "s1", Tool: "echo", Params: map[string]any{"value": "step1"}},
			{ID: "s2", Tool: "echo", Params: map[string]any{"value": "step2"}},
			{ID: "s3", Tool: "echo", Params: map[string]any{"value": "step3"}},
		},
	}

	result := engine.RunFlow(sess.RunID, flow)
	if result.Outcome != OutcomePass {
		t.Fatalf("expected PASSED outcome, got %s (%v)", result.Outcome, result.Errors)
	}
	if len(result.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(result.Steps))
	}
}

type observeUIToolRegistry struct {
	executor.MockToolRegistry
	callCount int
}

func (m *observeUIToolRegistry) Execute(tool string, params map[string]any) (any, error) {
	if tool == "observe_ui" {
		m.callCount++
		return map[string]any{
			"page_state": "loaded",
			"interactive": []any{
				map[string]any{"tag": "input", "selector": "#username", "text": ""},
				map[string]any{"tag": "button", "selector": "#submit", "text": "Login"},
			},
		}, nil
	}
	return m.MockToolRegistry.Execute(tool, params)
}

func (m *observeUIToolRegistry) HasTool(tool string) bool {
	return tool == "observe_ui"
}

func TestAutoObserve_AppendsAfterValidatorObservation(t *testing.T) {
	registry := &observeUIToolRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}

	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, nil, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	ctx := &types.ExecutionContext{
		RunID:        "run_test",
		FlowID:       "flow_test",
		Goal:         "test observation ordering",
		Mode:         sharedtypes.FlowModeAutonomous,
		Observations: make([]types.Observation, 0),
	}

	planStep := &types.PlanStep{
		StepID:    "step1",
		Tool:      "echo",
		Params:    map[string]any{"value": "hello"},
		StepIndex: 0,
	}

	result := engine.executeAndValidate(ctx, planStep)

	if !result.Success {
		t.Fatalf("step should succeed: %v", result.Error)
	}

	if len(ctx.Observations) != 2 {
		t.Fatalf("expected 2 observations (validator + observe_ui), got %d", len(ctx.Observations))
	}

	validatorObs := ctx.Observations[0]
	observeObs := ctx.Observations[1]

	if validatorObs.LastStep.Tool != "echo" {
		t.Errorf("first observation should be from validator (echo), got tool=%s", validatorObs.LastStep.Tool)
	}

	if observeObs.LastStep.Tool != "observe_ui" {
		t.Errorf("second observation should be observe_ui, got tool=%s", observeObs.LastStep.Tool)
	}

	if registry.callCount != 1 {
		t.Errorf("observe_ui should be called once, got %d", registry.callCount)
	}
}

func TestAutoObserve_ObserveUIIsLastObservation(t *testing.T) {
	registry := &observeUIToolRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}

	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, nil, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	ctx := &types.ExecutionContext{
		RunID:        "run_test",
		FlowID:       "flow_test",
		Goal:         "test observe_ui is last",
		Mode:         sharedtypes.FlowModeAutonomous,
		Observations: make([]types.Observation, 0),
	}

	planStep := &types.PlanStep{
		StepID:    "step1",
		Tool:      "echo",
		Params:    map[string]any{"value": "test"},
		StepIndex: 0,
	}

	engine.executeAndValidate(ctx, planStep)

	lastObs := ctx.Observations[len(ctx.Observations)-1]
	if lastObs.LastStep.Tool != "observe_ui" {
		t.Errorf("last observation should be observe_ui, got %s", lastObs.LastStep.Tool)
	}
}

func TestAutoObserve_TriggeredOnFailure(t *testing.T) {
	registry := &observeUIToolRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}

	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, nil, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	ctx := &types.ExecutionContext{
		RunID:        "run_test",
		FlowID:       "flow_test",
		Goal:         "test observe_ui on failure",
		Mode:         sharedtypes.FlowModeAutonomous,
		Observations: make([]types.Observation, 0),
	}

	planStep := &types.PlanStep{
		StepID:    "step1",
		Tool:      "unknown_tool",
		Params:    nil,
		StepIndex: 0,
	}

	result := engine.executeAndValidate(ctx, planStep)

	if result.Success {
		t.Fatal("step should fail with unknown_tool")
	}

	if len(ctx.Observations) != 2 {
		t.Fatalf("expected 2 observations (validator + observe_ui), got %d", len(ctx.Observations))
	}

	lastObs := ctx.Observations[len(ctx.Observations)-1]
	if lastObs.LastStep.Tool != "observe_ui" {
		t.Errorf("last observation should be observe_ui even on failure, got %s", lastObs.LastStep.Tool)
	}
}

func TestAutoObserve_SkippedInGuidedMode(t *testing.T) {
	registry := &observeUIToolRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}

	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, nil, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	ctx := &types.ExecutionContext{
		RunID:        "run_test",
		FlowID:       "flow_test",
		Goal:         "test guided mode skips autoObserve",
		Mode:         sharedtypes.FlowModeGuided,
		Observations: make([]types.Observation, 0),
	}

	planStep := &types.PlanStep{
		StepID:    "step1",
		Tool:      "echo",
		Params:    map[string]any{"value": "test"},
		StepIndex: 0,
	}

	engine.executeAndValidate(ctx, planStep)

	if len(ctx.Observations) != 1 {
		t.Fatalf("expected 1 observation (validator only), got %d", len(ctx.Observations))
	}

	if registry.callCount != 0 {
		t.Errorf("observe_ui should not be called in guided mode, got %d calls", registry.callCount)
	}
}

func TestAutoObserve_CapsObservations(t *testing.T) {
	registry := &observeUIToolRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}

	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, nil, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	ctx := &types.ExecutionContext{
		RunID:        "run_test",
		FlowID:       "flow_test",
		Goal:         "test observation capping",
		Mode:         sharedtypes.FlowModeAutonomous,
		Observations: make([]types.Observation, 0),
	}

	for i := 0; i < 12; i++ {
		planStep := &types.PlanStep{
			StepID:    fmt.Sprintf("step%d", i),
			Tool:      "echo",
			Params:    map[string]any{"value": fmt.Sprintf("test%d", i)},
			StepIndex: i,
		}
		engine.executeAndValidate(ctx, planStep)
	}

	if len(ctx.Observations) > 10 {
		t.Errorf("expected observations capped at 10, got %d", len(ctx.Observations))
	}
}

func TestStepSignature_Deterministic(t *testing.T) {
	sig1 := stepSignature(&types.PlanStep{Tool: "navigate", Params: map[string]any{"url": "https://example.com"}})
	sig2 := stepSignature(&types.PlanStep{Tool: "navigate", Params: map[string]any{"url": "https://example.com"}})
	if sig1 != sig2 {
		t.Errorf("signatures should be identical for same step, got %q vs %q", sig1, sig2)
	}
}

func TestStepSignature_OrderIndependent(t *testing.T) {
	sig1 := stepSignature(&types.PlanStep{Tool: "click", Params: map[string]any{"selector": "#btn", "modifier": "shift"}})
	sig2 := stepSignature(&types.PlanStep{Tool: "click", Params: map[string]any{"modifier": "shift", "selector": "#btn"}})
	if sig1 != sig2 {
		t.Errorf("signatures should be order-independent, got %q vs %q", sig1, sig2)
	}
}

func TestStepSignature_EmptyParams(t *testing.T) {
	sig := stepSignature(&types.PlanStep{Tool: "observe_ui", Params: map[string]any{}})
	if sig != "" {
		t.Errorf("empty params should return empty sig, got %q", sig)
	}
	sig = stepSignature(&types.PlanStep{Tool: "observe_ui", Params: nil})
	if sig != "" {
		t.Errorf("nil params should return empty sig, got %q", sig)
	}
}

func TestStepSignature_NilStep(t *testing.T) {
	sig := stepSignature(nil)
	if sig != "" {
		t.Errorf("nil step should return empty sig, got %q", sig)
	}
}

func TestStepSignature_DifferentTools(t *testing.T) {
	sig1 := stepSignature(&types.PlanStep{Tool: "click", Params: map[string]any{"selector": "#btn"}})
	sig2 := stepSignature(&types.PlanStep{Tool: "type_text", Params: map[string]any{"selector": "#btn"}})
	if sig1 == sig2 {
		t.Error("different tools with same param key should have different signatures")
	}
}

func TestAutoObserve_ObserveUIDataIsMapNotString(t *testing.T) {
	registry := &observeUIToolRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}

	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, nil, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	ctx := &types.ExecutionContext{
		RunID:        "run_test",
		FlowID:       "flow_test",
		Goal:         "test observe_ui data type",
		Mode:         sharedtypes.FlowModeAutonomous,
		Observations: make([]types.Observation, 0),
	}

	planStep := &types.PlanStep{
		StepID:    "step1",
		Tool:      "echo",
		Params:    map[string]any{"value": "test"},
		StepIndex: 0,
	}

	engine.executeAndValidate(ctx, planStep)

	observeObs := ctx.Observations[1]
	if _, ok := observeObs.State["data"].(map[string]any); !ok {
		t.Errorf("observe_ui data should be map[string]any, got %T", observeObs.State["data"])
	}
}

func TestAutonomousFlow_MaxStepsExhausted_ReturnsFail(t *testing.T) {
	registry := executor.NewMockToolRegistry()
	llmClient := &sequenceLLMClient{
		responses: []string{
			`[{"tool":"echo","params":{"value":"step1"},"reason":"first"}]`,
			`[{"tool":"echo","params":{"value":"step2"},"reason":"second"}]`,
		},
	}
	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	flow := types.Flow{
		ID:   "auto-flow",
		Mode: sharedtypes.FlowModeAutonomous,
		Goal: "Run until max steps exhausted",
		Config: sharedtypes.FlowConfig{
			MaxAutonomousSteps: 2,
		},
	}

	result := engine.RunFlow("run_test", flow)
	if result.Outcome != OutcomeFail {
		t.Fatalf("expected OutcomeFail, got %s with errors %v", result.Outcome, result.Errors)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error on max steps exhaustion")
	}
	if !strings.Contains(result.Errors[0], "max autonomous steps") {
		t.Fatalf("error should mention max autonomous steps, got: %v", result.Errors)
	}
}

func TestAutonomousFlow_RepeatHardBreak_ReturnsFail(t *testing.T) {
	registry := executor.NewMockToolRegistry()
	llmClient := &sequenceLLMClient{
		responses: []string{
			`[{"tool":"echo","params":{"value":"same"},"reason":"first execution"}]`,
			`[{"tool":"echo","params":{"value":"same"},"reason":"repeat 1"}]`,
			`[{"tool":"echo","params":{"value":"same"},"reason":"repeat 2"}]`,
			`[{"tool":"echo","params":{"value":"same"},"reason":"repeat 3"}]`,
		},
	}
	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	flow := types.Flow{
		ID:   "auto-flow",
		Mode: sharedtypes.FlowModeAutonomous,
		Goal: "Trigger repeat hard-break",
		Config: sharedtypes.FlowConfig{
			MaxAutonomousSteps: 10,
		},
	}

	result := engine.RunFlow("run_test", flow)
	if result.Outcome != OutcomeFail {
		t.Fatalf("expected OutcomeFail, got %s with errors %v", result.Outcome, result.Errors)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error on repeat hard-break")
	}
	if !strings.Contains(result.Errors[0], "stuck in loop") {
		t.Fatalf("error should mention stuck in loop, got: %v", result.Errors)
	}
}

// selectiveObserveRegistry returns observe_ui data with only specific selectors,
// simulating a page where only #username and #submit are interactive elements.
type selectiveObserveRegistry struct {
	executor.MockToolRegistry
}

func (m *selectiveObserveRegistry) Execute(tool string, params map[string]any) (any, error) {
	if tool == "observe_ui" {
		return map[string]any{
			"page_state": "loaded",
			"interactive": []any{
				map[string]any{"tag": "input", "selector": "#username", "text": ""},
				map[string]any{"tag": "button", "selector": "#submit", "text": "Login"},
			},
		}, nil
	}
	return m.MockToolRegistry.Execute(tool, params)
}

func (m *selectiveObserveRegistry) HasTool(tool string) bool {
	return tool == "observe_ui"
}

func TestSafetyNet4_ReadOnlyTool_SkipsSelectorValidation(t *testing.T) {
	registry := &selectiveObserveRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}
	llmClient := &sequenceLLMClient{
		responses: []string{
			`[{"tool":"get_text","params":{"selector":"table"},"reason":"check table content"}]`,
			`[{"tool":"finish","params":{"status":"success"},"reason":"goal achieved"}]`,
		},
	}
	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	flow := types.Flow{
		ID:   "auto-flow",
		Mode: sharedtypes.FlowModeAutonomous,
		Goal: "Verify structural selector passes through safety-net 4",
		Config: sharedtypes.FlowConfig{
			MaxAutonomousSteps: 5,
		},
	}

	result := engine.RunFlow("run_test", flow)
	if result.Outcome != OutcomePass {
		t.Fatalf("expected OutcomePass (get_text should skip selector validation), got %s with errors %v", result.Outcome, result.Errors)
	}
	// Verify get_text was actually executed (not blocked)
	foundGetText := false
	for _, step := range result.Steps {
		if step.Tool == "get_text" {
			foundGetText = true
			break
		}
	}
	if !foundGetText {
		t.Fatal("get_text step was blocked by safety-net 4 but should have passed through as read-only tool")
	}
}

func TestCheckpoint_SavesAndRestoresRuntimeState(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := session.NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create session store: %v", err)
	}

	campaign := &sharedtypes.Campaign{Name: "cp-test"}
	sess, err := store.Create(campaign)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	engine := NewAgentEngineWithStores(executor.NewMockToolRegistry(), store, nil, nil)

	ctx := &types.ExecutionContext{
		RunID:                   sess.RunID,
		FlowID:                  "flow-1",
		CurrentURL:              "https://example.com/page",
		LastStepSignature:       "click|selector=#btn",
		ConsecutiveObserveCount: 3,
		Plan:                    &types.Plan{Steps: []types.PlanStep{{StepID: "s1"}}},
	}

	planStep := &types.PlanStep{StepID: "s1", StepIndex: 0}
	engine.saveCheckpoint(sess.RunID, ctx, planStep)

	sess2, err := store.Get(sess.RunID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if sess2.Checkpoint == nil {
		t.Fatal("checkpoint should be saved")
	}
	if sess2.Checkpoint.StepID != "s1" {
		t.Errorf("StepID = %s, want s1", sess2.Checkpoint.StepID)
	}

	restoredCtx := &types.ExecutionContext{
		RunID:  sess.RunID,
		FlowID: "flow-1",
	}
	engine.restoreCheckpoint(restoredCtx)

	if restoredCtx.CurrentURL != "https://example.com/page" {
		t.Errorf("CurrentURL = %q, want %q", restoredCtx.CurrentURL, "https://example.com/page")
	}
	if restoredCtx.LastStepSignature != "click|selector=#btn" {
		t.Errorf("LastStepSignature = %q, want %q", restoredCtx.LastStepSignature, "click|selector=#btn")
	}
	if restoredCtx.ConsecutiveObserveCount != 3 {
		t.Errorf("ConsecutiveObserveCount = %d, want 3", restoredCtx.ConsecutiveObserveCount)
	}
}

func TestCheckpoint_SaveWithoutSessionStore(t *testing.T) {
	engine := NewAgentEngine()
	ctx := &types.ExecutionContext{
		RunID:  "test",
		FlowID: "f1",
		Plan:   &types.Plan{Steps: []types.PlanStep{{StepID: "s1"}}},
	}
	engine.saveCheckpoint("test", ctx, &types.PlanStep{StepID: "s1"})
	engine.restoreCheckpoint(ctx)
}

func TestCheckpoint_EmptyPayloadDoesNotPanic(t *testing.T) {
	engine := NewAgentEngine()
	ctx := &types.ExecutionContext{RunID: "test"}
	engine.restoreCheckpoint(ctx)
}

func TestSafetyNet4_InteractionTool_BlockedBySelectorValidation(t *testing.T) {
	registry := &selectiveObserveRegistry{
		MockToolRegistry: *executor.NewMockToolRegistry(),
	}
	// First generate observe_ui to populate observations with selectors, then
	// try click with a non-observed selector — should be blocked by safety-net 4.
	llmClient := &sequenceLLMClient{
		responses: []string{
			`[{"tool":"observe_ui","params":{},"reason":"observe the page first"}]`,
			`[{"tool":"click","params":{"selector":"table"},"reason":"click table"}]`,
			`[{"tool":"finish","params":{"status":"success"},"reason":"goal achieved"}]`,
		},
	}
	engine := NewAgentEngineWithLLM(registry, nil, nil, nil, llmClient, &mockBrowserTools{docs: []browsertools.ToolInfo{}})

	flow := types.Flow{
		ID:   "auto-flow",
		Mode: sharedtypes.FlowModeAutonomous,
		Goal: "Verify interaction tool is blocked by safety-net 4",
		Config: sharedtypes.FlowConfig{
			MaxAutonomousSteps: 5,
		},
	}

	result := engine.RunFlow("run_test", flow)
	if result.Outcome != OutcomePass {
		t.Fatalf("expected OutcomePass (click blocked, then finish), got %s with errors %v", result.Outcome, result.Errors)
	}
	// Verify click was NOT executed (blocked by safety-net 4)
	foundClick := false
	for _, step := range result.Steps {
		if step.Tool == "click" {
			foundClick = true
			break
		}
	}
	if foundClick {
		t.Fatal("click step was NOT blocked by safety-net 4 but should have been — 'table' is not in observed selectors")
	}
	// Verify observe_ui WAS executed (it's the step that populates observations)
	foundObserveUI := false
	for _, step := range result.Steps {
		if step.Tool == "observe_ui" {
			foundObserveUI = true
			break
		}
	}
	if !foundObserveUI {
		t.Fatal("observe_ui step should have executed to populate observations with selectors")
	}
}

func TestDrainSteeringEvents_InstructionTargetedByFlowID(t *testing.T) {
	eng := NewAgentEngine()
	lc := runtime.NewLifecycleController("run_steer")
	eng.SetLifecycleController(lc)

	ctx := &types.ExecutionContext{
		RunID:               "run_steer",
		FlowID:              "flow-a",
		SteeringInstructions: []string{},
	}

	// Submit an instruction targeted at flow-b (should be skipped)
	lc.SubmitSteering(&sharedtypes.SteeringEvent{
		RunID: "run_steer", FlowID: "flow-b",
		Command: sharedtypes.SteerInstruction, Instruction: "for flow-b",
	})
	// Submit an instruction targeted at flow-a (should be consumed)
	lc.SubmitSteering(&sharedtypes.SteeringEvent{
		RunID: "run_steer", FlowID: "flow-a",
		Command: sharedtypes.SteerInstruction, Instruction: "for flow-a",
	})
	// Submit a broadcast instruction (empty FlowID — should be consumed)
	lc.SubmitSteering(&sharedtypes.SteeringEvent{
		RunID: "run_steer", FlowID: "",
		Command: sharedtypes.SteerInstruction, Instruction: "broadcast",
	})

	eng.drainSteeringEvents(ctx, "run_steer", "flow-a")

	if len(ctx.SteeringInstructions) != 2 {
		t.Fatalf("expected 2 steering instructions (targeted+broadcast), got %d: %v",
			len(ctx.SteeringInstructions), ctx.SteeringInstructions)
	}
	if ctx.SteeringInstructions[0] != "for flow-a" {
		t.Errorf("expected first instruction 'for flow-a', got %q", ctx.SteeringInstructions[0])
	}
	if ctx.SteeringInstructions[1] != "broadcast" {
		t.Errorf("expected second instruction 'broadcast', got %q", ctx.SteeringInstructions[1])
	}
}

func TestDrainSteeringEvents_RingBufferCap(t *testing.T) {
	eng := NewAgentEngine()
	lc := runtime.NewLifecycleController("run_ring")
	eng.SetLifecycleController(lc)

	ctx := &types.ExecutionContext{
		RunID:               "run_ring",
		FlowID:              "flow-x",
		SteeringInstructions: []string{},
	}

	// Fill ctx with 15 existing instructions (to exercise ring buffer)
	existing := make([]string, 15)
	for i := range existing {
		existing[i] = fmt.Sprintf("existing-%d", i)
	}
	ctx.SteeringInstructions = existing

	// Submit 10 instructions (fits channel buffer of 10)
	for i := range 10 {
		lc.SubmitSteering(&sharedtypes.SteeringEvent{
			RunID: "run_ring", FlowID: "flow-x",
			Command: sharedtypes.SteerInstruction, Instruction: fmt.Sprintf("new-%d", i),
		})
	}

	eng.drainSteeringEvents(ctx, "run_ring", "flow-x")

	// 15 existing + 10 new = 25, cap at 20 — oldest 5 should be dropped
	if len(ctx.SteeringInstructions) != 20 {
		t.Fatalf("expected ring buffer to cap at 20, got %d", len(ctx.SteeringInstructions))
	}
	if ctx.SteeringInstructions[0] != "existing-5" {
		t.Errorf("expected first instruction 'existing-5' (oldest 5 dropped), got %q", ctx.SteeringInstructions[0])
	}
	if ctx.SteeringInstructions[19] != "new-9" {
		t.Errorf("expected last instruction 'new-9', got %q", ctx.SteeringInstructions[19])
	}
}

func TestDrainSteeringEvents_NoLifecycle(t *testing.T) {
	eng := NewAgentEngine()
	ctx := &types.ExecutionContext{RunID: "test", SteeringInstructions: []string{}}

	// Should not panic with nil lifecycle
	eng.drainSteeringEvents(ctx, "test", "flow")
}
