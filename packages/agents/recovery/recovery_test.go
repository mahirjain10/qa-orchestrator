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
