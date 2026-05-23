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
	ctrl := NewLifecycleController("run_123")

	for _, status := range []types.RunState{
		types.RunStatePending,
		types.RunStateRunning,
		types.RunStatePaused,
		types.RunStatePausing,
		types.RunStateWaitingInput,
	} {
		ctrl.SetStatus(status)
		if !ctrl.CanCancel() {
			t.Errorf("expected CanCancel() to be true for %s", status)
		}
	}

	for _, status := range []types.RunState{
		types.RunStateCompleted,
		types.RunStateCancelled,
	} {
		ctrl.SetStatus(status)
		if ctrl.CanCancel() {
			t.Errorf("expected CanCancel() to be false for %s", status)
		}
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
