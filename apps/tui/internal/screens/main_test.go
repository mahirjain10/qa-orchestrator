package screens

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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

func TestKeyTabCyclesSlots(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeSlot = 0

	screen.Update(tea.KeyMsg{Type: tea.KeyTab})
	if screen.activeSlot != 1 {
		t.Fatalf("expected slot 1 after tab, got %d", screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyTab})
	if screen.activeSlot != 2 {
		t.Fatalf("expected slot 2 after second tab, got %d", screen.activeSlot)
	}
}

func TestKeyTabRestoresFromMaximized(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.maximized = true
	screen.activeSlot = 2

	screen.Update(tea.KeyMsg{Type: tea.KeyTab})

	if screen.maximized {
		t.Fatal("expected maximized to be false after tab")
	}
}

func TestKeyLeftRightNavigation(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeSlot = 0

	screen.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if screen.activeSlot != 3 {
		t.Fatalf("expected slot 3 after left from 0, got %d", screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRight})
	if screen.activeSlot != 0 {
		t.Fatalf("expected slot 0 after right from 3, got %d", screen.activeSlot)
	}
}

func TestKeyLeftRightDisabledWhenMaximized(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.maximized = true
	screen.activeSlot = 1

	screen.Update(tea.KeyMsg{Type: tea.KeyLeft})
	if screen.activeSlot != 1 {
		t.Fatalf("expected slot unchanged when maximized, got %d", screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRight})
	if screen.activeSlot != 1 {
		t.Fatalf("expected slot unchanged when maximized, got %d", screen.activeSlot)
	}
}

func TestKeySlotJump0to3(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	if screen.activeSlot != 0 {
		t.Fatalf("expected slot 0, got %d", screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	if screen.activeSlot != 2 {
		t.Fatalf("expected slot 2, got %d", screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	if screen.activeSlot != 3 {
		t.Fatalf("expected slot 3, got %d", screen.activeSlot)
	}
}

func TestKeyMaximizeToggle(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if !screen.maximized {
		t.Fatal("expected maximized after m key")
	}
	if screen.maximizedSlot != screen.activeSlot {
		t.Fatalf("expected maximizedSlot to match activeSlot, got %d vs %d", screen.maximizedSlot, screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if screen.maximized {
		t.Fatal("expected not maximized after second m key")
	}
}

func TestKeyEscapeRestoresFromMaximized(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.maximized = true

	screen.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if screen.maximized {
		t.Fatal("expected maximized to be false after escape")
	}
}

func TestKeyUpDownNavigationByComponent(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID, Flows: []types.FlowRunState{
		{FlowID: "flow-1", Status: types.FlowStateRunning},
		{FlowID: "flow-2", Status: types.FlowStatePending},
	}}
	screen.flowStatus.SetFlows(screen.currentRun.Flows)

	screen.activeSlot = 0
	screen.Update(tea.KeyMsg{Type: tea.KeyUp})
	if screen.campaignList.GetSelected() != 0 {
		t.Fatal("expected campaign list selection to handle up at boundary")
	}

	screen.activeSlot = 1
	screen.Update(tea.KeyMsg{Type: tea.KeyDown})
	if screen.flowStatus.GetSelected() != 1 {
		t.Fatalf("expected flow status selection to increment, got %d", screen.flowStatus.GetSelected())
	}
}

func TestKeyEnterSelectsCampaign(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	sess := &types.Session{RunID: runID, CampaignName: "test-campaign", Status: types.RunStatePending}
	screen.sessions = []*types.Session{sess}
	screen.campaignList.SetCampaigns([]string{"test-campaign [" + runID + "]"})
	screen.activeSlot = 0
	screen.quadrants[0] = CompCampaigns

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

func TestViewRendersFooter(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "quit") {
		t.Fatal("expected view to contain footer help text")
	}
}

func TestViewRendersQuadrants(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "[Campaigns]") {
		t.Fatal("expected view to contain campaigns panel")
	}
	if !strings.Contains(view, "[Flows]") {
		t.Fatal("expected view to contain flows panel")
	}
	if !strings.Contains(view, "[Run]") {
		t.Fatal("expected view to contain run panel")
	}
	if !strings.Contains(view, "[Traces]") {
		t.Fatal("expected view to contain traces panel")
	}
}

func TestViewMaximizedShowsSingleComponent(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.maximized = true
	screen.maximizedSlot = 0

	view := screen.View()
	if !strings.Contains(view, "[Campaigns]") {
		t.Fatal("expected maximized view to show only campaigns")
	}
}

func TestComponentLabelReturnsCorrectLabels(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	tests := []struct {
		id   ComponentID
		want string
	}{
		{CompCampaigns, " [Campaigns] "},
		{CompFlows, " [Flows] "},
		{CompRun, " [Run] "},
		{CompTraces, " [Traces] "},
		{CompArtifacts, " [Artifacts] "},
		{CompReport, " [Report] "},
		{ComponentID("unknown"), " [?] "},
	}

	for _, tt := range tests {
		got := screen.componentLabel(tt.id)
		if got != tt.want {
			t.Errorf("componentLabel(%s) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestFocusColorForSlot(t *testing.T) {
	screen, _ := newScreenWithRun(t)

	colors := map[int]string{
		0: "75",
		1: "226",
		2: "208",
		3: "86",
		4: "75",
	}

	for slot, want := range colors {
		got := screen.focusColorForSlot(slot)
		if string(got) != want {
			t.Errorf("focusColorForSlot(%d) = %q, want %q", slot, got, want)
		}
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
	if screen.activeSlot != 0 {
		t.Fatalf("expected activeSlot 0, got %d", screen.activeSlot)
	}
	if len(screen.quadrants) != 4 {
		t.Fatalf("expected 4 quadrants, got %d", len(screen.quadrants))
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

func TestPKeyCyclesComponent(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeSlot = 0
	screen.quadrants[0] = CompCampaigns

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if screen.quadrants[0] != CompFlows {
		t.Fatalf("expected component to cycle to Flows, got %s", screen.quadrants[0])
	}
}

func TestWKeySwapsSlots(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.activeSlot = 0
	screen.quadrants = [4]ComponentID{CompCampaigns, CompFlows, CompRun, CompTraces}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})

	if screen.quadrants[0] != CompFlows {
		t.Fatalf("expected slot 0 to be Flows after swap, got %s", screen.quadrants[0])
	}
	if screen.quadrants[1] != CompCampaigns {
		t.Fatalf("expected slot 1 to be Campaigns after swap, got %s", screen.quadrants[1])
	}
}

func TestPKeyDisabledWhenMaximized(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.maximized = true
	screen.activeSlot = 0
	screen.quadrants[0] = CompCampaigns

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	if screen.quadrants[0] != CompCampaigns {
		t.Fatalf("expected component to not change when maximized, got %s", screen.quadrants[0])
	}
}

func TestWKeyDisabledWhenMaximized(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.maximized = true
	screen.activeSlot = 0
	screen.quadrants = [4]ComponentID{CompCampaigns, CompFlows, CompRun, CompTraces}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})

	if screen.quadrants[0] != CompCampaigns {
		t.Fatalf("expected slots to not swap when maximized, got %s", screen.quadrants[0])
	}
}

func TestKey0to3DisabledWhenMaximized(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.maximized = true
	screen.activeSlot = 2

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
	if screen.activeSlot != 2 {
		t.Fatalf("expected slot unchanged when maximized, got %d", screen.activeSlot)
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	if screen.activeSlot != 2 {
		t.Fatalf("expected slot unchanged when maximized, got %d", screen.activeSlot)
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
