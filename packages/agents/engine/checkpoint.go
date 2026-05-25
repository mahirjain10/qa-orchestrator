package engine

import (
	"fmt"

	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/trace"
)

func (e *AgentEngine) saveCheckpoint(runID string, ctx *agentstypes.ExecutionContext, planStep *agentstypes.PlanStep) {
	if ctx.Plan == nil {
		return
	}
	visitedURLs := make(map[string]bool)
	for k, v := range ctx.VisitedURLs {
		visitedURLs[k] = v
	}
	payload := map[string]any{
		"current_step":               planStep.StepID,
		"step_index":                 planStep.StepIndex,
		"current_url":                ctx.CurrentURL,
		"last_step_signature":        ctx.LastStepSignature,
		"consecutive_observe_count":  ctx.ConsecutiveObserveCount,
		"visited_urls":               visitedURLs,
		"repetition_blocked_success": ctx.RepetitionBlockedSuccess,
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

func (e *AgentEngine) restoreCheckpoint(ctx *agentstypes.ExecutionContext) {
	if e.sessionStore == nil {
		return
	}
	sess, err := e.sessionStore.Get(ctx.RunID)
	if err != nil || sess == nil || sess.Checkpoint == nil || sess.Checkpoint.Payload == nil {
		return
	}
	cp := sess.Checkpoint.Payload
	if url, ok := cp["current_url"].(string); ok {
		ctx.CurrentURL = url
	}
	if sig, ok := cp["last_step_signature"].(string); ok {
		ctx.LastStepSignature = sig
	}
	if countFloat, ok := cp["consecutive_observe_count"].(float64); ok {
		ctx.ConsecutiveObserveCount = int(countFloat)
	} else if countInt, ok := cp["consecutive_observe_count"].(int); ok {
		ctx.ConsecutiveObserveCount = countInt
	}
	if v, ok := cp["visited_urls"].(map[string]any); ok {
		if ctx.VisitedURLs == nil {
			ctx.VisitedURLs = make(map[string]bool)
		}
		for url := range v {
			ctx.VisitedURLs[url] = true
		}
	} else if v, ok := cp["visited_urls"].(map[string]bool); ok {
		if ctx.VisitedURLs == nil {
			ctx.VisitedURLs = make(map[string]bool)
		}
		for url, val := range v {
			ctx.VisitedURLs[url] = val
		}
	}
	if v, ok := cp["repetition_blocked_success"].(bool); ok {
		ctx.RepetitionBlockedSuccess = v
	}

	if ctx.Plan != nil && sess.Checkpoint.StepIndex > 0 {
		ctx.Plan.CurrentIdx = sess.Checkpoint.StepIndex
	}
}
