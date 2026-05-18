package session

import (
	"os"
	"path/filepath"
	"testing"

	"qa-orchestrator/packages/shared/types"
)

func TestSessionStore_Create(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session, err := store.Create("test-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if session.RunID == "" {
		t.Error("expected non-empty run ID")
	}
	if session.SessionID == "" {
		t.Error("expected non-empty session ID")
	}
	if session.CampaignName != "test-campaign" {
		t.Errorf("expected campaign name 'test-campaign', got %q", session.CampaignName)
	}
	if session.Status != types.RunStatePending {
		t.Errorf("expected status PENDING, got %s", session.Status)
	}
}

func TestSessionStore_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	created, err := store.Create("test-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	retrieved, err := store.Get(created.RunID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}

	if retrieved.RunID != created.RunID {
		t.Errorf("expected run ID %s, got %s", created.RunID, retrieved.RunID)
	}
}

func TestSessionStore_Save(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session, err := store.Create("test-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session.Status = types.RunStateRunning
	err = store.Save(session)
	if err != nil {
		t.Fatalf("failed to save session: %v", err)
	}

	retrieved, err := store.Get(session.RunID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if retrieved.Status != types.RunStateRunning {
		t.Errorf("expected status RUNNING, got %s", retrieved.Status)
	}
}

func TestSessionStore_UpdateFlowState(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session, err := store.Create("test-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session.AddFlowState(types.FlowRunState{
		FlowID: "flow-1",
		Name:   "Test Flow",
		Status: types.FlowStatePending,
	})
	err = store.Save(session)
	if err != nil {
		t.Fatalf("failed to save session with flow: %v", err)
	}

	err = store.UpdateFlowState(session.RunID, "flow-1", types.FlowStatePassed, "")
	if err != nil {
		t.Fatalf("failed to update flow state: %v", err)
	}

	retrieved, err := store.Get(session.RunID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if len(retrieved.Flows) != 1 {
		t.Fatalf("expected 1 flow state, got %d", len(retrieved.Flows))
	}
	if retrieved.Flows[0].Status != types.FlowStatePassed {
		t.Errorf("expected flow status PASSED, got %s", retrieved.Flows[0].Status)
	}
}

func TestSessionStore_SaveCheckpoint(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session, err := store.Create("test-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	cp := &types.Checkpoint{
		FlowID:    "flow-1",
		StepID:    "step-1",
		StepIndex: 5,
		Payload:   map[string]any{"key": "value"},
	}

	err = store.SaveCheckpoint(session.RunID, cp)
	if err != nil {
		t.Fatalf("failed to save checkpoint: %v", err)
	}

	retrieved, err := store.Get(session.RunID)
	if err != nil {
		t.Fatalf("failed to get session: %v", err)
	}
	if retrieved.Checkpoint == nil {
		t.Fatal("expected checkpoint to be set")
	}
	if retrieved.Checkpoint.StepIndex != 5 {
		t.Errorf("expected step index 5, got %d", retrieved.Checkpoint.StepIndex)
	}
}

func TestSessionStore_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session, err := store.Create("test-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	err = store.Delete(session.RunID)
	if err != nil {
		t.Fatalf("failed to delete session: %v", err)
	}

	_, err = store.Get(session.RunID)
	if err == nil {
		t.Error("expected error for deleted session")
	}
}

func TestSessionStore_PersistenceOnDisk(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	session, err := store.Create("persistent-campaign")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	sessionFile := filepath.Join(tmpDir, "sessions", session.RunID+".json")
	if _, err := os.Stat(sessionFile); os.IsNotExist(err) {
		t.Error("expected session file to exist on disk")
	}
}
