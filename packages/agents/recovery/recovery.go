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

		// Check for 404 observation FIRST — if the page is a 404, all subsequent
		// interaction failures are caused by the missing page, not by real element issues.
		// Delegate root-navigation to the engine (RecoveryActionRootNav) instead of
		// burdening the LLM with steering instructions.
		if ctx != nil && has404Warning(ctx) {
			decision.Action = types.RecoveryActionRootNav
			decision.Reason = "invalid URL or 404 detected, engine will navigate to root domain"
			return decision
		}

		if isSelectorTimeout(errStr) {
			decision.Action = types.RecoveryActionReplan
			decision.Reason = "selector timeout, replanning with fresh observation"
			return decision
		}

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

func isSelectorTimeout(err string) bool {
	lower := strings.ToLower(err)
	if !strings.Contains(lower, "timeout") && !strings.Contains(lower, "timed out") {
		return false
	}
	// Playwright timeout error patterns:
	//   "locator.click: Timeout 30000ms exceeded"    — has "locator" + "timeout"
	//   "locator.wait_for: Timeout 30000ms exceeded"  — has "locator" + "timeout"
	//   "page.wait_for_selector: Timeout 30000ms"     — has "selector" + "timeout"
	//   "expect(locator).to_have_text: Timeout ..."   — has "locator" + "timeout"
	//   playwright: ... Timeout ... selector           — rare but possible
	return strings.Contains(lower, "locator") || strings.Contains(lower, "selector")
}

func isNetworkError(err string) bool {
	err = strings.ToLower(err)
	networkErrors := []string{"connection refused", "dns", "econnreset", "enotfound"}
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
	locatorErrors := []string{"locator", "element not found", "no such element", "not visible", "not attached", "not found on page", "does not exist in current dom"}
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
	configErrors := []string{"unknown tool", "invalid configuration", "missing required"}
	for _, c := range configErrors {
		if strings.Contains(err, c) {
			return true
		}
	}
	return false
}

func (r *Recovery) ShouldRetry(decision *types.RecoveryDecision, retryCount int) bool {
	if (decision.Action == types.RecoveryActionRetry || decision.Action == types.RecoveryActionReplan) && retryCount < r.policy.MaxRetries {
		return true
	}
	return false
}

func (r *Recovery) ShouldEscalate(decision *types.RecoveryDecision, retryCount int) bool {
	if (decision.Action == types.RecoveryActionRetry || decision.Action == types.RecoveryActionReplan) && retryCount >= r.policy.MaxRetries && r.policy.EscalateOnMax {
		return true
	}
	return false
}

func (r *Recovery) Has404Warning(ctx *types.ExecutionContext) bool {
	return has404Warning(ctx)
}

func has404Warning(ctx *types.ExecutionContext) bool {
	for i := len(ctx.Observations) - 1; i >= 0; i-- {
		obs := ctx.Observations[i]
		if obs.State == nil {
			continue
		}
		// Check nested under "data" key (set by injectObserveStep / autoObserve)
		if data, ok := obs.State["data"].(map[string]any); ok {
			if warning, ok := data["warning"].(string); ok && warning != "" {
				return true
			}
		}
		// Also check direct state map for warning
		if warning, ok := obs.State["warning"].(string); ok && warning != "" {
			return true
		}
		// Check "last_output" which may contain serialized observe_ui result
		if output, ok := obs.State["last_output"].(map[string]any); ok {
			if warning, ok := output["warning"].(string); ok && warning != "" {
				return true
			}
		}
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
