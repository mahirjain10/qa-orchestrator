package screens

import (
	"fmt"
	"strings"
	"time"

	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"
	"qa-orchestrator/packages/shared/types"

	"github.com/charmbracelet/lipgloss"
)

// renderDashboardView renders the main dashboard view.
func (m *MainScreen) renderDashboardView() string {
	if m.currentRun == nil {
		selector := m.renderCampaignSelector()
		padding := (m.contentWidth() - 70) / 2
		if padding < 0 {
			padding = 0
		}
		return strings.Repeat(" ", padding) + selector
	}

	summary := m.renderRunSummary()
	flows := m.renderFlowTimeline()

	return lipgloss.JoinVertical(lipgloss.Left,
		summary,
		"",
		flows,
	)
}

// renderRunSummary renders a summary of the current run.
func (m *MainScreen) renderRunSummary() string {
	if m.currentRun == nil {
		return style.Dim.Render("  No run selected")
	}

	sess := m.currentRun
	statusStyle := statusStyleForRun(sess.Status)

	var spinnerStr string
	if sess.Status == types.RunStateRunning {
		spinnerStr = m.spinner.View() + " "
	}

	lines := []string{
		style.ViewTitle.Render(" Run Summary "),
		"",
		fmt.Sprintf("  %s%s", spinnerStr, statusStyle.Render(string(sess.Status))),
		style.Dim.Render("  Campaign: " + sess.CampaignName),
		style.Dim.Render("  Agent:    " + sess.CurrentAgent),
		style.Dim.Render("  Flow:     " + sess.CurrentFlowID),
	}

	var running, passed, failed int
	for _, f := range sess.Flows {
		switch f.Status {
		case types.FlowStateRunning:
			running++
		case types.FlowStatePassed:
			passed++
		case types.FlowStateFailed:
			failed++
		}
	}

	contentWidth := m.contentWidth()

	counts := fmt.Sprintf("  %d flows | %s  %s  %s",
		len(sess.Flows),
		style.StatusRunning.Render(fmt.Sprintf("R:%d", running)),
		style.StatusPassed.Render(fmt.Sprintf("P:%d", passed)),
		style.StatusFailed.Render(fmt.Sprintf("F:%d", failed)),
	)
	lines = append(lines, counts)

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return style.PanelBorder.Width(contentWidth).Padding(0, 1).Render(content)
}

// renderFlowTimeline renders the list of flows within the current run.
func (m *MainScreen) renderFlowTimeline() string {
	if m.currentRun == nil || len(m.currentRun.Flows) == 0 {
		return style.Dim.Render("  No flows")
	}

	lines := []string{
		style.Section.Render("  Flows"),
		"",
	}

	for i, f := range m.currentRun.Flows {
		statusStyle := statusStyleForFlow(f.Status)
		statusChar := statusCharForFlow(f.Status)

		indicator := "  "
		if i == m.flowStatus.GetSelected() && !m.sidebarFocus {
			indicator = style.SelectedBold.Render(" ▶ ")
		}

		row := fmt.Sprintf("%s%s  %s",
			indicator,
			statusStyle.Render(statusChar),
			f.FlowID,
		)
		lines = append(lines, row)
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderFlowsView renders the detailed flows view.
func (m *MainScreen) renderFlowsView() string {
	if m.currentRun == nil || len(m.currentRun.Flows) == 0 {
		return style.Dim.Render("  No flows")
	}

	lines := []string{
		style.ViewTitle.Render(" Flows "),
		"",
	}

	colFlow := util.SafeWidth(m.contentWidth()/3, 16)
	colMode := 10
	colPriority := 10
	colStatus := 12

	headerFmt := fmt.Sprintf("  %%-%ds %%-%ds %%-%ds %%-%ds", colFlow, colMode, colPriority, colStatus)
	lines = append(lines, style.Section.Render(fmt.Sprintf(headerFmt, "Flow", "Mode", "Priority", "Status")))
	lines = append(lines, style.Dim.Render("  "+strings.Repeat("─", m.contentWidth()-4)))

	for i, f := range m.currentRun.Flows {
		statusStyle := statusStyleForFlow(f.Status)

		cursor := "  "
		if i == m.flowStatus.GetSelected() && !m.sidebarFocus {
			cursor = style.SelectedBold.Render(" ▶ ")
		}

		flowID := util.Truncate(f.FlowID, colFlow-4)

		statusStr := statusStyle.Render(string(f.Status))
		statusPad := colStatus - lipgloss.Width(statusStr)
		if statusPad < 0 {
			statusPad = 0
		}
		line := fmt.Sprintf("%s%-*s %-*s %-*s %s%s",
			cursor, colFlow, flowID, colMode, string(f.Mode), colPriority, string(f.Priority),
			statusStr, strings.Repeat(" ", statusPad))
		lines = append(lines, line)

		if i == m.flowStatus.GetSelected() && m.flowStatus.Expanded && !m.sidebarFocus {
			detail := m.renderFlowDetail(f)
			lines = append(lines, detail)
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return style.PanelBorder.Width(m.contentWidth()).Padding(0, 1).Render(content)
}

// renderFlowDetail renders expanded detail for a single flow.
func (m *MainScreen) renderFlowDetail(f types.FlowRunState) string {
	lines := []string{
		style.Dim.Render("    ──────────────────────────────────────"),
	}

	if f.StartedAt != nil {
		lines = append(lines, style.Dim.Render("    Started:  "+f.StartedAt.Format("15:04:05")))
	}
	if f.FinishedAt != nil {
		lines = append(lines, style.Dim.Render("    Finished: "+f.FinishedAt.Format("15:04:05")))
		if f.StartedAt != nil {
			dur := f.FinishedAt.Sub(*f.StartedAt)
			lines = append(lines, style.Dim.Render("    Duration: "+dur.Round(time.Second).String()))
		}
	}
	if f.RetryCount > 0 {
		lines = append(lines, style.StatusRetrying.Render(fmt.Sprintf("    Retries:  %d", f.RetryCount)))
	}
	if f.Error != "" {
		lines = append(lines, style.StatusFailed.Render("    Error:    "+util.Truncate(f.Error, 60)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderTracesView renders the traces panel view.
func (m *MainScreen) renderTracesView() string {
	cw := m.contentWidth()
	ch := m.contentHeight() - 2
	if ch < 5 {
		ch = 5
	}

	m.tracePanel.SetSize(cw, ch)
	return style.PanelBorder.Width(cw).Height(ch).Padding(0, 1).Render(m.tracePanel.Viewport.View())
}

// renderReportView renders the report view.
func (m *MainScreen) renderReportView() string {
	cw := m.contentWidth()

	if m.reportView == "" {
		return style.Dim.Render("  No report generated. Select a run and press 'r' to refresh.")
	}

	return style.PanelBorder.Width(cw).Padding(0, 1).Render(m.reportView)
}

// renderCampaignSelector renders the campaign selection modal.
func (m *MainScreen) renderCampaignSelector() string {
	modalWidth := util.SafeWidth(m.width-20, 40)
	if modalWidth > 70 {
		modalWidth = 70
	}

	title := style.ViewTitle.Render(" Select a Campaign ")
	separator := strings.Repeat("─", modalWidth-4)

	visSessions := m.visualSessions()

	var items []string

	for vi, s := range visSessions {
		prefix := "  "
		if m.campaignList.GetSelected() == vi {
			prefix = style.SelectedBold.Render(" ▶ ")
		}
		age := m.formatSessionAge(s.StartedAt)

		if m.currentRun != nil && s.RunID == m.currentRun.RunID {
			items = append(items, prefix+lipgloss.NewStyle().Foreground(style.BrightGreen).Render(" ● ")+
				s.CampaignName+" ("+
				util.TruncateMiddle(s.RunID, 12)+") "+
				style.Dim.Render("["+age+"]"))
		} else {
			items = append(items, prefix+s.CampaignName+" ("+
				util.TruncateMiddle(s.RunID, 12)+") "+
				style.Dim.Render("["+age+"]"))
		}
	}

	if len(items) == 0 {
		items = append(items, style.Dim.Render("  No campaigns found. Run with: ./app campaign.yaml"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		title,
		separator,
		strings.Join(items, "\n"),
		"",
		style.Dim.Render(" ↑↓ navigate  enter: select  q: quit"),
	)

	return style.ModalBorder.Width(modalWidth).Padding(1, 2).Render(content)
}

// statusStyleForRun returns the lipgloss style for a given run state.
func statusStyleForRun(status types.RunState) lipgloss.Style {
	switch status {
	case types.RunStateRunning:
		return style.StatusRunning
	case types.RunStateCompleted:
		return style.StatusPassed
	case types.RunStateFailed:
		return style.StatusFailed
	case types.RunStatePaused, types.RunStatePausing:
		return style.StatusPaused
	case types.RunStatePending:
		return style.StatusPending
	case types.RunStateCancelled, types.RunStateCancelling:
		return style.StatusCancelled
	default:
		return style.StatusPending
	}
}

// statusStyleForFlow returns the lipgloss style for a given flow state.
func statusStyleForFlow(status types.FlowState) lipgloss.Style {
	switch status {
	case types.FlowStateRunning:
		return style.StatusRunning
	case types.FlowStatePassed:
		return style.StatusPassed
	case types.FlowStateFailed:
		return style.StatusFailed
	case types.FlowStatePaused:
		return style.StatusPaused
	case types.FlowStatePending:
		return style.StatusPending
	case types.FlowStateSkippedUpstream, types.FlowStateSkippedUser:
		return style.StatusCancelled
	case types.FlowStateRetrying:
		return style.StatusRetrying
	case types.FlowStateWaitingInput:
		return style.StatusPaused
	case types.FlowStateBlockedConfigError:
		return style.StatusFailed
	default:
		return style.StatusPending
	}
}

// statusCharForFlow returns the status character for a given flow state.
func statusCharForFlow(status types.FlowState) string {
	switch status {
	case types.FlowStateRunning:
		return "▶"
	case types.FlowStatePassed:
		return "✓"
	case types.FlowStateFailed:
		return "✗"
	case types.FlowStatePaused:
		return "⏸"
	case types.FlowStatePending:
		return "○"
	case types.FlowStateSkippedUpstream, types.FlowStateSkippedUser:
		return "○"
	case types.FlowStateRetrying:
		return "↻"
	case types.FlowStateWaitingInput:
		return "⏳"
	case types.FlowStateBlockedConfigError:
		return "!"
	default:
		return "·"
	}
}
