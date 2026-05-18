package types

import (
	"testing"
)

func TestNewRunID(t *testing.T) {
	id1 := NewRunID()
	id2 := NewRunID()

	if id1 == "" {
		t.Error("expected non-empty run ID")
	}
	if id1 == id2 {
		t.Error("expected unique run IDs")
	}
	if len(id1) < 10 {
		t.Error("run ID too short")
	}
}

func TestNewSessionID(t *testing.T) {
	id1 := NewSessionID()
	id2 := NewSessionID()

	if id1 == "" {
		t.Error("expected non-empty session ID")
	}
	if id1 == id2 {
		t.Error("expected unique session IDs")
	}
}

func TestNewSession(t *testing.T) {
	session := NewSession("Test Campaign")

	if session.RunID == "" {
		t.Error("expected non-empty run ID")
	}
	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.CampaignName != "Test Campaign" {
		t.Errorf("expected campaign name 'Test Campaign', got %q", session.CampaignName)
	}
	if session.Status != RunStatePending {
		t.Errorf("expected status PENDING, got %s", session.Status)
	}
	if session.StartedAt.IsZero() {
		t.Error("expected non-zero started time")
	}
}

func TestSession_AddFlowState(t *testing.T) {
	session := NewSession("Test")
	flow := FlowRunState{
		FlowID: "flow-1",
		Name:   "Test Flow",
		Status: FlowStatePending,
	}

	session.AddFlowState(flow)

	if len(session.Flows) != 1 {
		t.Errorf("expected 1 flow, got %d", len(session.Flows))
	}
}

func TestSession_UpdateFlowState(t *testing.T) {
	session := NewSession("Test")
	session.AddFlowState(FlowRunState{
		FlowID: "flow-1",
		Name:   "Test Flow",
		Status: FlowStatePending,
	})

	session.UpdateFlowState("flow-1", FlowStateRunning, "")

	if session.Flows[0].Status != FlowStateRunning {
		t.Errorf("expected status RUNNING, got %s", session.Flows[0].Status)
	}
	if session.Flows[0].StartedAt == nil {
		t.Error("expected started time to be set")
	}

	session.UpdateFlowState("flow-1", FlowStatePassed, "")
	if session.Flows[0].FinishedAt == nil {
		t.Error("expected finished time to be set")
	}
}

func TestSession_SetCheckpoint(t *testing.T) {
	session := NewSession("Test")
	session.SetCheckpoint("flow-1", "step-1", 5, map[string]any{"key": "value"})

	if session.Checkpoint == nil {
		t.Fatal("expected checkpoint to be set")
	}
	if session.Checkpoint.FlowID != "flow-1" {
		t.Errorf("expected flow ID 'flow-1', got %s", session.Checkpoint.FlowID)
	}
	if session.Checkpoint.StepIndex != 5 {
		t.Errorf("expected step index 5, got %d", session.Checkpoint.StepIndex)
	}
}

func TestDependencyError_Error(t *testing.T) {
	missingErr := &DependencyError{
		FlowID:      "f1",
		MissingDeps: []string{"missing1"},
	}
	if missingErr.Error() != "missing dependencies" {
		t.Error("expected 'missing dependencies' error message")
	}

	cycleErr := &DependencyError{
		FlowID:    "f1",
		CycleDeps: []string{"a", "b"},
	}
	if cycleErr.Error() != "circular dependency detected" {
		t.Error("expected 'circular dependency detected' error message")
	}
}
