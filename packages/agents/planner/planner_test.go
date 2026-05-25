package planner

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/llm"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

func TestPlannerCreatePlan(t *testing.T) {
	p := NewPlanner()
	steps := []types.Step{
		{ID: "step1", Tool: "log", Params: map[string]any{"message": "hello"}},
		{ID: "step2", Tool: "delay", Params: map[string]any{"ms": 100}},
	}

	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "test goal",
		Steps:  steps,
	}

	plan, err := p.CreatePlan(ctx)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan == nil {
		t.Fatal("Plan should not be nil")
	}

	if plan.FlowID != "flow_test" {
		t.Errorf("FlowID = %s, want flow_test", plan.FlowID)
	}

	if len(plan.Steps) != 2 {
		t.Errorf("len(Steps) = %d, want 2", len(plan.Steps))
	}

	if plan.CurrentIdx != 0 {
		t.Errorf("CurrentIdx = %d, want 0", plan.CurrentIdx)
	}
}

func TestPlannerGetNextStep(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log", Skip: false},
			{StepIndex: 1, StepID: "step2", Tool: "delay", Skip: false},
		},
	}

	step := p.GetNextStep(plan)
	if step == nil {
		t.Fatal("GetNextStep returned nil")
	}

	if step.StepID != "step1" {
		t.Errorf("StepID = %s, want step1", step.StepID)
	}

	p.Advance(plan)
	step = p.GetNextStep(plan)
	if step.StepID != "step2" {
		t.Errorf("StepID = %s, want step2", step.StepID)
	}
}

func TestPlannerSkipStep(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log", Skip: false},
			{StepIndex: 1, StepID: "step2", Tool: "delay", Skip: false},
		},
	}

	p.UpdatePlan(plan, 0, true, "intentional skip")
	step := p.GetNextStep(plan)

	if step.StepID != "step2" {
		t.Errorf("StepID = %s, want step2 (step1 should be skipped)", step.StepID)
	}
}

func TestPlannerShouldStop(t *testing.T) {
	p := NewPlanner()

	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log", Skip: false},
		},
	}

	if p.ShouldStop(plan) {
		t.Error("ShouldStop should be false with pending steps")
	}

	plan.CurrentIdx = 1
	if !p.ShouldStop(plan) {
		t.Error("ShouldStop should be true when all steps done")
	}
}

func TestPlannerShouldStop_AutonomousWithConsumedSteps(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		IsAutonomous: true,
		CurrentIdx:   1,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "s1", Tool: "echo"},
		},
	}
	if p.ShouldStop(plan) {
		t.Fatal("autonomous plan should not stop just because generated steps are consumed")
	}
}

func TestPlannerGetProgress(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 2,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log"},
			{StepIndex: 1, StepID: "step2", Tool: "delay"},
			{StepIndex: 2, StepID: "step3", Tool: "echo"},
		},
	}

	completed, total := p.GetProgress(plan)
	if completed != 2 {
		t.Errorf("completed = %d, want 2", completed)
	}
	if total != 3 {
		t.Errorf("total = %d, want 3", total)
	}
}

func TestPlannerCreatePlan_AutonomousMode(t *testing.T) {
	p := NewAutonomousPlanner(&mockLLMClient{}, nil)
	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "test autonomous goal",
		Mode:   sharedtypes.FlowModeAutonomous,
	}

	plan, err := p.CreatePlan(ctx)
	if err != nil {
		t.Fatalf("CreatePlan failed: %v", err)
	}

	if plan == nil {
		t.Fatal("Plan should not be nil")
	}

	if !plan.IsAutonomous {
		t.Error("Plan should be marked as autonomous")
	}

	if len(plan.Steps) != 0 {
		t.Errorf("Autonomous plan should start with empty steps, got %d", len(plan.Steps))
	}
}

func TestPlannerCreatePlan_GuidedEmptyStepsReturnsError(t *testing.T) {
	p := NewPlanner()
	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "test guided goal",
		Mode:   sharedtypes.FlowModeGuided,
		Steps:  []types.Step{},
	}

	_, err := p.CreatePlan(ctx)
	if err == nil {
		t.Fatal("expected error for guided mode with empty steps")
	}
	if !strings.Contains(err.Error(), "guided mode requires at least one step") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPlannerCreatePlan_AutonomousWithoutLLMReturnsError(t *testing.T) {
	p := NewPlanner()
	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "test autonomous goal",
		Mode:   sharedtypes.FlowModeAutonomous,
	}

	_, err := p.CreatePlan(ctx)
	if err == nil {
		t.Fatal("expected error for autonomous mode without LLM client")
	}
	if !strings.Contains(err.Error(), "LLM client") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPlannerIsAutonomousMode(t *testing.T) {
	p := NewPlanner()

	guidedCtx := &types.ExecutionContext{
		Mode: sharedtypes.FlowModeGuided,
	}
	if p.IsAutonomousMode(guidedCtx) {
		t.Error("Should not be autonomous for guided mode")
	}

	autonomousCtx := &types.ExecutionContext{
		Mode: sharedtypes.FlowModeAutonomous,
	}
	if !p.IsAutonomousMode(autonomousCtx) {
		t.Error("Should be autonomous for autonomous mode")
	}
}

func TestPlanAddStep(t *testing.T) {
	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "test goal",
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, 0),
	}

	plan.AddStep(types.PlanStep{
		StepID: "step1",
		Tool:   "navigate",
		Params: map[string]any{"url": "https://example.com"},
	})

	if len(plan.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(plan.Steps))
	}

	if plan.Steps[0].StepIndex != 0 {
		t.Errorf("StepIndex should be 0, got %d", plan.Steps[0].StepIndex)
	}

	plan.AddStep(types.PlanStep{
		StepID: "step2",
		Tool:   "click",
	})

	if len(plan.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(plan.Steps))
	}

	if plan.Steps[1].StepIndex != 1 {
		t.Errorf("StepIndex should be 1, got %d", plan.Steps[1].StepIndex)
	}
}

func TestPlanGetHistory(t *testing.T) {
	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "test goal",
		CurrentIdx: 2,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "navigate", Params: map[string]any{"url": "https://example.com/login"}, Reason: "Go to login page"},
			{StepIndex: 1, StepID: "step2", Tool: "type", Params: map[string]any{"selector": "#username", "value": "alice"}, Reason: "Enter username"},
		},
	}

	history := plan.GetHistory()
	if history == "" {
		t.Error("History should not be empty")
	}
	if !strings.Contains(history, "params=") {
		t.Errorf("Expected history to include params, got: %s", history)
	}

	emptyPlan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, 0),
	}

	emptyHistory := emptyPlan.GetHistory()
	if emptyHistory == "" {
		t.Error("Empty plan should still return history string")
	}
}

type mockLLMClient struct {
	response string
	err      error
}

func (m *mockLLMClient) Generate(ctx context.Context, prompt string) (string, error) {
	return m.response, m.err
}

func (m *mockLLMClient) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	return m.response, m.err
}

func TestPlannerGenerateNextStep(t *testing.T) {
	mockClient := &mockLLMClient{
		response: `[{"tool": "navigate", "params": {"url": "https://example.com"}, "reason": "Go to login page"}]`,
	}

	tools := []llm.ToolInfo{
		{
			Name:        "navigate",
			Description: "Navigate to a URL",
			Parameters: map[string]llm.ParameterInfo{
				"url": {Type: "string", Description: "The URL to navigate to", Required: true},
			},
		},
	}

	p := NewAutonomousPlanner(mockClient, tools)

	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "Test login flow",
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, 0),
	}

	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "Test login flow",
		Plan:   plan,
	}

	step, _, err := p.GenerateNextStep(context.Background(), ctx)
	if err != nil {
		t.Fatalf("GenerateNextStep failed: %v", err)
	}

	if step == nil {
		t.Fatal("Step should not be nil")
	}

	if step.Tool != "navigate" {
		t.Errorf("Expected tool 'navigate', got %s", step.Tool)
	}

	url, ok := step.Params["url"].(string)
	if !ok || url != "https://example.com" {
		t.Errorf("Expected url 'https://example.com', got %v", step.Params["url"])
	}

	if step.Reason != "Go to login page" {
		t.Errorf("Expected reason 'Go to login page', got %s", step.Reason)
	}
}

func TestPlannerGenerateNextStep_NoLLMClient(t *testing.T) {
	p := NewPlanner()

	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "Test goal",
		CurrentIdx: 0,
	}

	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "Test goal",
		Plan:   plan,
	}

	_, _, err := p.GenerateNextStep(context.Background(), ctx)
	if err == nil {
		t.Error("Expected error when LLM client is not configured")
	}
}

func TestPlannerGenerateNextStep_InvalidResponse(t *testing.T) {
	mockClient := &mockLLMClient{
		response: "invalid json response",
	}

	tools := []llm.ToolInfo{}
	p := NewAutonomousPlanner(mockClient, tools)

	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "Test goal",
		CurrentIdx: 0,
	}

	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "Test goal",
		Plan:   plan,
	}

	_, _, err := p.GenerateNextStep(context.Background(), ctx)
	if err == nil {
		t.Error("Expected error for invalid LLM response")
	}
}

func TestPlannerAddStepToPlan(t *testing.T) {
	p := NewPlanner()

	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "test goal",
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, 0),
	}

	step := &types.PlanStep{
		StepID: "new-step",
		Tool:   "click",
		Params: map[string]any{"selector": "#btn"},
	}

	p.AddStepToPlan(plan, step)

	if len(plan.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(plan.Steps))
	}

	if plan.Steps[0].StepID != "new-step" {
		t.Errorf("Expected step ID 'new-step', got %s", plan.Steps[0].StepID)
	}
}

func TestFormatObserveUIObservation_FormatsInteractiveElements(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state": "loaded",
				"interactive": []any{
					map[string]any{"tag": "input", "selector": "#username", "text": ""},
					map[string]any{"tag": "button", "selector": "#submit", "text": "Login"},
				},
			},
		},
	}

	result := formatObserveUIObservation(obs)

	if !strings.Contains(result, "Page observation after last step") {
		t.Errorf("expected header, got: %s", result)
	}
	if !strings.Contains(result, "Page state: loaded") {
		t.Errorf("expected page state, got: %s", result)
	}
	if !strings.Contains(result, "Interactive elements found (2 total") {
		t.Errorf("expected element count, got: %s", result)
	}
	if !strings.Contains(result, `selector="#username"`) {
		t.Errorf("expected username selector, got: %s", result)
	}
	if !strings.Contains(result, `selector="#submit"`) {
		t.Errorf("expected submit selector, got: %s", result)
	}
	if !strings.Contains(result, "Do not invent selectors") {
		t.Errorf("expected warning about inventing selectors, got: %s", result)
	}
}

func TestFormatObserveUIObservation_FromStringJSON(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data":   `{"page_state":"loaded","interactive":[{"tag":"input","selector":"#email","text":""}]}`,
		},
	}

	result := formatObserveUIObservation(obs)

	if !strings.Contains(result, "Page state: loaded") {
		t.Errorf("expected page state from string JSON, got: %s", result)
	}
	if !strings.Contains(result, `selector="#email"`) {
		t.Errorf("expected email selector from string JSON, got: %s", result)
	}
}

func TestFormatObserveUIObservation_EmptyPage(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state":  "empty",
				"interactive": []any{},
			},
		},
	}

	result := formatObserveUIObservation(obs)

	if !strings.Contains(result, "Page state: empty") {
		t.Errorf("expected empty page state, got: %s", result)
	}
	if !strings.Contains(result, "Interactive elements found (0 total") {
		t.Errorf("expected zero element count, got: %s", result)
	}
}

func TestFormatObserveUIObservation_With404Warning(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state":  "loaded",
				"interactive": []any{},
				"warning":     "⚠️ WARNING: Page appears to be a 404 or error page.",
			},
		},
	}

	result := formatObserveUIObservation(obs)

	if !strings.Contains(result, "WARNING: Page appears to be a 404 or error page") {
		t.Errorf("expected 404 warning in output, got: %s", result)
	}
}

func TestFormatObserveUIObservation_InvalidJSON(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data":   `not valid json`,
		},
	}

	result := formatObserveUIObservation(obs)

	if !strings.Contains(result, "Raw data: not valid json") {
		t.Errorf("expected raw data fallback, got: %s", result)
	}
}

func TestScanForRecentFailure_NoFailure(t *testing.T) {
	result := scanForRecentFailure(nil)
	if result != "" {
		t.Errorf("expected empty for nil, got %q", result)
	}

	obs := []types.Observation{
		{LastStep: &types.StepResult{Tool: "navigate", Success: true}},
	}
	result = scanForRecentFailure(obs)
	if result != "" {
		t.Errorf("expected empty for success obs, got %q", result)
	}
}

func TestScanForRecentFailure_ObservationError(t *testing.T) {
	obs := []types.Observation{
		{
			Error:    fmt.Errorf("timeout: page did not load"),
			LastStep: &types.StepResult{Tool: "navigate", Success: false},
		},
	}
	result := scanForRecentFailure(obs)
	if !strings.Contains(result, "RECENT FAILURE") {
		t.Errorf("expected failure prefix, got %q", result)
	}
	if !strings.Contains(result, "timeout") {
		t.Errorf("expected error message in result, got %q", result)
	}
	if !strings.Contains(result, "navigate") {
		t.Errorf("expected tool name in result, got %q", result)
	}
}

func TestScanForRecentFailure_ObservationErrorNoLastStep(t *testing.T) {
	obs := []types.Observation{
		{Error: fmt.Errorf("browser crashed")},
	}
	result := scanForRecentFailure(obs)
	if !strings.Contains(result, "RECENT FAILURE") {
		t.Errorf("expected failure prefix, got %q", result)
	}
	if !strings.Contains(result, "tool=?") {
		t.Errorf("expected 'tool=?' when LastStep is nil, got %q", result)
	}
}

func TestScanForRecentFailure_StepFailure(t *testing.T) {
	obs := []types.Observation{
		{
			LastStep: &types.StepResult{
				Tool:    "click",
				Success: false,
				Error:   fmt.Errorf("element #submit not found"),
			},
		},
	}
	result := scanForRecentFailure(obs)
	if !strings.Contains(result, "RECENT FAILURE") {
		t.Errorf("expected failure prefix, got %q", result)
	}
	if !strings.Contains(result, "element #submit not found") {
		t.Errorf("expected error text, got %q", result)
	}
	if !strings.Contains(result, "click") {
		t.Errorf("expected tool click, got %q", result)
	}
}

func TestScanForRecentFailure_StepFailureNoError(t *testing.T) {
	obs := []types.Observation{
		{
			LastStep: &types.StepResult{
				Tool:    "type_text",
				Success: false,
			},
		},
	}
	result := scanForRecentFailure(obs)
	if !strings.Contains(result, "unknown error") {
		t.Errorf("expected 'unknown error' fallback, got %q", result)
	}
}

func TestScanForRecentFailure_FindsMostRecent(t *testing.T) {
	obs := []types.Observation{
		{LastStep: &types.StepResult{Tool: "navigate", Success: true}},
		{Error: fmt.Errorf("old failure")},
		{LastStep: &types.StepResult{Tool: "click", Success: true}},
	}
	// Most recent (index 2) is success, should find index 1's failure
	result := scanForRecentFailure(obs)
	if !strings.Contains(result, "old failure") {
		t.Errorf("expected oldest failure, got %q", result)
	}
}

func TestScanForRecentFailure_ReturnsMostRecentFailure(t *testing.T) {
	obs := []types.Observation{
		{LastStep: &types.StepResult{Tool: "navigate", Success: false, Error: fmt.Errorf("first error")}},
		{LastStep: &types.StepResult{Tool: "click", Success: false, Error: fmt.Errorf("second error")}},
	}
	result := scanForRecentFailure(obs)
	if !strings.Contains(result, "second error") {
		t.Errorf("expected most recent error, got %q", result)
	}
}

func TestPlannerGenerateNextStep_UsesObserveUIObservation(t *testing.T) {
	mockClient := &mockLLMClient{
		response: `[{"tool": "click", "params": {"selector": "#submit"}, "reason": "Click the login button"}]`,
	}

	tools := []llm.ToolInfo{
		{
			Name:        "click",
			Description: "Click an element",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector", Required: true},
			},
		},
	}

	p := NewAutonomousPlanner(mockClient, tools)

	plan := &types.Plan{
		FlowID:     "test",
		Goal:       "Test login",
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, 0),
	}

	ctx := &types.ExecutionContext{
		RunID:  "run_test",
		FlowID: "flow_test",
		Goal:   "Test login",
		Plan:   plan,
		Observations: []types.Observation{
			{
				LastStep: &types.StepResult{
					StepID:  "observe_ui",
					Tool:    "observe_ui",
					Success: true,
				},
				State: map[string]any{
					"source": "observe_ui",
					"data": map[string]any{
						"page_state": "loaded",
						"interactive": []any{
							map[string]any{"tag": "button", "selector": "#submit", "text": "Login"},
						},
					},
				},
			},
		},
	}

	step, _, err := p.GenerateNextStep(context.Background(), ctx)
	if err != nil {
		t.Fatalf("GenerateNextStep failed: %v", err)
	}

	if step.Tool != "click" {
		t.Errorf("Expected tool 'click', got %s", step.Tool)
	}

	selector, ok := step.Params["selector"].(string)
	if !ok || selector != "#submit" {
		t.Errorf("Expected selector '#submit', got %v", step.Params["selector"])
	}
}

func TestPlannerGetNextStep_ReturnsCopyNotDanglingPointer(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log", Skip: false},
		},
	}

	step := p.GetNextStep(plan)
	if step == nil {
		t.Fatal("GetNextStep returned nil")
	}

	// AddStep may reallocate the backing array — pointer from GetNextStep must still be valid
	plan.AddStep(types.PlanStep{StepID: "step2", Tool: "delay"})
	plan.AddStep(types.PlanStep{StepID: "step3", Tool: "echo"})

	// The original step pointer should still be valid (not dangling)
	if step.StepID != "step1" {
		t.Errorf("StepID after reallocation = %s, want step1", step.StepID)
	}
	if step.Tool != "log" {
		t.Errorf("Tool after reallocation = %s, want log", step.Tool)
	}
}

func TestPlannerConcurrentAddStepAndGetNextStep(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, 0),
	}

	// Concurrently add steps and read current state
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			p.AddStepToPlan(plan, &types.PlanStep{
				StepID: fmt.Sprintf("step-%d", i),
				Tool:   "echo",
			})
		}
		close(done)
	}()

	for i := 0; i < 50; i++ {
		p.GetNextStep(plan)
		p.Advance(plan)
	}
	<-done
}

func TestPlannerConcurrentGetProgress(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 3,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "s1", Tool: "log"},
			{StepIndex: 1, StepID: "s2", Tool: "delay"},
			{StepIndex: 2, StepID: "s3", Tool: "echo"},
			{StepIndex: 3, StepID: "s4", Tool: "log"},
		},
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			p.Advance(plan)
		}
		close(done)
	}()

	for i := 0; i < 50; i++ {
		completed, total := p.GetProgress(plan)
		if total != 4 {
			t.Errorf("total should be 4, got %d", total)
		}
		_ = completed
	}
	<-done
}

func TestPlannerShouldStop_ConcurrentWithAdvance(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "s1", Tool: "log", Skip: false},
			{StepIndex: 1, StepID: "s2", Tool: "echo", Skip: true},
			{StepIndex: 2, StepID: "s3", Tool: "delay", Skip: false},
		},
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 20; i++ {
			p.Advance(plan)
		}
		close(done)
	}()

	for i := 0; i < 20; i++ {
		_ = p.ShouldStop(plan)
	}
	<-done
}

func TestPlannerUpdatePlan_LockPreventsRace(t *testing.T) {
	p := NewPlanner()
	plan := &types.Plan{
		FlowID:     "test",
		CurrentIdx: 0,
		Steps: []types.PlanStep{
			{StepIndex: 0, StepID: "step1", Tool: "log", Skip: false},
			{StepIndex: 1, StepID: "step2", Tool: "delay", Skip: false},
		},
	}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 50; i++ {
			p.UpdatePlan(plan, 0, true, "skipped")
			p.UpdatePlan(plan, 0, false, "")
		}
		close(done)
	}()

	for i := 0; i < 50; i++ {
		step := p.GetNextStep(plan)
		if step != nil {
			_ = step.StepID
		}
	}
	<-done
}

func TestSanitizeDOM_EscapesSpecialChars(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"null bytes", "abc\x00def", "abcdef"},
		{"code fences", "```code```", "code"},
		{"backslash quoted", `a"b`, `a&quot;b`},
		{"backslash first, then quote", `\"`, `\\&quot;`},
		{"newline removed", "line1\nline2", "line1line2"},
		{"angle brackets", "<script>", "&lt;script&gt;"},
		{"open bracket", "[test]", "&#91;test&#93;"},
		{"close bracket", "]", "&#93;"},
		{"all special chars mixed", "<a \"b\"\n[c]\\x>", `&lt;a &quot;b&quot;&#91;c&#93;\\x&gt;`},
		{"backslash before quote", "\\\"", "\\\\&quot;"},
		{"only backslash", "a\\b", "a\\\\b"},
		{"ampersand escaped", "a&b", "a&amp;b"},
		{"ampersand before angle", "<a&b>", "&lt;a&amp;b&gt;"},
		{"apostrophe escaped", "it's", "it&#39;s"},
		{"apostrophe in attribute", "a'b", "a&#39;b"},
		{"empty string", "", ""},
		{"no special chars", "hello world", "hello world"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDOM(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeDOM(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestFormatObserveUIObservation_TruncatesLargeRawData(t *testing.T) {
	largeData := strings.Repeat("ABCDE", 500) // 2500 chars — exceeds 2000 limit
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data":   largeData,
		},
	}

	result := formatObserveUIObservation(obs)

	// sanitizeDOM escapes [ and ], so expect the HTML-entity version
	if !strings.Contains(result, "&#91;truncated&#93;") {
		t.Errorf("expected truncation marker in output, got: %s", result)
	}
	if len(result) >= len(largeData) {
		t.Errorf("expected truncated result to be shorter than raw data, len=%d >= raw=%d", len(result), len(largeData))
	}
}

func TestFormatObserveUIObservation_SmallRawDataNotTruncated(t *testing.T) {
	smallData := `{"page_state":"loaded","interactive":[]}`
	obs := types.Observation{
		LastStep: &types.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Success: true,
		},
		State: map[string]any{
			"source": "observe_ui",
			"data":   smallData,
		},
	}

	result := formatObserveUIObservation(obs)

	if strings.Contains(result, "[truncated]") {
		t.Errorf("unexpected truncation for data under limit: %s", result)
	}
}

func TestRenderAttr_OmitsEmptyValue(t *testing.T) {
	line := "   1. <input>"
	elemMap := map[string]any{"tag": "input", "selector": "#foo", "placeholder": ""}
	used := map[string]bool{"tag": true, "selector": true}
	renderAttr(&line, elemMap, "placeholder", &used)
	if line != "   1. <input>" {
		t.Errorf("expected no change for empty value, got: %s", line)
	}
}

func TestRenderAttr_OmitsFalseBool(t *testing.T) {
	line := "   1. <input>"
	elemMap := map[string]any{"tag": "input", "selector": "#foo", "disabled": false}
	used := map[string]bool{"tag": true, "selector": true}
	renderAttr(&line, elemMap, "disabled", &used)
	if line != "   1. <input>" {
		t.Errorf("expected no change for false bool, got: %s", line)
	}
}

func TestRenderAttr_IncludesTrueBool(t *testing.T) {
	line := "   1. <input>"
	elemMap := map[string]any{"tag": "input", "selector": "#foo", "checked": true}
	used := map[string]bool{"tag": true, "selector": true}
	renderAttr(&line, elemMap, "checked", &used)
	if !strings.Contains(line, `checked="true"`) {
		t.Errorf("expected checked=true in output, got: %s", line)
	}
}

func TestRenderAttr_IncludesNonEmptyString(t *testing.T) {
	line := "   1. <input>"
	elemMap := map[string]any{"tag": "input", "selector": "#foo", "value": "standard_user"}
	used := map[string]bool{"tag": true, "selector": true}
	renderAttr(&line, elemMap, "value", &used)
	if !strings.Contains(line, `value="standard_user"`) {
		t.Errorf("expected value=standard_user in output, got: %s", line)
	}
}

func TestFormatObserveUIObservation_ValueRendered(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{StepID: "observe_ui", Tool: "observe_ui", Success: true},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state": "loaded",
				"interactive": []any{
					map[string]any{"tag": "input", "selector": "#user-name", "value": "standard_user", "text": ""},
				},
			},
		},
	}
	result := formatObserveUIObservation(obs)
	if !strings.Contains(result, `value="standard_user"`) {
		t.Errorf("expected runtime value in output, got: %s", result)
	}
}

func TestFormatObserveUIObservation_PasswordRedacted(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{StepID: "observe_ui", Tool: "observe_ui", Success: true},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state": "loaded",
				"interactive": []any{
					map[string]any{"tag": "input", "selector": "#password", "value": "********", "type": "password", "text": ""},
				},
			},
		},
	}
	result := formatObserveUIObservation(obs)
	if !strings.Contains(result, `value="********"`) {
		t.Errorf("expected redacted password value, got: %s", result)
	}
}

func TestFormatObserveUIObservation_DynamicAttrsAppear(t *testing.T) {
	obs := types.Observation{
		LastStep: &types.StepResult{StepID: "observe_ui", Tool: "observe_ui", Success: true},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state": "loaded",
				"interactive": []any{
					map[string]any{
						"tag":          "button",
						"selector":     "#login-btn",
						"data-testid":  "login-btn",
						"aria-expanded": "true",
						"text":         "Login",
					},
				},
			},
		},
	}
	result := formatObserveUIObservation(obs)
	if !strings.Contains(result, `data-testid="login-btn"`) {
		t.Errorf("expected dynamic data-testid attr, got: %s", result)
	}
	if !strings.Contains(result, `aria-expanded="true"`) {
		t.Errorf("expected dynamic aria-expanded attr, got: %s", result)
	}
}

func TestFormatObserveUIObservation_OrderingStable(t *testing.T) {
	input := map[string]any{
		"tag":   "input",
		"id":    "user-name",
		"name":  "username",
		"type":  "text",
		"class": "input_error form_input",
		"placeholder": "Username",
		"value": "standard_user",
		"checked": "true",
		"data-test": "username",
		"selector": "#user-name",
	}
	obs := types.Observation{
		LastStep: &types.StepResult{StepID: "observe_ui", Tool: "observe_ui", Success: true},
		State: map[string]any{
			"source": "observe_ui",
			"data": map[string]any{
				"page_state":  "loaded",
				"interactive": []any{input},
			},
		},
	}

	// Run twice to verify deterministic ordering
	r1 := formatObserveUIObservation(obs)
	r2 := formatObserveUIObservation(obs)
	if r1 != r2 {
		t.Errorf("non-deterministic output between runs:\nrun1: %s\nrun2: %s", r1, r2)
	}

	// Priority fields appear in defined order (id, name, type, placeholder, value, checked)
	result := r1
	idIdx := strings.Index(result, `id="user-name"`)
	nameIdx := strings.Index(result, `name="username"`)
	typeIdx := strings.Index(result, `type="text"`)
	valueIdx := strings.Index(result, `value="standard_user"`)
	checkedIdx := strings.Index(result, `checked="true"`)

	if idIdx > nameIdx || idIdx > typeIdx {
		t.Errorf("priority order violated: id should come before name/type")
	}
	if valueIdx > checkedIdx {
		t.Errorf("priority order violated: value should come before checked")
	}

	// data-test (non-priority) should come after priority fields
	dataTestIdx := strings.Index(result, `data-test="username"`)
	if dataTestIdx < checkedIdx {
		t.Errorf("non-priority field data-test should appear after priority fields")
	}

	// selector should be last
	selectorIdx := strings.Index(result, `selector="#user-name"`)
	if selectorIdx < dataTestIdx {
		t.Errorf("selector should appear last, but appears before data-test")
	}
}
