package runtime

import (
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

func TestLifecycleController_CanPause(t *testing.T) {
	ctrl := NewLifecycleController("run_123")

	ctrl.SetStatus(types.RunStatePending)
	if !ctrl.CanPause() {
		t.Error("expected CanPause() to be true for PENDING")
	}

	ctrl.SetStatus(types.RunStateRunning)
	if !ctrl.CanPause() {
		t.Error("expected CanPause() to be true for RUNNING")
	}

	ctrl.SetStatus(types.RunStatePaused)
	if ctrl.CanPause() {
		t.Error("expected CanPause() to be false for PAUSED")
	}
}

func TestLifecycleController_CanResume(t *testing.T) {
	ctrl := NewLifecycleController("run_123")

	ctrl.SetStatus(types.RunStatePaused)
	if !ctrl.CanResume() {
		t.Error("expected CanResume() to be true for PAUSED")
	}

	ctrl.SetStatus(types.RunStateRunning)
	if ctrl.CanResume() {
		t.Error("expected CanResume() to be false for RUNNING")
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

func TestLifecycleController_RequestPause(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	ctrl.SetStatus(types.RunStateRunning)

	if !ctrl.RequestPause() {
		t.Error("expected RequestPause() to succeed")
	}
	if ctrl.GetStatus() != types.RunStatePausing {
		t.Errorf("expected status PAUSING, got %s", ctrl.GetStatus())
	}

	select {
	case <-ctrl.PauseCh():
	case <-time.After(100 * time.Millisecond):
		t.Error("expected pause signal on channel")
	}
}

func TestLifecycleController_AcknowledgePause(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	ctrl.SetStatus(types.RunStatePausing)
	ctrl.AcknowledgePause()

	if ctrl.GetStatus() != types.RunStatePaused {
		t.Errorf("expected status PAUSED, got %s", ctrl.GetStatus())
	}
}

func TestLifecycleController_RequestResume(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	ctrl.SetStatus(types.RunStatePaused)

	if !ctrl.RequestResume() {
		t.Error("expected RequestResume() to succeed")
	}
	if ctrl.GetStatus() != types.RunStateResuming {
		t.Errorf("expected status RESUMING, got %s", ctrl.GetStatus())
	}

	select {
	case <-ctrl.ResumeCh():
	case <-time.After(100 * time.Millisecond):
		t.Error("expected resume signal on channel")
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

func TestLifecycleController_RequestInputSignal(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	ctrl.RequestInput()

	select {
	case <-ctrl.InputCh():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("expected input signal on channel")
	}
	if ctrl.GetStatus() != types.RunStateWaitingInput {
		t.Fatalf("expected WAITING_FOR_INPUT, got %s", ctrl.GetStatus())
	}
}

func TestLifecycleController_SteeringEvents(t *testing.T) {
	ctrl := NewLifecycleController("run_123")

	event1 := types.NewSteeringEvent("run_123", "flow-1", types.SteerRetry, "user requested retry")
	event2 := types.NewSteeringEvent("run_123", "flow-1", types.SteerSkip, "user requested skip")

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

func TestLifecycleController_WaitForMethods(t *testing.T) {
	ctrl := NewLifecycleController("run_123")
	ctrl.SetStatus(types.RunStateRunning)

	pauseReceived := make(chan struct{})
	go func() {
		ctrl.WaitForPause()
		close(pauseReceived)
	}()

	time.Sleep(20 * time.Millisecond)
	ctrl.SubmitSteering(types.NewSteeringEvent("run_123", "", types.SteerRetry, ""))

	ok := ctrl.RequestPause()
	if !ok {
		t.Error("expected RequestPause() to succeed")
	}

	select {
	case <-pauseReceived:
	case <-time.After(200 * time.Millisecond):
		t.Error("expected pause signal received by WaitForPause")
	}
}
