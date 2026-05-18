package planner

import (
	"testing"

	"qa-orchestrator/packages/agents/types"
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
