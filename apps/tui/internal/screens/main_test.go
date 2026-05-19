package screens

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func TestSteeringModeEscapeCancelsInput(t *testing.T) {
	screen, runID := newScreenWithRun(t)
	screen.state.SetCurrentRunID(runID)

	screen.steeringMode = true
	screen.steeringInput.SetValue("retry flow-1")

	model, _ := screen.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated := model.(*MainScreen)

	if updated.steeringMode {
		t.Fatal("expected steering mode to be disabled on ESC")
	}
	if updated.steeringInput.Value() != "" {
		t.Fatalf("expected steering input to be cleared, got %q", updated.steeringInput.Value())
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
		t.Fatal("trace panel should be stale before refresh")
	}
	if !strings.Contains(screen.artifactPanel.View(), "No artifacts") {
		t.Fatal("artifact panel should be stale before refresh")
	}
	if screen.reportView != "" {
		t.Fatal("report view should be empty before refresh")
	}

	screen.state.SetCurrentRunID(runID)
	screen.refreshAll()

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
