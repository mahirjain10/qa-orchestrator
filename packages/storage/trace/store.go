package trace

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	agentstypes "qa-orchestrator/packages/agents/types"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

type TraceStore struct {
	mu      sync.RWMutex
	baseDir string
	traces  map[string][]*sharedtypes.TraceEvent
}

func NewTraceStore(baseDir string) (*TraceStore, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("creating trace store directory: %w", err)
	}

	return &TraceStore{
		baseDir: baseDir,
		traces:  make(map[string][]*sharedtypes.TraceEvent),
	}, nil
}

func (s *TraceStore) Append(event *sharedtypes.TraceEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.traces[event.RunID] = append(s.traces[event.RunID], event)
	return s.persist(event.RunID)
}

func (s *TraceStore) AppendBatch(runID string, events []*sharedtypes.TraceEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, event := range events {
		event.RunID = runID
		s.traces[runID] = append(s.traces[runID], event)
	}
	return s.persist(runID)
}

func (s *TraceStore) GetByRunID(runID string) ([]*sharedtypes.TraceEvent, error) {
	s.mu.RLock()
	events, exists := s.traces[runID]
	s.mu.RUnlock()

	if exists {
		return events, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if events, exists = s.traces[runID]; exists {
		return events, nil
	}

	events, err := s.loadFromFile(runID)
	if err != nil {
		return nil, err
	}

	s.traces[runID] = events
	return events, nil
}

func (s *TraceStore) GetByFlowID(runID, flowID string) ([]*sharedtypes.TraceEvent, error) {
	events, err := s.GetByRunID(runID)
	if err != nil {
		return nil, err
	}

	var filtered []*sharedtypes.TraceEvent
	for _, e := range events {
		if e.FlowID == flowID {
			filtered = append(filtered, e)
		}
	}
	return filtered, nil
}

func (s *TraceStore) GetRecent(runID string, limit int) ([]*sharedtypes.TraceEvent, error) {
	events, err := s.GetByRunID(runID)
	if err != nil {
		return nil, err
	}

	if len(events) <= limit {
		return events, nil
	}
	return events[len(events)-limit:], nil
}

func (s *TraceStore) ListRunIDs() ([]string, error) {
	tracesDir := filepath.Join(s.baseDir, "traces")
	if _, err := os.Stat(tracesDir); os.IsNotExist(err) {
		return nil, nil
	}

	entries, err := os.ReadDir(tracesDir)
	if err != nil {
		return nil, fmt.Errorf("reading traces directory: %w", err)
	}

	var runIDs []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		runID := strings.TrimSuffix(entry.Name(), ".jsonl")
		runIDs = append(runIDs, runID)
	}
	return runIDs, nil
}

func (s *TraceStore) Delete(runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.traces, runID)
	path := s.filePath(runID)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting trace file: %w", err)
	}
	return nil
}

func (s *TraceStore) filePath(runID string) string {
	tracesDir := filepath.Join(s.baseDir, "traces")
	return filepath.Join(tracesDir, runID+".jsonl")
}

func (s *TraceStore) persist(runID string) error {
	tracesDir := filepath.Join(s.baseDir, "traces")
	if err := os.MkdirAll(tracesDir, 0755); err != nil {
		return fmt.Errorf("creating traces directory: %w", err)
	}

	path := s.filePath(runID)
	events := s.traces[runID]

	var lines []string
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return fmt.Errorf("marshaling trace event: %w", err)
		}
		lines = append(lines, string(data))
	}

	data := []byte(strings.Join(lines, "\n") + "\n")
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing trace file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming trace file: %w", err)
	}

	return nil
}

func (s *TraceStore) loadFromFile(runID string) ([]*sharedtypes.TraceEvent, error) {
	path := s.filePath(runID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return []*sharedtypes.TraceEvent{}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading trace file %s: %w", path, err)
	}

	var events []*sharedtypes.TraceEvent
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event sharedtypes.TraceEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			fmt.Fprintf(os.Stderr, "failed to unmarshal trace event line: %v\n", err)
			continue
		}
		events = append(events, &event)
	}

	return events, nil
}

func EmitStepExecution(store *TraceStore, runID, flowID string, stepResult *agentstypes.StepResult) {
	if store == nil {
		return
	}
	status := sharedtypes.TraceStatusSuccess
	if !stepResult.Success {
		status = sharedtypes.TraceStatusFailed
	}
	event := sharedtypes.NewTraceEvent(runID, flowID, "executor", sharedtypes.TraceEventStepExecution, stepResult.Tool, status)
	event.WithDetails(map[string]any{
		"step_id":  stepResult.StepID,
		"params":   stepResult.Params,
		"duration": stepResult.DurationMs,
	})
	if stepResult.Error != nil {
		event.WithDetail("error", stepResult.Error.Error())
	}
	store.Append(event)
}

func EmitAgentDecision(store *TraceStore, runID, flowID, agent string, action, reason string) {
	if store == nil {
		return
	}
	event := sharedtypes.NewTraceEvent(runID, flowID, agent, sharedtypes.TraceEventAgentDecision, action, sharedtypes.TraceStatusSuccess)
	event.WithDetail("reason", reason)
	store.Append(event)
}

func EmitRecoveryAction(store *TraceStore, runID, flowID string, decision *agentstypes.RecoveryDecision, stepResult *agentstypes.StepResult) {
	if store == nil {
		return
	}
	status := sharedtypes.TraceStatusSuccess
	action := "pending"
	reason := "analyzing failure"

	if decision != nil {
		action = string(decision.Action)
		reason = decision.Reason
		if decision.Action == agentstypes.RecoveryActionFail {
			status = sharedtypes.TraceStatusFailed
		}
	}

	event := sharedtypes.NewTraceEvent(runID, flowID, "recovery", sharedtypes.TraceEventRecoveryAction, action, status)
	event.WithDetail("reason", reason)
	if stepResult != nil {
		event.WithDetail("failed_step", stepResult.StepID)
		event.WithDetail("tool", stepResult.Tool)
	}
	store.Append(event)
}

func EmitLifecycleEvent(store *TraceStore, runID, flowID string, status sharedtypes.RunState, details map[string]any) {
	if store == nil {
		return
	}
	event := sharedtypes.NewTraceEvent(runID, flowID, "system", sharedtypes.TraceEventLifecycleState, string(status), sharedtypes.TraceStatusSuccess)
	event.WithDetails(details)
	store.Append(event)
}

func EmitCheckpoint(store *TraceStore, runID string, cp *sharedtypes.Checkpoint) {
	if store == nil {
		return
	}
	event := sharedtypes.NewTraceEvent(runID, cp.FlowID, "system", sharedtypes.TraceEventCheckpoint, "checkpoint_saved", sharedtypes.TraceStatusSuccess)
	event.WithDetails(map[string]any{
		"step_index": cp.StepIndex,
		"step_id":    cp.StepID,
	})
	store.Append(event)
}

func EmitArtifactEvent(store *TraceStore, runID, flowID, artifactType, path string, metadata map[string]any) {
	if store == nil {
		return
	}
	event := sharedtypes.NewTraceEvent(runID, flowID, "system", sharedtypes.TraceEventArtifact, artifactType, sharedtypes.TraceStatusSuccess)
	event.WithDetail("path", path)
	event.WithDetails(metadata)
	store.Append(event)
}
