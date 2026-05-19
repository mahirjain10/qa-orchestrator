package screens

import (
	"testing"
	"time"

	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func TestFetchSessionsCmdReturnsSessions(t *testing.T) {
	baseDir := t.TempDir()
	store, err := session.NewSessionStore(baseDir)
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}

	campaign := &types.Campaign{
		Name: "test",
		Flows: []types.Flow{{
			ID: "f1", Mode: types.FlowModeGuided, Priority: types.FlowPriorityHigh,
		}},
	}
	if _, err := store.Create(campaign); err != nil {
		t.Fatalf("create session: %v", err)
	}

	cmd := fetchSessionsCmd(store)
	msg := cmd()

	loaded, ok := msg.(sessionsLoadedMsg)
	if !ok {
		t.Fatalf("expected sessionsLoadedMsg, got %T", msg)
	}
	if len(loaded.sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(loaded.sessions))
	}
}

func TestFetchRunCmdReturnsNilForEmptyRunID(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := session.NewSessionStore(baseDir)

	cmd := fetchRunCmd(store, "")
	msg := cmd()

	if msg != nil {
		t.Fatalf("expected nil message for empty runID, got %T", msg)
	}
}

func TestFetchTracesCmdReturnsNilForEmptyRunID(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := trace.NewTraceStore(baseDir)

	cmd := fetchTracesCmd(store, "")
	msg := cmd()

	if msg != nil {
		t.Fatalf("expected nil message for empty runID, got %T", msg)
	}
}

func TestFetchTracesCmdReturnsNilForNilStore(t *testing.T) {
	cmd := fetchTracesCmd(nil, "run-123")
	msg := cmd()

	if msg != nil {
		t.Fatalf("expected nil message for nil store, got %T", msg)
	}
}

func TestFetchArtifactsCmdReturnsNilForEmptyRunID(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := artifact.NewArtifactStore(baseDir)

	cmd := fetchArtifactsCmd(store, "")
	msg := cmd()

	if msg != nil {
		t.Fatalf("expected nil message for empty runID, got %T", msg)
	}
}

func TestFetchArtifactsCmdReturnsNilForNilStore(t *testing.T) {
	cmd := fetchArtifactsCmd(nil, "run-123")
	msg := cmd()

	if msg != nil {
		t.Fatalf("expected nil message for nil store, got %T", msg)
	}
}

func TestStartRefreshTickerReturnsNilForEmptyRunID(t *testing.T) {
	cmd := startRefreshTicker("")
	if cmd != nil {
		t.Fatal("expected nil ticker for empty runID")
	}
}

func TestStartRefreshTickerReturnsCommandForValidRunID(t *testing.T) {
	cmd := startRefreshTicker("run-123")
	if cmd == nil {
		t.Fatal("expected ticker command for valid runID")
	}
}

func TestRefreshAllCmdReturnsBatch(t *testing.T) {
	baseDir := t.TempDir()
	sessionStore, _ := session.NewSessionStore(baseDir)
	traceStore, _ := trace.NewTraceStore(baseDir)
	artifactStore, _ := artifact.NewArtifactStore(baseDir)

	cmd := refreshAllCmd("run-123", sessionStore, traceStore, artifactStore, nil)
	if cmd == nil {
		t.Fatal("expected batch command")
	}
}

func TestSessionsLoadedMsgUpdatesCampaignList(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	sessions := []*types.Session{
		{RunID: "run-1", CampaignName: "Campaign A"},
		{RunID: "run-2", CampaignName: "Campaign B"},
	}

	model, _ := screen.Update(sessionsLoadedMsg{sessions: sessions})
	updated := model.(*MainScreen)

	if len(updated.sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(updated.sessions))
	}
	names := updated.campaignNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 campaign names, got %d", len(names))
	}
}

func TestRunLoadedMsgUpdatesCurrentRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	sess := &types.Session{
		RunID:        runID,
		CampaignName: "test",
		Status:       types.RunStateRunning,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning},
			{FlowID: "flow-2", Status: types.FlowStatePending},
		},
	}

	model, _ := screen.Update(runLoadedMsg{run: sess})
	updated := model.(*MainScreen)

	if updated.currentRun == nil {
		t.Fatal("expected currentRun to be set")
	}
	if updated.currentRun.RunID != runID {
		t.Fatalf("expected runID %s, got %s", runID, updated.currentRun.RunID)
	}
	if updated.flowStatus.GetSelected() != 0 {
		t.Fatalf("expected flow selected index 0, got %d", updated.flowStatus.GetSelected())
	}
}

func TestErrMsgDoesNotCrash(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(errMsg{err: errTest})
	updated := model.(*MainScreen)

	if updated.msg == "" {
		t.Fatal("expected error message to be set")
	}
}

var errTest = &testError{text: "test error"}

type testError struct{ text string }

func (e *testError) Error() string { return e.text }

func TestTickMsgTriggersRefresh(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	model, cmd := screen.Update(tickMsg{})
	updated := model.(*MainScreen)

	if updated.currentRun == nil {
		t.Fatal("expected currentRun to persist after tick")
	}
	if cmd == nil {
		t.Fatal("expected refresh command after tick")
	}
}

func TestSessionsLoadedWithNoSessions(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(sessionsLoadedMsg{sessions: []*types.Session{}})
	updated := model.(*MainScreen)

	if len(updated.sessions) != 0 {
		t.Fatalf("expected 0 sessions, got %d", len(updated.sessions))
	}
}

func TestRunLoadedWithNilRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(runLoadedMsg{run: nil})
	updated := model.(*MainScreen)

	if updated.currentRun != nil {
		t.Fatal("expected currentRun to remain nil when nil run loaded")
	}
}

func TestReportLoadedMsgSetsReportView(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(reportLoadedMsg{report: "Test report content"})
	updated := model.(*MainScreen)

	if updated.reportView != "Test report content" {
		t.Fatalf("expected report view to be set, got %q", updated.reportView)
	}
}

func TestArtifactsLoadedMsgUpdatesPanel(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	artifacts := []*artifact.Artifact{
		{ArtifactID: "art-1", Type: artifact.ArtifactLog, Path: "/tmp/run.log", Size: 100},
		{ArtifactID: "art-2", Type: artifact.ArtifactScreenshot, Path: "/tmp/screen.png", Size: 5000},
	}

	model, _ := screen.Update(artifactsLoadedMsg{artifacts: artifacts})
	updated := model.(*MainScreen)

	if len(updated.artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(updated.artifacts))
	}
}

func TestTracesLoadedMsgUpdatesPanel(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	traces := []*types.TraceEvent{
		types.NewTraceEvent(runID, "flow-1", "executor", types.TraceEventStepExecution, "click", types.TraceStatusSuccess),
		types.NewTraceEvent(runID, "flow-1", "executor", types.TraceEventStepExecution, "type", types.TraceStatusFailed),
	}

	model, _ := screen.Update(tracesLoadedMsg{traces: traces})
	updated := model.(*MainScreen)

	if len(updated.traces) != 2 {
		t.Fatalf("expected 2 traces, got %d", len(updated.traces))
	}
}

func TestFetchRunCmdReturnsErrorForNonexistentRun(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := session.NewSessionStore(baseDir)

	cmd := fetchRunCmd(store, "nonexistent-run-id")
	msg := cmd()

	_, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("expected errMsg for nonexistent run, got %T", msg)
	}
}

func TestFetchTracesCmdReturnsEmptyForNonexistentRun(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := trace.NewTraceStore(baseDir)

	cmd := fetchTracesCmd(store, "nonexistent-run-id")
	msg := cmd()

	loaded, ok := msg.(tracesLoadedMsg)
	if !ok {
		t.Fatalf("expected tracesLoadedMsg, got %T", msg)
	}
	if len(loaded.traces) != 0 {
		t.Fatalf("expected 0 traces for nonexistent run, got %d", len(loaded.traces))
	}
}

func TestFetchArtifactsCmdReturnsEmptyForNonexistentRun(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := artifact.NewArtifactStore(baseDir)

	cmd := fetchArtifactsCmd(store, "nonexistent-run-id")
	msg := cmd()

	loaded, ok := msg.(artifactsLoadedMsg)
	if !ok {
		t.Fatalf("expected artifactsLoadedMsg, got %T", msg)
	}
	if len(loaded.artifacts) != 0 {
		t.Fatalf("expected 0 artifacts for nonexistent run, got %d", len(loaded.artifacts))
	}
}

func TestUpdateFromStoresWithNoCurrentRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	screen.updateFromStores()

	if len(screen.sessions) == 0 {
		t.Fatal("expected sessions to be loaded even without current run")
	}
}

func TestUpdateFromStoresWithCurrentRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	event := types.NewTraceEvent(runID, "flow-1", "executor", types.TraceEventStepExecution, "test_action", types.TraceStatusSuccess)
	if err := screen.traceStore.Append(event); err != nil {
		t.Fatalf("append trace: %v", err)
	}

	screen.updateFromStores()

	if screen.currentRun == nil {
		t.Fatal("expected currentRun to be refreshed")
	}
	if len(screen.traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(screen.traces))
	}
}

func TestCurrentRunIDReturnsEmptyWhenNil(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	if screen.currentRunID() != "" {
		t.Fatalf("expected empty runID, got %q", screen.currentRunID())
	}
}

func TestCurrentRunIDReturnsIDWhenSet(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	if screen.currentRunID() != runID {
		t.Fatalf("expected runID %q, got %q", runID, screen.currentRunID())
	}
}

func TestSetMsgUpdatesTimestamp(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.msg = "old"
	screen.msgTime = time.Now().Add(-1 * time.Hour)

	screen.setMsg("new message")

	if screen.msg != "new message" {
		t.Fatalf("expected msg to be updated, got %q", screen.msg)
	}
	if time.Since(screen.msgTime) > time.Second {
		t.Fatal("expected msgTime to be recent")
	}
}
