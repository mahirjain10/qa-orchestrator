package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/components"
	"qa-orchestrator/apps/tui/internal/state"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

var (
	mainStyle = lipgloss.NewStyle().
			Width(120).
			Height(40)

	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	msgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86"))
)

type TickMsg time.Time

type MainScreen struct {
	state    *state.AppState
	handlers *CommandHandlers

	campaignList *components.CampaignListModel
	runPanel     *components.RunPanelModel
	flowStatus   *components.FlowStatusModel
	tracePanel   *components.TracePanelModel
	artifactPanel *components.ArtifactPanelModel

	traceStore    *trace.TraceStore
	artifactStore *artifact.ArtifactStore

	command       string
	msg           string
	steeringInput string
	steeringMode  bool
}

func NewMainScreen(store *session.SessionStore) *MainScreen {
	appState := state.NewAppState(store)
	handlers := NewCommandHandlers(store)

	return &MainScreen{
		state:          appState,
		handlers:       handlers,
		campaignList:  components.NewCampaignListModel(),
		runPanel:      components.NewRunPanelModel(),
		flowStatus:    components.NewFlowStatusModel(),
		tracePanel:    components.NewTracePanelModel(),
		artifactPanel: components.NewArtifactPanelModel(),
		command:       "",
		msg:           "Press ENTER to select a run, or type a command",
	}
}

func NewMainScreenWithStores(store *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) *MainScreen {
	screen := NewMainScreen(store)
	screen.traceStore = traceStore
	screen.artifactStore = artifactStore
	return screen
}

func (m *MainScreen) Init() tea.Cmd {
	m.refreshAll()
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (m *MainScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		m.refreshAll()
		return m, tea.Tick(time.Second, func(t time.Time) tea.Msg {
			return TickMsg(t)
		})

case tea.KeyMsg:
		if m.steeringMode {
			switch msg.String() {
			case "enter":
				if m.steeringInput != "" {
					m.processSteeringCommand(m.steeringInput)
					m.steeringMode = false
					m.steeringInput = ""
				}
			case "backspace":
				if len(m.steeringInput) > 0 {
					m.steeringInput = m.steeringInput[:len(m.steeringInput)-1]
				}
			case "Escape":
				m.state.SetView(state.ViewCampaignList)
				m.steeringMode = false
				m.steeringInput = ""
			default:
				if len(msg.Runes) > 0 && msg.Runes[0] >= 32 {
					m.steeringInput += string(msg.Runes[0])
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			m.campaignList.Prev()
			m.flowStatus.Prev()

		case "down", "j":
			m.campaignList.Next()
			m.flowStatus.Next()

		case "enter":
			sessions := m.state.GetSessions()
			idx := m.campaignList.GetSelected()
			if idx >= 0 && idx < len(sessions) {
				runID := sessions[idx].RunID
				m.state.SetCurrentRunID(runID)
				m.refreshRun()
				m.msg = fmt.Sprintf("Selected run: %s", runID)
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
			m.state.SetView(state.ViewTraces)
			m.refreshTraces()

		case "a":
			m.state.SetView(state.ViewArtifacts)
			m.refreshArtifacts()

		case "s":
			if m.state.GetCurrentRunID() != "" {
				m.steeringMode = true
				m.steeringInput = ""
				m.msg = "Steering mode: type command and press ENTER. ESC to cancel."
			} else {
				m.msg = "Select a run first before steering"
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *MainScreen) View() string {
	sessions := m.state.GetSessions()

	var campaignNames []string
	for _, s := range sessions {
		campaignNames = append(campaignNames, fmt.Sprintf("%s [%s]", s.CampaignName, s.RunID))
	}
	m.campaignList.SetCampaigns(campaignNames)

	var content string

	currentView := m.state.GetView()
	if currentView == state.ViewFlowStatus {
		content = m.flowStatus.View()
	} else if currentView == state.ViewTraces {
		content = m.tracePanel.View()
	} else if currentView == state.ViewArtifacts {
		content = m.artifactPanel.View()
	} else {
		leftPanel := lipgloss.JoinVertical(
			lipgloss.Left,
			m.campaignList.View(),
			"",
			lipgloss.NewStyle().Height(1).Render(""),
			m.flowStatus.View(),
		)

		rightPanel := m.runPanel.View()

		content = lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftPanel,
			lipgloss.NewStyle().Width(3).Render("  "),
			rightPanel,
)
	}

	footer := helpStyle.Render(" ↑↓ Navigate  Enter Select  Space Pause/Resume  x Cancel  r Refresh  f Flows  t Traces  a Artifacts  s Steer  q Quit")

	viewContent := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(lipgloss.Color("86")).Bold(true).Render("Zenact TUI - Campaign Runner"),
		lipgloss.NewStyle().Render("─────────────────────────────────────────────────────"),
		lipgloss.NewStyle().Height(1).Render(""),
		content,
		lipgloss.NewStyle().Height(1).Render(""),
	)

	if m.steeringMode {
		steeringBanner := lipgloss.NewStyle().
			Foreground(lipgloss.Color("208")).
			Bold(true).
			Render("STEERING MODE: Type command and press ENTER. ESC to cancel.")
		steeringInput := lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")).
			Render(fmt.Sprintf("> %s_", m.steeringInput))
		viewContent = lipgloss.JoinVertical(
			lipgloss.Left,
			viewContent,
			lipgloss.NewStyle().Height(1).Render(""),
			steeringBanner,
			steeringInput,
		)
	}

	viewContent = lipgloss.JoinVertical(
		lipgloss.Left,
		viewContent,
		msgStyle.Render("  "+m.msg),
		footer,
	)

	return mainStyle.Render(viewContent)
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
