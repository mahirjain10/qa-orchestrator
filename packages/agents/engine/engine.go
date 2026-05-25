package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/agents/planner"
	"qa-orchestrator/packages/agents/recovery"
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
		GenerateNextStep(ctx context.Context, execCtx *agentstypes.ExecutionContext) (*agentstypes.PlanStep, []map[string]any, error)
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
			lc.SetOnStatusChange(func(runID string, old, new sharedtypes.RunState) {
				e.mu.RLock()
				store := e.sessionStore
				e.mu.RUnlock()
				if store != nil {
					_ = store.UpdateStatus(runID, new)
				}
			})
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
	lifecycleWg       sync.WaitGroup
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
	e.lifecycleWg.Wait() // wait for previous goroutine
	ctx, cancel := context.WithCancel(context.Background())
	cancelCh := e.lifecycle.CancelCh()
	e.lifecycleWg.Add(1)
	go func() {
		defer e.lifecycleWg.Done()
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
	e.lifecycleWg.Wait()
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
		// Check session store for pre-existing cancel/pause state before execution
		if e.sessionStore != nil {
			sess, err := e.sessionStore.Get(runID)
			if err == nil && sess != nil {
				if sess.Status == sharedtypes.RunStateCancelling || sess.Status == sharedtypes.RunStateCancelled {
					result.Outcome = OutcomeSkip
					result.Errors = append(result.Errors, shared.ErrCancelled.Error())
					e.finalizeFlowState(runID, flow.ID, result)
					return result
				}
			}
		}
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

	var stepsExecuted int
	var steeringSkipUsed bool
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
			steeringSkipUsed = true
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

		stepsExecuted++
		e.planner.Advance(plan)
	}

	if stepsExecuted == 0 && !steeringSkipUsed && len(plan.Steps) > 0 {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, "all steps were skipped — no steps executed")
	}

done:
	e.finalizeRunResult(runID, flow.ID, result, start, nil)
	return result
}
func (e *AgentEngine) RunFlowWithRetry(runID string, flow sharedtypes.Flow, maxRetries int) *ExecutionResult {
	if maxRetries <= 0 {
		return e.RunFlow(runID, flow)
	}

	var lastResult *ExecutionResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 && e.lifecycle != nil {
			e.lifecycle.SetStatus(sharedtypes.RunStatePending)
		}
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

	registryTools := tools.ListToolsWithDocs()
	if len(registryTools) == 0 {
		return getDefaultLLMTools()
	}

	result := make([]llm.ToolInfo, 0, len(registryTools))
	for _, t := range registryTools {
		if t.Name == "evaluate" || t.Name == "echo" {
			continue
		}
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
			Name:        "select_option",
			Description: "Select an option in a <select> element by value, label, or index",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the <select> element", Required: true},
				"value":    {Type: "string", Description: "Option value to select", Required: false},
				"label":    {Type: "string", Description: "Option label/text to select", Required: false},
				"index":    {Type: "number", Description: "Option index to select (0-based)", Required: false},
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
			Name:        "get_html",
			Description: "Get the inner HTML of an element",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
			},
		},
		{
			Name:        "finish",
			Description: "Signal that the goal has been achieved (or is unachievable) and no more steps are needed. Use this when the task is complete.",
			Parameters: map[string]llm.ParameterInfo{
				"status": {Type: "string", Description: "Set to 'success' if goal is achieved, or 'fail' if the goal is unachievable (e.g. elements not found).", Required: false},
			},
		},
		{
			Name:        "observe_ui",
			Description: "Inspect the current page and return a list of visible interactive elements with their selectors",
			Parameters:  map[string]llm.ParameterInfo{},
		},
	}
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
