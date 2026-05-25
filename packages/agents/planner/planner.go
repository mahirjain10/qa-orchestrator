package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
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

func sanitizeDOM(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "[", "&#91;")
	s = strings.ReplaceAll(s, "]", "&#93;")
	return s
}

// renderAttr appends a single attr key="value" to the line string if the value is non-empty and not false
func renderAttr(line *string, m map[string]any, key string, used *map[string]bool) {
	v, ok := m[key]
	if !ok {
		return
	}
	vs := fmt.Sprintf("%v", v)
	if vs == "" || vs == "<nil>" || vs == "false" {
		return
	}
	(*used)[key] = true
	*line += fmt.Sprintf(` %s="%s"`, key, sanitizeDOM(vs))
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
				const maxRawDataLen = 2000
				if len(dataStr) > maxRawDataLen {
					dataStr = dataStr[:maxRawDataLen] + "... [truncated]"
				}
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
				totalElems := len(interactive)
				maxElems := 40
				var truncated bool
				if len(interactive) > maxElems {
					interactive = interactive[:maxElems]
					truncated = true
				}
				if truncated {
					result += fmt.Sprintf("  Interactive elements found (%d total, showing %d):\n", totalElems, maxElems)
				} else {
					result += fmt.Sprintf("  Interactive elements found (%d total):\n", totalElems)
				}
				for i, elem := range interactive {
					if elemMap, ok := elem.(map[string]any); ok {
						line := fmt.Sprintf("    %d. <%s>", i+1, sanitizeDOM(fmt.Sprintf("%v", elemMap["tag"])))

						// Priority fields rendered in semantically meaningful order
						priority := []string{"id", "name", "type", "role", "placeholder", "href",
							"value", "checked", "disabled", "selected", "aria-label"}

						used := map[string]bool{"tag": true, "selector": true}

						for _, k := range priority {
							renderAttr(&line, elemMap, k, &used)
						}

						// Remaining attrs alphabetically — schema-independent by design
						remaining := make([]string, 0)
						for k := range elemMap {
							if !used[k] {
								remaining = append(remaining, k)
							}
						}
						sort.Strings(remaining)
						for _, k := range remaining {
							renderAttr(&line, elemMap, k, &used)
						}

						// Selector rendered last — it's the longest and most important for LLM use
						renderAttr(&line, elemMap, "selector", &used)

						// Close tag and render text content
						line += ">"
						if text, ok := elemMap["text"].(string); ok && text != "" {
							line += sanitizeDOM(text)
						}

						// Selector is the most critical piece of info for the LLM
						if selector, ok := elemMap["selector"].(string); ok && selector != "" {
							line += fmt.Sprintf("  [selector: %s]", sanitizeDOM(selector))
						}
						result += line + "\n"
					}
				}
				if truncated {
					result += fmt.Sprintf("    ... and %d more elements\n", totalElems-maxElems)
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
	if execCtx == nil {
		return nil, fmt.Errorf("execution context is nil")
	}
	if execCtx.Plan == nil {
		return nil, fmt.Errorf("plan is nil")
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
		StepIndex: len(execCtx.Plan.Steps),
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
