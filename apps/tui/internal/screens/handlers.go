package screens

import (
	"fmt"

	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/shared"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/session"
)

type RunController interface {
	PauseRun(runID string) error
	ResumeRun(runID string) error
	CancelRun(runID string) error
	GetRunStatus(runID string) (*types.Session, error)
	SkipFlow(runID, flowID string) error
	RetryFlow(runID, flowID string) error
	AcknowledgeInputAndResume(runID string) error
	IsWaitingForInput() bool
}

type CommandHandlers struct {
	store     *session.SessionStore
	lifecycle *runtime.LifecycleController
}

func NewCommandHandlers(store *session.SessionStore) *CommandHandlers {
	return &CommandHandlers{
		store: store,
	}
}

func (h *CommandHandlers) SetLifecycleController(lc *runtime.LifecycleController) {
	h.lifecycle = lc
}

func (h *CommandHandlers) IsWaitingForInput() bool {
	if h.lifecycle != nil {
		return h.lifecycle.IsWaitingForInput()
	}
	return false
}

func (h *CommandHandlers) PauseRun(runID string) error {
	if h.lifecycle != nil {
		status := h.lifecycle.GetStatus()
		if status != types.RunStateRunning && status != types.RunStatePending {
			return fmt.Errorf("%w: run is in %s state", shared.ErrInvalidStateTransition, status)
		}
		h.lifecycle.SetStatus(types.RunStatePausing)
		return nil
	}
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.Status != types.RunStateRunning && sess.Status != types.RunStatePending {
		return fmt.Errorf("%w: run is in %s state", shared.ErrInvalidStateTransition, sess.Status)
	}
	return h.store.UpdateStatus(runID, types.RunStatePausing)
}

func (h *CommandHandlers) ResumeRun(runID string) error {
	if h.lifecycle != nil {
		status := h.lifecycle.GetStatus()
		if status != types.RunStatePaused && status != types.RunStatePausing {
			return fmt.Errorf("%w: run is in %s state, expected PAUSED or PAUSING", shared.ErrInvalidStateTransition, status)
		}
		h.lifecycle.SetStatus(types.RunStateResuming)
		return nil
	}
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.Status != types.RunStatePaused && sess.Status != types.RunStatePausing {
		return fmt.Errorf("%w: run is in %s state, expected PAUSED or PAUSING", shared.ErrInvalidStateTransition, sess.Status)
	}
	return h.store.UpdateStatus(runID, types.RunStateResuming)
}

func (h *CommandHandlers) CancelRun(runID string) error {
	if h.lifecycle != nil {
		if !h.lifecycle.RequestCancel() {
			status := h.lifecycle.GetStatus()
			return fmt.Errorf("%w: cannot cancel from %s state", shared.ErrInvalidStateTransition, status)
		}
		return nil
	}
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.Status == types.RunStateCompleted || sess.Status == types.RunStateCancelled {
		return fmt.Errorf("%w: run is already %s", shared.ErrInvalidStateTransition, sess.Status)
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
		return nil, fmt.Errorf("getting checkpoint for run %s: %w", runID, err)
	}
	return sess.Checkpoint, nil
}

func (h *CommandHandlers) SetCurrentFlow(runID, flowID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("loading session %s to set current flow: %w", runID, err)
	}
	sess.CurrentFlowID = flowID
	if err := h.store.Save(sess); err != nil {
		return fmt.Errorf("saving session %s after setting flow %s: %w", runID, flowID, err)
	}
	return nil
}

func (h *CommandHandlers) SetCurrentAgent(runID, agent string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("loading session %s to set agent: %w", runID, err)
	}
	sess.CurrentAgent = agent
	if err := h.store.Save(sess); err != nil {
		return fmt.Errorf("saving session %s after setting agent %s: %w", runID, agent, err)
	}
	return nil
}

func (h *CommandHandlers) IncrementRetryCount(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("loading session %s to increment retry count: %w", runID, err)
	}
	sess.RetryCount++
	if err := h.store.Save(sess); err != nil {
		return fmt.Errorf("saving session %s after incrementing retry count: %w", runID, err)
	}
	return nil
}

func (h *CommandHandlers) FinalizeRunCompletion(runID string) error {
	if err := h.store.FinalizeRun(runID); err != nil {
		return fmt.Errorf("finalizing run %s: %w", runID, err)
	}
	return nil
}

func (h *CommandHandlers) SetRunWaitingForInput(runID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("loading session %s to set waiting for input: %w", runID, err)
	}

	sess.Status = types.RunStateWaitingInput
	if err := h.store.Save(sess); err != nil {
		return fmt.Errorf("saving session %s after setting waiting for input: %w", runID, err)
	}
	return nil
}

func (h *CommandHandlers) AcknowledgeInputAndResume(runID string) error {
	if h.lifecycle != nil {
		if !h.lifecycle.IsWaitingForInput() {
			return fmt.Errorf("%w: run is not in WAITING_FOR_INPUT state: %s", shared.ErrInvalidStateTransition, h.lifecycle.GetStatus())
		}
		h.lifecycle.AcknowledgeInput()
		return nil
	}
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	if sess.Status != types.RunStateWaitingInput {
		return fmt.Errorf("%w: run is not in WAITING_FOR_INPUT state: %s", shared.ErrInvalidStateTransition, sess.Status)
	}
	sess.Status = types.RunStateRunning
	return h.store.Save(sess)
}

func (h *CommandHandlers) SkipFlow(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateSkippedUser, "user_skipped")
}

func (h *CommandHandlers) RetryFlow(runID, flowID string) error {
	sess, err := h.store.Get(runID)
	if err != nil {
		return fmt.Errorf("loading session %s for flow retry: %w", runID, err)
	}

	found := false
	for i, f := range sess.Flows {
		if f.FlowID == flowID {
			sess.Flows[i].Status = types.FlowStateRetrying
			sess.Flows[i].RetryCount++
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("%w: flow %q not found in run %s", shared.ErrFlowNotFound, flowID, runID)
	}

	if err := h.store.Save(sess); err != nil {
		return fmt.Errorf("saving session %s after setting flow %s to retrying: %w", runID, flowID, err)
	}
	return nil
}

func (h *CommandHandlers) MarkFlowWaitingInput(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateWaitingInput, "")
}

func (h *CommandHandlers) AcknowledgeFlowInput(runID, flowID string) error {
	return h.store.UpdateFlowState(runID, flowID, types.FlowStateRunning, "")
}
