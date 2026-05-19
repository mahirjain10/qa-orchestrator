package trace

import (
	"os"
	"path/filepath"
	"testing"

	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

func setupTestStore(t *testing.T) (*TraceStore, string) {
	tmpDir := t.TempDir()
	store, err := NewTraceStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create trace store: %v", err)
	}
	return store, tmpDir
}

func TestNewTraceStore(t *testing.T) {
	store, _ := setupTestStore(t)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestAppend(t *testing.T) {
	store, _ := setupTestStore(t)

	event := sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "navigate", sharedtypes.TraceStatusSuccess)
	err := store.Append(event)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	events, err := store.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Action != "navigate" {
		t.Fatalf("expected action navigate, got %s", events[0].Action)
	}
}

func TestAppendBatch(t *testing.T) {
	store, _ := setupTestStore(t)

	events := []*sharedtypes.TraceEvent{
		sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "navigate", sharedtypes.TraceStatusSuccess),
		sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "click", sharedtypes.TraceStatusSuccess),
	}
	err := store.AppendBatch("run_1", events)
	if err != nil {
		t.Fatalf("append batch failed: %v", err)
	}

	retrieved, err := store.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(retrieved) != 2 {
		t.Fatalf("expected 2 events, got %d", len(retrieved))
	}
}

func TestGetByFlowID(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "navigate", sharedtypes.TraceStatusSuccess))
	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_2", "executor", sharedtypes.TraceEventStepExecution, "click", sharedtypes.TraceStatusSuccess))
	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "type", sharedtypes.TraceStatusSuccess))

	events, err := store.GetByFlowID("run_1", "flow_1")
	if err != nil {
		t.Fatalf("get by flow ID failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events for flow_1, got %d", len(events))
	}
}

func TestGetRecent(t *testing.T) {
	store, _ := setupTestStore(t)

	for i := 0; i < 10; i++ {
		store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "step", sharedtypes.TraceStatusSuccess))
	}

	recent, err := store.GetRecent("run_1", 3)
	if err != nil {
		t.Fatalf("get recent failed: %v", err)
	}
	if len(recent) != 3 {
		t.Fatalf("expected 3 recent events, got %d", len(recent))
	}
}

func TestPersistence(t *testing.T) {
	store, tmpDir := setupTestStore(t)

	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "navigate", sharedtypes.TraceStatusSuccess))
	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "click", sharedtypes.TraceStatusSuccess))

	newStore, err := NewTraceStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create new store: %v", err)
	}

	events, err := newStore.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get after reload failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after reload, got %d", len(events))
	}

	path := filepath.Join(tmpDir, "traces", "run_1.jsonl")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected trace file to exist")
	}
}

func TestDelete(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "navigate", sharedtypes.TraceStatusSuccess))

	err := store.Delete("run_1")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	events, err := store.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get after delete failed: %v", err)
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events after delete, got %d", len(events))
	}
}

func TestEmitHelpers(t *testing.T) {
	store, _ := setupTestStore(t)

	EmitStepExecution(store, "run_1", "flow_1", &agentstypes.StepResult{
		StepID:  "step_1",
		Tool:    "navigate",
		Params:  map[string]any{"url": "https://example.com"},
		Success: true,
	})

	EmitAgentDecision(store, "run_1", "flow_1", "planner", "replan", "UI mismatch detected")

	EmitRecoveryAction(store, "run_1", "flow_1", &agentstypes.RecoveryDecision{
		Action: agentstypes.RecoveryActionRetry,
		Reason: "temporary network error",
	}, nil)

	EmitLifecycleEvent(store, "run_1", "", sharedtypes.RunStateRunning, nil)

	EmitCheckpoint(store, "run_1", &sharedtypes.Checkpoint{
		FlowID:    "flow_1",
		StepIndex: 3,
		StepID:    "step_4",
	})

	EmitArtifactEvent(store, "run_1", "flow_1", "screenshot", "/path/to/screenshot.png", map[string]any{"size": 1024})

	events, err := store.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(events) != 6 {
		t.Fatalf("expected 6 events, got %d", len(events))
	}
}

func TestEmitHelpersWithNilStore(t *testing.T) {
	EmitStepExecution(nil, "run_1", "flow_1", &agentstypes.StepResult{Success: true})
	EmitAgentDecision(nil, "run_1", "flow_1", "planner", "replan", "reason")
	EmitRecoveryAction(nil, "run_1", "flow_1", &agentstypes.RecoveryDecision{Action: agentstypes.RecoveryActionFail}, nil)
	EmitLifecycleEvent(nil, "run_1", "", sharedtypes.RunStateRunning, nil)
	EmitCheckpoint(nil, "run_1", &sharedtypes.Checkpoint{})
	EmitArtifactEvent(nil, "run_1", "flow_1", "screenshot", "/path", nil)
}

func TestEmitRecoveryAction_NilDecision(t *testing.T) {
	store, _ := setupTestStore(t)

	// This should not panic
	EmitRecoveryAction(store, "run_1", "flow_1", nil, &agentstypes.StepResult{
		StepID: "step_1",
		Tool:   "navigate",
	})

	events, err := store.GetByRunID("run_1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	event := events[0]
	if event.Action != "pending" {
		t.Errorf("expected action pending, got %s", event.Action)
	}
	if event.Details["reason"] != "analyzing failure" {
		t.Errorf("expected reason 'analyzing failure', got %v", event.Details["reason"])
	}
	if event.Details["failed_step"] != "step_1" {
		t.Errorf("expected failed_step step_1, got %v", event.Details["failed_step"])
	}
}

func TestListRunIDs(t *testing.T) {
	store, _ := setupTestStore(t)

	store.Append(sharedtypes.NewTraceEvent("run_1", "flow_1", "executor", sharedtypes.TraceEventStepExecution, "navigate", sharedtypes.TraceStatusSuccess))
	store.Append(sharedtypes.NewTraceEvent("run_2", "flow_2", "executor", sharedtypes.TraceEventStepExecution, "click", sharedtypes.TraceStatusSuccess))

	runIDs, err := store.ListRunIDs()
	if err != nil {
		t.Fatalf("list run IDs failed: %v", err)
	}
	if len(runIDs) != 2 {
		t.Fatalf("expected 2 run IDs, got %d", len(runIDs))
	}
}
