package engine

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"qa-orchestrator/packages/agents/planner"
	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/trace"
)

type autonomousFlowState struct {
	plan                      *agentstypes.Plan
	planner                   *planner.Planner
	maxSteps                  int
	stepCount                 int
	consecutiveFailures       int
	consecutiveRepeats        int
	blockedFinishSuccessCount int
	rootNavCount              int
	backoffCtx                context.Context
	backoffCancel             context.CancelFunc
	pendingSteps              []*agentstypes.PlanStep
}

func (e *AgentEngine) runAutonomousFlow(runID string, flow sharedtypes.Flow, ctx *agentstypes.ExecutionContext, result *ExecutionResult, start time.Time) *ExecutionResult {
	if e.llmClient == nil {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, "autonomous mode requires LLM client")
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "error", "LLM client not configured")
		return result
	}

	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "mode", "autonomous")

	llmTools := convertToLLMTools(e.browserTools)
	autonomousPlanner := planner.NewAutonomousPlanner(e.llmClient, llmTools)

	backoffCtx, backoffCancel := e.autonomousLLMContext(context.Background())
	defer backoffCancel()

	if !e.createInitialAutonomousPlan(runID, flow, ctx, result, autonomousPlanner) {
		return result
	}

	e.autoNavigateIfNeeded(runID, flow, ctx, autonomousPlanner, result)

	state := &autonomousFlowState{
		plan:          ctx.Plan,
		planner:       autonomousPlanner,
		backoffCtx:    backoffCtx,
		backoffCancel: backoffCancel,
	}
	state.maxSteps = flow.Config.MaxAutonomousSteps
	if state.maxSteps == 0 {
		state.maxSteps = 20
	}
	if ctx.VisitedURLs == nil {
		ctx.VisitedURLs = make(map[string]bool)
	}
	if ctx.CurrentURL != "" {
		ctx.VisitedURLs[ctx.CurrentURL] = true
	}

	for state.stepCount < state.maxSteps {
		e.drainSteeringEvents(ctx, runID, flow.ID)

		switch e.handlePauseResume(runID, ctx) {
		case pauseFail:
			result.Outcome = OutcomeFail
			result.Errors = append(result.Errors, "session deleted during autonomous execution")
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "error", "session deleted")
			goto done
		case pauseSkip:
			result.Outcome = OutcomeSkip
			result.Errors = append(result.Errors, "cancelled during autonomous execution")
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "cancelled", "autonomous flow cancelled")
			goto done
		}

		if e.handleSteeringCommands(ctx, runID, flow, result) {
			goto done
		}

		var planStep *agentstypes.PlanStep
		if len(state.pendingSteps) > 0 {
			planStep = state.pendingSteps[0]
			state.pendingSteps = state.pendingSteps[1:]
		} else {
			var shouldCont bool
			planStep, shouldCont = e.generateAutonomousStep(runID, flow, ctx, result, state)
			if planStep == nil {
				if shouldCont {
					continue
				}
				break
			}
		}

		if isFinish, shouldContinue := e.handleFinishStep(runID, flow.ID, planStep, ctx, result, state); isFinish {
			if shouldContinue {
				state.pendingSteps = nil
				continue
			}
			break
		}

		if e.handleRepeatDetection(runID, flow.ID, planStep, ctx, result, state) {
			state.pendingSteps = nil
			continue
		}

		if e.handleAlternationDetection(runID, flow.ID, planStep, ctx, result, state) {
			state.pendingSteps = nil
			continue
		}

		if e.handleObserveLoop(runID, flow.ID, planStep, ctx, result, state) {
			state.pendingSteps = nil
			continue
		}

		if e.handleSelectorValidation(runID, flow.ID, planStep, ctx, result, state) {
			state.pendingSteps = nil
			continue
		}

		state.planner.AddStepToPlan(state.plan, planStep)
		ctx.Plan = state.plan

		e.saveCheckpoint(runID, ctx, planStep)
		e.setCurrentAgent(runID, "executor")
		stepResult := e.executeAndValidate(ctx, planStep)
		result.Steps = append(result.Steps, stepResult)
		planStep.Result = stepResult
		if state.plan != nil && planStep.StepIndex >= 0 && planStep.StepIndex < len(state.plan.Steps) {
			state.plan.UpdateStepResult(planStep.StepIndex, stepResult)
		}

		if stepResult.Success && planStep.Tool == "navigate" {
			if url, ok := planStep.Params["url"].(string); ok && url != "" {
				ctx.CurrentURL = url
				if ctx.VisitedURLs != nil {
					ctx.VisitedURLs[url] = true
				}
			}
			e.injectObserveStep(runID, flow.ID, ctx, state.plan, state.planner, result)
		}

		if e.handle404Intercept(ctx, runID, flow, result, state) {
			if result.Outcome == OutcomeFail || result.Outcome == OutcomeSkip {
				goto done
			}
			state.pendingSteps = nil
			continue
		}

		if e.handleStepFailure(ctx, runID, flow, planStep, stepResult, result, state) {
			if result.Outcome == OutcomeFail || result.Outcome == OutcomeSkip {
				goto done
			}
			state.pendingSteps = nil
			continue
		}

		state.stepCount++
		state.planner.Advance(state.plan)

		if state.planner.ShouldStop(state.plan) {
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "plan_completed", fmt.Sprintf("generated %d steps", state.stepCount))
			break
		}

	}

	if state.stepCount >= state.maxSteps {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, "max autonomous steps reached without finishing")
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "max_steps_reached", fmt.Sprintf("reached max %d steps", state.maxSteps))
	}

done:
	e.finalizeRunResult(runID, flow.ID, result, start, map[string]any{"autonomous_steps": state.stepCount})
	return result
}

func (e *AgentEngine) createInitialAutonomousPlan(runID string, flow sharedtypes.Flow, ctx *agentstypes.ExecutionContext, result *ExecutionResult, autonomousPlanner *planner.Planner) bool {
	e.setCurrentAgent(runID, "planner (init)")
	plan, err := autonomousPlanner.CreatePlan(ctx)
	if err != nil {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create autonomous plan: %v", err))
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "failed", err.Error())
		return false
	}
	ctx.Plan = plan
	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "autonomous_plan_created", "starting iterative step generation")
	return true
}

func (e *AgentEngine) autoNavigateIfNeeded(runID string, flow sharedtypes.Flow, ctx *agentstypes.ExecutionContext, autonomousPlanner *planner.Planner, result *ExecutionResult) {
	if ctx.StartURL == "" {
		return
	}
	navStep := &agentstypes.PlanStep{
		StepIndex: -1, StepID: "auto-navigate", Tool: "navigate",
		Params: map[string]any{"url": ctx.StartURL},
		Skip:   false, Reason: "auto-navigate to configured start_url before LLM generates any steps",
	}
	stepResult := e.executeAndValidate(ctx, navStep)
	result.Steps = append(result.Steps, stepResult)
	if stepResult.Success {
		ctx.CurrentURL = ctx.StartURL
		e.injectObserveStep(runID, flow.ID, ctx, ctx.Plan, autonomousPlanner, result)
	} else {
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "auto_navigate_failed",
			fmt.Sprintf("auto-navigate to %s failed: %v", ctx.StartURL, stepResult.Error))
	}
}

func (e *AgentEngine) handleSteeringCommands(ctx *agentstypes.ExecutionContext, runID string, flow sharedtypes.Flow, result *ExecutionResult) bool {
	if ctx.SteeringRetryRequested {
		ctx.SteeringRetryRequested = false
		msg := "⚠ USER RETRY REQUESTED: The user wants a retry. Try a completely different approach or navigation path."
		if len(ctx.SteeringInstructions) >= 20 {
			ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
		}
		ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "steering", "retry_injected", msg)
		return false
	}
	if ctx.SteeringSkipRequested {
		ctx.SteeringSkipRequested = false
		result.Outcome = OutcomeSkip
		result.Errors = append(result.Errors, "user requested skip via steering")
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "steering", "skip_executed", "autonomous flow skipped by user")
		return true
	}
	return false
}

func (e *AgentEngine) generateAutonomousStep(runID string, flow sharedtypes.Flow, ctx *agentstypes.ExecutionContext, result *ExecutionResult, state *autonomousFlowState) (*agentstypes.PlanStep, bool) {
	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "generating_step", fmt.Sprintf("step %d", state.stepCount+1))
	e.setCurrentAgent(runID, fmt.Sprintf("planner (step %d)", state.stepCount+1))

	obsSummary := buildObservationSummary(ctx)
	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "observation_context", obsSummary)

	llmCtx, llmCancel := e.autonomousLLMContext(context.Background())
	defer llmCancel()
	planStep, remainingRaw, err := state.planner.GenerateNextStep(llmCtx, ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			result.Outcome = OutcomeSkip
			result.Errors = append(result.Errors, "cancelled during step generation")
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generation_cancelled", "context cancelled")
			return nil, false
		}
		if strings.Contains(err.Error(), "parsing") {
			msg := "Your previous response was not valid JSON. Please return ONLY a valid JSON array with no surrounding text, markdown, or explanation."
			if len(ctx.SteeringInstructions) >= 20 {
				ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
			}
			ctx.SteeringInstructions = append(ctx.SteeringInstructions, msg)
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "json_parse_error", msg)
			state.stepCount++
			return nil, true
		}
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, fmt.Sprintf("failed to generate step: %v", err))
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generation_failed", err.Error())
		return nil, false
	}

	for _, raw := range remainingRaw {
		tool, _ := raw["tool"].(string)
		if tool == "finish" {
			finishStep, convErr := planner.RawStepToPlanStep(raw, ctx.Plan)
			if convErr != nil {
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "batch_finish_skip",
					fmt.Sprintf("skipping malformed finish step: %v", convErr))
				break
			}
			state.pendingSteps = append(state.pendingSteps, finishStep)
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "batch_finish_queued",
				fmt.Sprintf("finish queued in pendingSteps after %d pre-finish steps", len(state.pendingSteps)-1))
			break
		}
		batchStep, convErr := planner.RawStepToPlanStep(raw, ctx.Plan)
		if convErr != nil {
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "batch_step_skip",
				fmt.Sprintf("skipping batch step: %v", convErr))
			continue
		}
		state.pendingSteps = append(state.pendingSteps, batchStep)
	}

	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generated",
		fmt.Sprintf("tool=%s params=%v reason=%s (batched %d)", planStep.Tool, planStep.Params, planStep.Reason, len(remainingRaw)))
	return planStep, false
}

func (e *AgentEngine) backoffAndCheck(state *autonomousFlowState) {
	if state == nil {
		return
	}
	backoff := time.Duration(1<<state.consecutiveFailures) * time.Second
	if backoff > 15*time.Second {
		backoff = 15 * time.Second
	}
	select {
	case <-state.backoffCtx.Done():
	case <-time.After(backoff):
	}
}

func (e *AgentEngine) injectObserveStep(runID, flowID string, ctx *agentstypes.ExecutionContext, plan *agentstypes.Plan, p *planner.Planner, result *ExecutionResult) *agentstypes.StepResult {
	if e.toolRegistry == nil {
		return nil
	}
	hasTool := false
	if reg, ok := e.toolRegistry.(interface{ HasTool(string) bool }); ok {
		hasTool = reg.HasTool("observe_ui")
	}
	if !hasTool {
		return nil
	}

	obsStepID := fmt.Sprintf("auto-observe-%d", len(plan.Steps)+1)
	obsStep := agentstypes.PlanStep{
		StepIndex: plan.CurrentIdx,
		StepID:    obsStepID,
		Tool:      "observe_ui",
		Params:    map[string]any{},
		Skip:      false,
		Reason:    "auto-injected observe step after navigate/failure to ground next action in current page state",
	}

	p.AddStepToPlan(plan, &obsStep)
	ctx.Plan = plan

	e.saveCheckpoint(runID, ctx, &obsStep)
	e.setCurrentAgent(runID, "executor")
	stepResult := e.executor.ExecuteStep(&obsStep)

	obs := e.validator.CreateObservation(stepResult)
	// Make sure the observation has "data" mapped to the output so planner can parse it
	obs.State["data"] = stepResult.Output
	ctx.Observations = append(ctx.Observations, *obs)
	const maxObservations = 50
	if len(ctx.Observations) > maxObservations {
		ctx.Observations = ctx.Observations[len(ctx.Observations)-maxObservations:]
	}

	trace.EmitStepExecution(e.traceStore, runID, flowID, stepResult)
	trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "injected_observe", fmt.Sprintf("observe_ui step %s injected", obsStepID))

	result.Steps = append(result.Steps, stepResult)
	return stepResult
}

// performRootNav navigates the browser to the root domain of the current URL,
// clears observation history (removing stale 404 warnings), and injects a
// fresh observe_ui so the LLM wakes up on the homepage with a clean slate.
func (e *AgentEngine) performRootNav(ctx *agentstypes.ExecutionContext, runID, flowID string, result *ExecutionResult) {
	if e.toolRegistry == nil {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "root_nav_failed", "no tool registry available")
		return
	}

	sourceURL := ctx.CurrentURL
	if sourceURL == "" {
		sourceURL = ctx.StartURL
	}
	if sourceURL == "" {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "root_nav_failed", "no URL to extract root domain from")
		return
	}

	parsed, err := url.Parse(sourceURL)
	if err != nil {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "root_nav_failed", fmt.Sprintf("failed to parse URL %s: %v", sourceURL, err))
		return
	}
	rootDomain := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)

	trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "root_nav_started",
		fmt.Sprintf("navigating to root domain %s (from source %s)", rootDomain, sourceURL))

	navStep := &agentstypes.PlanStep{
		StepIndex: -1,
		StepID:    "engine-root-nav",
		Tool:      "navigate",
		Params:    map[string]any{"url": rootDomain},
		Skip:      false,
		Reason:    "engine-initiated root domain navigation for 404 recovery",
	}
	navResult := e.executor.ExecuteStep(navStep)
	result.Steps = append(result.Steps, navResult)
	trace.EmitStepExecution(e.traceStore, runID, flowID, navResult)

	// Clear stale observations — the 404 warning and any subsequent failure
	// observations are no longer relevant.
	ctx.Observations = nil

	// Inject fresh observe_ui so the LLM gets a clean picture of the homepage.
	obsStep := &agentstypes.PlanStep{
		StepIndex: -1,
		StepID:    "engine-root-observe",
		Tool:      "observe_ui",
		Params:    map[string]any{},
		Skip:      false,
		Reason:    "engine-initiated observe after root domain navigation",
	}
	obsResult := e.executor.ExecuteStep(obsStep)
	obs := e.validator.CreateObservation(obsResult)
	obs.State["data"] = obsResult.Output
	ctx.Observations = append(ctx.Observations, *obs)
	result.Steps = append(result.Steps, obsResult)
	trace.EmitStepExecution(e.traceStore, runID, flowID, obsResult)

	ctx.CurrentURL = rootDomain
	// Reset visited URLs so the LLM can navigate to target pages again without
	// hitting the "URL ALREADY VISITED" alternation-deadlock.
	ctx.VisitedURLs = map[string]bool{rootDomain: true}
	if navResult.Success {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "root_nav_done",
			fmt.Sprintf("successfully navigated to root domain %s", rootDomain))
	} else {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "engine", "root_nav_warn",
			fmt.Sprintf("navigate to root domain %s returned error: %v — continuing anyway", rootDomain, navResult.Error))
	}
}
