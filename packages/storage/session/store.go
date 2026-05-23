package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"qa-orchestrator/packages/shared/types"
)

type SessionStore struct {
	mu       sync.RWMutex
	baseDir  string
	sessions map[string]*types.Session
}

func NewSessionStore(baseDir string) (*SessionStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating session store directory: %w", err)
	}

	return &SessionStore{
		baseDir:  baseDir,
		sessions: make(map[string]*types.Session),
	}, nil
}

func (s *SessionStore) Create(campaign *types.Campaign) (*types.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := types.NewSession(campaign.Name)

	for _, flow := range campaign.Flows {
		session.AddFlowState(types.FlowRunState{
			FlowID:   flow.ID,
			Name:     flow.Name,
			Mode:     flow.Mode,
			Priority: flow.Priority,
			Status:   types.FlowStatePending,
		})
	}

	s.sessions[session.RunID] = session

	if err := s.persist(session); err != nil {
		delete(s.sessions, session.RunID)
		return nil, fmt.Errorf("persisting initial session: %w", err)
	}

	return session, nil
}

func (s *SessionStore) Get(runID string) (*types.Session, error) {
	s.mu.RLock()
	session, exists := s.sessions[runID]
	s.mu.RUnlock()

	if exists {
		return cloneSession(session)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
	if session, exists := s.sessions[runID]; exists {
		return cloneSession(session)
	}

	session, err := s.loadFromFile(runID)
	if err != nil {
		return nil, err
	}

	s.sessions[runID] = session
	return cloneSession(session)
}

func (s *SessionStore) Save(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned, err := cloneSession(session)
	if err != nil {
		return err
	}
	cloned.UpdatedAt = time.Now().UTC()
	s.sessions[cloned.RunID] = cloned
	return s.persist(cloned)
}

func (s *SessionStore) List() ([]*types.Session, error) {
	s.mu.RLock()
	baseDir := s.baseDir
	cached := make([]*types.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		cloned, err := cloneSession(sess)
		if err != nil {
			s.mu.RUnlock()
			return nil, err
		}
		cached = append(cached, cloned)
	}
	s.mu.RUnlock()

	if len(cached) > 0 {
		return cached, nil
	}

	sessionsDir := filepath.Join(baseDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var sessions []*types.Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".json")
		if sess, exists := s.sessions[runID]; exists {
			cloned, err := cloneSession(sess)
			if err != nil {
				return nil, err
			}
			sessions = append(sessions, cloned)
			continue
		}
		session, err := s.loadFromFile(runID)
		if err != nil {
			return nil, fmt.Errorf("loading session %s: %w", runID, err)
		}
		s.sessions[runID] = session
		cloned, err := cloneSession(session)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, cloned)
	}

	return sessions, nil
}

func (s *SessionStore) UpdateStatus(runID string, status types.RunState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	cloned, err := cloneSession(session)
	if err != nil {
		return err
	}
	cloned.Status = status
	cloned.UpdatedAt = time.Now().UTC()
	s.sessions[runID] = cloned
	return s.persist(cloned)
}

func (s *SessionStore) UpdateFlowState(runID, flowID string, status types.FlowState, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	cloned, err := cloneSession(session)
	if err != nil {
		return err
	}
	cloned.UpdateFlowState(flowID, status, errMsg)
	s.sessions[runID] = cloned
	return s.persist(cloned)
}

func (s *SessionStore) SaveCheckpoint(runID string, cp *types.Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	cloned, err := cloneSession(session)
	if err != nil {
		return err
	}
	cloned.SetCheckpoint(cp.FlowID, cp.StepID, cp.StepIndex, cp.Payload)
	s.sessions[runID] = cloned
	return s.persist(cloned)
}

func (s *SessionStore) Delete(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := s.filePath(runID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting session file: %w", err)
	}
	delete(s.sessions, runID)
	return nil
}

// Private helpers

func cloneSession(sess *types.Session) (*types.Session, error) {
	data, err := json.Marshal(sess)
	if err != nil {
		return nil, fmt.Errorf("cloning session %s: %w", sess.RunID, err)
	}
	var clone types.Session
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil, fmt.Errorf("cloning session %s: %w", sess.RunID, err)
	}
	return &clone, nil
}

func (s *SessionStore) getOrLoadLocked(runID string) (*types.Session, error) {
	if session, exists := s.sessions[runID]; exists {
		return session, nil
	}
	session, err := s.loadFromFile(runID)
	if err != nil {
		return nil, err
	}
	s.sessions[runID] = session
	return session, nil
}

func (s *SessionStore) filePath(runID string) string {
	sessionsDir := filepath.Join(s.baseDir, "sessions")
	safeID := sanitizeID(runID)
	path := filepath.Join(sessionsDir, safeID+".json")
	cleanBase := filepath.Clean(sessionsDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(path), cleanBase) {
		path = filepath.Join(sessionsDir, "blocked_"+sanitizeID(runID)+".json")
	}
	return path
}

func sanitizeID(id string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		"..", "_",
		":", "_",
	)
	return replacer.Replace(id)
}

func (s *SessionStore) persist(session *types.Session) error {
	sessionsDir := filepath.Join(s.baseDir, "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("creating sessions directory: %w", err)
	}

	path := s.filePath(session.RunID)
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling session: %w", err)
	}

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming session file: %w", err)
	}

	return nil
}

func (s *SessionStore) loadFromFile(runID string) (*types.Session, error) {
	path := s.filePath(runID)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading session file %s: %w", path, err)
	}

	var session types.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("unmarshaling session: %w", err)
	}

	return &session, nil
}
