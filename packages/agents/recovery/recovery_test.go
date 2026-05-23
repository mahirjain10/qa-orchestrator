package recovery

import (
	"errors"
	"testing"

	"qa-orchestrator/packages/agents/types"
)

func TestRecoveryDecideOnSuccess(t *testing.T) {
	r := NewRecovery(nil)

	stepResult := &types.StepResult{Success: true}
	decision := r.Decide(nil, stepResult, nil)

	if decision.Action != types.RecoveryActionSucceed {
		t.Errorf("Action = %s, want succeed", decision.Action)
	}
}

func TestRecoveryDecideOnNetworkError(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("connection refused")
	decision := r.Decide(err, nil, nil)

	if decision.Action != types.RecoveryActionRetry {
		t.Errorf("Action = %s, want retry", decision.Action)
	}

	if decision.Reason != "network error, retrying" {
		t.Errorf("Reason = %s", decision.Reason)
	}
}

func TestRecoveryDecideOnTimeoutError(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("operation timed out")
	decision := r.Decide(err, nil, nil)

	if decision.Action != types.RecoveryActionRetry {
		t.Errorf("Action = %s, want retry", decision.Action)
	}
}

func TestRecoveryDecideOnSelectorTimeout(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("wait_for failed: playwright: Timeout 30000ms exceeded waiting for selector .my-class")
	decision := r.Decide(err, nil, nil)

	if decision.Action != types.RecoveryActionReplan {
		t.Errorf("Action = %s, want replan for selector timeout", decision.Action)
	}
}

func TestRecoveryDecideOnSelectorTimeout_RealBrowserPatterns(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{"locator.click timeout", errors.New("locator.click: Timeout 30000ms exceeded")},
		{"locator.wait_for timeout", errors.New("locator.wait_for: Timeout 5000ms exceeded")},
		{"page.wait_for_selector timeout", errors.New("page.wait_for_selector: Timeout 10000ms exceeded")},
		{"expect locator timeout", errors.New("expect(locator).to_have_text: Timeout 30000ms exceeded")},
		{"old playwright pattern", errors.New("playwright: Timeout 30000ms exceeded waiting for selector")},
		{"locator timed out variant", errors.New("locator timed out waiting for element")},
		{"wait_for_selector timed out", errors.New("page.wait_for_selector: timed out 5000ms exceeded")},
		{"navigate locator timed out", errors.New("page.navigate: locator timed out")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRecovery(nil)
			decision := r.Decide(tt.err, nil, nil)
			if decision.Action != types.RecoveryActionReplan {
				t.Errorf("Action = %s, want replan for selector timeout (error=%q)", decision.Action, tt.err.Error())
			}
		})
	}
}

func TestRecoveryDecideOnGenericTimeout(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("operation timed out")
	decision := r.Decide(err, nil, nil)

	if decision.Action != types.RecoveryActionRetry {
		t.Errorf("Action = %s, want retry for generic timeout", decision.Action)
	}
}

func TestRecoveryDecideOnLocatorError(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("element not found locator error")
	decision := r.Decide(err, nil, nil)

	if decision.Action != types.RecoveryActionReplan {
		t.Errorf("Action = %s, want replan", decision.Action)
	}
}

func TestRecoveryDecideOnConfigError(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("unknown tool config error")
	decision := r.Decide(err, nil, nil)

	if decision.Action != types.RecoveryActionFail {
		t.Errorf("Action = %s, want fail", decision.Action)
	}
}

func TestRecoveryDecideOn404Observation(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("playwright: Timeout waiting for selector .missing")

	ctx := &types.ExecutionContext{
		Observations: []types.Observation{
			{
				LastStep: &types.StepResult{
					Tool:    "navigate",
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
			},
		},
	}

	decision := r.Decide(err, nil, ctx)

	if decision.Action != types.RecoveryActionRootNav {
		t.Errorf("Action = %s, want root_nav for 404 observation", decision.Action)
	}

	if decision.Reason != "invalid URL or 404 detected, engine will navigate to root domain" {
		t.Errorf("Reason = %q, want 404-related reason", decision.Reason)
	}

	// Steering instructions should NOT be modified — engine handles root nav directly
	if len(ctx.SteeringInstructions) != 0 {
		t.Errorf("expected no steering instructions injected, got %v", ctx.SteeringInstructions)
	}
}

func TestRecoveryDecideNo404OnCleanObservation(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("playwright: Timeout waiting for selector .missing")

	ctx := &types.ExecutionContext{
		Observations: []types.Observation{
			{
				LastStep: &types.StepResult{
					Tool:    "navigate",
					Success: true,
				},
				State: map[string]any{
					"source": "observe_ui",
					"data": map[string]any{
						"page_state":  "loaded",
						"interactive": []any{map[string]any{"tag": "button", "selector": "#login", "text": "Login"}},
					},
				},
			},
		},
	}

	decision := r.Decide(err, nil, ctx)

	if decision.Action != types.RecoveryActionReplan {
		t.Errorf("Action = %s, want replan (selector timeout without 404)", decision.Action)
	}
}

func TestRecoveryDecide404InLastOutput(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("element not found on page")

	ctx := &types.ExecutionContext{
		Observations: []types.Observation{
			{
				LastStep: &types.StepResult{
					Tool:    "navigate",
					Success: true,
				},
				State: map[string]any{
					"last_step_id":      "navigate",
					"last_step_success": true,
					"last_output": map[string]any{
						"page_state":  "loaded",
						"interactive": []any{},
						"warning":     "⚠️ WARNING: Page appears to be a 404 or error page.",
					},
				},
			},
		},
	}

	decision := r.Decide(err, nil, ctx)

	if decision.Action != types.RecoveryActionRootNav {
		t.Errorf("Action = %s, want root_nav for 404 in last_output", decision.Action)
	}

	if len(ctx.SteeringInstructions) != 0 {
		t.Errorf("expected no steering instructions injected, got %v", ctx.SteeringInstructions)
	}
}

func TestRecoveryShouldRetry(t *testing.T) {
	r := NewRecovery(&RecoveryPolicy{MaxRetries: 3})

	decision := &types.RecoveryDecision{Action: types.RecoveryActionRetry}

	if !r.ShouldRetry(decision, 0) {
		t.Error("ShouldRetry should be true at retry 0")
	}

	if !r.ShouldRetry(decision, 2) {
		t.Error("ShouldRetry should be true at retry 2")
	}

	if r.ShouldRetry(decision, 3) {
		t.Error("ShouldRetry should be false at retry 3")
	}
}

func TestRecoveryShouldEscalate(t *testing.T) {
	r := NewRecovery(&RecoveryPolicy{MaxRetries: 2, EscalateOnMax: true})

	decision := &types.RecoveryDecision{Action: types.RecoveryActionRetry}

	if r.ShouldEscalate(decision, 1) {
		t.Error("ShouldEscalate should be false below max retries")
	}

	if !r.ShouldEscalate(decision, 2) {
		t.Error("ShouldEscalate should be true at max retries")
	}
}

func TestRecoveryCreateRetryObservation(t *testing.T) {
	r := NewRecovery(nil)

	err := errors.New("test error")
	obs := r.CreateRetryObservation(err, 2)

	if obs.State["retry_count"] != 2 {
		t.Errorf("retry_count = %v, want 2", obs.State["retry_count"])
	}

	if obs.Error != err {
		t.Error("Error should match")
	}
}
