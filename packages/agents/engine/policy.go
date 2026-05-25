package engine

import (
	"fmt"
	"strings"

	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/trace"
)

func (e *AgentEngine) handleFinishStep(runID string, flowID string, planStep *agentstypes.PlanStep, ctx *agentstypes.ExecutionContext, result *ExecutionResult, state *autonomousFlowState) (isFinish bool, shouldContinue bool) {
	if planStep.Tool != "finish" {
		return false, false
	}
	status, _ := planStep.Params["status"].(string)
	if status == "success" && state.consecutiveRepeats > 0 {
		state.blockedFinishSuccessCount++
		if state.blockedFinishSuccessCount >= 3 {
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "blocked_finish_success_limit",
				"LLM blocked finish(success) 3+ times after loop detection; failing flow")
			result.Outcome = OutcomeFail
			result.Errors = append(result.Errors, "LLM attempted finish(success) 3+ times after loop detection without making progress")
			return true, false
		}
		msg := "⚠ BLOCKED: finish(success) immediately after a loop detection. The goal may not be met. Either try a different approach or use finish(fail)."
		if len(ctx.SteeringInstructions) >= 5 {
			ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
		}
		ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
		ctx.RepetitionBlockedSuccess = true
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "blocked_finish_success_after_loop", msg)
		state.stepCount++
		return true, true
	}
	if status == "fail" {
		if state.stepCount < 3 {
			msg := fmt.Sprintf("⚠ EARLY EXIT: finish(fail) at step %d is too soon. The LLM should make at least 3 attempts before giving up. Observations so far: %d.", state.stepCount+1, len(ctx.Observations))
			if len(ctx.SteeringInstructions) >= 5 {
				ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
			}
			ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "early_exit_prevented", msg)
			state.stepCount++
			return true, true
		}
		result.Outcome = OutcomeFail
		errMsg := "LLM signaled that the goal is unachievable"
		if planStep.Reason != "" {
			errMsg += ": " + planStep.Reason
		}
		result.Errors = append(result.Errors, errMsg)
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "planner", "finish_signal_fail", planStep.Reason)
	} else if status == "success" {
		result.Outcome = OutcomePass
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "planner", "finish_signal", "LLM signaled completion")
	}
	return true, false
}

func (e *AgentEngine) handleRepeatDetection(runID string, flowID string, planStep *agentstypes.PlanStep, ctx *agentstypes.ExecutionContext, result *ExecutionResult, state *autonomousFlowState) bool {
	sig := stepSignature(planStep)
	if sig == "" || sig != ctx.LastStepSignature {
		ctx.LastStepSignature = sig
		state.consecutiveRepeats = 0
		state.blockedFinishSuccessCount = 0
		return false
	}
	state.consecutiveRepeats++
	if state.consecutiveRepeats >= 2 {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "loop_hard_break",
			fmt.Sprintf("LLM repeated same step %d times; aborting", state.consecutiveRepeats))
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, "LLM stuck in loop — repeated same step 2+ times despite steering")
		return true
	}
	msg := fmt.Sprintf("⚠ LOOP DETECTED: step %s %v repeated. Try a different approach. Do NOT finish with success unless observations confirm the goal is met.", planStep.Tool, planStep.Params)
	if len(ctx.SteeringInstructions) >= 5 {
		ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
	}
	ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
	trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "loop_detected", msg)
	state.stepCount++
	return true
}

func (e *AgentEngine) handleAlternationDetection(runID string, flowID string, planStep *agentstypes.PlanStep, ctx *agentstypes.ExecutionContext, result *ExecutionResult, state *autonomousFlowState) bool {
	_ = result
	if planStep.Tool != "navigate" {
		return false
	}
	url, ok := planStep.Params["url"].(string)
	if !ok || url == "" || ctx.VisitedURLs == nil || !ctx.VisitedURLs[url] {
		return false
	}
	msg := fmt.Sprintf("⚠ URL ALREADY VISITED: '%s' was already navigated to. Do not revisit. Try a different approach.", url)
	if len(ctx.SteeringInstructions) >= 5 {
		ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
	}
	ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
	trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "url_alternation_detected", msg)
	state.stepCount++
	return true
}

func (e *AgentEngine) handleObserveLoop(runID string, flowID string, planStep *agentstypes.PlanStep, ctx *agentstypes.ExecutionContext, result *ExecutionResult, state *autonomousFlowState) bool {
	_ = result
	if planStep.Tool != "observe_ui" {
		ctx.ConsecutiveObserveCount = 0
		return false
	}
	ctx.ConsecutiveObserveCount++
	if ctx.ConsecutiveObserveCount <= 3 {
		return false
	}
	msg := "⚠ OBSERVE LOOP: observe_ui called 4+ times without progress. Try a different tool. Do NOT finish with success unless observations confirm the goal is met."
	if len(ctx.SteeringInstructions) >= 5 {
		ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
	}
	ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
	trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "observe_loop", msg)
	state.stepCount++
	ctx.ConsecutiveObserveCount = 0
	return true
}

func (e *AgentEngine) handleSelectorValidation(runID string, flowID string, planStep *agentstypes.PlanStep, ctx *agentstypes.ExecutionContext, result *ExecutionResult, state *autonomousFlowState) bool {
	_ = result
	switch planStep.Tool {
	case "get_html", "get_text", "evaluate":
		return false
	}
	selector, ok := planStep.Params["selector"].(string)
	if !ok || selector == "" || isSafeGenericSelector(selector) {
		return false
	}
	valid := observedSelectors(ctx.Observations)
	if len(valid) == 0 || containsSelector(valid, selector) {
		return false
	}
	if text := extractTextFromSelector(selector); text != "" {
		if elements := observedElements(ctx.Observations); len(elements) > 0 {
			if best, ok := findBestMatchSelector(text, elements); ok {
				trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "selector_auto_replaced",
					fmt.Sprintf("auto-replaced '%s' → '%s' (text='%s')", selector, best, text))
				planStep.Params["selector"] = best
				return false
			}
		}
	}
	msg := fmt.Sprintf("⚠ INVALID SELECTOR: '%s' was not found in the observed page elements. Use only selectors from the observation. Valid selectors: %s", selector, strings.Join(valid, ", "))
	if len(ctx.SteeringInstructions) >= 5 {
		ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
	}
	ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
	trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "invalid_selector", msg)
	state.stepCount++
	return true
}

func (e *AgentEngine) handle404Intercept(ctx *agentstypes.ExecutionContext, runID string, flow sharedtypes.Flow, result *ExecutionResult, state *autonomousFlowState) bool {
	if !e.recovery.Has404Warning(ctx) {
		return false
	}
	state.rootNavCount++
	if state.rootNavCount > 2 {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, "root navigation attempted 3+ times — likely invalid target URL")
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "root_nav_limit", "root navigation retry limit reached")
		return true
	}
	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "404_intercept", "intercepted 404 warning, forcing root navigation")
	e.performRootNav(ctx, runID, flow.ID, result)
	result.Retries++
	state.consecutiveFailures = 0
	state.stepCount++
	return true
}

func (e *AgentEngine) handleStepFailure(ctx *agentstypes.ExecutionContext, runID string, flow sharedtypes.Flow, planStep *agentstypes.PlanStep, stepResult *agentstypes.StepResult, result *ExecutionResult, state *autonomousFlowState) bool {
	if stepResult.Success {
		state.consecutiveFailures = 0
		return false
	}

	state.consecutiveFailures++
	e.setCurrentAgent(runID, "recovery")
	trace.EmitRecoveryAction(e.traceStore, runID, flow.ID, nil, stepResult)
	decision := e.handleFailure(ctx, stepResult, result)
	trace.EmitRecoveryAction(e.traceStore, runID, flow.ID, decision, stepResult)

	switch decision.Action {
	case agentstypes.RecoveryActionRootNav:
		state.rootNavCount++
		if state.rootNavCount > 2 {
			result.Outcome = OutcomeFail
			result.Errors = append(result.Errors, "root navigation attempted 3+ times — likely invalid target URL")
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "root_nav_limit", "root navigation retry limit reached")
			return true
		}
		e.performRootNav(ctx, runID, flow.ID, result)
		result.Retries++
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "root_nav", decision.Reason)
		state.consecutiveFailures = 0
		state.stepCount++

	case agentstypes.RecoveryActionRetry:
		result.Retries++
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "retry", decision.Reason)
		e.injectObserveStep(runID, flow.ID, ctx, state.plan, state.planner, result)
		e.backoffAndCheck(state)
		if result.Outcome != "" {
			return true
		}

	case agentstypes.RecoveryActionReplan:
		result.Retries++
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "replan", decision.Reason)
		e.injectObserveStep(runID, flow.ID, ctx, state.plan, state.planner, result)
		e.backoffAndCheck(state)
		if result.Outcome != "" {
			return true
		}

	case agentstypes.RecoveryActionSkip:
		state.planner.UpdatePlan(state.plan, planStep.StepIndex, true, decision.Reason)
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "skip", decision.Reason)

	case agentstypes.RecoveryActionFail:
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, decision.Reason)
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "fail", decision.Reason)
		return true
	}
	return true
}
