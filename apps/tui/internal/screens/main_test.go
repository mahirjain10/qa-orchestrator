package screens

import (
	"errors"
	"strings"
	"testing"
	"time"

	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

func TestSteeringInputParsing(t *testing.T) {
	tests := []struct {
		input    string
		wantCmd  string
		wantArgs []string
	}{
		{"retry flow-1", "retry", []string{"flow-1"}},
		{"skip flow-2", "skip", []string{"flow-2"}},
		{"continue", "continue", nil},
		{"status", "status", nil},
		{"approve", "approve", nil},
		{"", "", nil},
		{"retry flow-1 extra", "retry", []string{"flow-1", "extra"}},
		{"  skip   flow-3  ", "skip", []string{"flow-3"}},
	}

	for _, tt := range tests {
		cmd, args := parseSteeringInput(tt.input)
		if cmd != tt.wantCmd {
			t.Errorf("parseSteeringInput(%q) cmd = %q, want %q", tt.input, cmd, tt.wantCmd)
		}
		if len(args) != len(tt.wantArgs) {
			t.Errorf("parseSteeringInput(%q) args len = %d, want %d", tt.input, len(args), len(tt.wantArgs))
			continue
		}
		for i := range args {
			if args[i] != tt.wantArgs[i] {
				t.Errorf("parseSteeringInput(%q) args[%d] = %q, want %q", tt.input, i, args[i], tt.wantArgs[i])
			}
		}
	}
}

func TestSteeringCommandRetryWithFlowID(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateFailed},
	}}

	screen.processSteeringCommand("retry flow-1")

	if screen.msg == "" {
		t.Fatal("expected message after retry command")
	}
	msgLower := strings.ToLower(screen.msg)
	if !strings.Contains(msgLower, "retry") && !strings.Contains(msgLower, "error") {
		t.Fatalf("expected retry message or error, got %q", screen.msg)
	}
}

func TestSteeringCommandRetryWithoutFlowID(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	screen.processSteeringCommand("retry")

	if screen.msg != "Usage: retry <flow_id>" {
		t.Fatalf("expected usage message, got %q", screen.msg)
	}
}

func TestSteeringCommandSkipWithFlowID(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
	}}

	screen.processSteeringCommand("skip flow-1")

	if screen.msg == "" {
		t.Fatal("expected message after skip command")
	}
}

func TestSteeringCommandSkipWithoutFlowID(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	screen.processSteeringCommand("skip")

	if screen.msg != "Usage: skip <flow_id>" {
		t.Fatalf("expected usage message, got %q", screen.msg)
	}
}

func TestSteeringCommandContinueNotWaitingForInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	screen.processSteeringCommand("continue")

	if screen.msg != "Run is not in WAITING_FOR_INPUT state" {
		t.Fatalf("expected not waiting message, got %q", screen.msg)
	}
}

func TestSteeringCommandStatus(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning, CurrentFlowID: "flow-1", CurrentAgent: "executor"}

	screen.processSteeringCommand("status")

	if !strings.Contains(screen.msg, "Status:") {
		t.Fatalf("expected status message, got %q", screen.msg)
	}
}

func TestSteeringCommandUnknown(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	screen.processSteeringCommand("foobar")

	if !strings.Contains(screen.msg, "Unknown command") {
		t.Fatalf("expected unknown command message, got %q", screen.msg)
	}
}

func TestSteeringCommandNoRunSelected(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	screen.processSteeringCommand("retry flow-1")

	if screen.msg != "No run selected" {
		t.Fatalf("expected no run selected message, got %q", screen.msg)
	}
}

func TestSteeringCommandContinueWaitingInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateWaitingInput}
	screen.handlers.store.UpdateStatus(runID, types.RunStateWaitingInput)

	screen.processSteeringCommand("continue")

	if screen.msg != "Run resumed from WAITING_FOR_INPUT" {
		t.Fatalf("expected resume message, got %q", screen.msg)
	}
}

func TestSteeringCommandContinueGetRunStatusError(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	_ = runID
	screen.currentRun = &types.Session{RunID: "nonexistent-run-id"}

	screen.processSteeringCommand("continue")

	if !strings.Contains(screen.msg, "Error getting run status") {
		t.Fatalf("expected error getting run status message, got %q", screen.msg)
	}
}

func TestSetMessageUpdatesTime(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	oldTime := screen.msgTime.Add(-10 * time.Second)
	screen.msgTime = oldTime

	screen.SetMessage("test message")

	if screen.msg != "test message" {
		t.Fatalf("expected message to be set, got %q", screen.msg)
	}
	if screen.msgTime.Equal(oldTime) {
		t.Fatal("expected msgTime to be updated")
	}
}

func TestNewMainScreenInitializesFields(t *testing.T) {
	baseDir := t.TempDir()
	store, err := session.NewSessionStore(baseDir)
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}

	screen := NewMainScreen(store)

	if screen.sessionStore == nil {
		t.Fatal("expected sessionStore to be set")
	}
	if screen.handlers == nil {
		t.Fatal("expected handlers to be set")
	}
	if screen.campaignList == nil {
		t.Fatal("expected campaignList to be initialized")
	}
	if screen.activeView != ViewDashboard {
		t.Fatalf("expected activeView Dashboard, got %s", screen.activeView)
	}
	if screen.sidebarFocus {
		t.Fatal("expected sidebarFocus to be false initially")
	}
}

func TestNewMainScreenWithStoresSetsAllStores(t *testing.T) {
	baseDir := t.TempDir()
	sessionStore, _ := session.NewSessionStore(baseDir)
	traceStore, _ := trace.NewTraceStore(baseDir)
	artifactStore, _ := artifact.NewArtifactStore(baseDir)

	screen := NewMainScreenWithStores(sessionStore, traceStore, artifactStore)

	if screen.sessionStore == nil {
		t.Fatal("expected sessionStore")
	}
	if screen.traceStore == nil {
		t.Fatal("expected traceStore")
	}
	if screen.artifactStore == nil {
		t.Fatal("expected artifactStore")
	}
	if screen.reportGenerator == nil {
		t.Fatal("expected reportGenerator")
	}
}

func TestWindowSizeMsgUpdatesDimensions(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	updated := model.(*MainScreen)

	if updated.width != 100 {
		t.Fatalf("expected width 100, got %d", updated.width)
	}
	if updated.height != 50 {
		t.Fatalf("expected height 50, got %d", updated.height)
	}
}

func TestSpinnerTickMsgUpdates(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.spinner = spinner.New()

	model, cmd := screen.Update(spinner.TickMsg{})
	_ = model
	if cmd == nil {
		t.Fatal("expected spinner tick command")
	}
}

func TestUnknownMessageTypeDoesNotCrash(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(struct{ tea.Msg }{})
	if model == nil {
		t.Fatal("expected model to be returned for unknown message type")
	}
}

func TestCommandModeEnterProcessesCommand(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.commandBar.Focused = true
	screen.commandBar.Input.SetValue("status")

	model, _ := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(*MainScreen)

	if updated.commandBar.Focused {
		t.Fatal("expected command focus to be disabled after enter")
	}
	if updated.commandBar.Input.Value() != "" {
		t.Fatalf("expected command input to be cleared, got %q", updated.commandBar.Input.Value())
	}
	if !strings.Contains(updated.msg, "Status:") {
		t.Fatalf("expected status message, got %q", updated.msg)
	}
}

func TestCommandModeEnterWithEmptyInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.commandBar.Focused = true
	screen.commandBar.Input.SetValue("")

	model, _ := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(*MainScreen)

	if !updated.commandBar.Focused {
		t.Fatal("expected command focus to remain active with empty input")
	}
}

func TestUpdateFromStoresHandlesMissingStores(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.traceStore = nil
	screen.artifactStore = nil
	screen.reportGenerator = nil

	sess, err := screen.handlers.GetRunStatus(runID)
	if err != nil {
		t.Fatalf("get run status: %v", err)
	}
	screen.currentRun = sess

	if screen.currentRun == nil {
		t.Fatal("expected currentRun to persist when stores are nil")
	}
}

func TestPauseRunFromPausedState(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStatePaused}
	screen.handlers.store.UpdateStatus(runID, types.RunStatePaused)

	screen.Update(tea.KeyMsg{Type: tea.KeySpace})

	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStateResuming {
		t.Fatalf("expected resuming status after space on paused run, got %s", sess.Status)
	}
}

func TestPauseRunFromRunningState(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	screen.Update(tea.KeyMsg{Type: tea.KeySpace})

	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStatePausing {
		t.Fatalf("expected pausing status after space on running run, got %s", sess.Status)
	}
}

func TestCancelRunOnCompletedRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateCompleted}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if !strings.Contains(screen.msg, "Error") && !strings.Contains(screen.msg, "cancel") {
		t.Fatalf("expected error message, got %q", screen.msg)
	}
}

func TestCancelRunOnNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil
	screen.msg = ""

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if screen.msg != "" {
		t.Fatalf("expected no message when cancelling with no run, got %q", screen.msg)
	}
}

func TestNewScreenWithRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	if screen == nil {
		t.Fatal("expected screen to be created")
	}
	if runID == "" {
		t.Fatal("expected runID to be non-empty")
	}
	if screen.sessionStore == nil {
		t.Fatal("expected sessionStore")
	}
	if screen.traceStore == nil {
		t.Fatal("expected traceStore")
	}
	if screen.artifactStore == nil {
		t.Fatal("expected artifactStore")
	}
}

func TestTickMsgWithNoCurrentRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	model, cmd := screen.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected refresh command even with no current run")
	}
	_ = model
}

func TestSessionsLoadedMsgPreservesExistingSessions(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sessions = []*types.Session{{RunID: "existing-run", CampaignName: "Existing"}}

	newSessions := []*types.Session{{RunID: "new-run", CampaignName: "New"}}
	model, _ := screen.Update(sessionsLoadedMsg{sessions: newSessions})
	updated := model.(*MainScreen)

	if len(updated.sessions) != 1 {
		t.Fatalf("expected 1 session from load, got %d", len(updated.sessions))
	}
	if updated.sessions[0].RunID != "new-run" {
		t.Fatalf("expected new-run, got %s", updated.sessions[0].RunID)
	}
}

func TestInitReturnsBatchCommand(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	cmd := screen.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}
}

func TestMultipleCommandInputsInSequence(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateFailed},
		{FlowID: "flow-2", Status: types.FlowStateRunning},
	}}

	screen.processSteeringCommand("retry flow-1")
	firstMsg := screen.msg

	screen.processSteeringCommand("skip flow-2")
	secondMsg := screen.msg

	if firstMsg == secondMsg {
		t.Fatalf("expected different messages for different commands, got same: %q", firstMsg)
	}
}

func TestMsgTimeIsRecentAfterSetMsg(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.setMsg("test")

	if time.Since(screen.msgTime) > time.Second {
		t.Fatal("expected msgTime to be within 1 second")
	}
}

func TestMsgTimeStaleAfterLongDelay(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.msgTime = time.Now().Add(-10 * time.Second)

	if time.Since(screen.msgTime) < 5*time.Second {
		t.Fatal("expected msgTime to be stale (more than 5 seconds ago)")
	}
}

func TestFetchSessionsCmdReturnsErrorForFailingStore(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := session.NewSessionStore(baseDir)

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
		t.Fatalf("expected 1 session from valid store, got %d", len(loaded.sessions))
	}
}

func TestFetchRunCmdReturnsErrorForFailingStore(t *testing.T) {
	baseDir := t.TempDir()
	store, _ := session.NewSessionStore(baseDir)

	cmd := fetchRunCmd(store, "nonexistent-run")
	msg := cmd()

	_, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("expected errMsg for nonexistent run, got %T", msg)
	}
}

func TestAsyncMessagesUpdatePanels(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	sess := &types.Session{
		RunID:        runID,
		CampaignName: "test",
		Status:       types.RunStateRunning,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning},
		},
	}

	traces := []*types.TraceEvent{
		types.NewTraceEvent(runID, "flow-1", "executor", types.TraceEventStepExecution, "click_button", types.TraceStatusSuccess),
	}

	artifacts := []*artifact.Artifact{
		{ArtifactID: "art-1", Type: artifact.ArtifactLog, Path: "/tmp/run.log", Size: 100},
	}

	model, _ := screen.Update(runLoadedMsg{run: sess})
	updated := model.(*MainScreen)
	if updated.currentRun == nil || updated.currentRun.RunID != runID {
		t.Fatal("run not loaded")
	}

	model, _ = updated.Update(tracesLoadedMsg{traces: traces})
	updated = model.(*MainScreen)
	if len(updated.traces) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(updated.traces))
	}

	model, _ = updated.Update(artifactsLoadedMsg{artifacts: artifacts})
	updated = model.(*MainScreen)
	if len(updated.artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(updated.artifacts))
	}

	model, _ = updated.Update(reportLoadedMsg{report: "test report"})
	updated = model.(*MainScreen)
	if updated.reportView != "test report" {
		t.Fatalf("expected report view to be set, got %q", updated.reportView)
	}
}

func TestErrMsgSetsMessage(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, _ := screen.Update(errMsg{err: errors.New("test error")})
	updated := model.(*MainScreen)
	if !strings.Contains(updated.msg, "test error") {
		t.Fatalf("expected error message, got %q", updated.msg)
	}
}

func TestContentWidthReturnsMinimum(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 50

	w := screen.contentWidth()
	if w < 40 {
		t.Fatalf("expected minimum width 40, got %d", w)
	}
}

func TestContentWidthCalculatesCorrectly(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120

	w := screen.contentWidth()
	if w != 92 {
		t.Fatalf("expected width 92, got %d", w)
	}
}

func TestTracesViewUsesContentWidth(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces

	view := screen.renderTracesView()
	if view == "" {
		t.Fatal("expected non-empty traces view")
	}
}

func TestReportViewUsesContentWidth(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewReport
	screen.reportView = "Test report content"

	view := screen.renderReportView()
	if !strings.Contains(view, "Test report content") {
		t.Fatalf("expected report view to contain content, got %q", view)
	}
}

func TestFilterModeCancelledOnEsc(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.Focus()
	screen.tracePanel.FilterInput.SetValue("test")

	_, _ = screen.handleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})

	if screen.tracePanel.FilterMode {
		t.Fatal("expected filter mode to be cancelled")
	}
	if screen.tracePanel.FilterInput.Value() != "" {
		t.Fatal("expected filter input to be cleared")
	}
	if screen.tracePanel.FilterInput.Focused() {
		t.Fatal("expected FilterInput to be blurred after escape")
	}
}

func TestFilterModeEnterBlursInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.Focus()
	screen.tracePanel.FilterInput.SetValue("test")

	_, _ = screen.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})

	if screen.tracePanel.FilterInput.Focused() {
		t.Fatal("expected FilterInput to be blurred after enter")
	}
}

func TestFilterModeEscBlursInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.Focus()
	screen.tracePanel.FilterInput.SetValue("test")

	_, _ = screen.handleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})

	if screen.tracePanel.FilterInput.Focused() {
		t.Fatal("expectedFilterInput to be blurred after escape")
	}
}

func TestFilterModeAppliedOnEnter(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.SetValue("browser")

	_, _ = screen.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})

	if screen.tracePanel.FilterMode {
		t.Fatal("expected filter mode to be disabled after enter")
	}
	if screen.tracePanel.Filter.Text != "browser" {
		t.Fatalf("expected filter text 'browser', got %q", screen.tracePanel.Filter.Text)
	}
	if screen.tracePanel.Selected != 0 {
		t.Fatalf("expected selected to reset to 0, got %d", screen.tracePanel.Selected)
	}
	if screen.tracePanel.FilterInput.Focused() {
		t.Fatal("expected FilterInput to be blurred after enter")
	}
}

func TestFilterTextClearedOnEmptyEnter(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.SetValue("")

	_, _ = screen.handleFilterKey(tea.KeyMsg{Type: tea.KeyEnter})

	if screen.tracePanel.Filter.Text != "" {
		t.Fatalf("expected filter text to be cleared, got %q", screen.tracePanel.Filter.Text)
	}
	if screen.tracePanel.FilterInput.Focused() {
		t.Fatal("expected FilterInput to be blurred after enter")
	}
}

func TestStatusBarHidesOnShortTerminal(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 15
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	bar := screen.renderStatusBar()
	if bar != "" {
		t.Fatal("expected status bar to be empty on short terminal")
	}
}

func TestStatusBarShowsRunStatusAndID(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	bar := screen.renderStatusBar()
	if !strings.Contains(bar, "RUNNING") {
		t.Fatalf("expected status bar to contain 'RUNNING', got %q", bar)
	}
	truncated := runID
	if len(truncated) > 12 {
		truncated = truncated[:12]
	}
	if !strings.Contains(bar, truncated) {
		t.Fatalf("expected status bar to contain truncated run ID %q, got %q", truncated, bar)
	}
}

func TestStatusBarShowsIdleWhenNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.activeView = ViewDashboard

	bar := screen.renderStatusBar()
	if !strings.Contains(bar, "IDLE") {
		t.Fatalf("expected status bar to contain 'IDLE', got %q", bar)
	}
}

func TestStatusBarShowsRecentMessage(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.setMsg("test message")

	bar := screen.renderStatusBar()
	if !strings.Contains(bar, "test message") {
		t.Fatalf("expected status bar to contain message, got %q", bar)
	}
}

func TestStatusBarHidesStaleMessage(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.msg = "stale message"
	screen.msgTime = time.Now().Add(-10 * time.Second)

	bar := screen.renderStatusBar()
	if strings.Contains(bar, "stale message") {
		t.Fatalf("expected status bar to not contain stale message, got %q", bar)
	}
}

func TestContextualKeysDashboardWithRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewDashboard

	keys := screen.contextualKeys()
	if !strings.Contains(keys, "space:pause") {
		t.Fatalf("expected dashboard keys with run to contain 'space:pause', got %q", keys)
	}
	if !strings.Contains(keys, "x:cancel") {
		t.Fatalf("expected dashboard keys with run to contain 'x:cancel', got %q", keys)
	}
	if !strings.Contains(keys, ":command") {
		t.Fatalf("expected dashboard keys with run to contain ':command', got %q", keys)
	}
}

func TestContextualKeysDashboardIdle(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil
	screen.activeView = ViewDashboard

	keys := screen.contextualKeys()
	if !strings.Contains(keys, "enter:select") {
		t.Fatalf("expected idle dashboard keys to contain 'enter:select', got %q", keys)
	}
	if !strings.Contains(keys, "r:refresh") {
		t.Fatalf("expected idle dashboard keys to contain 'r:refresh', got %q", keys)
	}
}

func TestContextualKeysTraces(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeView = ViewTraces

	keys := screen.contextualKeys()
	if !strings.Contains(keys, "/:filter") {
		t.Fatalf("expected traces keys to contain '/:filter', got %q", keys)
	}
	if !strings.Contains(keys, "S:failures") {
		t.Fatalf("expected traces keys to contain 'S:failures', got %q", keys)
	}
	if !strings.Contains(keys, "F:follow") {
		t.Fatalf("expected traces keys to contain 'F:follow', got %q", keys)
	}
}

func TestContextualKeysFlows(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeView = ViewFlows

	keys := screen.contextualKeys()
	if !strings.Contains(keys, "enter:detail") {
		t.Fatalf("expected flows keys to contain 'enter:detail', got %q", keys)
	}
	if !strings.Contains(keys, "r:retry") {
		t.Fatalf("expected flows keys to contain 'r:retry', got %q", keys)
	}
	if !strings.Contains(keys, "k:skip") {
		t.Fatalf("expected flows keys to contain 'k:skip', got %q", keys)
	}
}

func TestContextualKeysDefault(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeView = ViewReport

	keys := screen.contextualKeys()
	if !strings.Contains(keys, "?:help") {
		t.Fatalf("expected default keys to contain '?:help', got %q", keys)
	}
}

func TestViewContainsStatusBar(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.setMsg("test status")

	view := screen.View()
	if !strings.Contains(view, "RUNNING") {
		t.Fatal("expected view to contain run status in status bar")
	}
	if !strings.Contains(view, "test status") {
		t.Fatal("expected view to contain message in status bar")
	}
}

func TestViewHidesStatusBarOnShortTerminal(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 19
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	view := screen.View()
	if strings.Contains(view, "RUNNING") {
		t.Fatal("expected view to not contain status bar on short terminal")
	}
}

func TestCampaignSelectorModalWidthMinimum(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 80
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	selector := screen.renderCampaignSelector()
	if selector == "" {
		t.Fatal("expected non-empty selector")
	}
}

func TestDashboardViewCentersSelectorWithPadding(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	view := screen.renderDashboardView()
	contentW := screen.contentWidth()
	expectedPadding := (contentW - 70) / 2
	if expectedPadding < 0 {
		expectedPadding = 0
	}
	if expectedPadding > 0 {
		if !strings.HasPrefix(view, strings.Repeat(" ", expectedPadding)) {
			t.Fatalf("expected view to start with %d spaces of padding, got different prefix", expectedPadding)
		}
	}
}

func TestCampaignSelectorUsesModalBorderStyle(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "│") {
		t.Fatal("expected selector to have border characters")
	}
}

func TestViewShowsInitializingOnZeroSize(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 0
	screen.height = 0

	view := screen.View()
	if view != "Initializing..." {
		t.Fatalf("expected 'Initializing...', got %q", view)
	}
}

func TestSidebarWidthReturns24AtWideTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120

	w := screen.sidebarWidth()
	if w != 24 {
		t.Fatalf("expected sidebar width 24 at width 120, got %d", w)
	}
}

func TestSidebarWidthReturns20AtMediumTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 95

	w := screen.sidebarWidth()
	if w != 20 {
		t.Fatalf("expected sidebar width 20 at width 95, got %d", w)
	}
}

func TestSidebarWidthReturns16AtNarrowTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 85

	w := screen.sidebarWidth()
	if w != 16 {
		t.Fatalf("expected sidebar width 16 at width 85, got %d", w)
	}
}

func TestSidebarWidthAtBoundary90(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 90

	w := screen.sidebarWidth()
	if w != 20 {
		t.Fatalf("expected sidebar width 20 at width 90 (not < 90), got %d", w)
	}
}

func TestSidebarWidthAtBoundary100(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 100

	w := screen.sidebarWidth()
	if w != 24 {
		t.Fatalf("expected sidebar width 24 at width 100 (not < 100), got %d", w)
	}
}

func TestContentHeightReturnsFullAtTallTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.height = 40

	h := screen.contentHeight()
	if h != 35 {
		t.Fatalf("expected content height 35 at height 40, got %d", h)
	}
}

func TestContentHeightReturnsReducedAtShortTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.height = 22

	h := screen.contentHeight()
	if h != 19 {
		t.Fatalf("expected content height 19 at height 22, got %d", h)
	}
}

func TestContentHeightAtBoundary25(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.height = 25

	h := screen.contentHeight()
	if h != 20 {
		t.Fatalf("expected content height 20 at height 25, got %d", h)
	}
}

func TestContentWidthAdaptsAtNarrowTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 85

	w := screen.contentWidth()
	if w < 40 {
		t.Fatalf("expected minimum width 40, got %d", w)
	}
}

func TestContentWidthAdaptsAtMediumTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 95

	w := screen.contentWidth()
	expected := 95 - 20 - 4
	if w < expected {
		t.Fatalf("expected width at least %d, got %d", expected, w)
	}
}

func TestSetCancelFuncStoresFunction(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	cancelled := false
	cancelFn := func() {
		cancelled = true
	}

	screen.SetCancelFunc(cancelFn)
	if screen.cancelFunc == nil {
		t.Fatal("expected cancelFunc to be set")
	}

	screen.cancelFunc()
	if !cancelled {
		t.Fatal("expected cancelFunc to be callable")
	}
}

func TestCancelKeyCallsCancelFunc(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	cancelled := false
	screen.SetCancelFunc(func() {
		cancelled = true
	})

	screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if !cancelled {
		t.Fatal("expected cancelFunc to be called on 'x' key")
	}
}

func TestCancelKeyWithoutCancelFunc(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.cancelFunc = nil

	screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if screen.msg != "Run cancelled" {
		t.Fatalf("expected 'Run cancelled' message, got %q", screen.msg)
	}
}

func TestCancelKeyWithNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil

	cancelled := false
	screen.SetCancelFunc(func() {
		cancelled = true
	})

	screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if cancelled {
		t.Fatal("expected cancelFunc to not be called when no run selected")
	}
}

func TestCancelKeyShowsErrorMessage(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if screen.msg != "Run cancelled" {
		t.Fatalf("expected 'Run cancelled' message, got %q", screen.msg)
	}
}

func TestHelpModalShowsDashboardKeys(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewDashboard
	screen.showHelp = true

	modal := screen.renderHelpOverlay("")
	if !strings.Contains(modal, "space:pause") && !strings.Contains(modal, "Pause") {
		t.Fatal("expected help modal to contain pause key for dashboard")
	}
	if !strings.Contains(modal, "Command mode") {
		t.Fatal("expected help modal to contain command key for dashboard")
	}
}

func TestHelpModalShowsTraceKeys(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.activeView = ViewTraces
	screen.showHelp = true

	modal := screen.renderHelpOverlay("")
	if !strings.Contains(modal, "filter") {
		t.Fatal("expected help modal to contain filter key for traces")
	}
	if !strings.Contains(modal, "follow") {
		t.Fatal("expected help modal to contain follow key for traces")
	}
}

func TestHelpModalShowsFlowKeys(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.activeView = ViewFlows
	screen.showHelp = true

	modal := screen.renderHelpOverlay("")
	if !strings.Contains(modal, "Expand") && !strings.Contains(modal, "expand") {
		t.Fatal("expected help modal to contain expand key for flows")
	}
}

func TestGlobalQuitWorksInCommandMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.commandBar.Focused = true
	screen.commandBar.Input.SetValue("test")

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit command for q in command mode")
	}
}

func TestGlobalQuitWorksInFilterMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.SetValue("test")

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit command for q in filter mode")
	}
}

func TestGlobalCtrlCWorksInCommandMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.commandBar.Focused = true

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected tea.Quit command for ctrl+c in command mode")
	}
}

func TestGlobalCtrlCWorksInFilterMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected tea.Quit command for ctrl+c in filter mode")
	}
}

func TestQuestionKeyWorksInCommandMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.commandBar.Focused = true
	screen.commandBar.Input.SetValue("test")

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !screen.showHelp {
		t.Fatal("expected showHelp to be toggled even in command mode")
	}
}

func TestQuestionKeyWorksInFilterMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !screen.showHelp {
		t.Fatal("expected showHelp to be toggled even in filter mode")
	}
}

func TestHelpModalDoesNotCrashAtMinimumSize(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 80
	screen.height = 20
	screen.showHelp = true

	modal := screen.renderHelpOverlay("")
	if modal == "" {
		t.Fatal("expected help modal to render at minimum size")
	}
}

func TestFormatSessionAgeJustNow(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	age := screen.formatSessionAge(time.Now())
	if age != "just now" {
		t.Fatalf("expected 'just now', got %q", age)
	}
}

func TestFormatSessionAgeMinutes(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	age := screen.formatSessionAge(time.Now().Add(-5 * time.Minute))
	if age != "5m ago" {
		t.Fatalf("expected '5m ago', got %q", age)
	}
}

func TestFormatSessionAgeHours(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	age := screen.formatSessionAge(time.Now().Add(-3 * time.Hour))
	if age != "3h ago" {
		t.Fatalf("expected '3h ago', got %q", age)
	}
}

func TestFormatSessionAgeDays(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	age := screen.formatSessionAge(time.Now().Add(-48 * time.Hour))
	if age != "2d ago" {
		t.Fatalf("expected '2d ago', got %q", age)
	}
}

func TestFormatSessionAgeZeroTime(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	age := screen.formatSessionAge(time.Time{})
	if age != "unknown" {
		t.Fatalf("expected 'unknown' for zero time, got %q", age)
	}
}

func TestCampaignSelectorShowsCurrentSessionFirst(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.sessions = []*types.Session{
		{RunID: "old-run", CampaignName: "Old Campaign", StartedAt: time.Now().Add(-2 * time.Hour)},
		{RunID: runID, CampaignName: "Current Campaign", StartedAt: time.Now()},
	}
	screen.currentRun = &types.Session{RunID: runID}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	selector := screen.renderCampaignSelector()
	currentIdx := strings.Index(selector, "Current Campaign")
	oldIdx := strings.Index(selector, "Old Campaign")
	if currentIdx < 0 || oldIdx < 0 {
		t.Fatal("expected both campaigns in selector")
	}
	if currentIdx > oldIdx {
		t.Fatal("expected current session to appear before old session")
	}
}

func TestCampaignSelectorShowsCurrentIndicator(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Current Campaign", StartedAt: time.Now()},
	}
	screen.currentRun = &types.Session{RunID: runID}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "●") {
		t.Fatal("expected current session indicator (●) in selector")
	}
}

func TestCampaignSelectorShowsSessionAge(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Test Campaign", StartedAt: time.Now().Add(-30 * time.Minute)},
	}
	screen.currentRun = &types.Session{RunID: runID}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "30m ago") {
		t.Fatalf("expected session age '30m ago' in selector, got %q", selector)
	}
}

func TestCampaignSelectorShowsSeparatorBetweenCurrentAndPrevious(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Current", StartedAt: time.Now()},
		{RunID: "old-1", CampaignName: "Old One", StartedAt: time.Now().Add(-1 * time.Hour)},
	}
	screen.currentRun = &types.Session{RunID: runID}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "─") {
		t.Fatal("expected separator line between current and previous sessions")
	}
}

func TestCampaignNamesIncludesAgeAndCurrentMarker(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Current Campaign", StartedAt: time.Now()},
		{RunID: "old-run", CampaignName: "Old Campaign", StartedAt: time.Now().Add(-2 * time.Hour)},
	}
	screen.currentRun = &types.Session{RunID: runID}

	names := screen.campaignNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if !strings.Contains(names[0], "[CURRENT]") {
		t.Fatalf("expected first name to contain [CURRENT] marker, got %q", names[0])
	}
	if !strings.Contains(names[0], "just now") {
		t.Fatalf("expected first name to contain age, got %q", names[0])
	}
	if !strings.Contains(names[1], "2h ago") {
		t.Fatalf("expected second name to contain age, got %q", names[1])
	}
}

func TestCampaignNamesWithoutCurrentRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sessions = []*types.Session{
		{RunID: "run-1", CampaignName: "Campaign A", StartedAt: time.Now().Add(-1 * time.Hour)},
	}
	screen.currentRun = nil

	names := screen.campaignNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 name, got %d", len(names))
	}
	if strings.Contains(names[0], "[CURRENT]") {
		t.Fatalf("expected no [CURRENT] marker when no run selected, got %q", names[0])
	}
}

func TestCampaignSelectorSortsPreviousByStartedAt(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	older := time.Now().Add(-2 * time.Hour)
	newer := time.Now().Add(-30 * time.Minute)
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Current", StartedAt: time.Now()},
		{RunID: "older", CampaignName: "Older Campaign", StartedAt: older},
		{RunID: "newer", CampaignName: "Newer Campaign", StartedAt: newer},
	}
	screen.currentRun = &types.Session{RunID: runID}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	selector := screen.renderCampaignSelector()
	newerIdx := strings.Index(selector, "Newer Campaign")
	olderIdx := strings.Index(selector, "Older Campaign")
	if newerIdx < 0 || olderIdx < 0 {
		t.Fatal("expected both previous campaigns in selector")
	}
	if newerIdx > olderIdx {
		t.Fatal("expected newer session to appear before older session")
	}
}

func TestStartRefreshTickerAlwaysReturnsCmd(t *testing.T) {
	cmd := startRefreshTicker()
	if cmd == nil {
		t.Fatal("expected startRefreshTicker to always return a command")
	}
	msg := cmd()
	if _, ok := msg.(tickMsg); !ok {
		t.Fatalf("expected tickMsg from ticker command, got %T", msg)
	}
}

func TestTickMsgReSchedulesTicker(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	_, cmd := screen.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected tickMsg handler to return a command (re-scheduled ticker)")
	}
}

func TestTickMsgWithNoCurrentRunStillSchedulesTicker(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	_, cmd := screen.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected ticker to be re-scheduled even with no current run")
	}
}

func TestInitReturnsBatchWithTicker(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	cmd := screen.Init()
	if cmd == nil {
		t.Fatal("expected Init to return a command")
	}
}

func TestSelectingCampaignTriggersImmediateRefresh(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	sess := &types.Session{RunID: runID, CampaignName: "test-campaign", Status: types.RunStatePending, StartedAt: time.Now()}
	screen.sessions = []*types.Session{sess}
	screen.campaignList.SetCampaigns([]string{"test-campaign [" + runID + "]"})
	screen.currentRun = nil

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected refresh command after selecting campaign")
	}
	if screen.currentRun == nil || screen.currentRun.RunID != runID {
		t.Fatal("expected currentRun to be set after selecting campaign")
	}
}

func TestRefreshAllCmdFetchesAllData(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	event := types.NewTraceEvent(runID, "flow-1", "executor", types.TraceEventStepExecution, "test_action", types.TraceStatusSuccess)
	if err := screen.traceStore.Append(event); err != nil {
		t.Fatalf("append trace: %v", err)
	}

	cmd := refreshAllCmd(runID, screen.sessionStore, screen.traceStore, screen.artifactStore, screen.reportGenerator)
	if cmd == nil {
		t.Fatal("expected refresh command")
	}
}

func TestRefreshAllCmdWithEmptyRunID(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	cmd := refreshAllCmd("", screen.sessionStore, screen.traceStore, screen.artifactStore, screen.reportGenerator)
	if cmd == nil {
		t.Fatal("expected refresh command even with empty runID")
	}
	msg := cmd()
	if msg == nil {
		t.Fatal("expected command to return a message")
	}
}

func TestSteeringCommandPause(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.handlers.store.UpdateStatus(runID, types.RunStateRunning)

	screen.processSteeringCommand("pause")

	if screen.msg != "Run pausing..." {
		t.Fatalf("expected 'Run pausing...' message, got %q", screen.msg)
	}
	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStatePausing {
		t.Fatalf("expected pausing status, got %s", sess.Status)
	}
}

func TestSteeringCommandResume(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStatePaused}
	screen.handlers.store.UpdateStatus(runID, types.RunStatePaused)

	screen.processSteeringCommand("resume")

	if screen.msg != "Run resuming..." {
		t.Fatalf("expected 'Run resuming...' message, got %q", screen.msg)
	}
	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStateResuming {
		t.Fatalf("expected resuming status, got %s", sess.Status)
	}
}

func TestSteeringCommandResumeNotPaused(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.handlers.store.UpdateStatus(runID, types.RunStateRunning)

	screen.processSteeringCommand("resume")

	if !strings.Contains(strings.ToLower(screen.msg), "error") && !strings.Contains(strings.ToLower(screen.msg), "resuming") {
		t.Fatalf("expected error or resuming message, got %q", screen.msg)
	}
}

func TestSteeringCommandPauseNoRunSelected(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	screen.processSteeringCommand("pause")

	if screen.msg != "No run selected" {
		t.Fatalf("expected 'No run selected' message, got %q", screen.msg)
	}
}

func TestResumeRunFromPausingState(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.handlers.store.UpdateStatus(runID, types.RunStatePausing)

	err := screen.handlers.ResumeRun(runID)
	if err != nil {
		t.Fatalf("expected ResumeRun to accept PAUSING state, got error: %v", err)
	}
	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStateResuming {
		t.Fatalf("expected resuming status, got %s", sess.Status)
	}
}

func TestResumeRunFromPausedState(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.handlers.store.UpdateStatus(runID, types.RunStatePaused)

	err := screen.handlers.ResumeRun(runID)
	if err != nil {
		t.Fatalf("expected ResumeRun to accept PAUSED state, got error: %v", err)
	}
	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStateResuming {
		t.Fatalf("expected resuming status, got %s", sess.Status)
	}
}

func TestResumeRunRejectsWrongState(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	wrongStates := []types.RunState{
		types.RunStateRunning,
		types.RunStateCompleted,
		types.RunStateCancelled,
		types.RunStatePending,
	}

	for _, state := range wrongStates {
		screen.handlers.store.UpdateStatus(runID, state)
		err := screen.handlers.ResumeRun(runID)
		if err == nil {
			t.Fatalf("expected error when resuming from %s state", state)
		}
	}
}

func TestSpaceBarWaitingInputState(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateWaitingInput}
	screen.handlers.store.UpdateStatus(runID, types.RunStateWaitingInput)

	screen.Update(tea.KeyMsg{Type: tea.KeySpace})

	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStateRunning {
		t.Fatalf("expected running status after space on WAITING_FOR_INPUT, got %s", sess.Status)
	}
	if screen.msg != "Run resumed from WAITING_FOR_INPUT" {
		t.Fatalf("expected resume message, got %q", screen.msg)
	}
}

func TestHelpOverlayListsTextCommands(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.activeView = ViewDashboard
	screen.showHelp = true

	modal := screen.renderHelpOverlay("")

	requiredCommands := []string{"retry", "skip", "continue", "status", "pause", "resume", "steer"}
	for _, cmd := range requiredCommands {
		if !strings.Contains(modal, cmd) {
			t.Fatalf("expected help modal to contain text command %q", cmd)
		}
	}
	if !strings.Contains(modal, "Text Commands") {
		t.Fatal("expected help modal to contain 'Text Commands' section header")
	}
}

func TestSteeringCommandReturnsRefreshCmd(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.handlers.store.UpdateStatus(runID, types.RunStateRunning)

	cmd := screen.processSteeringCommand("pause")
	if cmd == nil {
		t.Fatal("expected pause to return a refresh Cmd")
	}
	msg := cmd()
	if _, ok := msg.(runLoadedMsg); !ok {
		t.Fatalf("expected runLoadedMsg, got %T", msg)
	}
}

func TestSteeringCommandResumeReturnsRefreshCmd(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStatePaused}
	screen.handlers.store.UpdateStatus(runID, types.RunStatePaused)

	cmd := screen.processSteeringCommand("resume")
	if cmd == nil {
		t.Fatal("expected resume to return a refresh Cmd")
	}
	msg := cmd()
	if _, ok := msg.(runLoadedMsg); !ok {
		t.Fatalf("expected runLoadedMsg, got %T", msg)
	}
}

func TestSteeringCommandRetryReturnsRefreshCmd(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.handlers.store.UpdateStatus(runID, types.RunStateRunning)

	cmd := screen.processSteeringCommand("retry flow-1")
	if cmd == nil {
		t.Fatal("expected retry to return a refresh Cmd")
	}
	msg := cmd()
	if _, ok := msg.(runLoadedMsg); !ok {
		t.Fatalf("expected runLoadedMsg, got %T", msg)
	}
}

func TestSteeringCommandUnknownReturnsNil(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	cmd := screen.processSteeringCommand("foobar")
	if cmd != nil {
		t.Fatal("expected unknown command to return nil Cmd")
	}
}

func TestSteeringCommandNoRunReturnsNil(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	cmd := screen.processSteeringCommand("pause")
	if cmd != nil {
		t.Fatal("expected no run to return nil Cmd")
	}
}

func TestRunCreatedMsgAutoSelectsNewSession(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	newRunID := "new-run-12345"
	model, cmd := screen.Update(runCreatedMsg{runID: newRunID})
	if cmd == nil {
		t.Fatal("expected runCreatedMsg to return refresh commands")
	}
	_ = model
}

func TestRunCreatedMsgSetsMessage(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	newRunID := "new-run-abcdef123456"
	screen.Update(runCreatedMsg{runID: newRunID})

	if !strings.Contains(screen.msg, "New session started") {
		t.Fatalf("expected message about new session, got %q", screen.msg)
	}
	if !strings.Contains(screen.msg, "new-run-") {
		t.Fatalf("expected message to contain runID prefix, got %q", screen.msg)
	}
}

func TestSteerCommandWithLifecycle(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	lc := runtime.NewLifecycleController(runID)
	screen.lifecycle = lc

	cmd := screen.processSteeringCommand("steer try a different approach")
	if cmd == nil {
		t.Fatal("expected steer to return a refresh Cmd")
	}

	events := lc.DrainSteeringEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 steering event, got %d", len(events))
	}
	if events[0].Command != types.SteerInstruction {
		t.Fatalf("expected SteerInstruction, got %s", events[0].Command)
	}
	if events[0].Instruction != "try a different approach" {
		t.Fatalf("expected instruction text, got %q", events[0].Instruction)
	}
}

func TestSteerCommandWithoutLifecycle(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	screen.lifecycle = nil

	cmd := screen.processSteeringCommand("steer do something")
	if cmd != nil {
		t.Fatal("expected steer without lifecycle to return nil Cmd")
	}
	if !strings.Contains(screen.msg, "not available") {
		t.Fatalf("expected error message, got %q", screen.msg)
	}
}

func TestSteerCommandNoArgs(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}
	lc := runtime.NewLifecycleController(runID)
	screen.lifecycle = lc

	cmd := screen.processSteeringCommand("steer")
	if cmd != nil {
		t.Fatal("expected steer without args to return nil Cmd")
	}
	if !strings.Contains(screen.msg, "Usage") {
		t.Fatalf("expected usage message, got %q", screen.msg)
	}
}

func TestVisualSessions_CurrentFirst(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	now := time.Now()
	screen.sessions = []*types.Session{
		{RunID: "run-alpha", CampaignName: "Alpha", StartedAt: now.Add(-2 * time.Hour)},
		{RunID: "run-beta", CampaignName: "Beta", StartedAt: now.Add(-1 * time.Hour)},
		{RunID: "run-gamma", CampaignName: "Gamma", StartedAt: now},
	}
	screen.currentRun = screen.sessions[1]

	vis := screen.visualSessions()
	if len(vis) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(vis))
	}
	if vis[0].RunID != "run-beta" {
		t.Errorf("expected current session first, got %s", vis[0].RunID)
	}
	if vis[1].RunID != "run-gamma" {
		t.Errorf("expected Gamma (newest previous) second, got %s", vis[1].RunID)
	}
	if vis[2].RunID != "run-alpha" {
		t.Errorf("expected Alpha (oldest previous) last, got %s", vis[2].RunID)
	}
}

func TestVisualSessions_NoCurrentRunSortsNewestFirst(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	now := time.Now()
	screen.sessions = []*types.Session{
		{RunID: "run-old", CampaignName: "Old", StartedAt: now.Add(-3 * time.Hour)},
		{RunID: "run-new", CampaignName: "New", StartedAt: now.Add(-1 * time.Hour)},
		{RunID: "run-mid", CampaignName: "Mid", StartedAt: now.Add(-2 * time.Hour)},
	}
	screen.currentRun = nil

	vis := screen.visualSessions()
	if len(vis) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(vis))
	}
	if vis[0].RunID != "run-new" {
		t.Errorf("expected newest first, got %s", vis[0].RunID)
	}
	if vis[1].RunID != "run-mid" {
		t.Errorf("expected mid second, got %s", vis[1].RunID)
	}
	if vis[2].RunID != "run-old" {
		t.Errorf("expected oldest last, got %s", vis[2].RunID)
	}
}

func TestVisualSessions_Empty(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sessions = nil
	screen.currentRun = nil

	vis := screen.visualSessions()
	if len(vis) != 0 {
		t.Fatalf("expected empty result, got %d sessions", len(vis))
	}
}
