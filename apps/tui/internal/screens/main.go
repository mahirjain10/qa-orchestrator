package screens

import (
	"fmt"
	"strings"
	"time"

	"qa-orchestrator/apps/tui/internal/components"
	"qa-orchestrator/apps/tui/internal/state"
	"qa-orchestrator/packages/reporting"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

var (
	baseStyle = lipgloss.NewStyle()

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))

	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240"))

	activeBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86")).
			Bold(true)

	highlightBorderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("86"))

	passColor    = lipgloss.Color("76")
	failColor    = lipgloss.Color("204")
	pausedColor  = lipgloss.Color("228")
	runningColor = lipgloss.Color("75")
	pendingColor = lipgloss.Color("245")

	focusCampaignsStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("75")).
		Bold(true)

	focusFlowsStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("226")).
		Bold(true)

	focusTracesStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("208")).
		Bold(true)

	steeringBarStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("86")).
		Bold(true)

	steeringInputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("46"))
)

type Pane string

const (
	PaneCampaigns Pane = "campaigns"
	PaneFlows     Pane = "flows"
	PaneTraces    Pane = "traces"
	PaneRun       Pane = "run"
)

type TickMsg time.Time

type MainScreen struct {
	state    *state.AppState
	handlers *CommandHandlers

	campaignList  *components.CampaignListModel
	runPanel      *components.RunPanelModel
	flowStatus    *components.FlowStatusModel
	tracePanel    *components.TracePanelModel
	artifactPanel *components.ArtifactPanelModel

	traceStore      *trace.TraceStore
	artifactStore   *artifact.ArtifactStore
	reportGenerator *reporting.ReportGenerator

	width  int
	height int

	activePane    Pane
	focusedRunPane bool
	spinner       spinner.Model
	steeringInput textinput.Model
	steeringMode  bool

	reportView string
	command    string
	msg        string
}

func NewMainScreen(store *session.SessionStore) *MainScreen {
	appState := state.NewAppState(store)
	handlers := NewCommandHandlers(store)

	sp := spinner.New()

	ti := textinput.New()
	ti.Placeholder = "Type steering command (retry, skip, continue, status)..."
	ti.Prompt = "│ > "
	ti.CharLimit = 256
	ti.Width = 60

	return &MainScreen{
		state:          appState,
		handlers:       handlers,
		campaignList:   components.NewCampaignListModel(),
		runPanel:       components.NewRunPanelModel(),
		flowStatus:     components.NewFlowStatusModel(),
		tracePanel:     components.NewTracePanelModel(),
		artifactPanel:  components.NewArtifactPanelModel(),
		activePane:     PaneCampaigns,
		spinner:        sp,
		steeringInput:  ti,
		command:        "",
		msg:            "Press ENTER to select a run, SPACE to pause/resume, TAB to switch panes",
	}
}

func NewMainScreenWithStores(store *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) *MainScreen {
	screen := NewMainScreen(store)
	screen.traceStore = traceStore
	screen.artifactStore = artifactStore
	screen.reportGenerator = reporting.NewReportGenerator(store, traceStore, artifactStore, "reports")
	return screen
}

func (m *MainScreen) SetMessage(msg string) {
	m.msg = msg
}

func (m *MainScreen) Init() tea.Cmd {
	m.refreshAll()
	return tea.Batch(
		tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}),
	)
}

func (m *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case TickMsg:
		m.refreshAll()
		return m, nil

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		if m.steeringMode {
			m.steeringInput, cmd = m.steeringInput.Update(msg)
			if msg.String() == "enter" {
				inputVal := m.steeringInput.Value()
				if inputVal != "" {
					m.processSteeringCommand(inputVal)
					m.steeringMode = false
					m.steeringInput.SetValue("")
				}
			}
			if msg.String() == "Escape" {
				m.state.SetView(state.ViewCampaignList)
				m.steeringMode = false
				m.steeringInput.SetValue("")
			}
			return m, cmd
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "tab":
			switch m.activePane {
			case PaneCampaigns:
				m.activePane = PaneFlows
			case PaneFlows:
				m.activePane = PaneTraces
			case PaneTraces:
				m.activePane = PaneRun
			case PaneRun:
				m.activePane = PaneCampaigns
			}
			m.msg = fmt.Sprintf("Focus: %s (Press TAB to switch)", m.activePane)

		case "up", "k":
			switch m.activePane {
			case PaneCampaigns:
				m.campaignList.Prev()
			case PaneFlows:
				m.flowStatus.Prev()
			case PaneTraces:
				m.tracePanel.Prev()
			}

		case "down", "j":
			switch m.activePane {
			case PaneCampaigns:
				m.campaignList.Next()
			case PaneFlows:
				m.flowStatus.Next()
			case PaneTraces:
				m.tracePanel.Next()
			}

		case "enter":
			if m.activePane == PaneCampaigns {
				sessions := m.state.GetSessions()
				idx := m.campaignList.GetSelected()
				if idx >= 0 && idx < len(sessions) {
					runID := sessions[idx].RunID
					m.state.SetCurrentRunID(runID)
					m.refreshRun()
					m.activePane = PaneRun
					m.msg = fmt.Sprintf("Selected run: %s | ↑↓ to navigate, TAB to switch panes", runID)
				}
			}

		case " ":
			runID := m.state.GetCurrentRunID()
			if runID != "" {
				sess, err := m.handlers.GetRunStatus(runID)
				if err == nil {
					switch sess.Status {
					case types.RunStatePending, types.RunStateRunning:
						err = m.handlers.PauseRun(runID)
						m.msg = "Run paused"
					case types.RunStatePaused:
						err = m.handlers.ResumeRun(runID)
						m.msg = "Run resumed"
					}
					if err != nil {
						m.msg = fmt.Sprintf("Error: %v", err)
					}
					m.refreshRun()
				}
			}

		case "x":
			runID := m.state.GetCurrentRunID()
			if runID != "" {
				err := m.handlers.CancelRun(runID)
				if err != nil {
					m.msg = fmt.Sprintf("Error cancelling: %v", err)
				} else {
					m.msg = "Run cancelled"
				}
				m.refreshRun()
			}

		case "r":
			m.refreshAll()
			m.msg = "Refreshed"

		case "l":
			if m.state.GetView() == state.ViewCampaignList {
				m.state.SetView(state.ViewActiveRun)
			} else {
				m.state.SetView(state.ViewCampaignList)
			}

		case "f":
			m.state.SetView(state.ViewFlowStatus)
			m.refreshFlowStatus()

		case "t":
			m.activePane = PaneTraces
			m.state.SetView(state.ViewTraces)
			m.refreshTraces()

		case "a":
			m.state.SetView(state.ViewArtifacts)
			m.refreshArtifacts()

		case "v":
			if m.state.GetCurrentRunID() != "" {
				m.state.SetView(state.ViewReport)
				m.refreshReport()
			} else {
				m.msg = "Select a run first to view report"
			}

		case "s":
			if m.state.GetCurrentRunID() != "" {
				m.steeringMode = true
				m.steeringInput.Focus()
				m.msg = "Steering mode: type command and press ENTER. ESC to cancel."
			} else {
				m.msg = "Select a run first before steering"
			}
		}
		return m, nil
	}

	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *MainScreen) View() string {
	sessions := m.state.GetSessions()

	var campaignNames []string
	for _, s := range sessions {
		campaignNames = append(campaignNames, fmt.Sprintf("%s [%s]", s.CampaignName, s.RunID))
	}
	m.campaignList.SetCampaigns(campaignNames)

	// Calculate dimensions
	leftWidth := 40
	rightWidth := 80
	mainHeight := m.height - 7 // account for header, footer, steering input, padding
	if mainHeight < 10 {
		mainHeight = 10
	}

	if m.width > 0 {
		leftWidth = m.width / 3
		if leftWidth < 35 {
			leftWidth = 35
		}
		rightWidth = m.width - leftWidth - 2
		if rightWidth < 50 {
			rightWidth = 50
		}
	}

	topHeight := mainHeight / 2
	bottomHeight := mainHeight - topHeight

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 1)

	focusedPanelStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("75")).
			Bold(true).
			Padding(0, 1)

	campaignListView := m.campaignList.ViewWithWidth(leftWidth - 4)
	flowStatusView := m.flowStatus.ViewWithWidth(leftWidth - 4)

	var leftTopPanel, leftBottomPanel string
	if m.activePane == PaneCampaigns {
		leftTopPanel = focusedPanelStyle.Width(leftWidth).Height(topHeight).Render(campaignListView)
	} else {
		leftTopPanel = panelStyle.Width(leftWidth).Height(topHeight).Render(campaignListView)
	}

	if m.activePane == PaneFlows {
		leftBottomPanel = focusedPanelStyle.Width(leftWidth).Height(bottomHeight).Render(flowStatusView)
	} else {
		leftBottomPanel = panelStyle.Width(leftWidth).Height(bottomHeight).Render(flowStatusView)
	}

	leftCol := lipgloss.JoinVertical(lipgloss.Left, leftTopPanel, leftBottomPanel)

	// Right Side (Run & Traces or Other Views)
	var rightCol string
	currentView := m.state.GetView()

	switch currentView {
	case state.ViewTraces:
		rightCol = panelStyle.Width(rightWidth).Height(mainHeight).Render(m.tracePanel.ViewCompact())
	case state.ViewArtifacts:
		rightCol = panelStyle.Width(rightWidth).Height(mainHeight).Render(m.artifactPanel.View())
	case state.ViewReport:
		rightCol = panelStyle.Width(rightWidth).Height(mainHeight).Render(m.reportView)
	default:
		runPanelView := m.runPanel.ViewWithWidth(rightWidth - 4)
		tracesView := m.tracePanel.ViewCompact()

		var rightTopStyle *lipgloss.Style
		if m.activePane == PaneRun {
			rightTopStyle = &focusedPanelStyle
		} else {
			rightTopStyle = &panelStyle
		}

		rightTopPanel := rightTopStyle.Width(rightWidth).Height(topHeight).Render(runPanelView)
		rightBottomPanel := panelStyle.Width(rightWidth).Height(bottomHeight).Render(tracesView)

		rightCol = lipgloss.JoinVertical(lipgloss.Left, rightTopPanel, rightBottomPanel)
	}

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, rightCol)

	header := headerStyle.Render("Zenact TUI - Campaign Runner") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" │ ") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("75")).Render("●") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(fmt.Sprintf(" Focus: %s ", m.activePane))

	footer := helpStyle.Render("↑↓ Navigate │ Enter Select │ Space Pause/Resume │ x Cancel │ r Refresh │ t Traces │ a Artifacts │ v Report │ s Steer │ q Quit")

	viewContent := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render(header),
		lipgloss.NewStyle().Render(strings.Repeat("─", m.width)),
		content,
	)

	if m.steeringMode {
		steeringBar := lipgloss.NewStyle().
			Background(lipgloss.Color("235")).
			Foreground(lipgloss.Color("86")).
			Width(m.width - 2).
			Render(" STEERING MODE ")

		steeringInputView := lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")).
			Render(fmt.Sprintf("> %s█", m.steeringInput.View()))

		steeringHint := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Render(" [ENTER] Execute  [ESC] Cancel")

		steeringBox := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("208")).
			Width(m.width - 2).
			Render(lipgloss.JoinVertical(
				lipgloss.Left,
				steeringBar,
				steeringInputView,
				steeringHint,
			))
		viewContent = lipgloss.JoinVertical(
			lipgloss.Left,
			viewContent,
			steeringBox,
		)
	} else {
		msgBox := lipgloss.NewStyle().
			Padding(1, 0, 0, 1).
			Render(msgStyle.Render(m.msg))
		viewContent = lipgloss.JoinVertical(lipgloss.Left, viewContent, msgBox)
	}

	viewContent = lipgloss.JoinVertical(
		lipgloss.Left,
		viewContent,
		lipgloss.NewStyle().Padding(1, 0, 0, 0).Render(footer),
	)

	return viewContent
}

func (m *MainScreen) refreshAll() {
	m.state.RefreshSessions()
	m.refreshRun()
	m.refreshFlowStatus()
}

func (m *MainScreen) refreshRun() {
	sess, err := m.handlers.GetRunStatus(m.state.GetCurrentRunID())
	if err == nil && sess != nil {
		m.runPanel.SetSession(sess)
		m.runPanel.Tick()
	}
}

func (m *MainScreen) refreshFlowStatus() {
	sess, err := m.handlers.GetRunStatus(m.state.GetCurrentRunID())
	if err == nil && sess != nil {
		m.flowStatus.SetFlows(sess.Flows)
	}
}

func (m *MainScreen) refreshTraces() {
	runID := m.state.GetCurrentRunID()
	if runID != "" && m.traceStore != nil {
		events, err := m.traceStore.GetRecent(runID, 50)
		if err == nil {
			m.tracePanel.SetEvents(events)
		}
	}
}

func (m *MainScreen) refreshArtifacts() {
	runID := m.state.GetCurrentRunID()
	if runID != "" && m.artifactStore != nil {
		artifacts, err := m.artifactStore.GetByRunID(runID)
		if err == nil {
			m.artifactPanel.SetArtifacts(artifacts)
		}
	}
}

func (m *MainScreen) refreshReport() {
	runID := m.state.GetCurrentRunID()
	if runID != "" && m.reportGenerator != nil {
		report, err := m.reportGenerator.GenerateTerminalSummary(runID)
		if err == nil {
			m.reportView = report
		} else {
			m.reportView = fmt.Sprintf("Error generating report: %v", err)
		}
	}
}

func (m *MainScreen) processSteeringCommand(input string) {
	runID := m.state.GetCurrentRunID()
	if runID == "" {
		m.msg = "No run selected"
		return
	}

	cmd, args := parseSteeringInput(input)

	switch cmd {
	case "retry":
		if len(args) > 0 {
			err := m.handlers.RetryFlow(runID, args[0])
			if err != nil {
				m.msg = fmt.Sprintf("Error: %v", err)
			} else {
				m.msg = fmt.Sprintf("Retry scheduled for flow: %s", args[0])
			}
		} else {
			m.msg = "Usage: retry <flow_id>"
		}

	case "skip":
		if len(args) > 0 {
			err := m.handlers.SkipFlow(runID, args[0])
			if err != nil {
				m.msg = fmt.Sprintf("Error: %v", err)
			} else {
				m.msg = fmt.Sprintf("Flow skipped: %s", args[0])
			}
		} else {
			m.msg = "Usage: skip <flow_id>"
		}

	case "continue":
		sess, _ := m.handlers.GetRunStatus(runID)
		if sess != nil && sess.Status == types.RunStateWaitingInput {
			err := m.handlers.AcknowledgeInputAndResume(runID)
			if err != nil {
				m.msg = fmt.Sprintf("Error: %v", err)
			} else {
				m.msg = "Run resumed from WAITING_FOR_INPUT"
			}
		} else {
			m.msg = "Run is not in WAITING_FOR_INPUT state"
		}

	case "approve":
		m.msg = "Approval noted"

	case "status":
		sess, err := m.handlers.GetRunStatus(runID)
		if err == nil && sess != nil {
			m.msg = fmt.Sprintf("Status: %s | Flow: %s | Agent: %s",
				sess.Status, sess.CurrentFlowID, sess.CurrentAgent)
		} else {
			m.msg = "Could not retrieve status"
		}

	default:
		m.msg = "Unknown command. Try: retry, skip, continue, approve, status"
	}
}

func parseSteeringInput(input string) (cmd string, args []string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
