package state

import (
	"sync"

	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/session"
)

type View string

const (
	ViewCampaignList View = "campaign_list"
	ViewActiveRun    View = "active_run"
	ViewFlowStatus   View = "flow_status"
	ViewTraces       View = "traces"
	ViewArtifacts    View = "artifacts"
)

type AppState struct {
	mu           sync.RWMutex
	SessionStore *session.SessionStore
	Sessions     []*types.Session
	CurrentRunID string
	CurrentView  View
	SelectedIdx  int
}

func NewAppState(store *session.SessionStore) *AppState {
	return &AppState{
		SessionStore: store,
		CurrentView:  ViewCampaignList,
		SelectedIdx:  0,
	}
}

func (s *AppState) SetView(v View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentView = v
}

func (s *AppState) GetView() View {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentView
}

func (s *AppState) SetCurrentRunID(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentRunID = runID
}

func (s *AppState) GetCurrentRunID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.CurrentRunID
}

func (s *AppState) RefreshSessions() error {
	sessions, err := s.SessionStore.List()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.Sessions = sessions
	s.mu.Unlock()
	return nil
}

func (s *AppState) GetSessions() []*types.Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Sessions
}

func (s *AppState) GetCurrentSession() (*types.Session, error) {
	runID := s.GetCurrentRunID()
	if runID == "" {
		return nil, nil
	}
	return s.SessionStore.Get(runID)
}

func (s *AppState) SetSelectedIdx(idx int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedIdx = idx
}

func (s *AppState) GetSelectedIdx() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SelectedIdx
}

func (s *AppState) IncrementSelected() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SelectedIdx++
}

func (s *AppState) DecrementSelected() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SelectedIdx > 0 {
		s.SelectedIdx--
	}
}
