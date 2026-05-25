package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"qa-orchestrator/packages/shared"
	"qa-orchestrator/packages/shared/types"
)

type SessionStore struct {
	mu       sync.RWMutex
	baseDir  string
	sessions map[string]*types.Session
	order    []string // insertion/access order; front = oldest
	maxSize  int
}

const DefaultMaxCacheSize = 50

func NewSessionStore(baseDir string) (*SessionStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating session store directory: %w", err)
	}

	return &SessionStore{
		baseDir:  baseDir,
		sessions: make(map[string]*types.Session),
		order:    make([]string, 0),
		maxSize:  DefaultMaxCacheSize,
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
	s.touchOrder(session.RunID)
	s.evictIfNeeded()

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
		s.mu.Lock()
		s.touchOrder(runID)
		s.mu.Unlock()
		return session.Clone(), nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if session, exists := s.sessions[runID]; exists {
		s.touchOrder(runID)
		return session.Clone(), nil
	}

	session, err := s.loadFromFile(runID)
	if err != nil {
		return nil, err
	}

	s.sessions[runID] = session
	s.touchOrder(runID)
	s.evictIfNeeded()
	return session.Clone(), nil
}

func (s *SessionStore) Save(session *types.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cloned := session.Clone()
	cloned.UpdatedAt = time.Now().UTC()
	s.sessions[cloned.RunID] = cloned
	s.touchOrder(cloned.RunID)
	s.evictIfNeeded()
	return s.persist(cloned)
}

func (s *SessionStore) List() ([]*types.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	baseDir := s.baseDir
	cached := make([]*types.Session, 0, len(s.sessions))
	for _, sess := range s.sessions {
		cloned := sess.Clone()
		cached = append(cached, cloned)
	}

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

	var sessions []*types.Session
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".json")
		if sess, exists := s.sessions[runID]; exists {
			sessions = append(sessions, sess.Clone())
			continue
		}
		session, err := s.loadFromFile(runID)
		if err != nil {
			return nil, fmt.Errorf("loading session %s: %w", runID, err)
		}
		s.sessions[runID] = session
		s.touchOrder(runID)
		s.evictIfNeeded()
		sessions = append(sessions, session.Clone())
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

	cloned := session.Clone()
	cloned.Status = status
	cloned.UpdatedAt = time.Now().UTC()
	s.sessions[runID] = cloned
	s.touchOrder(runID)
	s.evictIfNeeded()
	return s.persist(cloned)
}

func (s *SessionStore) UpdateFlowState(runID, flowID string, status types.FlowState, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	cloned := session.Clone()
	cloned.UpdateFlowState(flowID, status, errMsg)
	s.sessions[runID] = cloned
	s.touchOrder(runID)
	s.evictIfNeeded()
	return s.persist(cloned)
}

func (s *SessionStore) SaveCheckpoint(runID string, cp *types.Checkpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.getOrLoadLocked(runID)
	if err != nil {
		return err
	}

	cloned := session.Clone()
	cloned.SetCheckpoint(cp.FlowID, cp.StepID, cp.StepIndex, cp.Payload)
	s.sessions[runID] = cloned
	s.touchOrder(runID)
	s.evictIfNeeded()
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
	for i, id := range s.order {
		if id == runID {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	return nil
}

// Cache management

// touchOrder marks a session as recently accessed by moving it to the back.
// Must be called with s.mu held (write lock).
func (s *SessionStore) touchOrder(runID string) {
	for i, id := range s.order {
		if id == runID {
			s.order = append(s.order[:i], s.order[i+1:]...)
			break
		}
	}
	s.order = append(s.order, runID)
}

// evictIfNeeded removes the oldest terminal-state session from the cache
// when the cache exceeds maxSize. Never evicts sessions in a running/pending state.
// Must be called with s.mu held (write lock).
func (s *SessionStore) evictIfNeeded() {
	for len(s.sessions) > s.maxSize && len(s.order) > 0 {
		oldest := s.order[0]
		s.order = s.order[1:]

		if session, exists := s.sessions[oldest]; exists {
			switch session.Status {
			case types.RunStateRunning, types.RunStatePending,
				types.RunStateWaitingInput:
				// Put it back at the end and try the next one
				s.order = append(s.order, oldest)
				continue
			}
		}

		delete(s.sessions, oldest)
	}
}

// Private helpers

func (s *SessionStore) getOrLoadLocked(runID string) (*types.Session, error) {
	if session, exists := s.sessions[runID]; exists {
		s.touchOrder(runID)
		return session.Clone(), nil
	}
	session, err := s.loadFromFile(runID)
	if err != nil {
		return nil, err
	}
	s.sessions[runID] = session
	s.touchOrder(runID)
	s.evictIfNeeded()
	return session.Clone(), nil
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
	return shared.SanitizeID(id)
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

	if err := shared.AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing session file: %w", err)
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
