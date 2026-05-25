package screens

import (
	"fmt"
	"strings"
	"time"

	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"

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
