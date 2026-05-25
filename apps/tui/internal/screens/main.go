package screens

import (
	"context"
	"fmt"
	"time"

	"qa-orchestrator/apps/tui/internal/components"
	"qa-orchestrator/packages/reporting"
	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
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

func (m *MainScreen) setMsg(msg string) {
	m.msg = msg
	m.msgTime = time.Now()
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
			cmds = append(cmds,
				fetchSessionsCmd(m.sessionStore),
				fetchRunCmd(m.sessionStore, msg.runID),
				fetchTracesCmd(m.traceStore, msg.runID),
				fetchArtifactsCmd(m.artifactStore, msg.runID),
			)
			if m.runCreatedCh != nil {
				cmds = append(cmds, runCreatedCmd(m.runCreatedCh))
			}
			return m, tea.Batch(cmds...)
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
