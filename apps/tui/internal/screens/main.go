package screens

import (
	"fmt"
	"strings"
	"time"

	"qa-orchestrator/apps/tui/internal/components"
	"qa-orchestrator/apps/tui/internal/state"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/packages/reporting"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸"}

type Pane string

type ComponentID string

const (
	CompCampaigns ComponentID = "campaigns"
	CompFlows     ComponentID = "flows"
	CompRun       ComponentID = "run"
	CompTraces    ComponentID = "traces"
	CompArtifacts ComponentID = "artifacts"
	CompReport    ComponentID = "report"
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

	quadrants     [4]ComponentID
	activeSlot    int
	maximized     bool
	maximizedSlot int

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
		state:         appState,
		handlers:      handlers,
		campaignList:  components.NewCampaignListModel(),
		runPanel:      components.NewRunPanelModel(),
		flowStatus:    components.NewFlowStatusModel(),
		tracePanel:    components.NewTracePanelModel(),
		artifactPanel: components.NewArtifactPanelModel(),
		quadrants:     [4]ComponentID{CompCampaigns, CompFlows, CompRun, CompTraces},
		activeSlot:    0,
		spinner:       sp,
		steeringInput: ti,
		command:       "",
		msg:           "TAB: switch slot | p: cycle component | m: maximize | ←↑↓→: navigate",
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

func (m *MainScreen) componentLabel(id ComponentID) string {
	switch id {
	case CompCampaigns:
		return " [Campaigns] "
	case CompFlows:
		return " [Flows] "
	case CompRun:
		return " [Run] "
	case CompTraces:
		return " [Traces] "
	case CompArtifacts:
		return " [Artifacts] "
	case CompReport:
		return " [Report] "
	default:
		return " [?] "
	}
}

func (m *MainScreen) renderComponent(id ComponentID, width, height int, focused bool) string {
	var content string
	switch id {
	case CompCampaigns:
		campaignNames := []string{}
		sessions := m.state.GetSessions()
		for _, s := range sessions {
			campaignNames = append(campaignNames, fmt.Sprintf("%s [%s]", s.CampaignName, s.RunID))
		}
		m.campaignList.SetCampaigns(campaignNames)
		content = m.campaignList.ViewWithWidth(width - 4)
	case CompFlows:
		content = m.flowStatus.ViewWithWidth(width - 4)
	case CompRun:
		content = m.runPanel.ViewWithWidth(width - 4)
	case CompTraces:
		content = m.tracePanel.ViewCompact()
	case CompArtifacts:
		content = m.artifactPanel.View()
	case CompReport:
		content = m.reportView
	default:
		content = "Unknown component"
	}

	borderColor := lipgloss.Color("240")
	if focused {
		borderColor = m.focusColorForSlot(m.activeSlot)
	}

	label := m.componentLabel(id)

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Bold(focused).
		Padding(0, 1).
		Width(width).
		Height(height)

	titleStyle := lipgloss.NewStyle().
		Foreground(borderColor).
		Bold(focused)

	return style.Render(titleStyle.Render(label) + "\n" + content)
}

func (m *MainScreen) focusColorForSlot(slot int) lipgloss.Color {
	switch slot {
	case 0:
		return lipgloss.Color("75")
	case 1:
		return lipgloss.Color("226")
	case 2:
		return lipgloss.Color("208")
	case 3:
		return lipgloss.Color("86")
	default:
		return lipgloss.Color("75")
	}
}

func (m *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case TickMsg:
		m.refreshAll()
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})

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
			if msg.String() == "escape" || msg.String() == "esc" {
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
			if m.maximized {
				m.maximized = false
				m.msg = "Restored 4-quadrant view"
			} else {
				m.activeSlot = (m.activeSlot + 1) % 4
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}

		case "left":
			if !m.maximized {
				m.activeSlot = (m.activeSlot + 3) % 4
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}

		case "right":
			if !m.maximized {
				m.activeSlot = (m.activeSlot + 1) % 4
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}

		case "p":
			if !m.maximized {
				currentID := m.quadrants[m.activeSlot]
				allComponents := []ComponentID{CompCampaigns, CompFlows, CompRun, CompTraces, CompArtifacts, CompReport}
				nextIdx := 0
				for i, c := range allComponents {
					if c == currentID {
						nextIdx = (i + 1) % len(allComponents)
						break
					}
				}
				m.quadrants[m.activeSlot] = allComponents[nextIdx]
				m.msg = fmt.Sprintf("Slot %d → %s", m.activeSlot, m.quadrants[m.activeSlot])
			}

		case "w":
			if !m.maximized {
				nextSlot := (m.activeSlot + 1) % 4
				m.quadrants[m.activeSlot], m.quadrants[nextSlot] = m.quadrants[nextSlot], m.quadrants[m.activeSlot]
				m.msg = fmt.Sprintf("Swapped slot %d ↔ %d", m.activeSlot, nextSlot)
			}

		case "0":
			if !m.maximized {
				m.activeSlot = 0
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}
		case "1":
			if !m.maximized {
				m.activeSlot = 1
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}
		case "2":
			if !m.maximized {
				m.activeSlot = 2
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}
		case "3":
			if !m.maximized {
				m.activeSlot = 3
				m.msg = fmt.Sprintf("Slot %d: %s", m.activeSlot, m.quadrants[m.activeSlot])
			}

		case "m":
			if m.maximized {
				m.maximized = false
				m.msg = "Restored 4-quadrant view"
			} else {
				m.maximized = true
				m.maximizedSlot = m.activeSlot
				m.msg = fmt.Sprintf("Maximized: %s", m.quadrants[m.maximizedSlot])
			}

		case "escape", "esc":
			if m.maximized {
				m.maximized = false
				m.msg = "Restored 4-quadrant view"
			}

		case "up", "k":
			activeComp := m.quadrants[m.activeSlot]
			switch activeComp {
			case CompCampaigns:
				m.campaignList.Prev()
			case CompFlows:
				m.flowStatus.Prev()
			case CompTraces:
				m.tracePanel.Prev()
			}

		case "down", "j":
			activeComp := m.quadrants[m.activeSlot]
			switch activeComp {
			case CompCampaigns:
				m.campaignList.Next()
			case CompFlows:
				m.flowStatus.Next()
			case CompTraces:
				m.tracePanel.Next()
			}

		case "enter":
			if m.quadrants[m.activeSlot] == CompCampaigns {
				sessions := m.state.GetSessions()
				idx := m.campaignList.GetSelected()
				if idx >= 0 && idx < len(sessions) {
					runID := sessions[idx].RunID
					m.state.SetCurrentRunID(runID)
					m.refreshRun()
					m.msg = fmt.Sprintf("Selected run: %s | ↑↓ navigate | TAB switch slot", runID)
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
	if m.width == 0 || m.height == 0 {
		return "Initializing..."
	}

	header := style.Header.Render("QA Orchestrator TUI - Campaign Runner") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(" │ ") +
		lipgloss.NewStyle().Foreground(m.focusColorForSlot(m.activeSlot)).Render("●") +
		lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render(fmt.Sprintf(" Slot %d: %s ", m.activeSlot, m.quadrants[m.activeSlot]))

	contentWidth := m.width - 2
	contentHeight := m.height - 5

	var content string
	if m.maximized {
		slot := m.maximizedSlot
		focusedComp := m.quadrants[slot]
		content = m.renderComponent(focusedComp, contentWidth, contentHeight, true)
	} else {
		colWidth := contentWidth / 2
		if colWidth < 30 {
			colWidth = 30
		}
		rowHeight := contentHeight / 2
		if rowHeight < 5 {
			rowHeight = 5
		}

		q0 := m.renderComponent(m.quadrants[0], colWidth, rowHeight, m.activeSlot == 0)
		q1 := m.renderComponent(m.quadrants[1], colWidth, rowHeight, m.activeSlot == 1)
		q2 := m.renderComponent(m.quadrants[2], colWidth, rowHeight, m.activeSlot == 2)
		q3 := m.renderComponent(m.quadrants[3], colWidth, rowHeight, m.activeSlot == 3)

		topRow := lipgloss.JoinHorizontal(lipgloss.Top, q0, q1)
		bottomRow := lipgloss.JoinHorizontal(lipgloss.Top, q2, q3)
		content = lipgloss.JoinVertical(lipgloss.Left, topRow, bottomRow)
	}

	footer := style.Help.Render("TAB/←→: switch slot │ 0-3: jump │ p: cycle │ w: swap │ m: maximize │ ↑↓ Navigate │ Enter: select │ Space: pause │ x: cancel │ s: steer │ q: quit")

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
			Render(style.Msg.Render(m.msg))
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
	m.refreshTraces()
	m.refreshArtifacts()
	m.refreshReport()
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
