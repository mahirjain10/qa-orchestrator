package engine

import (
	"fmt"
	"strings"

	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/trace"
)

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

	const maxObservations = 50
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
	hasTool := false
	if reg, ok := e.toolRegistry.(interface{ HasTool(string) bool }); ok {
		hasTool = reg.HasTool("observe_ui")
	} else {
		hasTool = true
	}
	if !hasTool {
		return
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

func (e *AgentEngine) handleFailure(ctx *agentstypes.ExecutionContext, stepResult *agentstypes.StepResult, result *ExecutionResult) *agentstypes.RecoveryDecision {
	decision := e.recovery.Decide(stepResult.Error, stepResult, ctx)

	if e.recovery.ShouldEscalate(decision, result.Retries) {
		decision.Action = agentstypes.RecoveryActionFail
		decision.Reason = fmt.Sprintf("max retries (%d) exceeded", result.Retries)
	}

	return decision
}

func findStepByID(steps []sharedtypes.Step, stepID string) (sharedtypes.Step, bool) {
	for i := range steps {
		if steps[i].ID == stepID {
			return steps[i], true
		}
	}
	return sharedtypes.Step{}, false
}
