package types

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func NewRunID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return fmt.Sprintf("run_%s_%s", time.Now().Format("20060102"), hex.EncodeToString(b)[:6])
	}
	return fmt.Sprintf("run_%s_%d", time.Now().Format("20060102"), time.Now().UnixNano())
}

func NewSessionID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err == nil {
		return fmt.Sprintf("sess_%s", hex.EncodeToString(b))
	}
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

// NewSession creates a Session with a unique RunID and SessionID.
// It does NOT register the session with any SessionStore. Callers
// must call sessionStore.Create() to persist and register it.
func NewSession(campaignName string) *Session {
	now := time.Now().UTC()
	return &Session{
		RunID:        NewRunID(),
		SessionID:    NewSessionID(),
		CampaignName: campaignName,
		Status:       RunStatePending,
		StartedAt:    now,
		UpdatedAt:    now,
		Flows:        []FlowRunState{},
	}
}

func (s *Session) AddFlowState(frs FlowRunState) {
	s.Flows = append(s.Flows, frs)
	s.UpdatedAt = time.Now().UTC()
}

func (s *Session) UpdateFlowState(flowID string, status FlowState, errMsg string) {
	for i, f := range s.Flows {
		if f.FlowID == flowID {
			now := time.Now().UTC()
			flow := &s.Flows[i]
			flow.Status = status
			flow.Error = errMsg
			if status == FlowStateRunning && flow.StartedAt == nil {
				flow.StartedAt = &now
			}
			if status == FlowStatePassed || status == FlowStateFailed || status == FlowStateSkippedUpstream || status == FlowStateSkippedUser {
				flow.FinishedAt = &now
			}
			break
		}
	}
	s.UpdatedAt = time.Now().UTC()
}

func (s *Session) SetCheckpoint(flowID, stepID string, stepIndex int, payload map[string]any) {
	s.Checkpoint = &Checkpoint{
		FlowID:    flowID,
		StepID:    stepID,
		StepIndex: stepIndex,
		Payload:   payload,
		Timestamp: time.Now().UTC(),
	}
	s.UpdatedAt = time.Now().UTC()
}
