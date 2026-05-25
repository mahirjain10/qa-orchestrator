package planner

import (
	"context"
	"fmt"

	"qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/llm"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

type LLMClient interface {
	Generate(ctx context.Context, prompt string) (string, error)
	GenerateWithSystem(ctx context.Context, system, user string) (string, error)
}

type Planner struct {
	llmClient LLMClient
	tools     []llm.ToolInfo
}

func NewPlanner() *Planner {
	return &Planner{}
}

func NewAutonomousPlanner(client LLMClient, tools []llm.ToolInfo) *Planner {
	return &Planner{
		llmClient: client,
		tools:     tools,
	}
}

func (p *Planner) CreatePlan(ctx *types.ExecutionContext) (*types.Plan, error) {
	if ctx == nil {
		return nil, fmt.Errorf("execution context is nil")
	}
	if ctx.Mode == sharedtypes.FlowModeAutonomous {
		return p.CreateAutonomousPlan(ctx)
	}

	steps := ctx.Steps
	if len(steps) == 0 {
		return nil, fmt.Errorf("guided mode requires at least one step, but got 0")
	}

	plan := &types.Plan{
		FlowID:       ctx.FlowID,
		Goal:         ctx.Goal,
		CurrentIdx:   0,
		Steps:        make([]types.PlanStep, len(steps)),
		IsAutonomous: false,
	}

	for i, step := range steps {
		plan.Steps[i] = types.PlanStep{
			StepIndex: i,
			StepID:    step.ID,
			Tool:      step.Tool,
			Params:    step.Params,
			Skip:      false,
			Reason:    "",
		}
	}

	return plan, nil
}

func (p *Planner) UpdatePlan(plan *types.Plan, stepIdx int, skip bool, reason string) {
	plan.Lock()
	defer plan.Unlock()
	if stepIdx >= 0 && stepIdx < len(plan.Steps) {
		plan.Steps[stepIdx].Skip = skip
		plan.Steps[stepIdx].Reason = reason
		plan.SetHistoryDirty()
	}
}

func (p *Planner) GetNextStep(plan *types.Plan) *types.PlanStep {
	plan.Lock()
	defer plan.Unlock()
	for plan.CurrentIdx < len(plan.Steps) {
		current := plan.Steps[plan.CurrentIdx]
		if !current.Skip {
			step := current
			return &step
		}
		plan.CurrentIdx++
	}
	return nil
}

func (p *Planner) Advance(plan *types.Plan) {
	plan.Lock()
	defer plan.Unlock()
	plan.CurrentIdx++
}

func (p *Planner) ShouldStop(plan *types.Plan) bool {
	plan.RLock()
	defer plan.RUnlock()
	if plan.IsAutonomous {
		return false
	}
	return len(plan.Steps) == 0 || plan.CurrentIdx >= len(plan.Steps)
}

func (p *Planner) GetProgress(plan *types.Plan) (completed, total int) {
	plan.RLock()
	defer plan.RUnlock()
	total = len(plan.Steps)
	completed = plan.CurrentIdx
	return
}

func (p *Planner) CreateAutonomousPlan(ctx *types.ExecutionContext) (*types.Plan, error) {
	if ctx == nil {
		return nil, fmt.Errorf("execution context is nil")
	}
	if p.llmClient == nil {
		return nil, fmt.Errorf("LLM client is nil: autonomous mode requires a configured LLM client")
	}
	plan := &types.Plan{
		FlowID:       ctx.FlowID,
		Goal:         ctx.Goal,
		CurrentIdx:   0,
		Steps:        make([]types.PlanStep, 0),
		IsAutonomous: true,
	}
	return plan, nil
}

func (p *Planner) GenerateNextStep(ctx context.Context, execCtx *types.ExecutionContext) (*types.PlanStep, []map[string]any, error) {
	if p.llmClient == nil {
		return nil, nil, fmt.Errorf("LLM client not configured for autonomous mode")
	}
	if execCtx == nil {
		return nil, nil, fmt.Errorf("execution context is nil")
	}
	if execCtx.Plan == nil {
		return nil, nil, fmt.Errorf("plan is nil")
	}

	goal := execCtx.Goal
	history := ""
	if execCtx.Plan != nil {
		history = execCtx.Plan.GetHistory()
	}

	observation := ""
	if len(execCtx.Observations) > 0 {
		lastObs := execCtx.Observations[len(execCtx.Observations)-1]
		if lastObs.LastStep != nil {
			if lastObs.LastStep.Tool == "observe_ui" {
				observation = formatObserveUIObservation(lastObs)
			} else {
				observation = fmt.Sprintf("Last step: %s, Tool: %s, Success: %v",
					lastObs.LastStep.StepID, lastObs.LastStep.Tool, lastObs.LastStep.Success)
				if lastObs.LastStep.Output != nil {
					outputStr := fmt.Sprintf("%v", lastObs.LastStep.Output)
					const maxOutputLen = 2000
					if len(outputStr) > maxOutputLen {
						outputStr = outputStr[:maxOutputLen] + "... [truncated]"
					}
					observation += fmt.Sprintf(", Output: %s", sanitizeDOM(outputStr))
				}
			}
		}
	}

	if failureMsg := scanForRecentFailure(execCtx.Observations); failureMsg != "" {
		if observation != "" {
			observation = failureMsg + "\n" + observation
		} else {
			observation = failureMsg
		}
	}

	systemPrompt := llm.BuildSystemPrompt(p.tools, execCtx.DependencyContext)
	userPrompt := llm.BuildUserPrompt(llm.PlannerPromptData{
		Goal:        goal,
		StartURL:    execCtx.StartURL,
		CurrentURL:  execCtx.CurrentURL,
		History:     history,
		Observation: observation,
		Tools:       p.tools,
	})

	if len(execCtx.SteeringInstructions) > 0 {
		steeringCtx := "\n\nIMPORTANT \u2014 Steering instructions from the operator (follow these when generating the next step):\n"
		for i, inst := range execCtx.SteeringInstructions {
			steeringCtx += fmt.Sprintf("  %d. %s\n", i+1, sanitizeDOM(inst))
		}
		userPrompt += steeringCtx
	}

	response, err := p.llmClient.GenerateWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM request failed: %w", err)
	}

	steps, err := llm.ParseStepsFromResponse(response)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	if len(steps) == 0 {
		return nil, nil, fmt.Errorf("LLM returned no steps")
	}

	firstStep, err := RawStepToPlanStep(steps[0], execCtx.Plan)
	if err != nil {
		return nil, nil, fmt.Errorf("raw step to plan step: %w", err)
	}

	var remaining []map[string]any
	if len(steps) > 1 {
		remaining = steps[1:]
	}

	return firstStep, remaining, nil
}

func RawStepToPlanStep(stepData map[string]any, plan *types.Plan) (*types.PlanStep, error) {
	tool, ok := stepData["tool"].(string)
	if !ok {
		return nil, fmt.Errorf("step missing 'tool' field")
	}

	params, ok := stepData["params"].(map[string]any)
	if !ok {
		params = make(map[string]any)
	}

	reason, _ := stepData["reason"].(string)

	stepID := fmt.Sprintf("auto-step-%d", len(plan.Steps)+1)
	planStep := types.PlanStep{
		StepIndex: len(plan.Steps),
		StepID:    stepID,
		Tool:      tool,
		Params:    params,
		Skip:      false,
		Reason:    reason,
	}

	return &planStep, nil
}

func (p *Planner) AddStepToPlan(plan *types.Plan, step *types.PlanStep) {
	plan.AddStep(*step)
}

func (p *Planner) IsAutonomousMode(ctx *types.ExecutionContext) bool {
	return ctx.Mode == sharedtypes.FlowModeAutonomous
}

func scanForRecentFailure(observations []types.Observation) string {
	for i := len(observations) - 1; i >= 0; i-- {
		obs := observations[i]
		if obs.Error != nil {
			tool := "?"
			if obs.LastStep != nil {
				tool = obs.LastStep.Tool
			}
			return fmt.Sprintf("\u26a0 RECENT FAILURE: tool=%s error=%v", tool, obs.Error)
		}
		if obs.LastStep != nil && !obs.LastStep.Success {
			errMsg := "unknown error"
			if obs.LastStep.Error != nil {
				errMsg = obs.LastStep.Error.Error()
			}
			return fmt.Sprintf("\u26a0 RECENT FAILURE: tool=%s error=%s", obs.LastStep.Tool, errMsg)
		}
	}
	return ""
}
