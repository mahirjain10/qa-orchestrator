package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"
	"qa-orchestrator/packages/shared/types"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸"}

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
	m.spinner = spinnerFrames[m.spinnerT%4]
}

func (m *RunPanelModel) View() string {
	if m.session == nil {
		return style.Header.Render("Active Run") + "\n\n  No active run\n"
	}

	lines := []string{}
	lines = append(lines, style.Header.Render("Active Run"))
	lines = append(lines, "")

	lines = append(lines, style.Section.Render("  Run ID:    ")+style.Normal.Render(m.session.RunID))
	lines = append(lines, style.Section.Render("  Campaign:  ")+style.Normal.Render(m.session.CampaignName))

	statusStr := string(m.session.Status)
	statusColor := style.StatusPending

	switch m.session.Status {
	case types.RunStateRunning:
		statusColor = style.StatusRunning
	case types.RunStatePaused, types.RunStatePausing:
		statusColor = style.StatusPaused
	case types.RunStateCancelled, types.RunStateCancelling:
		statusColor = style.StatusCancelled
	case types.RunStateCompleted:
		statusColor = style.StatusPassed
	case types.RunStateFailed:
		statusColor = style.StatusFailed
	}

	var statusDisplay string
	if m.session.Status == types.RunStateRunning {
		statusDisplay = style.StatusRunning.Render(m.spinner) + " " + statusColor.Render(statusStr)
	} else {
		statusDisplay = statusColor.Render(statusStr)
	}
	lines = append(lines, style.Section.Render("  Status:    ")+statusDisplay)
	lines = append(lines, style.Section.Render("  Started:   ")+style.Normal.Render(formatTime(m.session.StartedAt)))

	if m.session.CompletedAt != nil {
		lines = append(lines, style.Section.Render("  Completed: ")+style.Normal.Render(formatTime(*m.session.CompletedAt)))
	}

	if m.session.CurrentFlowID != "" {
		lines = append(lines, style.Section.Render("  Flow:      ")+style.Normal.Render(m.session.CurrentFlowID))
	}

	if m.session.CurrentAgent != "" {
		agentColor := lipgloss.NewStyle().Foreground(style.Yellow)
		agentText := m.session.CurrentAgent
		if strings.Contains(strings.ToLower(agentText), "planner") {
			agentColor = lipgloss.NewStyle().Foreground(style.BrightYellow).Bold(true)
			agentText += " (planning...)"
		}
		lines = append(lines, style.Section.Render("  Agent:     ")+agentColor.Render(agentText))
	}

	lines = append(lines, "")
	lines = append(lines, style.Section.Render("  Flows:     ")+style.Normal.Render(fmt.Sprintf("%d total", len(m.session.Flows))))

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

	lines = append(lines, style.Section.Render("    Running:  ")+style.StatusRunning.Render(fmt.Sprintf("%d", runningCount)))
	lines = append(lines, style.Section.Render("    Passed:   ")+style.StatusPassed.Render(fmt.Sprintf("%d", passedCount)))
	lines = append(lines, style.Section.Render("    Failed:   ")+style.StatusFailed.Render(fmt.Sprintf("%d", failedCount)))

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *RunPanelModel) ViewWithWidth(width int) string {
	title := style.ViewTitle.Width(width - 2).Render(" Active Run ")

	if m.session == nil {
		return title + "\n\n  No active run\n"
	}

	var lines []string

	statusStr := string(m.session.Status)
	statusColor := style.StatusPending
	switch m.session.Status {
	case types.RunStateRunning:
		statusColor = style.StatusRunning
	case types.RunStatePaused, types.RunStatePausing:
		statusColor = style.StatusPaused
	case types.RunStateCancelled, types.RunStateCancelling:
		statusColor = style.StatusCancelled
	case types.RunStateCompleted:
		statusColor = style.StatusPassed
	case types.RunStateFailed:
		statusColor = style.StatusFailed
	}

	statusLine := fmt.Sprintf("Status: %s", statusColor.Render(statusStr))
	if m.session.Status == types.RunStateRunning {
		statusLine = fmt.Sprintf("Status: %s %s", style.StatusRunning.Render(m.spinner), statusColor.Render(statusStr))
	}

	runID := util.Truncate(m.session.RunID, util.SafeWidth(width-12, 4))

	lines = append(lines, fmt.Sprintf("Run: %s", style.Normal.Render(runID)))
	lines = append(lines, fmt.Sprintf("Campaign: %s", m.session.CampaignName))
	lines = append(lines, statusLine)

	if m.session.CurrentAgent != "" {
		agentColor := lipgloss.NewStyle().Foreground(style.Yellow)
		lines = append(lines, fmt.Sprintf("Agent: %s", agentColor.Render(m.session.CurrentAgent)))
	}

	if m.session.CurrentFlowID != "" {
		lines = append(lines, fmt.Sprintf("Flow: %s", style.Normal.Render(m.session.CurrentFlowID)))
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
		style.StatusRunning.Render("R:"), runningCount,
		style.StatusPassed.Render("P:"), passedCount,
		style.StatusFailed.Render("F:"), failedCount)
	lines = append(lines, summaryLine)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return title + "\n" + content
}

func formatTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}
