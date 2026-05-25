package engine

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/agents/planner"
	"qa-orchestrator/packages/agents/recovery"
	agenttools "qa-orchestrator/packages/agents/tools"
	agentstypes "qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/agents/validator"
	browsertools "qa-orchestrator/packages/browser-runtime/tools"
	"qa-orchestrator/packages/llm"
	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/shared"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

const (
	retryBackoffBaseMs     = 100
	resumePollInitialDelay = 200 * time.Millisecond
	resumePollMaxDelay     = 2 * time.Second

	cancelledErrPrefix = "cancelled "
)

type pauseAction int

const (
	pauseContinue pauseAction = iota
	pauseFail
	pauseSkip
)

type FlowOutcome string

const (
	OutcomePass FlowOutcome = "PASSED"
	OutcomeFail FlowOutcome = "FAILED"
	OutcomeSkip FlowOutcome = "SKIPPED"
)

type ExecutionResult struct {
	FlowID       string
	Outcome      FlowOutcome
	Steps        []*agentstypes.StepResult
	Errors       []string
	DurationMs   int64
	Retries      int
	ArtifactIDs  []string
	IsAutonomous bool
}

type (
	Planner interface {
		CreatePlan(ctx *agentstypes.ExecutionContext) (*agentstypes.Plan, error)
		UpdatePlan(plan *agentstypes.Plan, stepIdx int, skip bool, reason string)
		GetNextStep(plan *agentstypes.Plan) *agentstypes.PlanStep
		Advance(plan *agentstypes.Plan)
		ShouldStop(plan *agentstypes.Plan) bool
		GetProgress(plan *agentstypes.Plan) (completed, total int)
		CreateAutonomousPlan(ctx *agentstypes.ExecutionContext) (*agentstypes.Plan, error)
		GenerateNextStep(ctx context.Context, execCtx *agentstypes.ExecutionContext) (*agentstypes.PlanStep, error)
		AddStepToPlan(plan *agentstypes.Plan, step *agentstypes.PlanStep)
		IsAutonomousMode(ctx *agentstypes.ExecutionContext) bool
	}

	Executor interface {
		ExecuteStep(step *agentstypes.PlanStep) *agentstypes.StepResult
	}

	Validator interface {
		ValidateStep(step *agentstypes.Step, result *agentstypes.StepResult) *validator.ValidationResult
		CreateObservation(result *agentstypes.StepResult) *agentstypes.Observation
	}

	Recovery interface {
		Decide(err error, stepResult *agentstypes.StepResult, ctx *agentstypes.ExecutionContext) *agentstypes.RecoveryDecision
		ShouldRetry(decision *agentstypes.RecoveryDecision, retryCount int) bool
		ShouldEscalate(decision *agentstypes.RecoveryDecision, retryCount int) bool
		Has404Warning(ctx *agentstypes.ExecutionContext) bool
		CreateRetryObservation(err error, retryCount int) *agentstypes.Observation
	}

	EngineOption func(*AgentEngine)
)

func WithPlanner(p Planner) EngineOption {
	return func(e *AgentEngine) { e.planner = p }
}

func WithExecutor(ex Executor) EngineOption {
	return func(e *AgentEngine) { e.executor = ex }
}

func WithValidator(v Validator) EngineOption {
	return func(e *AgentEngine) { e.validator = v }
}

func WithRecovery(r Recovery) EngineOption {
	return func(e *AgentEngine) { e.recovery = r }
}

func WithSessionStore(s *session.SessionStore) EngineOption {
	return func(e *AgentEngine) { e.sessionStore = s }
}

func WithTraceStore(t *trace.TraceStore) EngineOption {
	return func(e *AgentEngine) { e.traceStore = t }
}

func WithArtifactStore(a *artifact.ArtifactStore) EngineOption {
	return func(e *AgentEngine) { e.artifactStore = a }
}

func WithLLMClient(c planner.LLMClient) EngineOption {
	return func(e *AgentEngine) { e.llmClient = c }
}

func WithLifecycle(lc *runtime.LifecycleController) EngineOption {
	return func(e *AgentEngine) {
		e.lifecycle = lc
		if lc != nil {
			e.initLifecycleCtx()
		}
	}
}

func WithToolRegistry(reg executor.ToolRegistry) EngineOption {
	return func(e *AgentEngine) {
		e.toolRegistry = reg
		e.executor = executor.NewExecutor(reg)
	}
}

func WithBrowserTools(bt interface {
	ListToolsWithDocs() []browsertools.ToolInfo
}) EngineOption {
	return func(e *AgentEngine) { e.browserTools = bt }
}

func WithDependencyContext(ctx string) EngineOption {
	return func(e *AgentEngine) { e.dependencyContext = ctx }
}

type AgentEngine struct {
	mu            sync.RWMutex
	planner       Planner
	executor      Executor
	validator     Validator
	recovery      Recovery
	traceStore    *trace.TraceStore
	artifactStore *artifact.ArtifactStore
	sessionStore  *session.SessionStore
	lifecycle     *runtime.LifecycleController
	llmClient     planner.LLMClient
	toolRegistry  executor.ToolRegistry
	browserTools  interface {
		ListToolsWithDocs() []browsertools.ToolInfo
	}
	dependencyContext string
	lifecycleCtx      context.Context
	lifecycleCancel   context.CancelFunc
}

func NewAgentEngine(opts ...EngineOption) *AgentEngine {
	e := &AgentEngine{
		planner:   planner.NewPlanner(),
		executor:  executor.NewExecutor(executor.NewMockToolRegistry()),
		validator: validator.NewValidator(),
		recovery:  recovery.NewRecovery(nil),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func NewAgentEngineWithRegistry(registry executor.ToolRegistry) *AgentEngine {
	return NewAgentEngine(WithToolRegistry(registry))
}

func NewAgentEngineWithStores(registry executor.ToolRegistry, sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) *AgentEngine {
	return NewAgentEngine(
		WithToolRegistry(registry),
		WithSessionStore(sessionStore),
		WithTraceStore(traceStore),
		WithArtifactStore(artifactStore),
		WithLifecycle(runtime.NewLifecycleController("")),
	)
}

func NewAgentEngineWithLLM(registry executor.ToolRegistry, sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, llmClient planner.LLMClient, browserTools interface {
	ListToolsWithDocs() []browsertools.ToolInfo
}) *AgentEngine {
	return NewAgentEngine(
		WithToolRegistry(registry),
		WithSessionStore(sessionStore),
		WithTraceStore(traceStore),
		WithArtifactStore(artifactStore),
		WithLLMClient(llmClient),
		WithBrowserTools(browserTools),
		WithLifecycle(runtime.NewLifecycleController("")),
	)
}

func (e *AgentEngine) initLifecycleCtx() {
	if e.lifecycleCancel != nil {
		e.lifecycleCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancelCh := e.lifecycle.CancelCh()
	go func() {
		select {
		case <-cancelCh:
			cancel()
		case <-ctx.Done():
		}
	}()
	e.lifecycleCtx = ctx
	e.lifecycleCancel = cancel
}

func (e *AgentEngine) Close() {
	if e.lifecycleCancel != nil {
		e.lifecycleCancel()
	}
}

func (e *AgentEngine) SetLLMClient(client planner.LLMClient) {
	e.mu.Lock()
	e.llmClient = client
	e.mu.Unlock()
}

func (e *AgentEngine) SetBrowserTools(tools interface {
	ListToolsWithDocs() []browsertools.ToolInfo
}) {
	e.mu.Lock()
	e.browserTools = tools
	e.mu.Unlock()
}

func (e *AgentEngine) RunFlow(runID string, flow sharedtypes.Flow) *ExecutionResult {
	e.SetLifecycleRunID(runID)
	start := time.Now()
	result := &ExecutionResult{
		FlowID:       flow.ID,
		Outcome:      OutcomePass,
		Steps:        []*agentstypes.StepResult{},
		Errors:       []string{},
		IsAutonomous: flow.Mode == sharedtypes.FlowModeAutonomous,
	}

	e.mu.RLock()
	depCtx := e.dependencyContext
	e.mu.RUnlock()

	ctx := &agentstypes.ExecutionContext{
		RunID:             runID,
		FlowID:            flow.ID,
		Goal:              flow.Goal,
		StartURL:          flow.StartURL,
		Mode:              flow.Mode,
		Steps:             flow.Steps,
		DependencyContext: depCtx,
	}

	if e.sessionStore != nil {
		e.syncSessionStore(runID, flow.ID, "update_flow_running", func() error {
			return e.sessionStore.UpdateFlowState(runID, flow.ID, sharedtypes.FlowStateRunning, "")
		})
		e.syncSessionStore(runID, flow.ID, "set_current_flow", func() error {
			sess, err := e.sessionStore.Get(runID)
			if err != nil || sess == nil {
				return err
			}
			sess.CurrentFlowID = flow.ID
			return e.sessionStore.Save(sess)
		})
	}

	if e.lifecycle != nil {
		cancelCh, ok := e.lifecycle.BeginExecution()
		if !ok {
			result.Outcome = OutcomeSkip
			result.Errors = append(result.Errors, shared.ErrCancelled.Error())
			e.finalizeFlowState(runID, flow.ID, result)
			return result
		}
		select {
		case <-cancelCh:
			result.Outcome = OutcomeSkip
			result.Errors = append(result.Errors, shared.ErrCancelled.Error())
			e.finalizeFlowState(runID, flow.ID, result)
			return result
		default:
		}
	}

	trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateRunning, map[string]any{
		"goal": flow.Goal,
		"mode": string(flow.Mode),
	})

	if flow.Mode == sharedtypes.FlowModeAutonomous {
		return e.runAutonomousFlow(runID, flow, ctx, result, start)
	}

	return e.runGuidedFlow(runID, flow, ctx, result, start)
}

func (e *AgentEngine) runGuidedFlow(runID string, flow sharedtypes.Flow, ctx *agentstypes.ExecutionContext, result *ExecutionResult, start time.Time) *ExecutionResult {
	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "mode", "guided")
	e.setCurrentAgent(ctx.RunID, "planner")

	plan, err := e.planner.CreatePlan(ctx)
	if err != nil {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create plan: %v", err))
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "failed", err.Error())
		return result
	}
	ctx.Plan = plan

	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "plan_created", fmt.Sprintf("created plan with %d steps", len(plan.Steps)))

	for !e.planner.ShouldStop(plan) {
		e.drainSteeringEvents(ctx, ctx.RunID, flow.ID)

		switch e.handlePauseResume(ctx.RunID, ctx) {
		case pauseFail:
			result.Outcome = OutcomeFail
			result.Errors = append(result.Errors, "session deleted during execution")
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "error", "session deleted")
			goto done
		case pauseSkip:
			result.Outcome = OutcomeSkip
			result.Errors = append(result.Errors, "cancelled during execution")
			goto done
		}

		if ctx.SteeringRetryRequested {
			ctx.SteeringRetryRequested = false
			if plan.Retreat() {
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "steering", "retrying_step", fmt.Sprintf("step %d", plan.CurrentIdx))
			}
		} else if ctx.SteeringSkipRequested {
			ctx.SteeringSkipRequested = false
			if ps := e.planner.GetNextStep(plan); ps != nil {
				e.planner.UpdatePlan(plan, ps.StepIndex, true, "user requested skip via steering")
				e.planner.Advance(plan)
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "steering", "skipping_step", fmt.Sprintf("step %d", ps.StepIndex))
			}
			continue
		}

		planStep := e.planner.GetNextStep(plan)
		if planStep == nil {
			break
		}

		e.saveCheckpoint(runID, ctx, planStep)
		e.setCurrentAgent(runID, "executor")
		stepResult := e.executeAndValidate(ctx, planStep)
		result.Steps = append(result.Steps, stepResult)
		planStep.Result = stepResult
		if plan != nil {
			plan.UpdateStepResult(planStep.StepIndex, stepResult)
		}

		if !stepResult.Success {
			e.setCurrentAgent(runID, "recovery")
			trace.EmitRecoveryAction(e.traceStore, runID, flow.ID, nil, stepResult)
			decision := e.handleFailure(ctx, stepResult, result)
			trace.EmitRecoveryAction(e.traceStore, runID, flow.ID, decision, stepResult)

		switch decision.Action {
		case agentstypes.RecoveryActionRetry:
			result.Retries++
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "retry", decision.Reason)
			continue
		case agentstypes.RecoveryActionReplan:
			// NOTE: RecoveryActionReplan does NOT actually replan — it's a retry
			// with a fresh observation injected. The name is historical. The recovery
			// agent returns this when selector timeouts suggest the LLM hallucinated
			// selectors, so we inject observe_ui to ground the next LLM call.
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "replan", decision.Reason)
			continue
		case agentstypes.RecoveryActionRootNav:
				result.Outcome = OutcomeFail
				result.Errors = append(result.Errors, "invalid URL or 404 detected — guided flow cannot auto-recover from 404")
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "root_nav_guided_fail", decision.Reason)
				goto done
			case agentstypes.RecoveryActionSkip:
				e.planner.UpdatePlan(plan, planStep.StepIndex, true, decision.Reason)
				e.planner.Advance(plan)
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "skip", decision.Reason)
				continue
			case agentstypes.RecoveryActionFail:
				result.Outcome = OutcomeFail
				result.Errors = append(result.Errors, decision.Reason)
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "fail", decision.Reason)
				goto done
			}
		}

		e.planner.Advance(plan)
	}

done:
	e.finalizeRunResult(runID, flow.ID, result, start, nil)
	return result
}

type autonomousFlowState struct {
	plan                   *agentstypes.Plan
	planner                *planner.Planner
	maxSteps               int
	stepCount              int
	consecutiveFailures    int
	consecutiveRepeats     int
	blockedFinishSuccessCount int
	rootNavCount           int
	backoffCtx             context.Context
	backoffCancel          context.CancelFunc
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
		plan:      ctx.Plan,
		planner:   autonomousPlanner,
		backoffCtx:   backoffCtx,
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

		planStep, shouldCont := e.generateAutonomousStep(runID, flow, ctx, result, state)
		if planStep == nil {
			if shouldCont {
				continue
			}
			break
		}

		if isFinish, shouldContinue := e.handleFinishStep(runID, flow.ID, planStep, ctx, result, state); isFinish {
			if shouldContinue {
				continue
			}
			break
		}

		if e.handleRepeatDetection(runID, flow.ID, planStep, ctx, result, state) {
			continue
		}

		if e.handleAlternationDetection(runID, flow.ID, planStep, ctx, result, state) {
			continue
		}

		if e.handleObserveLoop(runID, flow.ID, planStep, ctx, result, state) {
			continue
		}

		if e.handleSelectorValidation(runID, flow.ID, planStep, ctx, result, state) {
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
			continue
		}

		if e.handleStepFailure(ctx, runID, flow, planStep, stepResult, result, state) {
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
	planStep, err := state.planner.GenerateNextStep(llmCtx, ctx)
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

	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generated",
		fmt.Sprintf("tool=%s params=%v reason=%s", planStep.Tool, planStep.Params, planStep.Reason))
	return planStep, false
}

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
	} else {
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

func (e *AgentEngine) setCurrentAgent(runID, agent string) {
	if e.sessionStore == nil {
		return
	}
	e.syncSessionStore(runID, "", "save_current_agent", func() error {
		sess, err := e.sessionStore.Get(runID)
		if err != nil || sess == nil {
			return err
		}
		sess.CurrentAgent = agent
		return e.sessionStore.Save(sess)
	})
}

func (e *AgentEngine) executeAndValidate(ctx *agentstypes.ExecutionContext, planStep *agentstypes.PlanStep) *agentstypes.StepResult {
	stepResult := e.executor.ExecuteStep(planStep)

	obs := e.validator.CreateObservation(stepResult)
	ctx.Observations = append(ctx.Observations, *obs)

	trace.EmitStepExecution(e.traceStore, ctx.RunID, ctx.FlowID, stepResult)

	step, found := findStepByID(ctx.Steps, planStep.StepID)
	if found && len(step.Assertions) > 0 {
		validation := e.validator.ValidateStep(&step, stepResult)
		if !validation.Passed {
			stepResult.Success = false
			stepResult.Error = fmt.Errorf("%v", validation.Errors)
		}
	}

	e.autoObserve(ctx, stepResult)

	const maxObservations = 10
	if len(ctx.Observations) > maxObservations {
		ctx.Observations = ctx.Observations[len(ctx.Observations)-maxObservations:]
	}

	return stepResult
}

func (e *AgentEngine) autoObserve(ctx *agentstypes.ExecutionContext, stepResult *agentstypes.StepResult) {
	if ctx.Mode != sharedtypes.FlowModeAutonomous {
		return
	}
	if e.toolRegistry == nil {
		return
	}
	if reg, ok := e.toolRegistry.(interface{ HasTool(string) bool }); ok {
		if !reg.HasTool("observe_ui") {
			return
		}
	}
	result, err := e.toolRegistry.Execute("observe_ui", nil)
	if err != nil {
		if strings.Contains(err.Error(), "unknown tool") {
			return
		}
		trace.EmitAgentDecision(e.traceStore, ctx.RunID, ctx.FlowID, "engine", "auto_observe_failed", fmt.Sprintf("observe_ui error: %v", err))
		return
	}
	obs := agentstypes.Observation{
		State: map[string]any{"source": "observe_ui", "data": result},
		LastStep: &agentstypes.StepResult{
			StepID:  "observe_ui",
			Tool:    "observe_ui",
			Output:  result,
			Success: true,
		},
	}
	ctx.Observations = append(ctx.Observations, obs)
	trace.EmitAgentDecision(e.traceStore, ctx.RunID, ctx.FlowID, "engine", "auto_observe", fmt.Sprintf("triggered after %s (success=%v)", stepResult.Tool, stepResult.Success))
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
	const maxObservations = 10
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

func buildObservationSummary(ctx *agentstypes.ExecutionContext) string {
	if len(ctx.Observations) == 0 {
		return "no observations yet"
	}
	lastObs := ctx.Observations[len(ctx.Observations)-1]
	if lastObs.LastStep == nil {
		return "observation with no last step"
	}
	summary := fmt.Sprintf("last_obs_tool=%s success=%v", lastObs.LastStep.Tool, lastObs.LastStep.Success)
	if lastObs.LastStep.Tool == "observe_ui" {
		if data, ok := lastObs.State["data"].(map[string]any); ok {
			if pageState, ok := data["page_state"].(string); ok {
				summary += fmt.Sprintf(" page_state=%s", pageState)
			}
			if interactive, ok := data["interactive"].([]any); ok {
				summary += fmt.Sprintf(" elements=%d", len(interactive))
			}
			if warning, ok := data["warning"].(string); ok && warning != "" {
				summary += fmt.Sprintf(" warning=%s", warning)
			}
		}
	}
	if lastObs.Error != nil {
		summary += fmt.Sprintf(" error=%v", lastObs.Error)
	}
	return summary
}

func findStepByID(steps []sharedtypes.Step, stepID string) (sharedtypes.Step, bool) {
	for i := range steps {
		if steps[i].ID == stepID {
			return steps[i], true
		}
	}
	return sharedtypes.Step{}, false
}

func (e *AgentEngine) handleFailure(ctx *agentstypes.ExecutionContext, stepResult *agentstypes.StepResult, result *ExecutionResult) *agentstypes.RecoveryDecision {
	decision := e.recovery.Decide(stepResult.Error, stepResult, ctx)

	if e.recovery.ShouldEscalate(decision, result.Retries) {
		decision.Action = agentstypes.RecoveryActionFail
		decision.Reason = fmt.Sprintf("max retries (%d) exceeded", result.Retries)
	}

	return decision
}

func (e *AgentEngine) RunFlowWithRetry(runID string, flow sharedtypes.Flow, maxRetries int) *ExecutionResult {
	if maxRetries <= 0 {
		return e.RunFlow(runID, flow)
	}

	var lastResult *ExecutionResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result := e.RunFlow(runID, flow)
		lastResult = result

		if result.Outcome == OutcomePass {
			return result
		}

		if attempt < maxRetries {
			time.Sleep(time.Duration(retryBackoffBaseMs*(attempt+1)) * time.Millisecond)
		}
	}

	return lastResult
}

func (e *AgentEngine) GetPlanner() Planner {
	return e.planner
}

func (e *AgentEngine) GetExecutor() Executor {
	return e.executor
}

func (e *AgentEngine) GetValidator() Validator {
	return e.validator
}

func (e *AgentEngine) GetRecovery() Recovery {
	return e.recovery
}

func (e *AgentEngine) SetTraceStore(store *trace.TraceStore) {
	e.mu.Lock()
	e.traceStore = store
	e.mu.Unlock()
}

func (e *AgentEngine) SetArtifactStore(store *artifact.ArtifactStore) {
	e.mu.Lock()
	e.artifactStore = store
	e.mu.Unlock()
}

func (e *AgentEngine) saveCheckpoint(runID string, ctx *agentstypes.ExecutionContext, planStep *agentstypes.PlanStep) {
	if ctx.Plan == nil {
		return
	}
	visitedURLs := make(map[string]bool)
	for k, v := range ctx.VisitedURLs {
		visitedURLs[k] = v
	}
	payload := map[string]any{
		"current_step":               planStep.StepID,
		"step_index":                 planStep.StepIndex,
		"current_url":                ctx.CurrentURL,
		"last_step_signature":        ctx.LastStepSignature,
		"consecutive_observe_count":  ctx.ConsecutiveObserveCount,
		"visited_urls":               visitedURLs,
		"repetition_blocked_success": ctx.RepetitionBlockedSuccess,
	}
	for i, obs := range ctx.Observations {
		payload[fmt.Sprintf("obs_%d", i)] = obs.State
	}

	cp := &sharedtypes.Checkpoint{
		FlowID:    ctx.FlowID,
		StepID:    planStep.StepID,
		StepIndex: planStep.StepIndex,
		Payload:   payload,
	}

	if e.sessionStore != nil {
		e.syncSessionStore(runID, ctx.FlowID, "save_checkpoint", func() error {
			return e.sessionStore.SaveCheckpoint(runID, cp)
		})
	}

	trace.EmitCheckpoint(e.traceStore, runID, cp)
}

func (e *AgentEngine) EmitArtifact(runID, flowID string, artifactType artifact.ArtifactType, filename string, data []byte, metadata map[string]any) string {
	if e.artifactStore == nil {
		return ""
	}
	artifact, err := e.artifactStore.Save(runID, flowID, artifactType, filename, data, metadata)
	if err != nil {
		log.Printf("EmitArtifact: failed to save artifact: %v", err)
		return ""
	}
	trace.EmitArtifactEvent(e.traceStore, runID, flowID, string(artifactType), artifact.Path, metadata)
	return artifact.ArtifactID
}

func (e *AgentEngine) SetLifecycleController(lc *runtime.LifecycleController) {
	e.mu.Lock()
	e.lifecycle = lc
	e.mu.Unlock()
	if lc != nil {
		e.initLifecycleCtx()
	}
}

func (e *AgentEngine) SetLifecycleRunID(runID string) {
	e.mu.RLock()
	lc := e.lifecycle
	e.mu.RUnlock()
	if lc != nil {
		lc.SetRunID(runID)
	}
}

func (e *AgentEngine) SetToolRegistry(registry executor.ToolRegistry) {
	e.mu.Lock()
	e.toolRegistry = registry
	e.executor = executor.NewExecutor(registry)
	e.mu.Unlock()
}

func (e *AgentEngine) SetDependencyContext(ctx string) {
	e.mu.Lock()
	e.dependencyContext = ctx
	e.mu.Unlock()
}

func (e *AgentEngine) checkPauseState(runID string) (sharedtypes.RunState, bool) {
	if e.sessionStore == nil {
		return sharedtypes.RunStateRunning, true
	}
	sess, err := e.sessionStore.Get(runID)
	if err != nil || sess == nil {
		return sharedtypes.RunStateRunning, false
	}
	return sess.Status, true
}

func (e *AgentEngine) waitForResume(runID string) {
	if e.sessionStore == nil {
		return
	}
	pollInterval := resumePollInitialDelay
	maxPollInterval := resumePollMaxDelay
	for {
		time.Sleep(pollInterval)
		pollInterval *= 2
		if pollInterval > maxPollInterval {
			pollInterval = maxPollInterval
		}
		status, exists := e.checkPauseState(runID)
		if !exists {
			return
		}
		if status == sharedtypes.RunStateResuming || status == sharedtypes.RunStateRunning {
			e.syncSessionStore(runID, "", "transition_to_running", func() error {
				return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)
			})
			return
		}
		if status == sharedtypes.RunStateCancelling || status == sharedtypes.RunStateCancelled {
			return
		}
	}
}

func (e *AgentEngine) handlePauseResume(runID string, ctx *agentstypes.ExecutionContext) pauseAction {
	pauseStatus, exists := e.checkPauseState(runID)
	if !exists {
		return pauseFail
	}
	switch pauseStatus {
	case sharedtypes.RunStatePausing:
		// Re-check to avoid race with RequestCancel: if cancel fired
		// between checkPauseState above and this write, don't overwrite it.
		if currentStatus, _ := e.checkPauseState(runID); currentStatus == sharedtypes.RunStateCancelling || currentStatus == sharedtypes.RunStateCancelled {
			return pauseSkip
		}
		e.syncSessionStore(runID, ctx.FlowID, "transition_to_paused", func() error {
			return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStatePaused)
		})
		e.setCurrentAgent(runID, "idle (paused)")
		e.waitForResume(runID)
		e.restoreCheckpoint(ctx)
		cancelStatus, exists := e.checkPauseState(runID)
		if !exists {
			return pauseFail
		}
		if cancelStatus == sharedtypes.RunStateCancelling || cancelStatus == sharedtypes.RunStateCancelled {
			return pauseSkip
		}
		return pauseContinue
	case sharedtypes.RunStatePaused:
		e.waitForResume(runID)
		e.restoreCheckpoint(ctx)
		cancelStatus, exists := e.checkPauseState(runID)
		if !exists {
			return pauseFail
		}
		if cancelStatus == sharedtypes.RunStateCancelling || cancelStatus == sharedtypes.RunStateCancelled {
			return pauseSkip
		}
		return pauseContinue
	case sharedtypes.RunStateCancelling, sharedtypes.RunStateCancelled:
		return pauseSkip
	default:
		return pauseContinue
	}
}

func (e *AgentEngine) drainSteeringEvents(ctx *agentstypes.ExecutionContext, runID, flowID string) {
	if e.lifecycle == nil {
		return
	}
	events := e.lifecycle.DrainSteeringEvents()
	for _, evt := range events {
		if evt.FlowID != "" && evt.FlowID != flowID {
			continue
		}
		switch evt.Command {
		case sharedtypes.SteerInstruction:
			if evt.Instruction != "" {
				if len(ctx.SteeringInstructions) >= 20 {
					ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
				}
				ctx.SteeringInstructions = append(ctx.SteeringInstructions, evt.Instruction)
				trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "instruction_received", evt.Instruction)
			}
		case sharedtypes.SteerRetry:
			ctx.SteeringRetryRequested = true
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "retry_requested", evt.Reason)
		case sharedtypes.SteerSkip:
			ctx.SteeringSkipRequested = true
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "skip_requested", evt.Reason)
		case sharedtypes.SteerApprove:
			e.lifecycle.AcknowledgeInput()
			if currentStatus, _ := e.checkPauseState(runID); currentStatus == sharedtypes.RunStateCancelling || currentStatus == sharedtypes.RunStateCancelled {
				trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "approve_cancel_race", "cancel fired before approve; not overwriting with Running")
				break
			}
			e.syncSessionStore(runID, flowID, "approve_resume", func() error {
				return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)
			})
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "approved", evt.Reason)
		case sharedtypes.SteerContinue:
			e.lifecycle.AcknowledgeInput()
			if currentStatus, _ := e.checkPauseState(runID); currentStatus == sharedtypes.RunStateCancelling || currentStatus == sharedtypes.RunStateCancelled {
				trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "continue_cancel_race", "cancel fired before continue; not overwriting with Running")
				break
			}
			e.syncSessionStore(runID, flowID, "continue_resume", func() error {
				return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)
			})
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "continued", evt.Reason)
		case sharedtypes.SteerHumanReview:
			e.lifecycle.SetWaitingForInput()
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "human_review", evt.Reason)
		default:
			log.Printf("drainSteeringEvents: unknown command type=%q flow=%s", evt.Command, evt.FlowID)
		}
	}
}

func (e *AgentEngine) finalizeRunResult(runID, flowID string, result *ExecutionResult, start time.Time, extraDetails map[string]any) {
	result.DurationMs = time.Since(start).Milliseconds()

	if e.sessionStore != nil {
		e.finalizeFlowState(runID, flowID, result)
		e.setCurrentAgent(runID, "")
	}

	details := map[string]any{"duration_ms": result.DurationMs}
	for k, v := range extraDetails {
		details[k] = v
	}

	if result.Outcome == OutcomePass {
		trace.EmitLifecycleEvent(e.traceStore, runID, flowID, sharedtypes.RunStateCompleted, details)
	} else {
		trace.EmitLifecycleEvent(e.traceStore, runID, flowID, sharedtypes.RunStateFailed, details)
	}
}

func convertToLLMTools(tools interface {
	ListToolsWithDocs() []browsertools.ToolInfo
}) []llm.ToolInfo {
	if tools == nil {
		return getDefaultLLMTools()
	}

	if registry, ok := tools.(*browsertools.ToolRegistry); ok {
		return agenttools.RegistryToLLMTools(registry)
	}

	registryTools := tools.ListToolsWithDocs()
	if len(registryTools) == 0 {
		return getDefaultLLMTools()
	}

	result := make([]llm.ToolInfo, 0, len(registryTools))
	for _, t := range registryTools {
		params := make(map[string]llm.ParameterInfo, len(t.Parameters))
		for name, p := range t.Parameters {
			params[name] = llm.ParameterInfo{
				Type:        p.Type,
				Description: p.Description,
				Required:    p.Required,
			}
		}
		result = append(result, llm.ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  params,
		})
	}
	return result
}

func (e *AgentEngine) autonomousLLMContext(parent context.Context) (context.Context, context.CancelFunc) {
	if e.lifecycleCtx != nil {
		return context.WithCancel(e.lifecycleCtx)
	}
	return context.WithCancel(parent)
}

func getDefaultLLMTools() []llm.ToolInfo {
	return []llm.ToolInfo{
		{
			Name:        "navigate",
			Description: "Navigate to a URL in the browser",
			Parameters: map[string]llm.ParameterInfo{
				"url": {Type: "string", Description: "The URL to navigate to", Required: true},
			},
		},
		{
			Name:        "click",
			Description: "Click on an element identified by CSS selector",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the element to click", Required: true},
			},
		},
		{
			Name:        "type_text",
			Description: "Type text into an input field",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the input field", Required: true},
				"value":    {Type: "string", Description: "Text to type into the field", Required: true},
			},
		},
		{
			Name:        "wait_for",
			Description: "Wait for an element to reach a specific state",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the element to wait for", Required: true},
				"state":    {Type: "string", Description: "Wait state: visible, hidden, attached (default: visible)", Required: false},
			},
		},
		{
			Name:        "get_text",
			Description: "Get the text content of an element",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
			},
		},
		{
			Name:        "screenshot",
			Description: "Take a screenshot of the page",
			Parameters: map[string]llm.ParameterInfo{
				"path":      {Type: "string", Description: "File path to save the screenshot", Required: false},
				"full_page": {Type: "bool", Description: "Capture full page if true", Required: false},
			},
		},
		{
			Name:        "assert_text_visible",
			Description: "Assert that specific text is visible on the page",
			Parameters: map[string]llm.ParameterInfo{
				"text": {Type: "string", Description: "Text that should be visible on the page", Required: true},
			},
		},
		{
			Name:        "finish",
			Description: "Signal that the goal has been achieved (or is unachievable) and no more steps are needed. Use this when the task is complete.",
			Parameters: map[string]llm.ParameterInfo{
				"status": {Type: "string", Description: "Set to 'success' if goal is achieved, or 'fail' if the goal is unachievable (e.g. elements not found).", Required: false},
			},
		},
	}
}

// observedSelectors extracts valid selectors from the most recent observe_ui
// observation. Returns nil if no observe_ui data is available.
func observedSelectors(observations []agentstypes.Observation) []string {
	for i := len(observations) - 1; i >= 0; i-- {
		obs := observations[i]
		if obs.LastStep == nil || obs.LastStep.Tool != "observe_ui" {
			continue
		}
		data, ok := obs.State["data"].(map[string]any)
		if !ok {
			continue
		}
		interactive, ok := data["interactive"].([]any)
		if !ok {
			continue
		}
		result := make([]string, 0, len(interactive))
		for _, elem := range interactive {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			sel, ok := elemMap["selector"].(string)
			if !ok || sel == "" {
				continue
			}
			result = append(result, sel)
		}
		return result
	}
	return nil
}

var safeGenericSelectors = map[string]bool{
	"body":     true,
	"html":     true,
	"*":        true,
	"document": true,
	":root":    true,
}

func isSafeGenericSelector(selector string) bool {
	return safeGenericSelectors[selector]
}

func containsSelector(list []string, target string) bool {
	return shared.Contains(list, target)
}

// extractTextFromSelector parses `tag:has-text("Some text")` and returns the text.
// Returns "" if the selector does not use has-text.
var hasTextRE = regexp.MustCompile(`:has-text\("([^"]*)"\)`)

func extractTextFromSelector(selector string) string {
	match := hasTextRE.FindStringSubmatch(selector)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

// observedElements returns the full interactive element data from the most recent
// observe_ui observation. Each element is a map with keys: tag, text, id, name,
// placeholder, selector. Returns nil if no observe_ui data is available.
func observedElements(observations []agentstypes.Observation) []map[string]any {
	for i := len(observations) - 1; i >= 0; i-- {
		obs := observations[i]
		if obs.LastStep == nil || obs.LastStep.Tool != "observe_ui" {
			continue
		}
		data, ok := obs.State["data"].(map[string]any)
		if !ok {
			continue
		}
		interactive, ok := data["interactive"].([]any)
		if !ok {
			continue
		}
		result := make([]map[string]any, 0, len(interactive))
		for _, elem := range interactive {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, elemMap)
		}
		return result
	}
	return nil
}

// findBestMatchSelector finds the observed element whose text best matches the
// intent text. Matching is case-insensitive substring. Among matches, prefers
// the one with the closest text-length ratio (intent / element_text), preferring
// anchor tags and shorter text as tiebreakers. Returns (selector, true) on match.
func findBestMatchSelector(intentText string, elements []map[string]any) (string, bool) {
	if intentText == "" || len(elements) == 0 {
		return "", false
	}
	intentLower := strings.ToLower(intentText)

	type candidate struct {
		selector string
		tag      string
		textLen  int
		score    float64
	}

	var best *candidate

	for _, elem := range elements {
		text, _ := elem["text"].(string)
		if text == "" {
			continue
		}
		textLower := strings.ToLower(text)
		if !strings.Contains(textLower, intentLower) {
			continue
		}

		sel, _ := elem["selector"].(string)
		if sel == "" {
			continue
		}

		tag, _ := elem["tag"].(string)
		score := float64(len(intentText)) / float64(len(text))

		if best == nil ||
			score > best.score ||
			(score == best.score && tag == "a" && best.tag != "a") ||
			(score == best.score && len(text) < best.textLen) {
			best = &candidate{selector: sel, tag: tag, textLen: len(text), score: score}
		}
	}

	if best != nil {
		return best.selector, true
	}
	return "", false
}

// stepSignature produces a deterministic signature from a step's tool and
// parameter values. Returns "" if the step has no params.
func stepSignature(step *agentstypes.PlanStep) string {
	if step == nil || len(step.Params) == 0 {
		return ""
	}
	// Collect sorted param keys so signature is deterministic.
	keys := make([]string, 0, len(step.Params))
	for k := range step.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(step.Tool)
	for _, k := range keys {
		b.WriteString("|")
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(fmt.Sprint(step.Params[k]))
	}
	return b.String()
}

func (e *AgentEngine) syncSessionStore(runID, flowID, action string, fn func() error) bool {
	if e.sessionStore == nil {
		return true
	}
	if err := fn(); err != nil {
		trace.EmitAgentDecision(e.traceStore, runID, flowID, "session_sync", action+"_failed", err.Error())
		return false
	}
	return true
}

func (e *AgentEngine) restoreCheckpoint(ctx *agentstypes.ExecutionContext) {
	if e.sessionStore == nil {
		return
	}
	sess, err := e.sessionStore.Get(ctx.RunID)
	if err != nil || sess == nil || sess.Checkpoint == nil || sess.Checkpoint.Payload == nil {
		return
	}
	cp := sess.Checkpoint.Payload
	if url, ok := cp["current_url"].(string); ok {
		ctx.CurrentURL = url
	}
	if sig, ok := cp["last_step_signature"].(string); ok {
		ctx.LastStepSignature = sig
	}
	if countFloat, ok := cp["consecutive_observe_count"].(float64); ok {
		ctx.ConsecutiveObserveCount = int(countFloat)
	} else if countInt, ok := cp["consecutive_observe_count"].(int); ok {
		ctx.ConsecutiveObserveCount = countInt
	}
	if v, ok := cp["visited_urls"].(map[string]any); ok {
		if ctx.VisitedURLs == nil {
			ctx.VisitedURLs = make(map[string]bool)
		}
		for url := range v {
			ctx.VisitedURLs[url] = true
		}
	} else if v, ok := cp["visited_urls"].(map[string]bool); ok {
		if ctx.VisitedURLs == nil {
			ctx.VisitedURLs = make(map[string]bool)
		}
		for url, val := range v {
			ctx.VisitedURLs[url] = val
		}
	}
	if v, ok := cp["repetition_blocked_success"].(bool); ok {
		ctx.RepetitionBlockedSuccess = v
	}

	if ctx.Plan != nil && sess.Checkpoint.StepIndex > 0 {
		ctx.Plan.CurrentIdx = sess.Checkpoint.StepIndex
	}
}

func (e *AgentEngine) finalizeFlowState(runID, flowID string, result *ExecutionResult) {
	status := sharedtypes.FlowStatePassed
	switch result.Outcome {
	case OutcomePass:
		status = sharedtypes.FlowStatePassed
	case OutcomeSkip:
		if len(result.Errors) > 0 && result.Errors[0] == sharedtypes.ErrUpstreamFailed {
			status = sharedtypes.FlowStateSkippedUpstream
		} else if len(result.Errors) > 0 && strings.HasPrefix(result.Errors[0], cancelledErrPrefix) {
			status = sharedtypes.FlowStateSkippedUser
		} else {
			status = sharedtypes.FlowStateSkippedUser
		}
	case OutcomeFail:
		status = sharedtypes.FlowStateFailed
	default:
		status = sharedtypes.FlowStateFailed
	}

	errMsg := ""
	if len(result.Errors) > 0 {
		errMsg = result.Errors[0]
	}
	e.syncSessionStore(runID, flowID, "update_flow_final_state", func() error {
		return e.sessionStore.UpdateFlowState(runID, flowID, status, errMsg)
	})
}
