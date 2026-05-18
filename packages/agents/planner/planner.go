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
	if ctx.Mode == sharedtypes.FlowModeAutonomous {
		return p.CreateAutonomousPlan(ctx)
	}

	steps := ctx.Steps

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
	if stepIdx >= 0 && stepIdx < len(plan.Steps) {
		plan.Steps[stepIdx].Skip = skip
		plan.Steps[stepIdx].Reason = reason
		plan.InvalidateHistoryCache()
	}
}

func (p *Planner) GetNextStep(plan *types.Plan) *types.PlanStep {
	for plan.CurrentIdx < len(plan.Steps) {
		current := &plan.Steps[plan.CurrentIdx]
		if !current.Skip {
			return current
		}
		plan.CurrentIdx++
	}
	return nil
}

func (p *Planner) Advance(plan *types.Plan) {
	plan.CurrentIdx++
}

func (p *Planner) Observe(ctx *types.ExecutionContext) *types.Observation {
	obs := &types.Observation{
		State: make(map[string]any),
	}

	if ctx.Plan != nil && ctx.Plan.CurrentIdx > 0 {
		lastIdx := ctx.Plan.CurrentIdx - 1
		if lastIdx >= 0 && lastIdx < len(ctx.Steps) {
			lastStep := ctx.Steps[lastIdx]
			obs.State["last_step_id"] = lastStep.ID
			obs.State["last_step_tool"] = lastStep.Tool
		}
	}

	return obs
}

func (p *Planner) ShouldStop(plan *types.Plan) bool {
	return p.GetNextStep(plan) == nil
}

func (p *Planner) GetProgress(plan *types.Plan) (completed, total int) {
	total = len(plan.Steps)
	completed = plan.CurrentIdx
	return
}

func (p *Planner) GetPendingSteps(plan *types.Plan) int {
	pending := 0
	for _, step := range plan.Steps {
		if !step.Skip {
			pending++
		}
	}
	return pending
}

func PlanFromFlow(flow types.Flow) []types.PlanStep {
	var steps []types.PlanStep
	for i, s := range flow.Steps {
		steps = append(steps, types.PlanStep{
			StepIndex: i,
			StepID:    s.ID,
			Tool:      s.Tool,
			Params:    s.Params,
			Skip:      false,
		})
	}
	return steps
}

func (p *Planner) CreateAutonomousPlan(ctx *types.ExecutionContext) (*types.Plan, error) {
	plan := &types.Plan{
		FlowID:       ctx.FlowID,
		Goal:         ctx.Goal,
		CurrentIdx:   0,
		Steps:        make([]types.PlanStep, 0),
		IsAutonomous: true,
	}
	return plan, nil
}

func (p *Planner) GenerateNextStep(ctx context.Context, execCtx *types.ExecutionContext) (*types.PlanStep, error) {
	if p.llmClient == nil {
		return nil, fmt.Errorf("LLM client not configured for autonomous mode")
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
			observation = fmt.Sprintf("Last step: %s, Tool: %s, Success: %v",
				lastObs.LastStep.StepID, lastObs.LastStep.Tool, lastObs.LastStep.Success)
			if lastObs.LastStep.Output != nil {
				observation += fmt.Sprintf(", Output: %v", lastObs.LastStep.Output)
			}
		}
		if lastObs.Error != nil {
			observation += fmt.Sprintf(", Error: %v", lastObs.Error)
		}
	}

	systemPrompt := llm.BuildSystemPrompt(p.tools)
	userPrompt := llm.BuildUserPrompt(llm.PlannerPromptData{
		Goal:        goal,
		History:     history,
		Observation: observation,
		Tools:       p.tools,
	})

	response, err := p.llmClient.GenerateWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	steps, err := llm.ParseStepsFromResponse(response)
	if err != nil {
		return nil, fmt.Errorf("parsing LLM response: %w", err)
	}

	if len(steps) == 0 {
		return nil, fmt.Errorf("LLM returned no steps")
	}

	stepData := steps[0]
	tool, ok := stepData["tool"].(string)
	if !ok {
		return nil, fmt.Errorf("step missing 'tool' field")
	}

	params, ok := stepData["params"].(map[string]any)
	if !ok {
		params = make(map[string]any)
	}

	reason, _ := stepData["reason"].(string)

	stepID := fmt.Sprintf("auto-step-%d", execCtx.Plan.CurrentIdx+1)
	planStep := types.PlanStep{
		StepIndex: execCtx.Plan.CurrentIdx,
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
