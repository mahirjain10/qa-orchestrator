package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

const defaultRefreshInterval = 2 * time.Second

type sessionsLoadedMsg struct{ sessions []*types.Session }
type runLoadedMsg struct{ run *types.Session }
type runCreatedMsg struct{ runID string }
type tracesLoadedMsg struct{ traces []*types.TraceEvent }
type artifactsLoadedMsg struct{ artifacts []*artifact.Artifact }
type reportLoadedMsg struct{ report string }
type tickMsg time.Time
type errMsg struct{ err error }

func fetchSessionsCmd(store *session.SessionStore) tea.Cmd {
	return func() tea.Msg {
		sessions, err := store.List()
		if err != nil {
			return errMsg{err}
		}
		return sessionsLoadedMsg{sessions}
	}
}

func fetchRunCmd(store *session.SessionStore, runID string) tea.Cmd {
	return func() tea.Msg {
		if runID == "" {
			return nil
		}
		run, err := store.Get(runID)
		if err != nil {
			return errMsg{err}
		}
		return runLoadedMsg{run}
	}
}

func fetchTracesCmd(store *trace.TraceStore, runID string) tea.Cmd {
	return func() tea.Msg {
		if runID == "" || store == nil {
			return nil
		}
		traces, err := store.GetRecent(runID, 50)
		if err != nil {
			return errMsg{err}
		}
		return tracesLoadedMsg{traces}
	}
}

func fetchArtifactsCmd(store *artifact.ArtifactStore, runID string) tea.Cmd {
	return func() tea.Msg {
		if runID == "" || store == nil {
			return nil
		}
		artifacts, err := store.GetByRunID(runID)
		if err != nil {
			return errMsg{err}
		}
		return artifactsLoadedMsg{artifacts}
	}
}

func fetchReportCmd(reportGenerator interface{ GenerateTerminalSummary(string) (string, error) }, runID string) tea.Cmd {
	return func() tea.Msg {
		if runID == "" || reportGenerator == nil {
			return nil
		}
		report, err := reportGenerator.GenerateTerminalSummary(runID)
		if err != nil {
			return errMsg{err}
		}
		return reportLoadedMsg{report: report}
	}
}

func refreshAllCmd(runID string, sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, reportGenerator interface{ GenerateTerminalSummary(string) (string, error) }) tea.Cmd {
	return tea.Batch(
		fetchSessionsCmd(sessionStore),
		fetchRunCmd(sessionStore, runID),
		fetchTracesCmd(traceStore, runID),
		fetchArtifactsCmd(artifactStore, runID),
		fetchReportCmd(reportGenerator, runID),
	)
}

func startRefreshTicker() tea.Cmd {
	return tea.Tick(defaultRefreshInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func runCreatedCmd(ch <-chan string) tea.Cmd {
	return func() tea.Msg {
		runID := <-ch
		return runCreatedMsg{runID: runID}
	}
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
		sess, err := m.handlers.GetRunStatus(runID)
		if err != nil {
			m.setMsg(fmt.Sprintf("Error getting run status: %v", err))
		} else if sess != nil && sess.Status == types.RunStateWaitingInput {
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
		sess, err := m.handlers.GetRunStatus(runID)
		if err != nil {
			m.setMsg(fmt.Sprintf("Error getting run status: %v", err))
		} else if sess != nil && sess.Status == types.RunStateWaitingInput {
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
			m.lifecycle.SubmitSteering(&types.SteeringEvent{
				RunID:       runID,
				FlowID:      flowID,
				Command:     types.SteerInstruction,
				Instruction: instruction,
				Timestamp:   time.Now().UTC(),
			})
			m.setMsg(fmt.Sprintf("Steering instruction sent: %q", instruction))
			return fetchRunCmd(m.sessionStore, runID)
		}

	default:
		m.setMsg("Unknown command. Try: retry, skip, continue, approve, status, pause, resume, steer")
	}
	return nil
}

func parseSteeringInput(input string) (cmd string, args []string) {
	parts := strings.Fields(input)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}
