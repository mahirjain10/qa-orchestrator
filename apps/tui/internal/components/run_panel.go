package components

import (
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/packages/shared/types"
)

var (
	runPanelTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	runPanelLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	runPanelValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	runPanelStatusStyle = lipgloss.NewStyle().
				Bold(true)
)

type RunPanelModel struct {
	session    *types.Session
	width      int
}

func NewRunPanelModel() *RunPanelModel {
	return &RunPanelModel{
		width: 60,
	}
}

func (m *RunPanelModel) SetSession(sess *types.Session) {
	m.session = sess
}

func (m *RunPanelModel) View() string {
	if m.session == nil {
		return runPanelTitleStyle.Render("Active Run") + "\n\n  No active run\n"
	}

	lines := []string{}
	lines = append(lines, runPanelTitleStyle.Render("Active Run"))
	lines = append(lines, "")

	lines = append(lines, runPanelLabelStyle.Render("  Run ID:    ")+runPanelValueStyle.Render(m.session.RunID))
	lines = append(lines, runPanelLabelStyle.Render("  Campaign:  ")+runPanelValueStyle.Render(m.session.CampaignName))

	statusStr := string(m.session.Status)
	statusColor := statusPending

	switch m.session.Status {
	case types.RunStateRunning:
		statusColor = statusRunning
	case types.RunStatePaused, types.RunStatePausing:
		statusColor = statusPaused
	case types.RunStateCancelled, types.RunStateCancelling:
		statusColor = statusCancelled
	case types.RunStateCompleted:
		statusColor = statusPassed
	case types.RunStateFailed:
		statusColor = statusFailed
	}

	lines = append(lines, runPanelLabelStyle.Render("  Status:    ")+statusColor.Render(statusStr))
	lines = append(lines, runPanelLabelStyle.Render("  Started:   ")+runPanelValueStyle.Render(formatTime(m.session.StartedAt)))

	if m.session.CompletedAt != nil {
		lines = append(lines, runPanelLabelStyle.Render("  Completed: ")+runPanelValueStyle.Render(formatTime(*m.session.CompletedAt)))
	}

	if m.session.CurrentFlowID != "" {
		lines = append(lines, runPanelLabelStyle.Render("  Flow:      ")+runPanelValueStyle.Render(m.session.CurrentFlowID))
	}

	lines = append(lines, "")
	lines = append(lines, runPanelLabelStyle.Render("  Flows:     ")+runPanelValueStyle.Render(fmt.Sprintf("%d total", len(m.session.Flows))))

	var runningCount, passedCount, failedCount int
	for _, f := range m.session.Flows {
		switch f.Status {
		case types.FlowStateRunning:
			runningCount++
		case types.FlowStatePassed:
			passedCount++
		case types.FlowStateFailed:
			failedCount++
		}
	}

	lines = append(lines, runPanelLabelStyle.Render("    Running:  ")+statusRunning.Render(fmt.Sprintf("%d", runningCount)))
	lines = append(lines, runPanelLabelStyle.Render("    Passed:   ")+statusPassed.Render(fmt.Sprintf("%d", passedCount)))
	lines = append(lines, runPanelLabelStyle.Render("    Failed:   ")+statusFailed.Render(fmt.Sprintf("%d", failedCount)))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
