package engine

import (
	"fmt"
	"time"

	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/agents/planner"
	"qa-orchestrator/packages/agents/recovery"
	agentstypes "qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/agents/validator"
	"qa-orchestrator/packages/runtime"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/trace"
)

type FlowOutcome string

const (
	OutcomePass FlowOutcome = "PASSED"
	OutcomeFail FlowOutcome = "FAILED"
	OutcomeSkip FlowOutcome = "SKIPPED"
)

type ExecutionResult struct {
	FlowID      string
	Outcome     FlowOutcome
	Steps       []*agentstypes.StepResult
	Errors      []string
	DurationMs  int64
	Retries     int
	ArtifactIDs []string
}

type AgentEngine struct {
	planner      *planner.Planner
	executor     *executor.Executor
	validator    *validator.Validator
	recovery     *recovery.Recovery
	traceStore   *trace.TraceStore
	artifactStore *artifact.ArtifactStore
	lifecycle    *runtime.LifecycleController
}

func NewAgentEngine() *AgentEngine {
	return &AgentEngine{
		planner:     planner.NewPlanner(),
		executor:    executor.NewExecutor(executor.NewMockToolRegistry()),
		validator:   validator.NewValidator(),
		recovery:    recovery.NewRecovery(nil),
	}
}

func NewAgentEngineWithRegistry(registry executor.ToolRegistry) *AgentEngine {
	return &AgentEngine{
		planner:     planner.NewPlanner(),
		executor:    executor.NewExecutor(registry),
		validator:   validator.NewValidator(),
		recovery:    recovery.NewRecovery(nil),
	}
}

func NewAgentEngineWithStores(registry executor.ToolRegistry, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) *AgentEngine {
	return &AgentEngine{
		planner:       planner.NewPlanner(),
		executor:      executor.NewExecutor(registry),
		validator:     validator.NewValidator(),
		recovery:      recovery.NewRecovery(nil),
		traceStore:    traceStore,
		artifactStore: artifactStore,
		lifecycle:     runtime.NewLifecycleController(""),
	}
}

func (e *AgentEngine) RunFlow(runID string, flow sharedtypes.Flow) *ExecutionResult {
	start := time.Now()
	result := &ExecutionResult{
		FlowID:  flow.ID,
		Outcome: OutcomePass,
		Steps:   []*agentstypes.StepResult{},
		Errors:  []string{},
	}

	ctx := &agentstypes.ExecutionContext{
		RunID:  runID,
		FlowID: flow.ID,
		Goal:   flow.Goal,
		Steps:  flow.Steps,
	}

	if e.lifecycle != nil {
		e.lifecycle.SetStatus(sharedtypes.RunStateRunning)
	}

	trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateRunning, map[string]any{"goal": flow.Goal})

	if e.lifecycle != nil && e.lifecycle.GetStatus() == sharedtypes.RunStateCancelling {
		result.Outcome = OutcomeSkip
		result.Errors = append(result.Errors, "cancelled before execution")
		return result
	}

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
				return result
			case <-e.lifecycle.PauseCh():
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
		stepResult := e.executeAndValidate(ctx, planStep)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Success {
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
				newPlan, replanErr := e.planner.CreatePlan(ctx)
				if replanErr != nil {
					result.Outcome = OutcomeFail
					result.Errors = append(result.Errors, fmt.Sprintf("failed to replan: %v", replanErr))
					goto done
				}
				plan = newPlan
				ctx.Plan = plan
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

	if result.Outcome == OutcomePass {
		trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateCompleted, map[string]any{"duration_ms": result.DurationMs})
	} else {
		trace.EmitLifecycleEvent(e.traceStore, runID, flow.ID, sharedtypes.RunStateFailed, map[string]any{"errors": result.Errors})
	}

	return result
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
	trace.EmitCheckpoint(e.traceStore, runID, &sharedtypes.Checkpoint{
		FlowID:    ctx.FlowID,
		StepID:    planStep.StepID,
		StepIndex: planStep.StepIndex,
		Payload:   payload,
	})
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