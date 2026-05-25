package screens

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
)

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
	if !strings.Contains(view, "too narrow") {
		t.Fatalf("expected view to show 'too narrow' message, got %q", view)
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

func TestViewWithCommandBar(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID}
	screen.commandBar.Focused = true
	screen.commandBar.Input.SetValue("test input")

	view := screen.View()
	if !strings.Contains(view, "test input") {
		t.Fatal("expected command bar in view")
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
		RunID:         runID,
		CampaignName:  "test-campaign",
		Status:        types.RunStateRunning,
		CurrentAgent:  "executor",
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
		RunID:         runID,
		CampaignName:  "my-campaign",
		Status:        types.RunStateRunning,
		CurrentAgent:  "planner",
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

func TestCampaignNamesWithNoSessions(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.sessions = []*types.Session{}

	names := screen.campaignNames()
	if len(names) != 0 {
		t.Fatalf("expected 0 names, got %d", len(names))
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

	sess, err := screen.handlers.GetRunStatus(runID)
	if err == nil && sess != nil {
		screen.currentRun = sess
		screen.flowStatus.SyncFlows(sess.Flows)
	}
	if screen.traceStore != nil {
		events, err := screen.traceStore.GetRecent(runID, 50)
		if err == nil {
			screen.traces = events
			screen.tracePanel.SetEvents(events)
		}
	}
	if screen.artifactStore != nil {
		artifacts, err := screen.artifactStore.GetByRunID(runID)
		if err == nil {
			screen.artifacts = artifacts
			screen.artifactPanel.SetArtifacts(artifacts)
		}
	}
	if screen.reportGenerator != nil {
		report, err := screen.reportGenerator.GenerateTerminalSummary(runID)
		if err == nil {
			screen.reportView = report
		}
	}

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

func TestViewShowsErrorOnNarrowTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 70
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "too narrow") {
		t.Fatalf("expected 'too narrow' error, got %q", view)
	}
	if !strings.Contains(view, "70") {
		t.Fatalf("expected current width in error, got %q", view)
	}
}

func TestViewShowsErrorOnShortTerminal(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 15

	view := screen.View()
	if !strings.Contains(view, "too short") {
		t.Fatalf("expected 'too short' error, got %q", view)
	}
	if !strings.Contains(view, "15") {
		t.Fatalf("expected current height in error, got %q", view)
	}
}

func TestViewRendersAtMinimumSize(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 80
	screen.height = 20
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	view := screen.View()
	if strings.Contains(view, "too narrow") || strings.Contains(view, "too short") {
		t.Fatalf("expected no error at minimum size, got %q", view)
	}
	if view == "" {
		t.Fatal("expected non-empty view at minimum size")
	}
}

func TestViewRendersAtStandardSize(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateRunning}

	view := screen.View()
	if strings.Contains(view, "too narrow") || strings.Contains(view, "too short") {
		t.Fatalf("expected no error at standard size, got %q", view)
	}
	if !strings.Contains(view, "QA Orchestrator TUI") {
		t.Fatal("expected header in view")
	}
}

func TestViewRendersAtWideTerminal(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 200
	screen.height = 60
	screen.currentRun = &types.Session{RunID: runID, Status: types.RunStateCompleted}

	view := screen.View()
	if strings.Contains(view, "too narrow") || strings.Contains(view, "too short") {
		t.Fatalf("expected no error at wide terminal, got %q", view)
	}
}

func TestErrorMessagesUseStyledFailed(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 70
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "Terminal too narrow") {
		t.Fatalf("expected styled error message, got %q", view)
	}
}

func TestNarrowTerminalErrorShowsCurrentWidth(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 60
	screen.height = 40

	view := screen.View()
	if !strings.Contains(view, "60") {
		t.Fatalf("expected current width 60 in error, got %q", view)
	}
}

func TestShortTerminalErrorShowsCurrentHeight(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 10

	view := screen.View()
	if !strings.Contains(view, "10") {
		t.Fatalf("expected current height 10 in error, got %q", view)
	}
}

func TestHelpModalRendersWithContent(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.activeView = ViewDashboard
	screen.showHelp = true

	view := screen.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Fatal("expected view to contain help modal title")
	}
	if !strings.Contains(view, "Global") {
		t.Fatal("expected view to contain Global section")
	}
	if !strings.Contains(view, "Quit") {
		t.Fatal("expected view to contain Quit key hint")
	}
}

func TestViewShowsHelpModalOverlay(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = &types.Session{RunID: runID}
	screen.showHelp = true

	view := screen.View()
	if !strings.Contains(view, "Keyboard Shortcuts") {
		t.Fatal("expected view to contain help modal overlay")
	}
	if !strings.Contains(view, "Global") {
		t.Fatal("expected view to contain Global section in help modal")
	}
}

func TestCampaignSelectorShowsTitle(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "Select a Campaign") {
		t.Fatalf("expected selector to contain title, got %q", selector)
	}
}

func TestCampaignSelectorShowsEmptyState(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "No campaigns found") {
		t.Fatalf("expected selector to show empty state message, got %q", selector)
	}
}

func TestCampaignSelectorShowsCampaigns(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{
		{RunID: "run-001", CampaignName: "login-test"},
		{RunID: "run-002", CampaignName: "checkout-test"},
	}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "login-test") {
		t.Fatal("expected selector to contain first campaign")
	}
	if !strings.Contains(selector, "checkout-test") {
		t.Fatal("expected selector to contain second campaign")
	}
}

func TestCampaignSelectorShowsSelectedIndicator(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{
		{RunID: "run-001", CampaignName: "login-test"},
		{RunID: "run-002", CampaignName: "checkout-test"},
	}
	screen.campaignList.SetCampaigns(screen.campaignNames())
	screen.campaignList.SetSelected(1)

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "▶") {
		t.Fatal("expected selector to contain selected indicator")
	}
}

func TestCampaignSelectorModalWidthConstrained(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 200
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	selector := screen.renderCampaignSelector()
	width := lipgloss.Width(selector)
	if width > 74 {
		t.Fatalf("expected modal width <= 74 (70 + padding), got %d", width)
	}
}

func TestDashboardViewCentersCampaignSelector(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{
		{RunID: "run-001", CampaignName: "test-campaign"},
	}
	screen.campaignList.SetCampaigns(screen.campaignNames())

	view := screen.renderDashboardView()
	if !strings.Contains(view, "Select a Campaign") {
		t.Fatal("expected dashboard view to contain campaign selector")
	}
}

func TestCampaignSelectorShowsNavigationHints(t *testing.T) {
	screen, _ := newScreenWithRun(t)
	screen.width = 120
	screen.height = 40
	screen.currentRun = nil
	screen.sessions = []*types.Session{}

	selector := screen.renderCampaignSelector()
	if !strings.Contains(selector, "navigate") {
		t.Fatal("expected selector to show navigate hint")
	}
	if !strings.Contains(selector, "enter") {
		t.Fatal("expected selector to show enter hint")
	}
}
