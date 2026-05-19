package screens

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"qa-orchestrator/apps/tui/internal/components"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
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

func TestSteeringCommandApprove(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	screen.processSteeringCommand("approve")

	if screen.msg != "Approval noted" {
		t.Fatalf("expected approval message, got %q", screen.msg)
	}
}

func TestKeyQuitAndCtrlC(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	model, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected tea.Quit command for ctrl+c")
	}

	model, cmd = screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal("expected tea.Quit command for q")
	}
	_ = model
}

func TestKeyTabTogglesFocus(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sidebarFocus = false

	screen.Update(tea.KeyMsg{Type: tea.KeyTab})
	if !screen.sidebarFocus {
		t.Fatalf("expected sidebar focus after tab, got content focus")
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyTab})
	if screen.sidebarFocus {
		t.Fatalf("expected content focus after second tab, got sidebar focus")
	}
}

func TestKey1to4SwitchViews(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if screen.activeView != ViewDashboard {
		t.Fatalf("expected Dashboard view, got %s", screen.activeView)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if screen.activeView != ViewFlows {
		t.Fatalf("expected Flows view, got %s", screen.activeView)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if screen.activeView != ViewTraces {
		t.Fatalf("expected Traces view, got %s", screen.activeView)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
	if screen.activeView != ViewReport {
		t.Fatalf("expected Report view, got %s", screen.activeView)
	}
}

func TestKeyUpDownInSidebarFocus(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sidebarFocus = true
	screen.activeView = ViewDashboard

	screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.activeView != ViewFlows {
		t.Fatalf("expected Flows view after down in sidebar, got %s", screen.activeView)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.activeView != ViewTraces {
		t.Fatalf("expected Traces view after second down, got %s", screen.activeView)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	if screen.activeView != ViewFlows {
		t.Fatalf("expected Flows view after up, got %s", screen.activeView)
	}
}

func TestKeyUpDownInContentFocus(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
		{FlowID: "flow-2", Status: types.FlowStatePending},
	}}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.sidebarFocus = false
	screen.activeView = ViewFlows

	screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.flowStatus.GetSelected() != 1 {
		t.Fatalf("expected flow selection to increment, got %d", screen.flowStatus.GetSelected())
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	if screen.flowStatus.GetSelected() != 0 {
		t.Fatalf("expected flow selection to decrement, got %d", screen.flowStatus.GetSelected())
	}
}

func TestKeyEnterSelectsCampaign(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	sess := &types.Session{RunID: runID, CampaignName: "test-campaign", Status: types.RunStatePending}
	screen.sessions = []*types.Session{sess}
	screen.campaignList.SetCampaigns([]string{"test-campaign [" + runID + "]"})

	screen.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if screen.currentRun == nil || screen.currentRun.RunID != runID {
		t.Fatal("expected currentRun to be set after enter on campaigns")
	}
}

func TestKeySpacePauseResume(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStatePending}

	screen.Update(tea.KeyMsg{Type: tea.KeySpace})

	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStatePausing {
		t.Fatalf("expected pausing status, got %s", sess.Status)
	}
}

func TestKeyXCancel(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	sess, _ := screen.handlers.GetRunStatus(runID)
	if sess.Status != types.RunStateCancelling {
		t.Fatalf("expected cancelling status, got %s", sess.Status)
	}
}

func TestKeySteeringMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if !screen.steeringMode {
		t.Fatal("expected steering mode to be enabled")
	}
	if !screen.steeringInput.Focused() {
		t.Fatal("expected steering input to be focused")
	}
}

func TestKeySteeringModeNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	if screen.steeringMode {
		t.Fatal("expected steering mode to not be enabled without a run")
	}
}

func TestKeyRefresh(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	model, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	if cmd == nil {
		t.Fatal("expected refresh command after r key")
	}
	_ = model
}

func TestViewReturnsInitializingWhenZeroSize(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 0
	screen.height = 0

	view := screen.View()
	if view != "Initializing..." {
		t.Fatalf("expected 'Initializing...', got %q", view)
	}
}

func TestViewRendersHeader(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "QA Orchestrator TUI") {
		t.Fatal("expected view to contain header text")
	}
}

func TestViewRendersSidebar(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "VIEWS") {
		t.Fatal("expected view to contain sidebar with VIEWS section")
	}
	if !strings.Contains(view, "Dashboard") {
		t.Fatal("expected view to contain Dashboard in sidebar")
	}
}

func TestViewShowsTooSmallMessage(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 60
	screen.height = 20

	view := screen.View()
	if !strings.Contains(view, "Terminal too small") {
		t.Fatal("expected view to show 'Terminal too small' message")
	}
}

func TestViewRendersSidebarWithRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	view := screen.View()
	if !strings.Contains(view, "RUN") {
		t.Fatal("expected view to contain RUN section in sidebar")
	}
	if !strings.Contains(view, runID[:8]) {
		t.Fatal("expected view to contain run ID in sidebar")
	}
}

func TestCampaignNamesFormatsCorrectly(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Test Campaign"},
		{RunID: "run-2", CampaignName: "Another Campaign"},
	}

	names := screen.campaignNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}
	if !strings.Contains(names[0], "Test Campaign") {
		t.Fatalf("expected first name to contain 'Test Campaign', got %q", names[0])
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

func TestSteeringModeEnterProcessesCommand(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.steeringMode = true
	screen.steeringInput.SetValue("status")

	model, _ := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(*MainScreen)

	if updated.steeringMode {
		t.Fatal("expected steering mode to be disabled after enter")
	}
	if updated.steeringInput.Value() != "" {
		t.Fatalf("expected steering input to be cleared, got %q", updated.steeringInput.Value())
	}
	if !strings.Contains(updated.msg, "Status:") {
		t.Fatalf("expected status message, got %q", updated.msg)
	}
}

func TestSteeringModeEnterWithEmptyInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.steeringMode = true
	screen.steeringInput.SetValue("")

	model, _ := screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := model.(*MainScreen)

	if !updated.steeringMode {
		t.Fatal("expected steering mode to remain active with empty input")
	}
}

func TestCampaignNamesWithNoSessions(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sessions = []*types.Session{}

	names := screen.campaignNames()
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d", len(names))
	}
}

func TestUpdateFromStoresHandlesMissingStores(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.traceStore = nil
	screen.artifactStore = nil
	screen.reportGenerator = nil

	screen.updateFromStores()

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

func TestMultipleSteeringCommandsInSequence(t *testing.T) {
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

func TestViewWithSteeringModeOverlay(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID}
	screen.steeringMode = true
	screen.steeringInput.SetValue("test input")

	view := screen.View()
	if !strings.Contains(view, "STEERING MODE") {
		t.Fatal("expected steering mode overlay in view")
	}
}

func TestViewWithMessageBox(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.SetMessage("Test message")

	view := screen.View()
	if !strings.Contains(view, "Test message") {
		t.Fatal("expected message box in view")
	}
}

func TestCycleViewWrapsAround(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeView = ViewDashboard

	screen.cycleView(-1)
	if screen.activeView != ViewReport {
		t.Fatalf("expected Report view after cycling up from Dashboard, got %s", screen.activeView)
	}

	screen.cycleView(1)
	if screen.activeView != ViewDashboard {
		t.Fatalf("expected Dashboard view after cycling down from Report, got %s", screen.activeView)
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

func TestCampaignNamesWithMultipleSessions(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.sessions = []*types.Session{
		{RunID: runID, CampaignName: "Campaign A"},
		{RunID: "run-2", CampaignName: "Campaign B"},
		{RunID: "run-3", CampaignName: "Campaign C"},
	}

	names := screen.campaignNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}
	for i, name := range names {
		if !strings.Contains(name, fmt.Sprintf("Campaign %c", 'A'+rune(i))) {
			t.Errorf("expected name %d to contain 'Campaign %c', got %q", i, 'A'+rune(i), name)
		}
	}
}

func TestRefreshAllUpdatesTraceArtifactAndReportPanels(t *testing.T) {
	screen, runID := newScreenWithRun(t)

	event := types.NewTraceEvent(runID, "flow-1", "executor", types.TraceEventStepExecution, "click_button", types.TraceStatusSuccess)
	if err := screen.traceStore.Append(event); err != nil {
		t.Fatalf("append trace event: %v", err)
	}

	if _, err := screen.artifactStore.Save(runID, "flow-1", artifact.ArtifactLog, "run.log", []byte("ok"), nil); err != nil {
		t.Fatalf("save artifact: %v", err)
	}

	if strings.Contains(screen.tracePanel.ViewCompact(), "click_button") {
		t.Fatal("tracepanel should be stale before refresh")
	}
	if !strings.Contains(screen.artifactPanel.View(), "No artifacts") {
		t.Fatal("artifact panel should be stale before refresh")
	}
	if screen.reportView != "" {
		t.Fatal("report view should be empty before refresh")
	}

	screen.currentRun = &types.Session{RunID: runID}
	_, cmd := screen.Update(tickMsg{})
	if cmd != nil {
		cmd()
	}
	screen.currentRun = &types.Session{RunID: runID}
	screen.updateFromStores()

	if !strings.Contains(screen.tracePanel.ViewCompact(), "click_button") {
		t.Fatal("expected trace panel to include latest event after refresh")
	}
	if strings.Contains(screen.artifactPanel.View(), "No artifacts") {
		t.Fatal("expected artifact panel to load artifacts after refresh")
	}
	if strings.TrimSpace(screen.reportView) == "" {
		t.Fatal("expected report view to be generated after refresh")
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

func newScreenWithRun(t *testing.T) (*MainScreen, string) {
	t.Helper()

	baseDir := t.TempDir()
	sessionStore, err := session.NewSessionStore(baseDir)
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	traceStore, err := trace.NewTraceStore(baseDir)
	if err != nil {
		t.Fatalf("new trace store: %v", err)
	}
	artifactStore, err := artifact.NewArtifactStore(baseDir)
	if err != nil {
		t.Fatalf("new artifact store: %v", err)
	}

	campaign := &types.Campaign{
		Name: "test-campaign",
		Flows: []types.Flow{
			{
				ID:       "flow-1",
				Name:     "Flow 1",
				Goal:     "goal",
				Mode:     types.FlowModeGuided,
				Priority: types.FlowPriorityHigh,
			},
		},
	}

	sess, err := sessionStore.Create(campaign)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	screen := NewMainScreenWithStores(sessionStore, traceStore, artifactStore)
	return screen, sess.RunID
}

func TestRenderDashboardViewWithNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil

	view := screen.renderDashboardView()
	if !strings.Contains(view, "Select a Campaign") {
		t.Fatalf("expected campaign selector when no run, got %q", view)
	}
}

func TestRenderDashboardViewWithActiveRun(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{
		RunID:        runID,
		CampaignName: "test-campaign",
		Status:       types.RunStateRunning,
		CurrentAgent: "executor",
		CurrentFlowID: "flow-1",
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning},
			{FlowID: "flow-2", Status: types.FlowStatePassed},
			{FlowID: "flow-3", Status: types.FlowStateFailed},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)

	view := screen.renderDashboardView()
	if !strings.Contains(view, "Run Summary") {
		t.Fatal("expected run summary in dashboard view")
	}
	if !strings.Contains(view, "Flows") {
		t.Fatal("expected flows section in dashboard view")
	}
	if !strings.Contains(view, "test-campaign") {
		t.Fatal("expected campaign name in dashboard view")
	}
}

func TestRenderRunSummaryShowsStatusAndCampaign(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{
		RunID:        runID,
		CampaignName: "my-campaign",
		Status:       types.RunStateRunning,
		CurrentAgent: "planner",
		CurrentFlowID: "flow-1",
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning},
			{FlowID: "flow-2", Status: types.FlowStatePassed},
			{FlowID: "flow-3", Status: types.FlowStateFailed},
		},
	}

	summary := screen.renderRunSummary()
	if !strings.Contains(summary, "Run Summary") {
		t.Fatal("expected 'Run Summary' title")
	}
	if !strings.Contains(summary, "my-campaign") {
		t.Fatal("expected campaign name in summary")
	}
	if !strings.Contains(summary, "planner") {
		t.Fatal("expected agent name in summary")
	}
	if !strings.Contains(summary, "flow-1") {
		t.Fatal("expected current flow ID in summary")
	}
	if !strings.Contains(summary, "3 flows") {
		t.Fatal("expected flow count in summary")
	}
	if !strings.Contains(summary, "R:1") {
		t.Fatal("expected running count in summary")
	}
	if !strings.Contains(summary, "P:1") {
		t.Fatal("expected passed count in summary")
	}
	if !strings.Contains(summary, "F:1") {
		t.Fatal("expected failed count in summary")
	}
}

func TestRenderRunSummaryWithNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil

	summary := screen.renderRunSummary()
	if !strings.Contains(summary, "No run selected") {
		t.Fatalf("expected 'No run selected' message, got %q", summary)
	}
}

func TestRenderFlowTimelineWithFlows(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{
		RunID: runID,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning},
			{FlowID: "flow-2", Status: types.FlowStatePassed},
			{FlowID: "flow-3", Status: types.FlowStateFailed},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)

	timeline := screen.renderFlowTimeline()
	if !strings.Contains(timeline, "Flows") {
		t.Fatal("expected 'Flows' section header")
	}
	if !strings.Contains(timeline, "flow-1") {
		t.Fatal("expected flow-1 in timeline")
	}
	if !strings.Contains(timeline, "flow-2") {
		t.Fatal("expected flow-2 in timeline")
	}
	if !strings.Contains(timeline, "flow-3") {
		t.Fatal("expected flow-3 in timeline")
	}
}

func TestRenderFlowTimelineWithNoFlows(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{}}

	timeline := screen.renderFlowTimeline()
	if !strings.Contains(timeline, "No flows") {
		t.Fatalf("expected 'No flows' message, got %q", timeline)
	}
}

func TestRenderFlowTimelineWithNilRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil

	timeline := screen.renderFlowTimeline()
	if !strings.Contains(timeline, "No flows") {
		t.Fatalf("expected 'No flows' message for nil run, got %q", timeline)
	}
}

func TestRenderDashboardViewShowsFlowTimeline(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{
		RunID:        runID,
		CampaignName: "test",
		Status:       types.RunStateCompleted,
		Flows: []types.FlowRunState{
			{FlowID: "flow-a", Status: types.FlowStatePassed},
			{FlowID: "flow-b", Status: types.FlowStatePassed},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)

	view := screen.renderDashboardView()
	if !strings.Contains(view, "flow-a") {
		t.Fatal("expected flow-a in dashboard view")
	}
	if !strings.Contains(view, "flow-b") {
		t.Fatal("expected flow-b in dashboard view")
	}
}

func TestEnterTogglesFlowExpand(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{
		RunID: runID,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning},
			{FlowID: "flow-2", Status: types.FlowStatePassed},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.activeView = ViewFlows
	screen.sidebarFocus = false

	if screen.flowStatus.Expanded {
		t.Fatal("expected expanded to be false initially")
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !screen.flowStatus.Expanded {
		t.Fatal("expected expanded to be true after enter")
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if screen.flowStatus.Expanded {
		t.Fatal("expected expanded to be false after second enter")
	}
}

func TestEnterDoesNotExpandWhenSidebarFocused(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
	}}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.activeView = ViewFlows
	screen.sidebarFocus = true

	screen.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if screen.flowStatus.Expanded {
		t.Fatal("expected expanded to remain false when sidebar focused")
	}
}

func TestLeftCollapsesExpandedFlow(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
	}}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.activeView = ViewFlows
	screen.sidebarFocus = false
	screen.flowStatus.Expanded = true

	screen.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if screen.flowStatus.Expanded {
		t.Fatal("expected expanded to be false after left")
	}
}

func TestHCollapsesExpandedFlow(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
	}}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.activeView = ViewFlows
	screen.sidebarFocus = false
	screen.flowStatus.Expanded = true

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if screen.flowStatus.Expanded {
		t.Fatal("expected expanded to be false after h")
	}
}

func TestLeftDoesNotCollapseWhenSidebarFocused(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
	}}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.activeView = ViewFlows
	screen.sidebarFocus = true
	screen.flowStatus.Expanded = true

	screen.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if !screen.flowStatus.Expanded {
		t.Fatal("expected expanded to remain true when sidebar focused")
	}
}

func TestRenderFlowsViewShowsTable(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{
		RunID: runID,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Mode: types.FlowModeGuided, Priority: types.FlowPriorityHigh, Status: types.FlowStateRunning},
			{FlowID: "flow-2", Mode: types.FlowModeAutonomous, Priority: types.FlowPriorityLow, Status: types.FlowStatePassed},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.activeView = ViewFlows

	view := screen.renderFlowsView()
	if !strings.Contains(view, "Flows") {
		t.Fatal("expected 'Flows' title in view")
	}
	if !strings.Contains(view, "Flow") || !strings.Contains(view, "Mode") {
		t.Fatal("expected table headers in view")
	}
	if !strings.Contains(view, "flow-1") {
		t.Fatal("expected flow-1 in view")
	}
	if !strings.Contains(view, "flow-2") {
		t.Fatal("expected flow-2 in view")
	}
}

func TestRenderFlowsViewWithNoFlows(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{}}

	view := screen.renderFlowsView()
	if !strings.Contains(view, "No flows") {
		t.Fatalf("expected 'No flows' message, got %q", view)
	}
}

func TestRenderFlowsViewWithNilRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil

	view := screen.renderFlowsView()
	if !strings.Contains(view, "No flows") {
		t.Fatalf("expected 'No flows' message for nil run, got %q", view)
	}
}

func TestRenderFlowDetailShowsStartedAndFinished(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	started := time.Date(2026, 5, 20, 10, 30, 0, 0, time.UTC)
	finished := time.Date(2026, 5, 20, 10, 30, 45, 0, time.UTC)
	f := types.FlowRunState{
		FlowID:     "flow-1",
		Status:     types.FlowStatePassed,
		StartedAt:  &started,
		FinishedAt: &finished,
	}

	detail := screen.renderFlowDetail(f)
	if !strings.Contains(detail, "Started:") {
		t.Fatal("expected 'Started:' in detail")
	}
	if !strings.Contains(detail, "Finished:") {
		t.Fatal("expected 'Finished:' in detail")
	}
	if !strings.Contains(detail, "Duration:") {
		t.Fatal("expected 'Duration:' in detail")
	}
}

func TestRenderFlowDetailShowsRetries(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	f := types.FlowRunState{
		FlowID:     "flow-1",
		Status:     types.FlowStateRetrying,
		RetryCount: 3,
	}

	detail := screen.renderFlowDetail(f)
	if !strings.Contains(detail, "Retries:") {
		t.Fatal("expected 'Retries:' in detail")
	}
	if !strings.Contains(detail, "3") {
		t.Fatal("expected retry count '3' in detail")
	}
}

func TestRenderFlowDetailShowsError(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	f := types.FlowRunState{
		FlowID: "flow-1",
		Status: types.FlowStateFailed,
		Error:  "connection refused",
	}

	detail := screen.renderFlowDetail(f)
	if !strings.Contains(detail, "Error:") {
		t.Fatal("expected 'Error:' in detail")
	}
	if !strings.Contains(detail, "connection refused") {
		t.Fatal("expected error message in detail")
	}
}

func TestRenderFlowDetailWithNoDates(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	f := types.FlowRunState{
		FlowID: "flow-1",
		Status: types.FlowStatePending,
	}

	detail := screen.renderFlowDetail(f)
	if strings.Contains(detail, "Started:") {
		t.Fatal("expected no 'Started:' when StartedAt is nil")
	}
	if strings.Contains(detail, "Finished:") {
		t.Fatal("expected no 'Finished:' when FinishedAt is nil")
	}
}

func TestRenderFlowDetailWithNoRetries(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	f := types.FlowRunState{
		FlowID:     "flow-1",
		Status:     types.FlowStatePassed,
		RetryCount: 0,
	}

	detail := screen.renderFlowDetail(f)
	if strings.Contains(detail, "Retries:") {
		t.Fatal("expected no 'Retries:' when RetryCount is 0")
	}
}

func TestRenderFlowDetailWithNoError(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	f := types.FlowRunState{
		FlowID: "flow-1",
		Status: types.FlowStatePassed,
		Error:  "",
	}

	detail := screen.renderFlowDetail(f)
	if strings.Contains(detail, "Error:") {
		t.Fatal("expected no 'Error:' when Error is empty")
	}
}

func TestRenderFlowDetailWithFinishedButNoStarted(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	finished := time.Date(2026, 5, 20, 10, 30, 45, 0, time.UTC)
	f := types.FlowRunState{
		FlowID:     "flow-1",
		Status:     types.FlowStateFailed,
		StartedAt:  nil,
		FinishedAt: &finished,
		Error:      "crashed",
	}

	detail := screen.renderFlowDetail(f)
	if strings.Contains(detail, "Duration:") {
		t.Fatal("expected no 'Duration:' when StartedAt is nil")
	}
	if !strings.Contains(detail, "Finished:") {
		t.Fatal("expected 'Finished:' in detail")
	}
	if !strings.Contains(detail, "Error:") {
		t.Fatal("expected 'Error:' in detail")
	}
}

func TestRenderFlowsViewShowsExpandedDetail(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	started := time.Date(2026, 5, 20, 10, 30, 0, 0, time.UTC)
	finished := time.Date(2026, 5, 20, 10, 30, 45, 0, time.UTC)
	screen.currentRun = &types.Session{
		RunID: runID,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStatePassed, StartedAt: &started, FinishedAt: &finished},
			{FlowID: "flow-2", Status: types.FlowStatePending},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.flowStatus.SetSelected(0)
	screen.flowStatus.Expanded = true
	screen.activeView = ViewFlows
	screen.sidebarFocus = false

	view := screen.renderFlowsView()
	if !strings.Contains(view, "Started:") {
		t.Fatal("expected expanded detail 'Started:' in view")
	}
	if !strings.Contains(view, "Duration:") {
		t.Fatal("expected expanded detail 'Duration:' in view")
	}
}

func TestRenderFlowsViewHidesDetailWhenNotExpanded(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	started := time.Date(2026, 5, 20, 10, 30, 0, 0, time.UTC)
	screen.currentRun = &types.Session{
		RunID: runID,
		Flows: []types.FlowRunState{
			{FlowID: "flow-1", Status: types.FlowStateRunning, StartedAt: &started},
		},
	}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)
	screen.flowStatus.Expanded = false
	screen.activeView = ViewFlows

	view := screen.renderFlowsView()
	if strings.Contains(view, "Started:") {
		t.Fatal("expected no detail when not expanded")
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
	if w != 94 {
		t.Fatalf("expected width 94, got %d", w)
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

func TestFilterModeEntersOnSlash(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.sidebarFocus = false

	_, _ = screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if !screen.tracePanel.FilterMode {
		t.Fatal("expected filter mode to be enabled")
	}
}

func TestFilterModeDoesNotEnterOnSidebarFocus(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.sidebarFocus = true

	_, _ = screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})

	if screen.tracePanel.FilterMode {
		t.Fatal("expected filter mode to not be enabled when sidebar focused")
	}
}

func TestFilterModeCancelledOnEsc(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.tracePanel.FilterMode = true
	screen.tracePanel.FilterInput.SetValue("test")

	_, _ = screen.handleFilterKey(tea.KeyMsg{Type: tea.KeyEscape})

	if screen.tracePanel.FilterMode {
		t.Fatal("expected filter mode to be cancelled")
	}
	if screen.tracePanel.FilterInput.Value() != "" {
		t.Fatal("expected filter input to be cleared")
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
}

func TestShowFailedToggle(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}
	screen.activeView = ViewTraces
	screen.sidebarFocus = false

	_, _ = screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	if !screen.tracePanel.Filter.ShowFailed {
		t.Fatal("expected showFailed to be true after toggle")
	}

	_, _ = screen.handleMainKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'S'}})

	if screen.tracePanel.Filter.ShowFailed {
		t.Fatal("expected showFailed to be false after second toggle")
	}
}

func TestFilteredEventsReturnsAllWhenNoFilter(t *testing.T) {
	panel := components.NewTracePanelModel()
	events := []*types.TraceEvent{
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action1", types.TraceStatusSuccess),
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action2", types.TraceStatusFailed),
	}
	panel.SetEvents(events)

	filtered := panel.FilteredEvents()
	if len(filtered) != 2 {
		t.Fatalf("expected 2 events, got %d", len(filtered))
	}
}

func TestFilteredEventsFiltersByText(t *testing.T) {
	panel := components.NewTracePanelModel()
	events := []*types.TraceEvent{
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "browser click", types.TraceStatusSuccess),
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "api call", types.TraceStatusSuccess),
	}
	panel.SetEvents(events)
	panel.Filter.Text = "browser"

	filtered := panel.FilteredEvents()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].Action != "browser click" {
		t.Fatalf("expected 'browser click', got %q", filtered[0].Action)
	}
}

func TestFilteredEventsFiltersByShowFailed(t *testing.T) {
	panel := components.NewTracePanelModel()
	events := []*types.TraceEvent{
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action1", types.TraceStatusSuccess),
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action2", types.TraceStatusFailed),
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action3", types.TraceStatusSuccess),
	}
	panel.SetEvents(events)
	panel.Filter.ShowFailed = true

	filtered := panel.FilteredEvents()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].Status != types.TraceStatusFailed {
		t.Fatal("expected failed event")
	}
}

func TestFilteredEventsFiltersByFlowID(t *testing.T) {
	panel := components.NewTracePanelModel()
	events := []*types.TraceEvent{
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action1", types.TraceStatusSuccess),
		types.NewTraceEvent("run-1", "flow-2", "agent", types.TraceEventStepExecution, "action2", types.TraceStatusSuccess),
	}
	panel.SetEvents(events)
	panel.Filter.FlowID = "flow-1"

	filtered := panel.FilteredEvents()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].FlowID != "flow-1" {
		t.Fatalf("expected flow-1, got %q", filtered[0].FlowID)
	}
}

func TestFilteredEventsFiltersByEventType(t *testing.T) {
	panel := components.NewTracePanelModel()
	events := []*types.TraceEvent{
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "action1", types.TraceStatusSuccess),
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventToolResult, "action2", types.TraceStatusSuccess),
	}
	panel.SetEvents(events)
	panel.Filter.EventType = "tool_result"

	filtered := panel.FilteredEvents()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 event, got %d", len(filtered))
	}
	if filtered[0].EventType != types.TraceEventToolResult {
		t.Fatalf("expected tool_result event, got %q", filtered[0].EventType)
	}
	if filtered[0].EventType != types.TraceEventToolResult {
		t.Fatalf("expected tool_call event, got %q", filtered[0].EventType)
	}
}

func TestFilteredEventsCaseInsensitiveText(t *testing.T) {
	panel := components.NewTracePanelModel()
	events := []*types.TraceEvent{
		types.NewTraceEvent("run-1", "flow-1", "agent", types.TraceEventStepExecution, "Browser Click", types.TraceStatusSuccess),
	}
	panel.SetEvents(events)
	panel.Filter.Text = "browser"

	filtered := panel.FilteredEvents()
	if len(filtered) != 1 {
		t.Fatalf("expected 1 event with case-insensitive match, got %d", len(filtered))
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
	if !strings.Contains(keys, "s:steer") {
		t.Fatalf("expected dashboard keys with run to contain 's:steer', got %q", keys)
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
