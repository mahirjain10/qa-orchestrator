package types

import "testing"

func TestPlanAddStepCachesParamsJSON(t *testing.T) {
	p := &Plan{}
	p.AddStep(PlanStep{
		StepID: "s1",
		Tool:   "click",
		Params: map[string]any{"selector": "#submit"},
	})

	if len(p.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(p.Steps))
	}
	if p.Steps[0].paramsJSON == "" {
		t.Fatal("expected paramsJSON to be cached on AddStep")
	}
}

func TestPlanGetHistoryBackfillsParamsJSON(t *testing.T) {
	p := &Plan{
		CurrentIdx: 1,
		Steps: []PlanStep{
			{StepID: "s1", Tool: "click", Params: map[string]any{"selector": "#submit"}},
		},
	}

	history := p.GetHistory()
	if history == "" {
		t.Fatal("expected non-empty history")
	}
	if p.Steps[0].paramsJSON == "" {
		t.Fatal("expected paramsJSON to be cached during history generation")
	}
}

func TestPlanGetHistoryCachesAndInvalidates(t *testing.T) {
	p := &Plan{
		CurrentIdx: 1,
		Steps: []PlanStep{
			{StepID: "s1", Tool: "click", Reason: "first"},
		},
	}

	first := p.GetHistory()
	if first == "" {
		t.Fatal("expected history")
	}

	p.Steps[0].Reason = "changed"
	second := p.GetHistory()
	if second != first {
		t.Fatal("expected cached history to remain unchanged before invalidation")
	}

	p.InvalidateHistoryCache()
	third := p.GetHistory()
	if third == first {
		t.Fatal("expected invalidated history to rebuild with updated content")
	}
}
