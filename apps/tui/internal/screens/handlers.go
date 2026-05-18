package screens

import (
	"fmt"
	"time"

	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/session"
)

type CommandHandlers struct {
	store *session.SessionStore
}

func NewCommandHandlers(store *session.SessionStore) *CommandHandlers {
	return &CommandHandlers{
		store: store,
	}
}

func (h *CommandHandlers) StartCampaign(campaignName string) (*types.Session, error) {
	return h.store.Create(campaignName)
}

func (h *CommandHandlers) PauseRun(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}

	if sess.Status != types.RunStateRunning && sess.Status != types.RunStatePending {
		return fmt.Errorf("cannot pause: run is in %s state", sess.Status)
	}

	return h.store.UpdateStatus(runID, types.RunStatePausing)
}

func (h *CommandHandlers) ResumeRun(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}

	if sess.Status != types.RunStatePaused {
		return fmt.Errorf("cannot resume: run is in %s state, expected PAUSED", sess.Status)
	}

	return h.store.UpdateStatus(runID, types.RunStateResuming)
}

func (h *CommandHandlers) CancelRun(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}

	if sess.Status == types.RunStateCompleted || sess.Status == types.RunStateCancelled {
		return fmt.Errorf("cannot cancel: run is already %s", sess.Status)
	}

	return h.store.UpdateStatus(runID, types.RunStateCancelling)
}

func (h *CommandHandlers) GetRunStatus(runID string) (*types.Session, error) {
	return h.store.Get(runID)
}

func (h *CommandHandlers) ListRuns() ([]*types.Session, error) {
	return h.store.List()
}

func (h *CommandHandlers) UpdateFlowState(runID, flowID string, status types.FlowState, errMsg string) error {
	return h.store.UpdateFlowState(runID, flowID, status, errMsg)
}

func (h *CommandHandlers) SaveCheckpoint(runID string, cp *types.Checkpoint) error {
	return h.store.SaveCheckpoint(runID, cp)
}

func (h *CommandHandlers) MarkFlowPaused(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStatePaused, "")
}

func (h *CommandHandlers) MarkRunCompleted(runID string) error {
	return h.store.UpdateStatus(runID, types.RunStateCompleted)
}

func (h *CommandHandlers) MarkRunFailed(runID string) error {
	return h.store.UpdateStatus(runID, types.RunStateFailed)
}

func (h *CommandHandlers) MarkFlowPassed(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStatePassed, "")
}

func (h *CommandHandlers) MarkFlowFailed(runID, flowID string, errMsg string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateFailed, errMsg)
}

func (h *CommandHandlers) GetCheckpoint(runID string) (*types.Checkpoint, error) {
	sess, err := h.store.Get(runID)
	if err != nil {
		return nil, err
	}
	return sess.Checkpoint, nil
}

func (h *CommandHandlers) SetCurrentFlow(runID, flowID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}
	sess.CurrentFlowID = flowID
	return h.store.Save(sess)
}

func (h *CommandHandlers) SetCurrentAgent(runID, agent string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}
	sess.CurrentAgent = agent
	return h.store.Save(sess)
}

func (h *CommandHandlers) IncrementRetryCount(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}
	sess.RetryCount++
	return h.store.Save(sess)
}

func (h *CommandHandlers) FinalizeRunCompletion(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	sess.CompletedAt = &now

	hasFailures := false
	for _, f := range sess.Flows {
		if f.Status == types.FlowStateFailed {
			hasFailures = true
			break
		}
	}

	if hasFailures {
		sess.Status = types.RunStateFailed
	} else {
		sess.Status = types.RunStateCompleted
	}

	return h.store.Save(sess)
}

func (h *CommandHandlers) SetRunWaitingForInput(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}

	sess.Status = types.RunStateWaitingInput
	return h.store.Save(sess)
}

func (h *CommandHandlers) AcknowledgeInputAndResume(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}

	if sess.Status != types.RunStateWaitingInput {
		return fmt.Errorf("run is not in WAITING_FOR_INPUT state: %s", sess.Status)
	}

	sess.Status = types.RunStateRunning
	return h.store.Save(sess)
}

func (h *CommandHandlers) SkipFlow(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateSkippedUpstream, "user_skipped")
}

func (h *CommandHandlers) RetryFlow(runID, flowID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return err
	}

	for i, f := range sess.Flows {
		if f.FlowID == flowID {
			sess.Flows[i].Status = types.FlowStateRetrying
			sess.Flows[i].RetryCount++
			break
		}
	}

	return h.store.Save(sess)
}

func (h *CommandHandlers) MarkFlowWaitingInput(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateWaitingInput, "")
}

func (h *CommandHandlers) AcknowledgeFlowInput(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateRunning, "")
}
