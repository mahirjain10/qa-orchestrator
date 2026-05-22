package screens

import (
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
