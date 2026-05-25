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

func (m *MainScreen) View() string {
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	if m.width < 80 {
		return style.StatusFailed.Render(fmt.Sprintf("  Terminal too narrow (min 80 columns). Current: %d", m.width))
	}
	if m.height < 20 {
		return style.StatusFailed.Render(fmt.Sprintf("  Terminal too short (min 20 rows). Current: %d", m.height))
	}

	sidebar := m.renderSidebar()
	mainContent := m.renderMainContent()

	sbw := m.sidebarWidth()
	cw := m.width - sbw - 2
	ch := m.contentHeight()

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		style.SidebarBorder.Width(sbw).Height(ch).Render(sidebar),
		lipgloss.NewStyle().Width(cw).Height(ch).Render(mainContent),
	)

	viewContent := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		body,
		m.renderCommandBar(),
		m.renderStatusBar(),
	)

	if m.showHelp {
		viewContent = m.renderHelpOverlay(viewContent)
	}

	return viewContent
}

func (m *MainScreen) renderHeader() string {
	runID := ""
	if m.currentRun != nil {
		runID = " | " + m.currentRun.RunID[:min(8, len(m.currentRun.RunID))]
	}
	return style.Header.Render("QA Orchestrator TUI") +
		lipgloss.NewStyle().Foreground(style.DimGray).Render(runID)
}

func (m *MainScreen) renderHelpOverlay(underlay string) string {
	modalW := 60
	if m.width-20 < modalW {
		modalW = util.SafeWidth(m.width-20, 50)
	}
	if modalW > 60 {
		modalW = 60
	}
	modalH := 22

	title := style.ViewTitle.Render(" Keyboard Shortcuts ")
	sep := strings.Repeat("─", modalW-4)

	var lines []string
	lines = append(lines, style.Section.Render("  Global (always available)"))
	lines = append(lines, style.Dim.Render("  q / ctrl+c    Quit"))
	lines = append(lines, style.Dim.Render("  ?              Toggle this help"))
	lines = append(lines, "")

	switch m.activeView {
	case ViewDashboard:
		lines = append(lines, style.Section.Render("  Dashboard"))
		lines = append(lines, style.Dim.Render("  1-4            Switch views"))
		lines = append(lines, style.Dim.Render("  TAB            Toggle sidebar/content focus"))
		lines = append(lines, style.Dim.Render("  ↑/k  ↓/j       Navigate (context-aware)"))
		lines = append(lines, style.Dim.Render("  enter          Select campaign / expand flow"))
		lines = append(lines, style.Dim.Render("  space          Pause / resume run"))
		lines = append(lines, style.Dim.Render("  x              Cancel run"))
		lines = append(lines, style.Dim.Render("  :              Command mode"))
		lines = append(lines, style.Dim.Render("  r              Manual refresh"))
	case ViewFlows:
		lines = append(lines, style.Section.Render("  Flows"))
		lines = append(lines, style.Dim.Render("  1-4            Switch views"))
		lines = append(lines, style.Dim.Render("  TAB            Toggle sidebar/content focus"))
		lines = append(lines, style.Dim.Render("  ↑/k  ↓/j       Navigate flows"))
		lines = append(lines, style.Dim.Render("  enter          Expand / collapse flow detail"))
		lines = append(lines, style.Dim.Render("  left / h       Collapse detail"))
		lines = append(lines, style.Dim.Render("  r              Refresh"))
	case ViewTraces:
		lines = append(lines, style.Section.Render("  Traces"))
		lines = append(lines, style.Dim.Render("  1-4            Switch views"))
		lines = append(lines, style.Dim.Render("  TAB            Toggle sidebar/content focus"))
		lines = append(lines, style.Dim.Render("  ↑/k  ↓/j       Navigate events"))
		lines = append(lines, style.Dim.Render("  pgup/pgdown    Scroll viewport"))
		lines = append(lines, style.Dim.Render("  /              Filter traces"))
		lines = append(lines, style.Dim.Render("  S              Toggle failures-only"))
		lines = append(lines, style.Dim.Render("  f              Toggle follow-tail"))
	case ViewReport:
		lines = append(lines, style.Section.Render("  Report"))
		lines = append(lines, style.Dim.Render("  1-4            Switch views"))
		lines = append(lines, style.Dim.Render("  r              Refresh report"))
	}

	lines = append(lines, "")
	lines = append(lines, style.Section.Render("  Text Commands (type : to enter command mode)"))
	lines = append(lines, style.Dim.Render("  retry <id>     Retry a failed flow"))
	lines = append(lines, style.Dim.Render("  skip <id>      Skip a flow"))
	lines = append(lines, style.Dim.Render("  continue       Resume from WAITING_FOR_INPUT"))
	lines = append(lines, style.Dim.Render("  approve        Approve pending input and resume"))
	lines = append(lines, style.Dim.Render("  status         Show current run status"))
	lines = append(lines, style.Dim.Render("  pause          Pause the current run"))
	lines = append(lines, style.Dim.Render("  resume         Resume a paused run"))
	lines = append(lines, style.Dim.Render("  steer <text>   Send instruction to autonomous flow"))
	lines = append(lines, "")
	lines = append(lines, style.Section.Render("  Command Mode"))
	lines = append(lines, style.Dim.Render("  enter          Execute command"))
	lines = append(lines, style.Dim.Render("  esc            Exit command mode"))
	lines = append(lines, "")
	lines = append(lines, style.Section.Render("  Filter Mode"))
	lines = append(lines, style.Dim.Render("  enter          Apply filter"))
	lines = append(lines, style.Dim.Render("  esc            Cancel filter"))

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	modal := style.ModalBorder.Width(modalW).Padding(1, 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, sep, content),
	)

	vertOffset := max(0, (m.height-modalH)/2)
	horizOffset := max(0, (m.width-modalW)/2)

	dimmed := style.DimmedBg.Render(underlay)

	centered := lipgloss.NewStyle().
		PaddingTop(vertOffset).
		PaddingLeft(horizOffset).
		Render(modal)

	return lipgloss.JoinVertical(lipgloss.Top, dimmed, centered)
}

func (m *MainScreen) renderStatusBar() string {
	if m.height < 20 {
		return ""
	}

	var left, right string

	if m.currentRun != nil {
		statusStyle := statusStyleForRun(m.currentRun.Status)
		truncated := m.currentRun.RunID
		if len(truncated) > 12 {
			truncated = truncated[:12]
		}
		left = statusStyle.Render(" "+string(m.currentRun.Status)+" ") +
			style.Dim.Render(" "+truncated)
	} else {
		left = style.Dim.Render(" IDLE")
	}

	right = m.contextualKeys()

	rightLen := lipgloss.Width(right)
	leftLen := lipgloss.Width(left)
	gap := m.width - leftLen - rightLen - 4
	if gap < 0 {
		gap = 0
	}
	spacer := lipgloss.NewStyle().Width(gap).Render("")

	bar := lipgloss.NewStyle().
		Background(style.BgDark).
		Width(m.width).
		Render(left + spacer + right)

	var msgLine string
	if time.Since(m.msgTime) < messageDisplayTimeout && m.msg != "" {
		msgLine = style.Msg.Render(" " + m.msg + " ")
	}

	if msgLine != "" {
		return lipgloss.JoinVertical(lipgloss.Left, msgLine, bar)
	}
	return bar
}

func (m *MainScreen) contextualKeys() string {
	switch m.activeView {
	case ViewDashboard:
		if m.currentRun != nil {
			return style.Dim.Render("space:pause  x:cancel  :command  ?:help")
		}
		return style.Dim.Render("enter:select  r:refresh  ?:help")
	case ViewTraces:
		return style.Dim.Render("/:filter  S:failures  F:follow  ?:help")
	case ViewFlows:
		return style.Dim.Render("enter:detail  r:retry  k:skip  ?:help")
	default:
		return style.Dim.Render("?:help")
	}
}

func (m *MainScreen) renderCommandBar() string {
	if m.height < 20 {
		return ""
	}

	m.commandBar.SetWidth(m.width)
	return m.commandBar.View()
}

func (m *MainScreen) renderSidebar() string {
	views := []struct {
		id    View
		label string
		key   string
	}{
		{ViewDashboard, "Dashboard", "1"},
		{ViewFlows, "Flows", "2"},
		{ViewTraces, "Traces", "3"},
		{ViewReport, "Report", "4"},
	}

	lines := []string{
		style.Section.Render("  VIEWS"),
		"",
	}

	for _, v := range views {
		var line string
		if v.id == m.activeView && m.sidebarFocus {
			line = style.SelectedBold.Render(" " + v.key + " " + v.label + " ")
		} else if v.id == m.activeView {
			line = style.Normal.Bold(true).Render(" " + v.key + " " + v.label)
		} else {
			line = style.Dim.Render(" " + v.key + " " + v.label)
		}
		lines = append(lines, line)
	}

	if m.currentRun != nil {
		lines = append(lines, "")
		lines = append(lines, style.Section.Render("  RUN"))
		lines = append(lines, style.Dim.Render("  "+m.currentRun.RunID[:min(8, len(m.currentRun.RunID))]))
		statusStyle := statusStyleForRun(m.currentRun.Status)
		lines = append(lines, statusStyle.Render("  "+string(m.currentRun.Status)))
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *MainScreen) renderMainContent() string {
	switch m.activeView {
	case ViewDashboard:
		return m.renderDashboardView()
	case ViewFlows:
		return m.renderFlowsView()
	case ViewTraces:
		return m.renderTracesView()
	case ViewReport:
		return m.renderReportView()
	default:
		return style.Dim.Render("  Unknown view")
	}
}

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

func (m *MainScreen) sidebarWidth() int {
	if m.width < 90 {
		return 16
	}
	if m.width < 100 {
		return 20
	}
	return 24
}

func (m *MainScreen) contentWidth() int {
	w := m.width - m.sidebarWidth() - 4
	if w < 40 {
		w = 40
	}
	return w
}

func (m *MainScreen) contentHeight() int {
	ch := m.height - 5
	if m.height < 25 {
		ch = m.height - 3
	}
	if m.commandBar != nil && m.commandBar.Focused {
		ch -= 2 + m.commandBar.SuggestionCount()
	}
	if ch < 5 {
		return 5
	}
	return ch
}

func (m *MainScreen) renderTracesView() string {
	cw := m.contentWidth()
	ch := m.contentHeight() - 2
	if ch < 5 {
		ch = 5
	}

	m.tracePanel.SetSize(cw, ch)
	return style.PanelBorder.Width(cw).Height(ch).Padding(0, 1).Render(m.tracePanel.Viewport.View())
}

func (m *MainScreen) renderReportView() string {
	cw := m.contentWidth()

	if m.reportView == "" {
		return style.Dim.Render("  No report generated. Select a run and press 'r' to refresh.")
	}

	return style.PanelBorder.Width(cw).Padding(0, 1).Render(m.reportView)
}

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
