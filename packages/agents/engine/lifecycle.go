package engine

import (
	"log"
	"time"

	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/trace"
)

func (e *AgentEngine) checkPauseState(runID string) (sharedtypes.RunState, bool) {
	if e.lifecycle != nil {
		return e.lifecycle.GetStatus(), true
	}
	if e.sessionStore == nil {
		return sharedtypes.RunStateRunning, true
	}
	sess, err := e.sessionStore.Get(runID)
	if err != nil || sess == nil {
		return sharedtypes.RunStateRunning, false
	}
	return sess.Status, true
}

func (e *AgentEngine) waitForResume(runID string) {
	if e.sessionStore == nil {
		return
	}
	pollInterval := resumePollInitialDelay
	maxPollInterval := resumePollMaxDelay
	for {
		time.Sleep(pollInterval)
		pollInterval *= 2
		if pollInterval > maxPollInterval {
			pollInterval = maxPollInterval
		}
		status, exists := e.checkPauseState(runID)
		if !exists {
			return
		}
		if status == sharedtypes.RunStateResuming || status == sharedtypes.RunStateRunning {
			if e.lifecycle != nil {
				e.lifecycle.SetStatus(sharedtypes.RunStateRunning)
			}
			e.syncSessionStore(runID, "", "transition_to_running", func() error {
				return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)
			})
			return
		}
		if status == sharedtypes.RunStateCancelling || status == sharedtypes.RunStateCancelled {
			return
		}
	}
}

func (e *AgentEngine) handlePauseResume(runID string, ctx *agentstypes.ExecutionContext) pauseAction {
	pauseStatus, exists := e.checkPauseState(runID)
	if !exists {
		return pauseFail
	}
	switch pauseStatus {
	case sharedtypes.RunStatePausing:
		// Re-check to avoid race with RequestCancel: if cancel fired
		// between checkPauseState above and this write, don't overwrite it.
		currentStatus, exists := e.checkPauseState(runID)
		if !exists {
			return pauseFail
		}
		if currentStatus == sharedtypes.RunStateCancelling || currentStatus == sharedtypes.RunStateCancelled {
			return pauseSkip
		}
		if e.lifecycle != nil {
			if ls := e.lifecycle.GetStatus(); ls == sharedtypes.RunStateCancelling || ls == sharedtypes.RunStateCancelled {
				return pauseSkip
			}
		}
		if e.lifecycle != nil {
			e.lifecycle.SetStatus(sharedtypes.RunStatePaused)
		}
		e.syncSessionStore(runID, ctx.FlowID, "transition_to_paused", func() error {
			return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStatePaused)
		})
		e.setCurrentAgent(runID, "idle (paused)")
		e.waitForResume(runID)
		e.restoreCheckpoint(ctx)
		cancelStatus, exists := e.checkPauseState(runID)
		if !exists {
			return pauseFail
		}
		if cancelStatus == sharedtypes.RunStateCancelling || cancelStatus == sharedtypes.RunStateCancelled {
			return pauseSkip
		}
		return pauseContinue
	case sharedtypes.RunStatePaused:
		e.waitForResume(runID)
		e.restoreCheckpoint(ctx)
		cancelStatus, exists := e.checkPauseState(runID)
		if !exists {
			return pauseFail
		}
		if cancelStatus == sharedtypes.RunStateCancelling || cancelStatus == sharedtypes.RunStateCancelled {
			return pauseSkip
		}
		return pauseContinue
	case sharedtypes.RunStateCancelling, sharedtypes.RunStateCancelled:
		return pauseSkip
	default:
		return pauseContinue
	}
}

func (e *AgentEngine) drainSteeringEvents(ctx *agentstypes.ExecutionContext, runID, flowID string) {
	if e.lifecycle == nil {
		return
	}
	events := e.lifecycle.DrainSteeringEvents()
	for _, evt := range events {
		if evt.FlowID != "" && evt.FlowID != flowID {
			continue
		}
		switch evt.Command {
		case sharedtypes.SteerInstruction:
			if evt.Instruction != "" {
				if len(ctx.SteeringInstructions) >= 20 {
					ctx.SteeringInstructions = ctx.SteeringInstructions[1:]
				}
				ctx.SteeringInstructions = append(ctx.SteeringInstructions, evt.Instruction)
				trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "instruction_received", evt.Instruction)
			}
		case sharedtypes.SteerRetry:
			ctx.SteeringRetryRequested = true
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "retry_requested", evt.Reason)
		case sharedtypes.SteerSkip:
			ctx.SteeringSkipRequested = true
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "skip_requested", evt.Reason)
		case sharedtypes.SteerApprove, sharedtypes.SteerContinue:
			e.lifecycle.AcknowledgeInput()
			if currentStatus, _ := e.checkPauseState(runID); currentStatus == sharedtypes.RunStateCancelling || currentStatus == sharedtypes.RunStateCancelled {
				trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "resume_cancel_race", "cancel fired before resume; not overwriting with Running")
				break
			}
			e.syncSessionStore(runID, flowID, "resume", func() error {
				return e.sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)
			})
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "resumed", evt.Reason)
		case sharedtypes.SteerHumanReview:
			e.lifecycle.SetWaitingForInput()
			trace.EmitAgentDecision(e.traceStore, runID, flowID, "steering", "human_review", evt.Reason)
		default:
			log.Printf("drainSteeringEvents: unknown command type=%q flow=%s", evt.Command, evt.FlowID)
		}
	}
}
