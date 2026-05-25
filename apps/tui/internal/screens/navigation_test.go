package screens

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"qa-orchestrator/packages/shared/types"
)

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

func TestKeyCommandMode(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.currentRun = &types.Session{RunID: runID}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})

	if !screen.commandBar.Focused {
		t.Fatal("expected command focus to be enabled")
	}
	if !screen.commandBar.Input.Focused() {
		t.Fatal("expected command input to be focused")
	}
}

func TestKeyCommandModeNoRun(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.currentRun = nil

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})

	if screen.commandBar.Focused {
		t.Fatal("expected command focus to not be enabled without a run")
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

func TestEscDismissesHelpModal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.showHelp = true

	screen.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if screen.showHelp {
		t.Fatal("expected help modal to be dismissed after ESC")
	}
}

func TestEscDoesNotQuitWhenHelpModalOpen(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.showHelp = true

	_, cmd := screen.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if cmd != nil {
		t.Fatal("expected no tea.Quit command when ESC dismisses help modal")
	}
}

func TestKeyQuestionTogglesHelp(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40

	if screen.showHelp {
		t.Fatal("expected showHelp to be false initially")
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if !screen.showHelp {
		t.Fatal("expected showHelp to be true after ?")
	}

	screen.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'?'}})
	if screen.showHelp {
		t.Fatal("expected showHelp to be false after second ?")
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
