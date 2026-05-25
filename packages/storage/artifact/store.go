package artifact

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"qa-orchestrator/packages/shared"
)

type ArtifactType string

const (
	ArtifactScreenshot ArtifactType = "screenshot"
	ArtifactLog        ArtifactType = "log"
	ArtifactReport     ArtifactType = "report"
	ArtifactHTML       ArtifactType = "html"
	ArtifactTrace      ArtifactType = "trace"
)

type Artifact struct {
	ArtifactID string         `json:"artifact_id"`
	RunID      string         `json:"run_id"`
	FlowID     string         `json:"flow_id"`
	Type       ArtifactType   `json:"type"`
	Filename   string         `json:"filename"`
	Path       string         `json:"path"`
	Size       int64          `json:"size"`
	MimeType   string         `json:"mime_type"`
	CreatedAt  time.Time      `json:"created_at"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

type ArtifactStore struct {
	mu        sync.RWMutex
	baseDir   string
	artifacts map[string][]*Artifact
	index     map[string]*Artifact
}

func NewArtifactStore(baseDir string) (*ArtifactStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating artifact store directory: %w", err)
	}

	return &ArtifactStore{
		baseDir:   baseDir,
		artifacts: make(map[string][]*Artifact),
		index:     make(map[string]*Artifact),
	}, nil
}

func (s *ArtifactStore) Save(runID, flowID string, artifactType ArtifactType, filename string, data []byte, metadata map[string]any) (*Artifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	artifactID := newArtifactID()
	artifactDir := s.artifactDir(runID, flowID, string(artifactType))
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, fmt.Errorf("creating artifact directory: %w", err)
	}

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = s.defaultExtension(artifactType)
	}
	actualFilename := fmt.Sprintf("%s%s", artifactID, ext)
	path := filepath.Join(artifactDir, actualFilename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return nil, fmt.Errorf("writing artifact file: %w", err)
	}

	artifact := &Artifact{
		ArtifactID: artifactID,
		RunID:      runID,
		FlowID:     flowID,
		Type:       artifactType,
		Filename:   actualFilename,
		Path:       path,
		Size:       int64(len(data)),
		MimeType:   s.mimeType(artifactType),
		CreatedAt:  time.Now().UTC(),
		Metadata:   metadata,
	}

	s.artifacts[runID] = append(s.artifacts[runID], artifact)
	s.index[artifactID] = artifact
	if err := s.persistIndex(); err != nil {
		artifacts := s.artifacts[runID]
		if len(artifacts) > 0 && artifacts[len(artifacts)-1].ArtifactID == artifactID {
			s.artifacts[runID] = artifacts[:len(artifacts)-1]
		}
		delete(s.index, artifactID)
		os.Remove(path)
		return nil, fmt.Errorf("persisting artifact index: %w", err)
	}

	return artifact, nil
}

func (s *ArtifactStore) SaveFromFile(runID, flowID string, artifactType ArtifactType, sourcePath string, metadata map[string]any) (*Artifact, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("reading source file: %w", err)
	}

	filename := filepath.Base(sourcePath)
	return s.Save(runID, flowID, artifactType, filename, data, metadata)
}

func (s *ArtifactStore) Get(artifactID string) (*Artifact, error) {
	s.mu.RLock()
	artifact, exists := s.index[artifactID]
	s.mu.RUnlock()

	if exists {
		return shared.CloneDeep(artifact)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if artifact, exists = s.index[artifactID]; exists {
		return shared.CloneDeep(artifact)
	}

	if err := s.loadIndex(); err != nil {
		return nil, err
	}
	if artifact, exists = s.index[artifactID]; exists {
		return shared.CloneDeep(artifact)
	}

	return nil, fmt.Errorf("%w: %s", shared.ErrNotFound, artifactID)
}

func (s *ArtifactStore) GetByRunID(runID string) ([]*Artifact, error) {
	s.mu.RLock()
	artifacts, exists := s.artifacts[runID]
	s.mu.RUnlock()

	if exists {
		return shared.CloneDeepSlice(artifacts)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if artifacts, exists = s.artifacts[runID]; exists {
		return shared.CloneDeepSlice(artifacts)
	}

	if err := s.loadIndex(); err != nil {
		return nil, err
	}
	var filtered []*Artifact
	for _, a := range s.index {
		if a.RunID == runID {
			filtered = append(filtered, a)
		}
	}
	s.artifacts[runID] = filtered
	return shared.CloneDeepSlice(filtered)
}

func (s *ArtifactStore) GetByFlowID(runID, flowID string) ([]*Artifact, error) {
	artifacts, err := s.GetByRunID(runID)
	if err != nil {
		return nil, err
	}

	var filtered []*Artifact
	for _, a := range artifacts {
		if a.FlowID == flowID {
			filtered = append(filtered, a)
		}
	}
	return shared.CloneDeepSlice(filtered)
}

func (s *ArtifactStore) ListByType(runID string, artifactType ArtifactType) ([]*Artifact, error) {
	artifacts, err := s.GetByRunID(runID)
	if err != nil {
		return nil, err
	}

	var filtered []*Artifact
	for _, a := range artifacts {
		if a.Type == artifactType {
			filtered = append(filtered, a)
		}
	}
	return filtered, nil
}

func (s *ArtifactStore) Delete(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	artifacts := s.artifacts[runID]

	// Try to remove all files first. If any fail, abort without modifying state.
	for _, a := range artifacts {
		if err := os.Remove(a.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing artifact %s: %w", a.Path, err)
		}
	}

	// All files removed — now update in-memory state atomically.
	for _, a := range artifacts {
		delete(s.index, a.ArtifactID)
	}
	delete(s.artifacts, runID)

	if err := s.persistIndex(); err != nil {
		return fmt.Errorf("persisting artifact index after delete: %w", err)
	}
	return nil
}

func (s *ArtifactStore) artifactDir(runID, flowID, artifactType string) string {
	artifactsDir := filepath.Join(s.baseDir, "artifacts")
	safeRunID := sanitizeID(runID)
	safeFlowID := sanitizeID(flowID)
	dir := filepath.Join(artifactsDir, safeRunID, safeFlowID, artifactType)
	cleanBase := filepath.Clean(artifactsDir) + string(filepath.Separator)
	if !strings.HasPrefix(filepath.Clean(dir), cleanBase) {
		dir = filepath.Join(artifactsDir, "blocked", artifactType)
	}
	return dir
}

func sanitizeID(id string) string {
	return shared.SanitizeID(id)
}

func (s *ArtifactStore) indexPath() string {
	return filepath.Join(s.baseDir, "artifacts", "index.json")
}

func (s *ArtifactStore) persistIndex() error {
	indexDir := filepath.Join(s.baseDir, "artifacts")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return fmt.Errorf("creating artifact index directory: %w", err)
	}

	var all []*Artifact
	for _, a := range s.index {
		all = append(all, a)
	}

	path := s.indexPath()

	if len(all) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing artifact index: %w", err)
		}
		return nil
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling artifact index: %w", err)
	}

	if err := shared.AtomicWriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing artifact index: %w", err)
	}
	return nil
}

func (s *ArtifactStore) loadIndex() error {
	path := s.indexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading artifact index: %w", err)
	}

	var all []*Artifact
	if err := json.Unmarshal(data, &all); err != nil {
		return fmt.Errorf("parsing artifact index: %w", err)
	}

	for _, a := range all {
		s.index[a.ArtifactID] = a
		s.artifacts[a.RunID] = append(s.artifacts[a.RunID], a)
	}
	return nil
}

func newArtifactID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("art_%d", time.Now().UnixNano())
	}
	return "art_" + hex.EncodeToString(b)
}

func (s *ArtifactStore) defaultExtension(t ArtifactType) string {
	switch t {
	case ArtifactScreenshot:
		return ".png"
	case ArtifactLog:
		return ".log"
	case ArtifactReport:
		return ".html"
	case ArtifactHTML:
		return ".html"
	case ArtifactTrace:
		return ".json"
	default:
		return ".bin"
	}
}

func (s *ArtifactStore) mimeType(t ArtifactType) string {
	switch t {
	case ArtifactScreenshot:
		return "image/png"
	case ArtifactLog:
		return "text/plain"
	case ArtifactReport, ArtifactHTML:
		return "text/html"
	case ArtifactTrace:
		return "application/json"
	default:
		return "application/octet-stream"
	}
}
