package engine

import (
	"fmt"
	"time"

	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/agents/planner"
	"qa-orchestrator/packages/agents/recovery"
	"qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/agents/validator"
)

type FlowOutcome string

const (
	OutcomePass FlowOutcome = "PASSED"
	OutcomeFail FlowOutcome = "FAILED"
	OutcomeSkip FlowOutcome = "SKIPPED"
)

type ExecutionResult struct {
	FlowID     string
	Outcome    FlowOutcome
	Steps      []*types.StepResult
	Errors     []string
	DurationMs int64
	Retries    int
}

type AgentEngine struct {
	planner   *planner.Planner
	executor  *executor.Executor
	validator *validator.Validator
	recovery  *recovery.Recovery
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
		planner:   planner.NewPlanner(),
		executor:  executor.NewExecutor(registry),
		validator: validator.NewValidator(),
		recovery:  recovery.NewRecovery(nil),
	}
}

func (e *AgentEngine) RunFlow(runID string, flow types.Flow) *ExecutionResult {
	start := time.Now()
	result := &ExecutionResult{
		FlowID:  flow.ID,
		Outcome: OutcomePass,
		Steps:   []*types.StepResult{},
		Errors:  []string{},
	}

	ctx := &types.ExecutionContext{
		RunID:  runID,
		FlowID: flow.ID,
		Goal:   flow.Goal,
		Steps:  flow.Steps,
	}

	plan, err := e.planner.CreatePlan(ctx)
	if err != nil {
		result.Outcome = OutcomeFail
		result.Errors = append(result.Errors, fmt.Sprintf("failed to create plan: %v", err))
		return result
	}
	ctx.Plan = plan

	for !e.planner.ShouldStop(plan) {
		planStep := e.planner.GetNextStep(plan)
		if planStep == nil {
			break
		}

		stepResult := e.executeAndValidate(ctx, planStep)
		result.Steps = append(result.Steps, stepResult)

		if !stepResult.Success {
			decision := e.handleFailure(ctx, stepResult, result)
			switch decision.Action {
			case types.RecoveryActionRetry:
				result.Retries++
				continue
			case types.RecoveryActionReplan:
				newPlan, replanErr := e.planner.CreatePlan(ctx)
				if replanErr != nil {
					result.Outcome = OutcomeFail
					result.Errors = append(result.Errors, fmt.Sprintf("failed to replan: %v", replanErr))
					goto done
				}
				plan = newPlan
				ctx.Plan = plan
				continue
			case types.RecoveryActionSkip:
				e.planner.UpdatePlan(plan, planStep.StepIndex, true, decision.Reason)
				e.planner.Advance(plan)
				continue
			case types.RecoveryActionFail:
				result.Outcome = OutcomeFail
				result.Errors = append(result.Errors, decision.Reason)
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

	return result
}

func (e *AgentEngine) executeAndValidate(ctx *types.ExecutionContext, planStep *types.PlanStep) *types.StepResult {
	stepResult := e.executor.ExecuteStep(planStep)
	obs := e.validator.CreateObservation(stepResult)
	ctx.Observations = append(ctx.Observations, *obs)

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

func findStepByID(steps []types.Step, stepID string) *types.Step {
	for _, step := range steps {
		if step.ID == stepID {
			return &step
		}
	}
	return nil
}

func (e *AgentEngine) handleFailure(ctx *types.ExecutionContext, stepResult *types.StepResult, result *ExecutionResult) *types.RecoveryDecision {
	decision := e.recovery.Decide(stepResult.Error, stepResult, ctx)

	if e.recovery.ShouldEscalate(decision, result.Retries) {
		decision.Action = types.RecoveryActionFail
		decision.Reason = fmt.Sprintf("max retries (%d) exceeded", result.Retries)
	}

	return decision
}

func (e *AgentEngine) RunFlowWithRetry(runID string, flow types.Flow, maxRetries int) *ExecutionResult {
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
