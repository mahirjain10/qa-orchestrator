package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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

func (p *Planner) ShouldStop(plan *types.Plan) bool {
	if plan.IsAutonomous {
		// Autonomous mode termination is handled by the `finish` tool or maxSteps check
		// in the engine loop, not by ShouldStop.
		return false
	}
	return p.GetNextStep(plan) == nil
}

func (p *Planner) GetProgress(plan *types.Plan) (completed, total int) {
	total = len(plan.Steps)
	completed = plan.CurrentIdx
	return
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

func sanitizeDOM(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func formatObserveUIObservation(obs types.Observation) string {
	result := "Page observation after last step:\n"
	if obs.LastStep != nil {
		result += fmt.Sprintf("  Last step: %s, Tool: %s, Success: %v\n",
			obs.LastStep.StepID, obs.LastStep.Tool, obs.LastStep.Success)
	}
	if obs.State != nil {
		var parsed map[string]any
		if data, ok := obs.State["data"].(map[string]any); ok {
			parsed = data
		} else if dataStr, ok := obs.State["data"].(string); ok {
			if err := json.Unmarshal([]byte(dataStr), &parsed); err != nil {
				result += fmt.Sprintf("  Raw data: %s\n", sanitizeDOM(dataStr))
			}
		}
		if parsed != nil {
			if warning, ok := parsed["warning"].(string); ok && warning != "" {
				result += fmt.Sprintf("  ⚠ %s\n", sanitizeDOM(warning))
			}
			if pageState, ok := parsed["page_state"].(string); ok {
				result += fmt.Sprintf("  Page state: %s\n", sanitizeDOM(pageState))
			}
			if interactive, ok := parsed["interactive"].([]any); ok {
				result += fmt.Sprintf("  Interactive elements found (%d):\n", len(interactive))
				for i, elem := range interactive {
					if elemMap, ok := elem.(map[string]any); ok {
						tag := fmt.Sprintf("%v", elemMap["tag"])
						selector := fmt.Sprintf("%v", elemMap["selector"])
						text := fmt.Sprintf("%v", elemMap["text"])
						var classStr string
					if c, ok := elemMap["class"].(string); ok {
						classStr = c
					}
					result += fmt.Sprintf("    %d. <%s> selector=\"%s\" text=\"%s\" class=\"%s\"\n", i+1, sanitizeDOM(tag), sanitizeDOM(selector), sanitizeDOM(text), sanitizeDOM(classStr))
					}
				}
			}
		}
	}
	result += "Use the selectors above when generating your next step. Do not invent selectors."
	return result
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
			if lastObs.LastStep.Tool == "observe_ui" {
				observation = formatObserveUIObservation(lastObs)
			} else {
				observation = fmt.Sprintf("Last step: %s, Tool: %s, Success: %v",
					lastObs.LastStep.StepID, lastObs.LastStep.Tool, lastObs.LastStep.Success)
				if lastObs.LastStep.Output != nil {
					observation += fmt.Sprintf(", Output: %v", lastObs.LastStep.Output)
				}
			}
		}
	}

	// Prepend failure context if any observation has an error/failure.
	// This is done AFTER building the observation string so the failure
	// warning appears prominently, separate from the observation details.
	// We intentionally do NOT also append lastObs.Error to observation
	// because scanForRecentFailure already includes it and duplicates waste tokens.
	if failureMsg := scanForRecentFailure(execCtx.Observations); failureMsg != "" {
		if observation != "" {
			observation = failureMsg + "\n" + observation
		} else {
			observation = failureMsg
		}
	}

	systemPrompt := llm.BuildSystemPrompt(p.tools, execCtx.DependencyContext)
	userPrompt := llm.BuildUserPrompt(llm.PlannerPromptData{
		Goal:              goal,
		StartURL:          execCtx.StartURL,
		CurrentURL:        execCtx.CurrentURL,
		History:           history,
		Observation:       observation,
		Tools:             p.tools,
		DependencyContext: execCtx.DependencyContext,
	})

	if len(execCtx.SteeringInstructions) > 0 {
		steeringCtx := "\n\nIMPORTANT — Steering instructions from the operator (follow these when generating the next step):\n"
		for i, inst := range execCtx.SteeringInstructions {
			steeringCtx += fmt.Sprintf("  %d. %s\n", i+1, inst)
		}
		userPrompt += steeringCtx
	}

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

	stepID := fmt.Sprintf("auto-step-%d", len(execCtx.Plan.Steps)+1)
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

// scanForRecentFailure scans observations for the most recent error and returns
// a formatted warning string, or empty string if no failure found.
// It checks both the observation-level Error and the step-level !Success.
func scanForRecentFailure(observations []types.Observation) string {
	for i := len(observations) - 1; i >= 0; i-- {
		obs := observations[i]
		// Observation-level error (e.g., tool execution failure)
		if obs.Error != nil {
			tool := "?"
			if obs.LastStep != nil {
				tool = obs.LastStep.Tool
			}
			return fmt.Sprintf("⚠ RECENT FAILURE: tool=%s error=%v", tool, obs.Error)
		}
		// Step-level failure (success=false)
		if obs.LastStep != nil && !obs.LastStep.Success {
			errMsg := "unknown error"
			if obs.LastStep.Error != nil {
				errMsg = obs.LastStep.Error.Error()
			}
			return fmt.Sprintf("⚠ RECENT FAILURE: tool=%s error=%s", obs.LastStep.Tool, errMsg)
		}
	}
	return ""
}
