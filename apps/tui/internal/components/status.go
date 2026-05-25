package components

import (
	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/packages/shared/types"
)

// GetFlowStatusStyle returns the lipgloss style for a given flow state.
func GetFlowStatusStyle(status types.FlowState) lipgloss.Style {
	switch status {
	case types.FlowStateRunning:
		return style.StatusRunning
	case types.FlowStatePassed:
		return style.StatusPassed
	case types.FlowStateFailed:
		return style.StatusFailed
	case types.FlowStatePaused:
		return style.StatusPaused
	case types.FlowStatePending:
		return style.StatusPending
	case types.FlowStateRetrying:
		return style.StatusRetrying
	case types.FlowStateSkippedUpstream, types.FlowStateSkippedUser, types.FlowStateBlockedConfigError:
		return style.StatusCancelled
	case types.FlowStateWaitingInput:
		return style.StatusPaused
	default:
		return style.StatusPending
	}
}

// GetRunStatusStyle returns the lipgloss style for a given run state.
func GetRunStatusStyle(status types.RunState) lipgloss.Style {
	switch status {
	case types.RunStateRunning, types.RunStateResuming:
		return style.StatusRunning
	case types.RunStatePaused, types.RunStatePausing:
		return style.StatusPaused
	case types.RunStateCancelled, types.RunStateCancelling:
		return style.StatusCancelled
	case types.RunStateCompleted:
		return style.StatusPassed
	case types.RunStateFailed:
		return style.StatusFailed
	case types.RunStateWaitingInput:
		return style.StatusRetrying
	case types.RunStatePending:
		return style.StatusPending
	default:
		return style.StatusPending
	}
}
