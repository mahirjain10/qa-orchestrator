package screens

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"qa-orchestrator/apps/tui/internal/components"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"
	"qa-orchestrator/packages/reporting"
	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const messageDisplayTimeout = 5 * time.Second

type View string

const (
	ViewDashboard View = "dashboard"
	ViewFlows     View = "flows"
	ViewTraces    View = "traces"
	ViewReport    View = "report"
)

type MainScreen struct {
	sessionStore    *session.SessionStore
	traceStore      *trace.TraceStore
	artifactStore   *artifact.ArtifactStore
	handlers        *CommandHandlers
	reportGenerator *reporting.ReportGenerator

	sessions   []*types.Session
	currentRun *types.Session
	traces     []*types.TraceEvent
	artifacts  []*artifact.Artifact

	width  int
	height int

	activeView   View
	sidebarFocus bool

	campaignList  *components.CampaignListModel
	runPanel      *components.RunPanelModel
	flowStatus    *components.FlowStatusModel
	tracePanel    *components.TracePanelModel
	artifactPanel *components.ArtifactPanelModel

	spinner    spinner.Model
	commandBar *components.CommandBarModel

	reportView string
	msg        string
	msgTime    time.Time
	loading    bool

	cancelFunc   context.CancelFunc
	showHelp     bool
	resumeID     string
	runCreatedCh chan string
	lifecycle    *runtime.LifecycleController
}

func NewMainScreen(store *session.SessionStore) *MainScreen {
	handlers := NewCommandHandlers(store)

	sp := spinner.New()

	cb := components.NewCommandBarModel()

	return &MainScreen{
		sessionStore:  store,
		handlers:      handlers,
		campaignList:  components.NewCampaignListModel(),
		runPanel:      components.NewRunPanelModel(),
		flowStatus:    components.NewFlowStatusModel(),
		tracePanel:    components.NewTracePanelModel(),
		artifactPanel: components.NewArtifactPanelModel(),
		activeView:    ViewDashboard,
		spinner:       sp,
		commandBar:    cb,
		msg:           "1:Dashboard 2:Flows 3:Traces 4:Report | TAB: focus sidebar/content | ↑↓ navigate | q: quit",
	}
}

func NewMainScreenWithStores(store *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) *MainScreen {
	if store == nil {
		panic("session store is required")
	}
	screen := NewMainScreen(store)
	screen.traceStore = traceStore
	screen.artifactStore = artifactStore
	if traceStore != nil && artifactStore != nil {
		screen.reportGenerator = reporting.NewReportGenerator(store, traceStore, artifactStore, "reports")
	}
	return screen
}

func (m *MainScreen) SetMessage(msg string) {
	m.msg = msg
	m.msgTime = time.Now()
}

func (m *MainScreen) SetCancelFunc(fn context.CancelFunc) {
	m.cancelFunc = fn
}

func (m *MainScreen) SetResumeID(id string) {
	m.resumeID = id
}

func (m *MainScreen) SetRunCreatedChannel(ch chan string) {
	m.runCreatedCh = ch
}

func (m *MainScreen) SetLifecycleController(lc *runtime.LifecycleController) {
	m.lifecycle = lc
}

func (m *MainScreen) currentRunID() string {
	if m.currentRun != nil {
		return m.currentRun.RunID
	}
	return ""
}

func (m *MainScreen) Init() tea.Cmd {
	cmds := []tea.Cmd{
		fetchSessionsCmd(m.sessionStore),
		startRefreshTicker(),
	}
	if m.runCreatedCh != nil {
		cmds = append(cmds, runCreatedCmd(m.runCreatedCh))
	}
	return tea.Batch(cmds...)
}

func (m *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		runID := m.currentRunID()
		cmds = append(cmds, refreshAllCmd(runID, m.sessionStore, m.traceStore, m.artifactStore, m.reportGenerator))
		cmds = append(cmds, startRefreshTicker())
		return m, tea.Batch(cmds...)

	case sessionsLoadedMsg:
		m.sessions = msg.sessions
		m.campaignList.SetCampaigns(m.campaignNames())
		if m.currentRun != nil {
			for i, s := range m.sessions {
				if s.RunID == m.currentRun.RunID {
					m.currentRun = m.sessions[i]
					m.runPanel.SetSession(m.currentRun)
					break
				}
			}
		}
		return m, nil

	case runLoadedMsg:
		if msg.run != nil {
			m.currentRun = msg.run
			m.runPanel.SetSession(msg.run)
			m.runPanel.Tick()
			m.flowStatus.SyncFlows(msg.run.Flows)
			for i, s := range m.sessions {
				if s.RunID == msg.run.RunID {
					m.sessions[i] = msg.run
					break
				}
			}
		}
		return m, nil

	case runCreatedMsg:
		if msg.runID != "" {
			m.setMsg(fmt.Sprintf("New session started: %s", msg.runID[:min(8, len(msg.runID))]))
			return m, tea.Batch(
				fetchSessionsCmd(m.sessionStore),
				fetchRunCmd(m.sessionStore, msg.runID),
				fetchTracesCmd(m.traceStore, msg.runID),
				fetchArtifactsCmd(m.artifactStore, msg.runID),
			)
		}
		return m, nil

	case tracesLoadedMsg:
		m.traces = msg.traces
		m.tracePanel.SetEvents(msg.traces)
		return m, nil

	case artifactsLoadedMsg:
		m.artifacts = msg.artifacts
		m.artifactPanel.SetArtifacts(msg.artifacts)
		return m, nil

	case reportLoadedMsg:
		m.reportView = msg.report
		return m, nil

	case errMsg:
		m.setMsg("Error: " + msg.err.Error())
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "?":
			m.showHelp = !m.showHelp
			return m, nil
		case "esc", "escape":
			if m.showHelp {
				m.showHelp = false
				return m, nil
			}
		}
		if m.commandBar.Focused {
			return m.handleCommandKey(msg)
		}
		if m.tracePanel.FilterMode {
			return m.handleFilterKey(msg)
		}
		if m.activeView == ViewTraces {
			switch msg.String() {
			case "pgup", "pgdown", "home", "end":
				if cmd := m.tracePanel.Update(msg); cmd != nil {
					cmds = append(cmds, cmd)
				}
			}
		}
		return m.handleMainKey(msg)
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *MainScreen) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		inputVal := m.commandBar.Input.Value()
		if inputVal != "" {
			cmd := m.processSteeringCommand(inputVal)
			m.commandBar.Blur()
			return m, cmd
		}
		return m, nil
	}

	cmd, handled := m.commandBar.Update(msg)
	if handled {
		return m, cmd
	}
	return m, nil
}

func (m *MainScreen) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.tracePanel.FilterInput, cmd = m.tracePanel.FilterInput.Update(msg)

	if msg.String() == "enter" {
		m.tracePanel.Filter.Text = m.tracePanel.FilterInput.Value()
		m.tracePanel.FilterMode = false
		m.tracePanel.FilterInput.SetValue("")
		m.tracePanel.Selected = 0
		m.tracePanel.UpdateViewportContent()
		if m.tracePanel.Filter.Text != "" {
			m.setMsg(fmt.Sprintf("Filter: \"%s\"", m.tracePanel.Filter.Text))
		} else {
			m.setMsg("Filter cleared")
		}
	}
	if msg.String() == "escape" || msg.String() == "esc" {
		m.tracePanel.FilterMode = false
		m.tracePanel.FilterInput.SetValue("")
		m.setMsg("Filter cancelled")
	}
	return m, cmd
}

func (m *MainScreen) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.activeView = ViewDashboard
		m.setMsg("Dashboard view")

	case "2":
		m.activeView = ViewFlows
		m.setMsg("Flows view")

	case "3":
		m.activeView = ViewTraces
		m.setMsg("Traces view")

	case "4":
		m.activeView = ViewReport
		m.setMsg("Report view")

	case "tab":
		m.sidebarFocus = !m.sidebarFocus
		m.setMsg(map[bool]string{true: "Sidebar focused", false: "Content focused"}[m.sidebarFocus])

	case "up", "k":
		if m.sidebarFocus {
			m.cycleView(-1)
		} else {
			m.handleContentUp()
		}

	case "down", "j":
		if m.sidebarFocus {
			m.cycleView(1)
		} else {
			m.handleContentDown()
		}

	case "enter":
		if m.activeView == ViewFlows && !m.sidebarFocus {
			m.flowStatus.Expanded = !m.flowStatus.Expanded
		} else if m.currentRun == nil {
			idx := m.campaignList.GetSelected()
			if idx >= 0 && idx < len(m.sessions) {
				runID := m.sessions[idx].RunID
				m.currentRun = m.sessions[idx]
				m.setMsg(fmt.Sprintf("Selected run: %s", runID))
				return m, tea.Batch(
					refreshAllCmd(runID, m.sessionStore, m.traceStore, m.artifactStore, m.reportGenerator),
					startRefreshTicker(),
				)
			}
		}

	case "left", "h":
		if m.activeView == ViewFlows && m.flowStatus.Expanded && !m.sidebarFocus {
			m.flowStatus.Expanded = false
		}

	case " ":
		runID := m.currentRunID()
		if runID != "" {
			sess, err := m.handlers.GetRunStatus(runID)
			if err == nil {
				switch sess.Status {
				case types.RunStatePending, types.RunStateRunning:
					err = m.handlers.PauseRun(runID)
					if err == nil {
						m.setMsg("Run pausing...")
						return m, fetchRunCmd(m.sessionStore, runID)
					}
				case types.RunStatePaused:
					err = m.handlers.ResumeRun(runID)
					if err == nil {
						m.setMsg("Run resuming...")
						return m, fetchRunCmd(m.sessionStore, runID)
					}
				case types.RunStatePausing:
					m.setMsg("Run is pausing, please wait")
				case types.RunStateResuming:
					m.setMsg("Run is resuming, please wait")
				case types.RunStateCancelling:
					m.setMsg("Run is cancelling, please wait")
				case types.RunStateWaitingInput:
					err = m.handlers.AcknowledgeInputAndResume(runID)
					if err == nil {
						m.setMsg("Run resumed from WAITING_FOR_INPUT")
						return m, fetchRunCmd(m.sessionStore, runID)
					}
				}
				if err != nil {
					m.setMsg(fmt.Sprintf("Error: %v", err))
				}
			}
		}

	case "x":
		runID := m.currentRunID()
		if runID != "" {
			err := m.handlers.CancelRun(runID)
			if err != nil {
				m.setMsg(fmt.Sprintf("Error cancelling: %v", err))
			} else {
				if m.lifecycle != nil {
					m.lifecycle.RequestCancel()
				}
				if m.cancelFunc != nil {
					m.cancelFunc()
				}
				m.setMsg("Run cancelled")
				return m, fetchRunCmd(m.sessionStore, runID)
			}
		}

	case "r":
		runID := m.currentRunID()
		return m, tea.Batch(
			refreshAllCmd(runID, m.sessionStore, m.traceStore, m.artifactStore, m.reportGenerator),
		)

	case ":":
		if m.currentRunID() != "" {
			m.commandBar.Focus()
			m.setMsg("Command mode: type and press ENTER. ESC to cancel.")
		} else {
			m.setMsg("Select a run first before using commands")
		}

	case "f":
		if m.activeView == ViewTraces {
			m.tracePanel.FollowTail = !m.tracePanel.FollowTail
			m.setMsg(fmt.Sprintf("Follow tail: %v", m.tracePanel.FollowTail))
		}

	case "/":
		if m.activeView == ViewTraces && !m.sidebarFocus {
			m.tracePanel.FilterMode = true
			m.tracePanel.FilterInput.Focus()
			m.setMsg("Filter traces (ESC to cancel)")
		}

	case "S":
		if m.activeView == ViewTraces && !m.sidebarFocus {
			m.tracePanel.Filter.ShowFailed = !m.tracePanel.Filter.ShowFailed
			m.tracePanel.Selected = 0
			m.tracePanel.UpdateViewportContent()
			m.setMsg(fmt.Sprintf("Show failed only: %v", m.tracePanel.Filter.ShowFailed))
		}
	}
	return m, nil
}

func (m *MainScreen) cycleView(dir int) {
	views := []View{ViewDashboard, ViewFlows, ViewTraces, ViewReport}
	idx := 0
	for i, v := range views {
		if v == m.activeView {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(views)) % len(views)
	m.activeView = views[idx]
}

func (m *MainScreen) handleContentUp() {
	switch m.activeView {
	case ViewDashboard:
		m.campaignList.Prev()
	case ViewFlows:
		m.flowStatus.Prev()
	case ViewTraces:
		m.tracePanel.Prev()
	}
}

func (m *MainScreen) handleContentDown() {
	switch m.activeView {
	case ViewDashboard:
		m.campaignList.Next()
	case ViewFlows:
		m.flowStatus.Next()
	case ViewTraces:
		m.tracePanel.Next()
	}
}

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

		row := fmt.Sprintf("%s%%-%ds %%-%ds %%-%ds %%-%ds", cursor, colFlow, colMode, colPriority, colStatus)
		line := fmt.Sprintf(row, flowID, string(f.Mode), string(f.Priority), statusStyle.Render(string(f.Status)))
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
		ch -= 8 // Approximate size of suggestions box
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

	var currentSession *types.Session
	var previousSessions []*types.Session

	for _, s := range m.sessions {
		if m.currentRun != nil && s.RunID == m.currentRun.RunID {
			currentSession = s
		} else {
			previousSessions = append(previousSessions, s)
		}
	}

	sort.Slice(previousSessions, func(i, j int) bool {
		return previousSessions[i].StartedAt.After(previousSessions[j].StartedAt)
	})

	var items []string
	selIdx := 0

	if currentSession != nil {
		prefix := "  "
		if m.campaignList.GetSelected() == selIdx {
			prefix = style.SelectedBold.Render(" ▶ ")
		}
		age := m.formatSessionAge(currentSession.StartedAt)
		items = append(items, prefix+lipgloss.NewStyle().Foreground(style.BrightGreen).Render(" ● ")+
			currentSession.CampaignName+" ("+
			util.TruncateMiddle(currentSession.RunID, 12)+") "+
			style.Dim.Render("["+age+"]"))
		selIdx++
	}

	if len(previousSessions) > 0 {
		items = append(items, style.Dim.Render("  "+strings.Repeat("─", modalWidth-6)))
	}

	for _, s := range previousSessions {
		prefix := "  "
		if m.campaignList.GetSelected() == selIdx {
			prefix = style.SelectedBold.Render(" ▶ ")
		}
		age := m.formatSessionAge(s.StartedAt)
		items = append(items, prefix+s.CampaignName+" ("+
			util.TruncateMiddle(s.RunID, 12)+") "+
			style.Dim.Render("["+age+"]"))
		selIdx++
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

func (m *MainScreen) campaignNames() []string {
	names := []string{}
	for _, s := range m.sessions {
		age := m.formatSessionAge(s.StartedAt)
		marker := ""
		if m.currentRun != nil && s.RunID == m.currentRun.RunID {
			marker = " [CURRENT]"
		}
		names = append(names, fmt.Sprintf("%s [%s] (%s)%s", s.CampaignName, s.RunID, age, marker))
	}
	return names
}

func (m *MainScreen) setMsg(msg string) {
	m.msg = msg
	m.msgTime = time.Now()
}

func (m *MainScreen) processSteeringCommand(input string) tea.Cmd {
	runID := m.currentRunID()
	if runID == "" {
		m.setMsg("No run selected")
		return nil
	}

	cmd, args := parseSteeringInput(input)

	switch cmd {
	case "retry":
		if len(args) > 0 {
			err := m.handlers.RetryFlow(runID, args[0])
			if err != nil {
				m.setMsg(fmt.Sprintf("Error: %v", err))
			} else {
				m.setMsg(fmt.Sprintf("Retry scheduled for flow: %s", args[0]))
				return fetchRunCmd(m.sessionStore, runID)
			}
		} else {
			m.setMsg("Usage: retry <flow_id>")
		}

	case "skip":
		if len(args) > 0 {
			err := m.handlers.SkipFlow(runID, args[0])
			if err != nil {
				m.setMsg(fmt.Sprintf("Error: %v", err))
			} else {
				m.setMsg(fmt.Sprintf("Flow skipped: %s", args[0]))
				return fetchRunCmd(m.sessionStore, runID)
			}
		} else {
			m.setMsg("Usage: skip <flow_id>")
		}

	case "continue":
		sess, _ := m.handlers.GetRunStatus(runID)
		if sess != nil && sess.Status == types.RunStateWaitingInput {
			err := m.handlers.AcknowledgeInputAndResume(runID)
			if err != nil {
				m.setMsg(fmt.Sprintf("Error: %v", err))
			} else {
				m.setMsg("Run resumed from WAITING_FOR_INPUT")
				return fetchRunCmd(m.sessionStore, runID)
			}
		} else {
			m.setMsg("Run is not in WAITING_FOR_INPUT state")
		}

	case "approve":
		sess, _ := m.handlers.GetRunStatus(runID)
		if sess != nil && sess.Status == types.RunStateWaitingInput {
			err := m.handlers.AcknowledgeInputAndResume(runID)
			if err != nil {
				m.setMsg(fmt.Sprintf("Error: %v", err))
			} else {
				m.setMsg("Approval noted and run resumed")
				return fetchRunCmd(m.sessionStore, runID)
			}
		} else {
			m.setMsg("Run is not in WAITING_FOR_INPUT state")
		}

	case "status":
		sess, err := m.handlers.GetRunStatus(runID)
		if err == nil && sess != nil {
			m.setMsg(fmt.Sprintf("Status: %s | Flow: %s | Agent: %s",
				sess.Status, sess.CurrentFlowID, sess.CurrentAgent))
		} else {
			m.setMsg("Could not retrieve status")
		}

	case "pause":
		err := m.handlers.PauseRun(runID)
		if err != nil {
			m.setMsg(fmt.Sprintf("Error pausing: %v", err))
		} else {
			m.setMsg("Run pausing...")
			return fetchRunCmd(m.sessionStore, runID)
		}

	case "resume":
		err := m.handlers.ResumeRun(runID)
		if err != nil {
			m.setMsg(fmt.Sprintf("Error resuming: %v", err))
		} else {
			m.setMsg("Run resuming...")
			return fetchRunCmd(m.sessionStore, runID)
		}

	case "steer":
		if len(args) == 0 {
			m.setMsg("Usage: steer <instruction text>")
		} else if m.lifecycle == nil {
			m.setMsg("Steering not available (no lifecycle controller)")
		} else {
			instruction := strings.Join(args, " ")
			flowID := ""
			if m.currentRun != nil {
				flowID = m.currentRun.CurrentFlowID
			}
			ok := m.lifecycle.SubmitSteering(&types.SteeringEvent{
				RunID:       runID,
				FlowID:      flowID,
				Command:     types.SteerInstruction,
				Instruction: instruction,
				Timestamp:   time.Now().UTC(),
			})
			if ok {
				m.setMsg(fmt.Sprintf("Steering instruction sent: %q", instruction))
			} else {
				m.setMsg("Steering channel full — instruction dropped. Try again later.")
			}
			return fetchRunCmd(m.sessionStore, runID)
		}

	default:
		m.setMsg("Unknown command. Try: retry, skip, continue, approve, status, pause, resume, steer")
	}
	return nil
}

func (m *MainScreen) formatSessionAge(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	elapsed := time.Since(t)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}

func parseSteeringInput(input string) (cmd string, args []string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
