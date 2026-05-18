package artifact

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
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
	ArtifactID  string        `json:"artifact_id"`
	RunID       string        `json:"run_id"`
	FlowID      string        `json:"flow_id"`
	Type        ArtifactType  `json:"type"`
	Filename    string        `json:"filename"`
	Path        string        `json:"path"`
	Size        int64         `json:"size"`
	MimeType    string        `json:"mime_type"`
	CreatedAt   time.Time     `json:"created_at"`
	Metadata    map[string]any `json:"metadata,omitempty"`
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
		RunID:       runID,
		FlowID:      flowID,
		Type:        artifactType,
		Filename:    actualFilename,
		Path:        path,
		Size:        int64(len(data)),
		MimeType:     s.mimeType(artifactType),
		CreatedAt:   time.Now().UTC(),
		Metadata:    metadata,
	}

	s.artifacts[runID] = append(s.artifacts[runID], artifact)
	s.index[artifactID] = artifact
	s.persistIndex()

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
		return artifact, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if artifact, exists = s.index[artifactID]; exists {
		return artifact, nil
	}

	s.loadIndex()
	if artifact, exists = s.index[artifactID]; exists {
		return artifact, nil
	}

	return nil, fmt.Errorf("artifact not found: %s", artifactID)
}

func (s *ArtifactStore) GetByRunID(runID string) ([]*Artifact, error) {
	s.mu.RLock()
	artifacts, exists := s.artifacts[runID]
	s.mu.RUnlock()

	if exists {
		return artifacts, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if artifacts, exists = s.artifacts[runID]; exists {
		return artifacts, nil
	}

	s.loadIndex()
	var filtered []*Artifact
	for _, a := range s.index {
		if a.RunID == runID {
			filtered = append(filtered, a)
		}
	}
	s.artifacts[runID] = filtered
	return filtered, nil
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
	return filtered, nil
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
	for _, a := range artifacts {
		os.Remove(a.Path)
		delete(s.index, a.ArtifactID)
	}
	delete(s.artifacts, runID)
	s.persistIndex()
	return nil
}

func (s *ArtifactStore) GetRecentPaths(runID string, limit int) []string {
	artifacts, err := s.GetByRunID(runID)
	if err != nil {
		return nil
	}

	if len(artifacts) <= limit {
		paths := make([]string, len(artifacts))
		for i, a := range artifacts {
			paths[i] = a.Path
		}
		return paths
	}

	paths := make([]string, limit)
	for i := len(artifacts) - limit; i < len(artifacts); i++ {
		paths[i-(len(artifacts)-limit)] = artifacts[i].Path
	}
	return paths
}

func (s *ArtifactStore) artifactDir(runID, flowID, artifactType string) string {
	return filepath.Join(s.baseDir, "artifacts", runID, flowID, artifactType)
}

func (s *ArtifactStore) indexPath() string {
	return filepath.Join(s.baseDir, "artifacts", "index.json")
}

func (s *ArtifactStore) persistIndex() {
	indexDir := filepath.Join(s.baseDir, "artifacts")
	if err := os.MkdirAll(indexDir, 0755); err != nil {
		return
	}

	var all []*Artifact
	for _, a := range s.index {
		all = append(all, a)
	}

	path := s.indexPath()
	
	if len(all) == 0 {
		os.Remove(path)
		return
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return
	}
	
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return
	}
	os.Rename(tmpPath, path)
}

func (s *ArtifactStore) loadIndex() {
	path := s.indexPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var all []*Artifact
	if err := json.Unmarshal(data, &all); err != nil {
		return
	}

	for _, a := range all {
		s.index[a.ArtifactID] = a
		s.artifacts[a.RunID] = append(s.artifacts[a.RunID], a)
	}
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}

func SaveArtifact[T []byte | string](store *ArtifactStore, runID, flowID string, artifactType ArtifactType, filename string, content T, metadata map[string]any) (string, error) {
	if store == nil {
		return "", nil
	}
	var data []byte
	switch v := any(content).(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	}
	artifact, err := store.Save(runID, flowID, artifactType, filename, data, metadata)
	if err != nil {
		return "", err
	}
	return artifact.Path, nil
}

func GetArtifactPaths(store *ArtifactStore, runID string, artifactType ArtifactType, limit int) []string {
	if store == nil {
		return nil
	}
	artifacts, err := store.ListByType(runID, artifactType)
	if err != nil {
		return nil
	}
	if len(artifacts) <= limit {
		paths := make([]string, len(artifacts))
		for i, a := range artifacts {
			paths[i] = a.Path
		}
		return paths
	}
	paths := make([]string, limit)
	for i := len(artifacts) - limit; i < len(artifacts); i++ {
		paths[i-(len(artifacts)-limit)] = artifacts[i].Path
	}
	return paths
}

func newArtifactID() string {
	b := make([]byte, 8)
	rand.Read(b)
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