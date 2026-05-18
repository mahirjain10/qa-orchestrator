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

func (s *SessionStore) Create(name string) (*types.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := types.NewSession(name)
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
		return session, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check after acquiring write lock
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

func (s *SessionStore) Save(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session.UpdatedAt = time.Now().UTC()
	s.sessions[session.RunID] = session
	return s.persist(session)
}

func (s *SessionStore) List() ([]*types.Session, error) {
	s.mu.RLock()
	baseDir := s.baseDir
	s.mu.RUnlock()

	sessionsDir := filepath.Join(baseDir, "sessions")
	if _, err := os.Stat(sessionsDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		return nil, fmt.Errorf("reading sessions directory: %w", err)
	}

	var sessions []*types.Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".json")
		session, err := s.loadFromFile(runID)
		if err != nil {
			return nil, fmt.Errorf("loading session %s: %w", runID, err)
		}
		sessions = append(sessions, session)
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

	session.Status = status
	session.UpdatedAt = time.Now().UTC()
	return s.persist(session)
}

func (s *SessionStore) UpdateFlowState(runID, flowID string, status types.FlowState, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	session.UpdateFlowState(flowID, status, errMsg)
	return s.persist(session)
}

func (s *SessionStore) SaveCheckpoint(runID string, cp *types.Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	session.SetCheckpoint(cp.FlowID, cp.StepID, cp.StepIndex, cp.Payload)
	return s.persist(session)
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
	return filepath.Join(sessionsDir, runID+".json")
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
