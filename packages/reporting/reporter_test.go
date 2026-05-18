package reporting

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

func TestReportGenerator_GenerateCampaignSummary(t *testing.T) {
	tmpDir := t.TempDir()

	sessionStore, err := session.NewSessionStore(tmpDir)
	if err != nil {
		t.Fatalf("creating session store: %v", err)
	}

	sess, err := sessionStore.Create("test-campaign")
	if err != nil {
		t.Fatalf("creating session: %v", err)
	}

	now := time.Now().UTC()
	sess.Status = sharedtypes.RunStateCompleted
	sess.StartedAt = now.Add(-10 * time.Minute)
	sess.UpdatedAt = now

	sess.Flows = []sharedtypes.FlowRunState{
		{
			FlowID:     "flow-1",
			Name:       "Login Flow",
			Status:     sharedtypes.FlowStatePassed,
			StartedAt:  &now,
			FinishedAt: &now,
		},
		{
			FlowID:     "flow-2",
			Name:       "Checkout Flow",
			Status:     sharedtypes.FlowStateFailed,
			Error:      "payment failed",
			StartedAt:  &now,
			FinishedAt: &now,
		},
		{
			FlowID: "flow-3",
			Name:   "Skipped Flow",
			Status: sharedtypes.FlowStateSkippedUpstream,
		},
	}

	err = sessionStore.Save(sess)
	if err != nil {
		t.Fatalf("saving session: %v", err)
	}

	traceStore, err := trace.NewTraceStore(tmpDir)
	if err != nil {
		t.Fatalf("creating trace store: %v", err)
	}

	artifactStore, err := artifact.NewArtifactStore(tmpDir)
	if err != nil {
		t.Fatalf("creating artifact store: %v", err)
	}

	reportGen := NewReportGenerator(sessionStore, traceStore, artifactStore, tmpDir)

	summary, err := reportGen.GenerateCampaignSummary(sess.RunID)
	if err != nil {
		t.Fatalf("generating summary: %v", err)
	}

	if summary.TotalFlows != 3 {
		t.Errorf("expected 3 flows, got %d", summary.TotalFlows)
	}

	if summary.PassedFlows != 1 {
		t.Errorf("expected 1 passed flow, got %d", summary.PassedFlows)
	}

	if summary.FailedFlows != 1 {
		t.Errorf("expected 1 failed flow, got %d", summary.FailedFlows)
	}

	if summary.SkippedFlows != 1 {
		t.Errorf("expected 1 skipped flow, got %d", summary.SkippedFlows)
	}

	if summary.Status != sharedtypes.RunStateCompleted {
		t.Errorf("expected status COMPLETED, got %s", summary.Status)
	}
}

func TestReportGenerator_GenerateMarkdownReport(t *testing.T) {
	tmpDir := t.TempDir()

	sessionStore, _ := session.NewSessionStore(tmpDir)
	sess, _ := sessionStore.Create("markdown-test")
	sess.Status = sharedtypes.RunStateCompleted
	sess.Flows = []sharedtypes.FlowRunState{
		{
			FlowID: "flow-a",
			Name:   "Test A",
			Status: sharedtypes.FlowStatePassed,
		},
	}
	sessionStore.Save(sess)

	traceStore, _ := trace.NewTraceStore(tmpDir)
	artifactStore, _ := artifact.NewArtifactStore(tmpDir)

	reportGen := NewReportGenerator(sessionStore, traceStore, artifactStore, tmpDir)

	report, err := reportGen.GenerateMarkdownReport(sess.RunID)
	if err != nil {
		t.Fatalf("generating markdown report: %v", err)
	}

	if !contains(report, "# Campaign Execution Report") {
		t.Error("report should contain header")
	}

	if !contains(report, "Test A") {
		t.Error("report should contain flow name")
	}

	if !contains(report, "Passed") {
		t.Error("report should contain status")
	}
}

func TestReportGenerator_GenerateTerminalSummary(t *testing.T) {
	tmpDir := t.TempDir()

	sessionStore, _ := session.NewSessionStore(tmpDir)
	sess, _ := sessionStore.Create("terminal-test")
	sess.Status = sharedtypes.RunStateFailed
	sess.Flows = []sharedtypes.FlowRunState{
		{
			FlowID: "flow-x",
			Name:   "Failed Test",
			Status: sharedtypes.FlowStateFailed,
			Error:  "assertion failed",
		},
	}
	sessionStore.Save(sess)

	traceStore, _ := trace.NewTraceStore(tmpDir)
	artifactStore, _ := artifact.NewArtifactStore(tmpDir)

	reportGen := NewReportGenerator(sessionStore, traceStore, artifactStore, tmpDir)

	summary, err := reportGen.GenerateTerminalSummary(sess.RunID)
	if err != nil {
		t.Fatalf("generating terminal summary: %v", err)
	}

	if !contains(summary, "Failed Test") {
		t.Error("terminal summary should contain failed flow name")
	}

	if !contains(summary, "assertion failed") {
		t.Error("terminal summary should contain error message")
	}
}

func TestReportGenerator_SaveMarkdownReport(t *testing.T) {
	tmpDir := t.TempDir()
	reportsDir := filepath.Join(tmpDir, "reports")

	sessionStore, _ := session.NewSessionStore(tmpDir)
	sess, _ := sessionStore.Create("save-test")
	sess.Status = sharedtypes.RunStateCompleted
	sessionStore.Save(sess)

	traceStore, _ := trace.NewTraceStore(tmpDir)
	artifactStore, _ := artifact.NewArtifactStore(tmpDir)

	reportGen := NewReportGenerator(sessionStore, traceStore, artifactStore, reportsDir)

	path, err := reportGen.SaveMarkdownReport(sess.RunID)
	if err != nil {
		t.Fatalf("saving markdown report: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("report file should exist")
	}

	expectedPath := filepath.Join(reportsDir, "report_"+sess.RunID+".md")
	if path != expectedPath {
		t.Errorf("expected path %s, got %s", expectedPath, path)
	}
}

func TestGetReportPath(t *testing.T) {
	path := GetReportPath("run_123", "output")
	if !contains(path, "output") || !contains(path, "report_run_123.md") {
		t.Errorf("expected path containing output/report_run_123.md, got %s", path)
	}

	path = GetReportPath("run_456", "")
	if !contains(path, "reports") || !contains(path, "report_run_456.md") {
		t.Errorf("expected path containing reports/report_run_456.md, got %s", path)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}