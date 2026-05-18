package reporting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

type ReportGenerator struct {
	sessionStore  *session.SessionStore
	traceStore    *trace.TraceStore
	artifactStore *artifact.ArtifactStore
	outputDir     string
}

type CampaignSummary struct {
	RunID          string
	CampaignName   string
	Status         sharedtypes.RunState
	StartedAt      time.Time
	CompletedAt    time.Time
	Duration       time.Duration
	TotalFlows     int
	PassedFlows    int
	FailedFlows    int
	SkippedFlows   int
	PendingFlows   int
	RetryCount     int
	Flows          []FlowSummary
	ArtifactsCount int
	TracesCount    int
}

type FlowSummary struct {
	FlowID       string
	Name         string
	Status       sharedtypes.FlowState
	StartedAt    *time.Time
	FinishedAt   *time.Time
	Duration     time.Duration
	RetryCount   int
	Error        string
	ArtifactPath string
}

func NewReportGenerator(sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, outputDir string) *ReportGenerator {
	return &ReportGenerator{
		sessionStore:  sessionStore,
		traceStore:    traceStore,
		artifactStore: artifactStore,
		outputDir:     outputDir,
	}
}

func (r *ReportGenerator) GenerateCampaignSummary(runID string) (*CampaignSummary, error) {
	sess, err := r.sessionStore.Get(runID)
	if err != nil {
		return nil, fmt.Errorf("getting session: %w", err)
	}

	artifacts, _ := r.artifactStore.GetByRunID(runID)
	traces, _ := r.traceStore.GetByRunID(runID)

	summary := &CampaignSummary{
		RunID:          sess.RunID,
		CampaignName:   sess.CampaignName,
		Status:         sess.Status,
		StartedAt:       sess.StartedAt,
		CompletedAt:    sess.UpdatedAt,
		RetryCount:     sess.RetryCount,
		ArtifactsCount: len(artifacts),
		TracesCount:    len(traces),
	}

	if !sess.StartedAt.IsZero() && !sess.UpdatedAt.IsZero() {
		summary.Duration = sess.UpdatedAt.Sub(sess.StartedAt)
	}

	for _, flow := range sess.Flows {
		flowSummary := FlowSummary{
			FlowID:     flow.FlowID,
			Name:       flow.Name,
			Status:     flow.Status,
			StartedAt:  flow.StartedAt,
			FinishedAt: flow.FinishedAt,
			RetryCount: flow.RetryCount,
			Error:      flow.Error,
		}

		if flow.StartedAt != nil && flow.FinishedAt != nil {
			flowSummary.Duration = flow.FinishedAt.Sub(*flow.StartedAt)
		}

		for _, a := range artifacts {
			if a.FlowID == flow.FlowID {
				flowSummary.ArtifactPath = a.Path
				break
			}
		}

		summary.Flows = append(summary.Flows, flowSummary)

		switch flow.Status {
		case sharedtypes.FlowStatePassed:
			summary.PassedFlows++
		case sharedtypes.FlowStateFailed:
			summary.FailedFlows++
		case sharedtypes.FlowStateSkippedUpstream, sharedtypes.FlowStateBlockedConfigError:
			summary.SkippedFlows++
		case sharedtypes.FlowStatePending:
			summary.PendingFlows++
		}
	}

	summary.TotalFlows = len(sess.Flows)

	return summary, nil
}

func (r *ReportGenerator) GenerateMarkdownReport(runID string) (string, error) {
	summary, err := r.GenerateCampaignSummary(runID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString("# Campaign Execution Report\n\n")
	sb.WriteString(fmt.Sprintf("**Run ID:** %s\n", summary.RunID))
	sb.WriteString(fmt.Sprintf("**Campaign:** %s\n", summary.CampaignName))
	sb.WriteString(fmt.Sprintf("**Status:** %s\n", summary.Status))
	sb.WriteString(fmt.Sprintf("**Started:** %s\n", summary.StartedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Completed:** %s\n", summary.CompletedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Duration:** %s\n", summary.Duration))
	sb.WriteString("\n---\n\n")

	sb.WriteString("## Summary\n\n")
	sb.WriteString(fmt.Sprintf("| Metric | Count |\n"))
	sb.WriteString(fmt.Sprintf("|--------|-------|\n"))
	sb.WriteString(fmt.Sprintf("| Total Flows | %d |\n", summary.TotalFlows))
	sb.WriteString(fmt.Sprintf("| Passed | %d |\n", summary.PassedFlows))
	sb.WriteString(fmt.Sprintf("| Failed | %d |\n", summary.FailedFlows))
	sb.WriteString(fmt.Sprintf("| Skipped | %d |\n", summary.SkippedFlows))
	sb.WriteString(fmt.Sprintf("| Pending | %d |\n", summary.PendingFlows))
	sb.WriteString(fmt.Sprintf("| Retries | %d |\n", summary.RetryCount))
	sb.WriteString(fmt.Sprintf("| Artifacts | %d |\n", summary.ArtifactsCount))
	sb.WriteString(fmt.Sprintf("| Trace Events | %d |\n", summary.TracesCount))
	sb.WriteString("\n---\n\n")

	sb.WriteString("## Flow Details\n\n")
	sb.WriteString(fmt.Sprintf("| Flow ID | Name | Status | Duration | Error |\n"))
	sb.WriteString(fmt.Sprintf("|---------|------|--------|----------|-------|\n"))

	for _, flow := range summary.Flows {
		duration := flow.Duration.Round(time.Second).String()
		if duration == "0s" || duration == "" {
			duration = "-"
		}
		errorMsg := flow.Error
		if errorMsg == "" {
			errorMsg = "-"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			flow.FlowID, flow.Name, flow.Status, duration, errorMsg))
	}

	sb.WriteString("\n---\n\n")

	if summary.ArtifactsCount > 0 {
		sb.WriteString("## Artifacts\n\n")
		for _, flow := range summary.Flows {
			if flow.ArtifactPath != "" {
				sb.WriteString(fmt.Sprintf("- **%s:** `%s`\n", flow.Name, flow.ArtifactPath))
			}
		}
	}

	sb.WriteString("\n---\n\n")
	sb.WriteString("*Report generated by Zenact QA Orchestrator*\n")

	return sb.String(), nil
}

func (r *ReportGenerator) SaveMarkdownReport(runID string) (string, error) {
	report, err := r.GenerateMarkdownReport(runID)
	if err != nil {
		return "", err
	}

	filename := fmt.Sprintf("report_%s.md", runID)
	if r.outputDir == "" {
		r.outputDir = "reports"
	}

	if err := os.MkdirAll(r.outputDir, 0755); err != nil {
		return "", fmt.Errorf("creating reports directory: %w", err)
	}

	path := filepath.Join(r.outputDir, filename)
	if err := os.WriteFile(path, []byte(report), 0644); err != nil {
		return "", fmt.Errorf("writing report file: %w", err)
	}

	return path, nil
}

func (r *ReportGenerator) GenerateTerminalSummary(runID string) (string, error) {
	summary, err := r.GenerateCampaignSummary(runID)
	if err != nil {
		return "", err
	}

	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("═══ Campaign Report: %s ═══\n\n", summary.CampaignName))
	sb.WriteString(fmt.Sprintf("Run ID:     %s\n", summary.RunID))
	sb.WriteString(fmt.Sprintf("Status:     %s\n", summary.Status))
	sb.WriteString(fmt.Sprintf("Duration:   %s\n", summary.Duration))

	sb.WriteString("\n─── Flow Summary ───\n")
	sb.WriteString(fmt.Sprintf("Total:   %d\n", summary.TotalFlows))
	sb.WriteString(fmt.Sprintf("Passed:  %d\n", summary.PassedFlows))
	sb.WriteString(fmt.Sprintf("Failed:  %d\n", summary.FailedFlows))
	sb.WriteString(fmt.Sprintf("Skipped: %d\n", summary.SkippedFlows))

	if summary.FailedFlows > 0 {
		sb.WriteString("\n─── Failed Flows ───\n")
		for _, flow := range summary.Flows {
			if flow.Status == sharedtypes.FlowStateFailed {
				sb.WriteString(fmt.Sprintf("✗ %s (%s): %s\n", flow.Name, flow.FlowID, flow.Error))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\nArtifacts: %d | Traces: %d\n", summary.ArtifactsCount, summary.TracesCount))

	return sb.String(), nil
}

func GetReportPath(runID, outputDir string) string {
	if outputDir == "" {
		outputDir = "reports"
	}
	return filepath.Join(outputDir, fmt.Sprintf("report_%s.md", runID))
}