package planner

import (
	"qa-orchestrator/packages/agents/types"
)

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (p *Planner) CreatePlan(ctx *types.ExecutionContext) (*types.Plan, error) {
	steps := ctx.Steps

	plan := &types.Plan{
		FlowID:     ctx.FlowID,
		Goal:       ctx.Goal,
		CurrentIdx: 0,
		Steps:      make([]types.PlanStep, len(steps)),
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
