package engine

import (
	"context"
	"errors"
	"fmt"
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
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
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

type AgentEngine struct {
	planner       *planner.Planner
	executor      *executor.Executor
	validator     *validator.Validator
	recovery      *recovery.Recovery
	traceStore    *trace.TraceStore
	artifactStore *artifact.ArtifactStore
	sessionStore  *session.SessionStore
	lifecycle     *runtime.LifecycleController
	llmClient     planner.LLMClient
	toolRegistry  executor.ToolRegistry
	browserTools  interface {
		ListToolsWithDocs() []browsertools.ToolInfo
	}
}

func NewAgentEngine() *AgentEngine {
	return &AgentEngine{
		planner:   planner.NewPlanner(),
		executor:  executor.NewExecutor(executor.NewMockToolRegistry()),
		validator: validator.NewValidator(),
		recovery:  recovery.NewRecovery(nil),
	}
}

func NewAgentEngineWithRegistry(registry executor.ToolRegistry) *AgentEngine {
	return &AgentEngine{
		planner:      planner.NewPlanner(),
		executor:     executor.NewExecutor(registry),
		validator:    validator.NewValidator(),
		recovery:     recovery.NewRecovery(nil),
		toolRegistry: registry,
	}
}

func NewAgentEngineWithStores(registry executor.ToolRegistry, sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) *AgentEngine {
	return &AgentEngine{
		planner:       planner.NewPlanner(),
		executor:      executor.NewExecutor(registry),
		validator:     validator.NewValidator(),
		recovery:      recovery.NewRecovery(nil),
		sessionStore:  sessionStore,
		traceStore:    traceStore,
		artifactStore: artifactStore,
		lifecycle:     runtime.NewLifecycleController(""),
		toolRegistry:  registry,
	}
}

func NewAgentEngineWithLLM(registry executor.ToolRegistry, sessionStore *session.SessionStore, llmClient planner.LLMClient, browserTools interface {
	ListToolsWithDocs() []browsertools.ToolInfo
}) *AgentEngine {
	return &AgentEngine{
		planner:      planner.NewPlanner(),
		executor:     executor.NewExecutor(registry),
		validator:    validator.NewValidator(),
		recovery:     recovery.NewRecovery(nil),
		sessionStore: sessionStore,
		llmClient:    llmClient,
		toolRegistry: registry,
		browserTools: browserTools,
	}
}

func (e *AgentEngine) SetLLMClient(client planner.LLMClient) {
	e.llmClient = client
}

func (e *AgentEngine) SetBrowserTools(tools interface {
	ListToolsWithDocs() []browsertools.ToolInfo
}) {
	e.browserTools = tools
}

func (e *AgentEngine) RunFlow(runID string, flow sharedtypes.Flow) *ExecutionResult {
	start := time.Now()
	result := &ExecutionResult{
		FlowID:       flow.ID,
		Outcome:      OutcomePass,
		Steps:        []*agentstypes.StepResult{},
		Errors:       []string{},
		IsAutonomous: flow.Mode == sharedtypes.FlowModeAutonomous,
	}

	ctx := &agentstypes.ExecutionContext{
		RunID:  runID,
		FlowID: flow.ID,
		Goal:   flow.Goal,
		Mode:   flow.Mode,
		Steps:  flow.Steps,
	}

	if e.sessionStore != nil {
		e.syncSessionStore(runID, flow.ID, "update_flow_running", func() error {
			return e.sessionStore.UpdateFlowState(runID, flow.ID, sharedtypes.FlowStateRunning, "")
		})
		sess, err := e.sessionStore.Get(runID)
		if sess != nil {
			sess.CurrentFlowID = flow.ID
			e.syncSessionStore(runID, flow.ID, "save_current_flow", func() error {
				return e.sessionStore.Save(sess)
			})
		} else if err != nil {
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "session_sync", "get_failed", err.Error())
		}
	}

	if e.lifecycle != nil {
		e.lifecycle.SetStatus(sharedtypes.RunStateRunning)
	}

	trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateRunning, map[string]any{
		"goal": flow.Goal,
		"mode": string(flow.Mode),
	})

	if e.lifecycle != nil && e.lifecycle.GetStatus() == sharedtypes.RunStateCancelling {
		result.Outcome = OutcomeSkip
		result.Errors = append(result.Errors, "cancelled before execution")
		e.finalizeFlowState(runID, flow.ID, result)
		return result
	}

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
		if e.lifecycle != nil {
			select {
			case <-e.lifecycle.CancelCh():
				result.Outcome = OutcomeSkip
				result.Errors = append(result.Errors, "cancelled during execution")
				goto done
			case <-e.lifecycle.PauseCh():
				e.setCurrentAgent(ctx.RunID, "idle (paused)")
				e.lifecycle.AcknowledgePause()
				<-e.lifecycle.ResumeCh()
				e.lifecycle.AcknowledgeResume()
			case evt := <-e.lifecycle.SteerCh():
				e.handleSteeringEvent(evt, ctx, result, plan)
			default:
			}
		}

		planStep := e.planner.GetNextStep(plan)
		if planStep == nil {
			break
		}

		e.saveCheckpoint(runID, ctx, planStep)
		e.setCurrentAgent(ctx.RunID, "executor")
		stepResult := e.executeAndValidate(ctx, planStep)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Success {
			e.setCurrentAgent(ctx.RunID, "recovery")
			trace.EmitRecoveryAction(e.traceStore, runID, flow.ID, nil, stepResult)
			decision := e.handleFailure(ctx, stepResult, result)
			trace.EmitRecoveryAction(e.traceStore, runID, flow.ID, decision, stepResult)

			switch decision.Action {
			case agentstypes.RecoveryActionRetry:
				result.Retries++
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "retry", decision.Reason)
				continue
			case agentstypes.RecoveryActionReplan:
				// Guided plans are static YAML-defined steps; replanning just recreates the same plan.
				// Downgrade to retry so locator errors can recover without resetting progress.
				result.Retries++
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "retry_instead_of_replan", decision.Reason)
				continue
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
	result.DurationMs = time.Since(start).Milliseconds()
	if result.Outcome == OutcomePass && len(result.Errors) > 0 {
		result.Outcome = OutcomeFail
	}

	if e.sessionStore != nil {
		e.finalizeFlowState(runID, flow.ID, result)
		e.setCurrentAgent(runID, "")
	}

	if result.Outcome == OutcomePass {
		trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateCompleted, map[string]any{"duration_ms": result.DurationMs})
	} else {
		trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateFailed, map[string]any{"errors": result.Errors})
	}

	return result
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

	e.setCurrentAgent(runID, "planner (init)")
	plan, err := autonomousPlanner.CreatePlan(ctx)
	if err != nil {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create autonomous plan: %v", err))
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "failed", err.Error())
		return result
	}
	ctx.Plan = plan

	trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "autonomous_plan_created", "starting iterative step generation")

	maxAutonomousSteps := 20
	stepCount := 0

	for stepCount < maxAutonomousSteps {
		if e.lifecycle != nil {
			select {
			case <-e.lifecycle.CancelCh():
				result.Outcome = OutcomeSkip
				result.Errors = append(result.Errors, "cancelled during autonomous execution")
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "engine", "cancelled", "autonomous flow cancelled")
				goto done
			case <-e.lifecycle.PauseCh():
				e.setCurrentAgent(runID, "idle (paused)")
				e.lifecycle.AcknowledgePause()
				<-e.lifecycle.ResumeCh()
				e.lifecycle.AcknowledgeResume()
			case evt := <-e.lifecycle.SteerCh():
				e.handleSteeringEvent(evt, ctx, result, plan)
			default:
			}
		}

		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "generating_step", fmt.Sprintf("step %d", stepCount+1))
		e.setCurrentAgent(runID, fmt.Sprintf("planner (step %d)", stepCount+1))

		llmCtx, llmCancel := e.autonomousLLMContext(context.Background())
		planStep, err := autonomousPlanner.GenerateNextStep(llmCtx, ctx)
		llmCancel()
		if err != nil {
			if errors.Is(err, context.Canceled) {
				result.Outcome = OutcomeSkip
				result.Errors = append(result.Errors, "cancelled during step generation")
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generation_cancelled", "context cancelled")
				break
			}
			result.Outcome = OutcomeFail
			result.Errors = append(result.Errors, fmt.Sprintf("failed to generate step: %v", err))
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generation_failed", err.Error())
			break
		}

		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "step_generated",
			fmt.Sprintf("tool=%s params=%v reason=%s", planStep.Tool, planStep.Params, planStep.Reason))

		if planStep.Tool == "finish" {
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "finish_signal", "LLM signaled completion")
			break
		}

		autonomousPlanner.AddStepToPlan(plan, planStep)
		ctx.Plan = plan

		e.saveCheckpoint(runID, ctx, planStep)
		e.setCurrentAgent(runID, "executor")
		stepResult := e.executeAndValidate(ctx, planStep)
		result.Steps = append(result.Steps, stepResult)

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
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "replan", decision.Reason)
				continue
			case agentstypes.RecoveryActionSkip:
				autonomousPlanner.UpdatePlan(plan, planStep.StepIndex, true, decision.Reason)
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "skip", decision.Reason)
			case agentstypes.RecoveryActionFail:
				result.Outcome = OutcomeFail
				result.Errors = append(result.Errors, decision.Reason)
				trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "recovery", "fail", decision.Reason)
				goto done
			}
		}

		stepCount++
		autonomousPlanner.Advance(plan)

		if autonomousPlanner.ShouldStop(plan) {
			trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "plan_completed", fmt.Sprintf("generated %d steps", stepCount))
			break
		}
	}

	if stepCount >= maxAutonomousSteps {
		trace.EmitAgentDecision(e.traceStore, runID, flow.ID, "planner", "max_steps_reached", fmt.Sprintf("reached max %d steps", maxAutonomousSteps))
	}

done:
	result.DurationMs = time.Since(start).Milliseconds()
	if result.Outcome == OutcomePass && len(result.Errors) > 0 {
		result.Outcome = OutcomeFail
	}

	if e.sessionStore != nil {
		e.finalizeFlowState(runID, flow.ID, result)
		e.setCurrentAgent(runID, "")
	}

	if result.Outcome == OutcomePass {
		trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateCompleted, map[string]any{
			"duration_ms":      result.DurationMs,
			"autonomous_steps": stepCount,
		})
	} else {
		trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateFailed, map[string]any{"errors": result.Errors})
	}

	return result
}

func (e *AgentEngine) setCurrentAgent(runID, agent string) {
	if e.sessionStore == nil {
		return
	}
	sess, err := e.sessionStore.Get(runID)
	if err == nil && sess != nil {
		sess.CurrentAgent = agent
		e.syncSessionStore(runID, sess.CurrentFlowID, "save_current_agent", func() error {
			return e.sessionStore.Save(sess)
		})
		return
	}
	if err != nil {
		trace.EmitAgentDecision(e.traceStore, runID, "", "session_sync", "get_failed", err.Error())
	}
}

func (e *AgentEngine) executeAndValidate(ctx *agentstypes.ExecutionContext, planStep *agentstypes.PlanStep) *agentstypes.StepResult {
	stepResult := e.executor.ExecuteStep(planStep)
	obs := e.validator.CreateObservation(stepResult)
	ctx.Observations = append(ctx.Observations, *obs)

	trace.EmitStepExecution(e.traceStore, ctx.RunID, ctx.FlowID, stepResult)

	step := findStepByID(ctx.Steps, planStep.StepID)
	if step != nil && len(step.Assertions) > 0 {
		validation := e.validator.ValidateStep(step, stepResult)
		if !validation.Passed {
			stepResult.Success = false
			stepResult.Error = fmt.Errorf("%v", validation.Errors)
		}
	}

	return stepResult
}

func findStepByID(steps []sharedtypes.Step, stepID string) *sharedtypes.Step {
	for _, step := range steps {
		if step.ID == stepID {
			return &step
		}
	}
	return nil
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
	var lastResult *ExecutionResult

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result := e.RunFlow(runID, flow)
		lastResult = result

		if result.Outcome == OutcomePass {
			return result
		}

		if attempt < maxRetries {
			time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)
		}
	}

	return lastResult
}

func (e *AgentEngine) GetPlanner() *planner.Planner {
	return e.planner
}

func (e *AgentEngine) GetExecutor() *executor.Executor {
	return e.executor
}

func (e *AgentEngine) GetValidator() *validator.Validator {
	return e.validator
}

func (e *AgentEngine) GetRecovery() *recovery.Recovery {
	return e.recovery
}

func (e *AgentEngine) RegisterTool(name string, fn func(params map[string]any) (any, error)) {
	registry := e.executor.GetRegistry()
	if mockRegistry, ok := registry.(*executor.MockToolRegistry); ok {
		mockRegistry.Register(name, fn)
	}
}

func (e *AgentEngine) SetTraceStore(store *trace.TraceStore) {
	e.traceStore = store
}

func (e *AgentEngine) SetArtifactStore(store *artifact.ArtifactStore) {
	e.artifactStore = store
}

func (e *AgentEngine) saveCheckpoint(runID string, ctx *agentstypes.ExecutionContext, planStep *agentstypes.PlanStep) {
	if ctx.Plan == nil {
		return
	}
	payload := map[string]any{
		"current_step": planStep.StepID,
		"step_index":   planStep.StepIndex,
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
		return ""
	}
	trace.EmitArtifactEvent(e.traceStore, runID, flowID, string(artifactType), artifact.Path, metadata)
	return artifact.ArtifactID
}

func (e *AgentEngine) SetLifecycleController(lc *runtime.LifecycleController) {
	e.lifecycle = lc
}

func (e *AgentEngine) handleSteeringEvent(evt *sharedtypes.SteeringEvent, ctx *agentstypes.ExecutionContext, result *ExecutionResult, plan *agentstypes.Plan) {
	trace.EmitAgentDecision(e.traceStore, ctx.RunID, ctx.FlowID, "steering", string(evt.Command), evt.Reason)

	switch evt.Command {
	case sharedtypes.SteerSkip:
		if evt.FlowID == "" || evt.FlowID == ctx.FlowID {
			result.Outcome = OutcomeSkip
			result.Errors = append(result.Errors, fmt.Sprintf("skipped by steering: %s", evt.Reason))
			return
		}
	case sharedtypes.SteerRetry:
		if evt.FlowID == "" || evt.FlowID == ctx.FlowID {
			result.Retries++
		}
	case sharedtypes.SteerHumanReview:
		if e.lifecycle != nil {
			e.lifecycle.SetWaitingForInput()
		}
		trace.EmitAgentDecision(e.traceStore, ctx.RunID, ctx.FlowID, "steering", "waiting_for_input", "human review requested")
	case sharedtypes.SteerApprove, sharedtypes.SteerContinue:
		if e.lifecycle != nil && e.lifecycle.IsWaitingForInput() {
			e.lifecycle.AcknowledgeInput()
		}
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
	ctx, cancel := context.WithCancel(parent)
	if e.lifecycle == nil {
		return ctx, cancel
	}

	go func() {
		select {
		case <-e.lifecycle.CancelCh():
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
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
			Name:        "get_html",
			Description: "Get the inner HTML of an element",
			Parameters: map[string]llm.ParameterInfo{
				"selector": {Type: "string", Description: "CSS selector for the element", Required: true},
			},
		},
		{
			Name:        "evaluate",
			Description: "Evaluate a JavaScript expression in the browser context",
			Parameters: map[string]llm.ParameterInfo{
				"expression": {Type: "string", Description: "JavaScript expression to evaluate", Required: true},
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
		status = sharedtypes.FlowStateSkippedUser
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
