package components

import (
	"fmt"
	"strings"
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

	runSpinnerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("75")).
				Bold(true)
)

type RunPanelModel struct {
	session  *types.Session
	width    int
	spinner  string
	spinnerT int
}

func NewRunPanelModel() *RunPanelModel {
	return &RunPanelModel{
		width:    60,
		spinner:  "⠋",
		spinnerT: 0,
	}
}

func (m *RunPanelModel) SetSession(sess *types.Session) {
	m.session = sess
}

func (m *RunPanelModel) Tick() {
	m.spinnerT++
	switch m.spinnerT % 4 {
	case 0:
		m.spinner = "⠋"
	case 1:
		m.spinner = "⠙"
	case 2:
		m.spinner = "⠹"
	case 3:
		m.spinner = "⠸"
	}
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

	var statusDisplay string
	if m.session.Status == types.RunStateRunning {
		statusDisplay = runSpinnerStyle.Render(m.spinner) + " " + statusColor.Render(statusStr)
	} else {
		statusDisplay = statusColor.Render(statusStr)
	}
	lines = append(lines, runPanelLabelStyle.Render("  Status:    ")+statusDisplay)
	lines = append(lines, runPanelLabelStyle.Render("  Started:   ")+runPanelValueStyle.Render(formatTime(m.session.StartedAt)))

	if m.session.CompletedAt != nil {
		lines = append(lines, runPanelLabelStyle.Render("  Completed: ")+runPanelValueStyle.Render(formatTime(*m.session.CompletedAt)))
	}

	if m.session.CurrentFlowID != "" {
		lines = append(lines, runPanelLabelStyle.Render("  Flow:      ")+runPanelValueStyle.Render(m.session.CurrentFlowID))
	}

	if m.session.CurrentAgent != "" {
		agentColor := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
		agentText := m.session.CurrentAgent
		if strings.Contains(strings.ToLower(agentText), "planner") {
			agentColor = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
			agentText += " (planning...)"
		}
		lines = append(lines, runPanelLabelStyle.Render("  Agent:     ")+agentColor.Render(agentText))
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

func (m *RunPanelModel) ViewWithWidth(width int) string {
	title := panelTitleStyle.Width(width - 2).Render(" Active Run ")

	if m.session == nil {
		return title + "\n\n  No active run\n"
	}

	var lines []string

	statusStr := string(m.session.Status)
	statusColor := statusPending
	switch m.session.Status {
	case types.RunStateRunning:
		statusColor = statusRunning
	case types.RunStatePaused, types.RunStatePausing:
		statusColor = statusPaused
	case types.RunStateCompleted:
		statusColor = statusPassed
	case types.RunStateFailed:
		statusColor = statusFailed
	}

	statusLine := fmt.Sprintf("Status: %s", statusColor.Render(statusStr))
	if m.session.Status == types.RunStateRunning {
		statusLine = fmt.Sprintf("Status: %s %s", runSpinnerStyle.Render(m.spinner), statusColor.Render(statusStr))
	}

	runID := m.session.RunID
	if len(runID) > width-12 {
		runID = runID[:width-15] + "..."
	}

	lines = append(lines, fmt.Sprintf("Run: %s", runPanelValueStyle.Render(runID)))
	lines = append(lines, fmt.Sprintf("Campaign: %s", m.session.CampaignName))
	lines = append(lines, statusLine)

	if m.session.CurrentAgent != "" {
		agentColor := lipgloss.NewStyle().Foreground(lipgloss.Color("226"))
		lines = append(lines, fmt.Sprintf("Agent: %s", agentColor.Render(m.session.CurrentAgent)))
	}

	if m.session.CurrentFlowID != "" {
		lines = append(lines, fmt.Sprintf("Flow: %s", runPanelValueStyle.Render(m.session.CurrentFlowID)))
	}

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

	summaryLine := fmt.Sprintf("%d flows | %s %d %s %d %s %d",
		len(m.session.Flows),
		statusRunning.Render("R:"), runningCount,
		statusPassed.Render("P:"), passedCount,
		statusFailed.Render("F:"), failedCount)
	lines = append(lines, summaryLine)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return title + "\n" + content
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
