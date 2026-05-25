package runtime

import (
	"fmt"
	"testing"
	"time"

	"qa-orchestrator/packages/shared/types"
)

func TestLifecycleController_New(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	if ctrl.GetRunID() != "run_123" {
		t.Errorf("expected runID run_123, got %s", ctrl.GetRunID())
	}
	if ctrl.GetStatus() != types.RunStatePending {
		t.Errorf("expected initial status PENDING, got %s", ctrl.GetStatus())
	}
}

func TestLifecycleController_CanCancel(t *testing.T) {
	// PENDING (initial state) — should be cancellable
	ctrl := NewLifecycleController("run_pending")
	if !ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be true for PENDING")
	}

	// RUNNING — should be cancellable
	ctrl = NewLifecycleController("run_running")
	ctrl.SetStatus(types.RunStateRunning)
	if !ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be true for RUNNING")
	}

	// PAUSING — should be cancellable
	ctrl = NewLifecycleController("run_pausing")
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetStatus(types.RunStatePausing)
	if !ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be true for PAUSING")
	}

	// PAUSED — should be cancellable
	ctrl = NewLifecycleController("run_paused")
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetStatus(types.RunStatePausing)
	ctrl.SetStatus(types.RunStatePaused)
	if !ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be true for PAUSED")
	}

	// WAITING_FOR_INPUT — should be cancellable
	ctrl = NewLifecycleController("run_waiting")
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetStatus(types.RunStateWaitingInput)
	if !ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be true for WAITING_FOR_INPUT")
	}

	// COMPLETED — should NOT be cancellable
	ctrl = NewLifecycleController("run_completed")
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetStatus(types.RunStateCompleted)
	if ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be false for COMPLETED")
	}

	// CANCELLED — should NOT be cancellable
	ctrl = NewLifecycleController("run_cancelled")
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.RequestCancel()
	ctrl.AcknowledgeCancel()
	if ctrl.CanCancel() {
		t.Errorf("expected CanCancel() to be false for CANCELLED")
	}
}

func TestLifecycleController_RequestCancel(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	ctrl.SetStatus(types.RunStateRunning)

	if !ctrl.RequestCancel() {
		t.Error("expected RequestCancel() to succeed")
	}
	if ctrl.GetStatus() != types.RunStateCancelling {
		t.Errorf("expected status CANCELLING, got %s", ctrl.GetStatus())
	}

	select {
	case <-ctrl.CancelCh():
	case <-time.After(100 * time.Millisecond):
		t.Error("expected cancel signal on channel")
	}
}

func TestLifecycleController_WaitingForInput(t *testing.T) {
	ctrl := NewLifecycleController("run_123")

	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetWaitingForInput()
	if !ctrl.IsWaitingForInput() {
		t.Error("expected IsWaitingForInput() to be true")
	}
	if ctrl.GetStatus() != types.RunStateWaitingInput {
		t.Errorf("expected status WAITING_FOR_INPUT, got %s", ctrl.GetStatus())
	}

	ctrl.AcknowledgeInput()
	if ctrl.IsWaitingForInput() {
		t.Error("expected IsWaitingForInput() to be false after AcknowledgeInput()")
	}
	if ctrl.GetStatus() != types.RunStateRunning {
		t.Errorf("expected status RUNNING after AcknowledgeInput(), got %s", ctrl.GetStatus())
	}
}

func TestLifecycleController_SteeringEvents(t *testing.T) {
	ctrl := NewLifecycleController("run_123")

	event1 := types.NewSteeringEvent("run_123", "flow-1", types.SteerRetry, "user requested retry", "")
	event2 := types.NewSteeringEvent("run_123", "flow-1", types.SteerSkip, "user requested skip", "")

	ctrl.SubmitSteering(event1)
	ctrl.SubmitSteering(event2)

	events := ctrl.DrainSteeringEvents()
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
	if events[0].Command != types.SteerRetry {
		t.Errorf("expected first event to be SteerRetry")
	}
	if events[1].Command != types.SteerSkip {
		t.Errorf("expected second event to be SteerSkip")
	}

	events = ctrl.DrainSteeringEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 events after drain, got %d", len(events))
	}
}

func TestSubmitSteering_AcceptsUpToCapacity(t *testing.T) {
	ctrl := NewLifecycleController("run_cap")

	// Submit up to maxSteeringQueue — all should be retained
	for i := 0; i < maxSteeringQueue; i++ {
		evt := types.NewSteeringEvent("run_cap", "flow-x", types.SteerInstruction, "", fmt.Sprintf("instruction-%d", i))
		if !ctrl.SubmitSteering(evt) {
			t.Fatalf("SubmitSteering returned false at index %d — should never return false", i)
		}
	}

	events := ctrl.DrainSteeringEvents()
	if len(events) != maxSteeringQueue {
		t.Fatalf("expected %d events within capacity, got %d", maxSteeringQueue, len(events))
	}
	for i, evt := range events {
		if evt.Instruction != fmt.Sprintf("instruction-%d", i) {
			t.Errorf("event %d: expected instruction %q, got %q", i, fmt.Sprintf("instruction-%d", i), evt.Instruction)
		}
	}

	remaining := ctrl.DrainSteeringEvents()
	if len(remaining) != 0 {
		t.Errorf("expected 0 events after full drain, got %d", len(remaining))
	}
}

func TestSubmitSteering_OverflowTrimsOldest(t *testing.T) {
	ctrl := NewLifecycleController("run_overflow")

	// Fill to capacity
	for i := 0; i < maxSteeringQueue; i++ {
		ctrl.SubmitSteering(types.NewSteeringEvent("run_overflow", "flow-x", types.SteerInstruction, "", fmt.Sprintf("old-%d", i)))
	}

	// Submit one more — should trim the oldest
	ctrl.SubmitSteering(types.NewSteeringEvent("run_overflow", "flow-x", types.SteerInstruction, "", "new-last"))

	events := ctrl.DrainSteeringEvents()
	if len(events) != maxSteeringQueue {
		t.Fatalf("expected %d events after overflow trim, got %d", maxSteeringQueue, len(events))
	}

	// The second-oldest event (index 1) should now be first (oldest at index 0 was trimmed)
	if events[0].Instruction != "old-1" {
		t.Errorf("expected first event 'old-1' (oldest 'old-0' trimmed), got %q", events[0].Instruction)
	}

	// The last event should be the one we just submitted
	if events[len(events)-1].Instruction != "new-last" {
		t.Errorf("expected last event 'new-last', got %q", events[len(events)-1].Instruction)
	}
}

// ── transitionValid tests ──────────────────────────────────────────────────

func TestTransitionValid_AllowsKnownTransitions(t *testing.T) {
	tests := []struct {
		name string
		from types.RunState
		to   types.RunState
	}{
		{"PENDING→RUNNING", types.RunStatePending, types.RunStateRunning},
		{"RUNNING→PAUSING", types.RunStateRunning, types.RunStatePausing},
		{"RUNNING→CANCELLING", types.RunStateRunning, types.RunStateCancelling},
		{"RUNNING→COMPLETED", types.RunStateRunning, types.RunStateCompleted},
		{"RUNNING→FAILED", types.RunStateRunning, types.RunStateFailed},
		{"RUNNING→WAITING_INPUT", types.RunStateRunning, types.RunStateWaitingInput},
		{"PAUSING→PAUSED", types.RunStatePausing, types.RunStatePaused},
		{"PAUSED→RESUMING", types.RunStatePaused, types.RunStateResuming},
		{"PAUSED→CANCELLING", types.RunStatePaused, types.RunStateCancelling},
		{"RESUMING→RUNNING", types.RunStateResuming, types.RunStateRunning},
		{"WAITING_INPUT→RUNNING", types.RunStateWaitingInput, types.RunStateRunning},
		{"WAITING_INPUT→CANCELLING", types.RunStateWaitingInput, types.RunStateCancelling},
		{"CANCELLING→CANCELLED", types.RunStateCancelling, types.RunStateCancelled},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !transitionValid(tt.from, tt.to) {
				t.Errorf("transitionValid(%v, %v) = false, want true", tt.from, tt.to)
			}
		})
	}
}

func TestTransitionValid_SameStateIsAlwaysValid(t *testing.T) {
	states := []types.RunState{
		types.RunStatePending,
		types.RunStateRunning,
		types.RunStatePausing,
		types.RunStatePaused,
		types.RunStateResuming,
		types.RunStateCancelling,
		types.RunStateCancelled,
		types.RunStateCompleted,
		types.RunStateFailed,
		types.RunStateWaitingInput,
	}
	for _, s := range states {
		t.Run(string(s), func(t *testing.T) {
			if !transitionValid(s, s) {
				t.Errorf("transitionValid(%v, %v) = false, want true (same-state)", s, s)
			}
		})
	}
}

func TestTransitionValid_RejectsInvalidTransitions(t *testing.T) {
	tests := []struct {
		name string
		from types.RunState
		to   types.RunState
	}{
		// PENDING only → RUNNING
		{"PENDING→PAUSING", types.RunStatePending, types.RunStatePausing},
		{"PENDING→CANCELLING", types.RunStatePending, types.RunStateCancelling},
		{"PENDING→COMPLETED", types.RunStatePending, types.RunStateCompleted},
		{"PENDING→FAILED", types.RunStatePending, types.RunStateFailed},
		{"PENDING→WAITING_INPUT", types.RunStatePending, types.RunStateWaitingInput},

		// RUNNING only → non-RUNNING targets that aren't in its whitelist
		{"RUNNING→PENDING", types.RunStateRunning, types.RunStatePending},
		{"RUNNING→PAUSED", types.RunStateRunning, types.RunStatePaused},
		{"RUNNING→RESUMING", types.RunStateRunning, types.RunStateResuming},

		// PAUSING only → PAUSED
		{"PAUSING→RUNNING", types.RunStatePausing, types.RunStateRunning},
		{"PAUSING→COMPLETED", types.RunStatePausing, types.RunStateCompleted},
		{"PAUSING→FAILED", types.RunStatePausing, types.RunStateFailed},
		{"PAUSING→CANCELLING", types.RunStatePausing, types.RunStateCancelling},

		// PAUSED only → RESUMING, CANCELLING
		{"PAUSED→RUNNING", types.RunStatePaused, types.RunStateRunning},
		{"PAUSED→COMPLETED", types.RunStatePaused, types.RunStateCompleted},
		{"PAUSED→FAILED", types.RunStatePaused, types.RunStateFailed},
		{"PAUSED→WAITING_INPUT", types.RunStatePaused, types.RunStateWaitingInput},

		// RESUMING only → RUNNING
		{"RESUMING→PENDING", types.RunStateResuming, types.RunStatePending},
		{"RESUMING→COMPLETED", types.RunStateResuming, types.RunStateCompleted},
		{"RESUMING→FAILED", types.RunStateResuming, types.RunStateFailed},

		// WAITING_INPUT only → RUNNING, CANCELLING
		{"WAITING_INPUT→PENDING", types.RunStateWaitingInput, types.RunStatePending},
		{"WAITING_INPUT→COMPLETED", types.RunStateWaitingInput, types.RunStateCompleted},
		{"WAITING_INPUT→FAILED", types.RunStateWaitingInput, types.RunStateFailed},

		// CANCELLING only → CANCELLED
		{"CANCELLING→RUNNING", types.RunStateCancelling, types.RunStateRunning},
		{"CANCELLING→COMPLETED", types.RunStateCancelling, types.RunStateCompleted},
		{"CANCELLING→FAILED", types.RunStateCancelling, types.RunStateFailed},
		{"CANCELLING→PAUSED", types.RunStateCancelling, types.RunStatePaused},

		// Terminal states reject everything
		{"CANCELLED→RUNNING", types.RunStateCancelled, types.RunStateRunning},
		{"CANCELLED→PENDING", types.RunStateCancelled, types.RunStatePending},
		{"COMPLETED→RUNNING", types.RunStateCompleted, types.RunStateRunning},
		{"COMPLETED→PENDING", types.RunStateCompleted, types.RunStatePending},
		{"FAILED→RUNNING", types.RunStateFailed, types.RunStateRunning},
		{"FAILED→PENDING", types.RunStateFailed, types.RunStatePending},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if transitionValid(tt.from, tt.to) {
				t.Errorf("transitionValid(%v, %v) = true, want false", tt.from, tt.to)
			}
		})
	}
}

// ── RequestCancel with validation ───────────────────────────────────────────

func TestRequestCancel_ReturnsFalseForInvalidStates(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(ctrl *LifecycleController)
	}{
		{"PENDING", func(ctrl *LifecycleController) { /* initial state PENDING */ }},
		{"COMPLETED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStateCompleted)
		}},
		{"CANCELLED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.RequestCancel()
			ctrl.AcknowledgeCancel()
		}},
		{"FAILED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStateFailed)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewLifecycleController("run_req_cancel_" + tt.name)
			tt.setup(ctrl)
			if ctrl.RequestCancel() {
				t.Errorf("RequestCancel() = true from state %v, want false", ctrl.GetStatus())
			}
		})
	}
}

// ── AcknowledgeCancel guard ─────────────────────────────────────────────────

func TestAcknowledgeCancel_ReturnsTrueOnlyFromCancelling(t *testing.T) {
	t.Run("from CANCELLING succeeds", func(t *testing.T) {
		ctrl := NewLifecycleController("ack_cancel_ok")
		ctrl.SetStatus(types.RunStateRunning)
		ctrl.RequestCancel() // → CANCELLING
		if !ctrl.AcknowledgeCancel() {
			t.Fatal("AcknowledgeCancel() from CANCELLING returned false")
		}
		if ctrl.GetStatus() != types.RunStateCancelled {
			t.Errorf("status = %v, want CANCELLED", ctrl.GetStatus())
		}
	})

	tests := []struct {
		name  string
		setup func(ctrl *LifecycleController)
	}{
		{"PENDING", func(*LifecycleController) {}},
		{"RUNNING", func(ctrl *LifecycleController) { ctrl.SetStatus(types.RunStateRunning) }},
		{"COMPLETED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStateCompleted)
		}},
		{"FAILED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStateFailed)
		}},
	}
	for _, tt := range tests {
		t.Run("from "+tt.name+" fails", func(t *testing.T) {
			ctrl := NewLifecycleController("ack_cancel_fail_" + tt.name)
			tt.setup(ctrl)
			before := ctrl.GetStatus()
			if ctrl.AcknowledgeCancel() {
				t.Errorf("AcknowledgeCancel() = true from state %v, want false", before)
			}
			if ctrl.GetStatus() != before {
				t.Errorf("status changed from %v to %v after failed AcknowledgeCancel", before, ctrl.GetStatus())
			}
		})
	}
}

// ── SetWaitingForInput / AcknowledgeInput validation ────────────────────────

func TestSetWaitingForInput_NoopFromInvalidStates(t *testing.T) {
	tests := []struct {
		name  string
		setup func(ctrl *LifecycleController)
	}{
		{"PENDING", func(*LifecycleController) {}},
		{"PAUSED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStatePausing)
			ctrl.SetStatus(types.RunStatePaused)
		}},
		{"COMPLETED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStateCompleted)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewLifecycleController("swfi_noop_" + tt.name)
			tt.setup(ctrl)
			before := ctrl.GetStatus()
			ctrl.SetWaitingForInput()
			if ctrl.IsWaitingForInput() {
				t.Errorf("IsWaitingForInput() = true after SetWaitingForInput from %v", before)
			}
			if ctrl.GetStatus() != before {
				t.Errorf("status changed from %v to %v (should be no-op)", before, ctrl.GetStatus())
			}
		})
	}
}

func TestAcknowledgeInput_NoopFromInvalidStates(t *testing.T) {
	tests := []struct {
		name  string
		setup func(ctrl *LifecycleController)
	}{
		{"PENDING", func(*LifecycleController) {}},
		{"RUNNING", func(ctrl *LifecycleController) { ctrl.SetStatus(types.RunStateRunning) }},
		{"COMPLETED", func(ctrl *LifecycleController) {
			ctrl.SetStatus(types.RunStateRunning)
			ctrl.SetStatus(types.RunStateCompleted)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := NewLifecycleController("ack_input_noop_" + tt.name)
			tt.setup(ctrl)
			before := ctrl.GetStatus()
			ctrl.AcknowledgeInput()
			if ctrl.GetStatus() != before {
				t.Errorf("status changed from %v to %v (should be no-op)", before, ctrl.GetStatus())
			}
		})
	}
}

func TestAcknowledgeInput_TransitionsCorrectly(t *testing.T) {
	ctrl := NewLifecycleController("ack_input_ok")
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetWaitingForInput()
	if !ctrl.IsWaitingForInput() {
		t.Fatal("expected IsWaitingForInput() after SetWaitingForInput")
	}
	ctrl.AcknowledgeInput()
	if ctrl.IsWaitingForInput() {
		t.Error("IsWaitingForInput() still true after AcknowledgeInput")
	}
	if ctrl.GetStatus() != types.RunStateRunning {
		t.Errorf("status = %v, want RUNNING", ctrl.GetStatus())
	}
}

// ── SetStatus validation ────────────────────────────────────────────────────

func TestSetStatus_NoopForInvalidTransition(t *testing.T) {
	ctrl := NewLifecycleController("set_status_noop")
	// PENDING is the initial state. Attempting to go to COMPLETED directly should be a no-op.
	ctrl.SetStatus(types.RunStateCompleted)
	if ctrl.GetStatus() != types.RunStatePending {
		t.Errorf("status = %v, want PENDING (no-op attempted COMPLETED)", ctrl.GetStatus())
	}

	// RUNNING → WAITING_FOR_INPUT is valid
	ctrl.SetStatus(types.RunStateRunning)
	ctrl.SetStatus(types.RunStateWaitingInput)
	if ctrl.GetStatus() != types.RunStateWaitingInput {
		t.Fatalf("status = %v, want WAITING_FOR_INPUT", ctrl.GetStatus())
	}

	// WAITING_FOR_INPUT → PAUSED is invalid — should stay WAITING_FOR_INPUT
	ctrl.SetStatus(types.RunStatePaused)
	if ctrl.GetStatus() != types.RunStateWaitingInput {
		t.Errorf("status = %v, want WAITING_FOR_INPUT (no-op attempted PAUSED)", ctrl.GetStatus())
	}
}

func TestSubmitSteering_MultipleOverflowsStayBounded(t *testing.T) {
	ctrl := NewLifecycleController("run_multi")

	// Fill well past capacity without draining
	total := maxSteeringQueue + 50
	for i := 0; i < total; i++ {
		ctrl.SubmitSteering(types.NewSteeringEvent("run_multi", "flow-x", types.SteerInstruction, "", fmt.Sprintf("evt-%d", i)))
	}

	events := ctrl.DrainSteeringEvents()
	if len(events) != maxSteeringQueue {
		t.Fatalf("expected %d events after overflow of %d, got %d", maxSteeringQueue, total, len(events))
	}

	// First event should be the oldest surviving one (trimmed 50 oldest)
	if events[0].Instruction != "evt-50" {
		t.Errorf("expected first event 'evt-50' (50 oldest trimmed), got %q", events[0].Instruction)
	}

	// Last event should be the most recently submitted
	if events[len(events)-1].Instruction != "evt-249" {
		t.Errorf("expected last event 'evt-249', got %q", events[len(events)-1].Instruction)
	}
}
