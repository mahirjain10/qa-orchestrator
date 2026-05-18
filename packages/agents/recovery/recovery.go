package recovery

import (
	"strings"

	"qa-orchestrator/packages/agents/types"
)

type RecoveryPolicy struct {
	MaxRetries    int
	RetryDelayMs  int64
	EscalateOnMax bool
}

func DefaultPolicy() *RecoveryPolicy {
	return &RecoveryPolicy{
		MaxRetries:    3,
		RetryDelayMs:  1000,
		EscalateOnMax: true,
	}
}

type Recovery struct {
	policy *RecoveryPolicy
}

func NewRecovery(policy *RecoveryPolicy) *Recovery {
	if policy == nil {
		policy = DefaultPolicy()
	}
	return &Recovery{
		policy: policy,
	}
}

func (r *Recovery) Decide(err error, stepResult *types.StepResult, ctx *types.ExecutionContext) *types.RecoveryDecision {
	decision := &types.RecoveryDecision{
		MaxRetries: r.policy.MaxRetries,
	}

	if err != nil {
		errStr := err.Error()

		if isNetworkError(errStr) {
			decision.Action = types.RecoveryActionRetry
			decision.Reason = "network error, retrying"
			return decision
		}

		if isTimeoutError(errStr) {
			decision.Action = types.RecoveryActionRetry
			decision.Reason = "timeout error, retrying"
			return decision
		}

		if isLocatorError(errStr) {
			decision.Action = types.RecoveryActionReplan
			decision.Reason = "locator error, replanning"
			return decision
		}

		if isAssertionError(errStr) {
			decision.Action = types.RecoveryActionFail
			decision.Reason = "assertion failed permanently"
			return decision
		}

		if isConfigError(errStr) {
			decision.Action = types.RecoveryActionFail
			decision.Reason = "configuration error"
			return decision
		}

		decision.Action = types.RecoveryActionRetry
		decision.Reason = "unknown error, retrying"
		return decision
	}

	if stepResult == nil || stepResult.Success {
		decision.Action = types.RecoveryActionSucceed
		decision.Reason = "step completed successfully"
		return decision
	}

	decision.Action = types.RecoveryActionRetry
	decision.Reason = "step failed without error, retrying"
	return decision
}

func isNetworkError(err string) bool {
	err = strings.ToLower(err)
	networkErrors := []string{"connection refused", "timeout", "network", "dns", "econnreset", "enotfound"}
	for _, ne := range networkErrors {
		if strings.Contains(err, ne) {
			return true
		}
	}
	return false
}

func isTimeoutError(err string) bool {
	err = strings.ToLower(err)
	timeouts := []string{"timeout", "timed out", "deadline exceeded", "i/o timeout"}
	for _, t := range timeouts {
		if strings.Contains(err, t) {
			return true
		}
	}
	return false
}

func isLocatorError(err string) bool {
	err = strings.ToLower(err)
	locatorErrors := []string{"locator", "element not found", "no such element", "not visible", "not attached"}
	for _, l := range locatorErrors {
		if strings.Contains(err, l) {
			return true
		}
	}
	return false
}

func isAssertionError(err string) bool {
	err = strings.ToLower(err)
	return strings.Contains(err, "assertion failed") || strings.Contains(err, "assertion")
}

func isConfigError(err string) bool {
	err = strings.ToLower(err)
	configErrors := []string{"config", "invalid", "not found", "unknown tool"}
	for _, c := range configErrors {
		if strings.Contains(err, c) {
			return true
		}
	}
	return false
}

func (r *Recovery) ShouldRetry(decision *types.RecoveryDecision, retryCount int) bool {
	if decision.Action == types.RecoveryActionRetry && retryCount < r.policy.MaxRetries {
		return true
	}
	return false
}

func (r *Recovery) ShouldEscalate(decision *types.RecoveryDecision, retryCount int) bool {
	if decision.Action == types.RecoveryActionRetry && retryCount >= r.policy.MaxRetries && r.policy.EscalateOnMax {
		return true
	}
	return false
}

func (r *Recovery) CreateRetryObservation(err error, retryCount int) *types.Observation {
	return &types.Observation{
		State: map[string]any{
			"retry_count": retryCount,
			"error":       err.Error(),
			"action":      "retry",
		},
		Error: err,
	}
}
