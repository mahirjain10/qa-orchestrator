package types

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

type TraceEventType string

const (
	TraceEventStepExecution   TraceEventType = "step_execution"
	TraceEventAgentDecision   TraceEventType = "agent_decision"
	TraceEventToolResult      TraceEventType = "tool_result"
	TraceEventRecoveryAction  TraceEventType = "recovery_action"
	TraceEventLifecycleState  TraceEventType = "lifecycle_state"
	TraceEventSteeringCommand TraceEventType = "steering_command"
	TraceEventCheckpoint      TraceEventType = "checkpoint"
	TraceEventArtifact        TraceEventType = "artifact"
)

type TraceStatus string

const (
	TraceStatusSuccess TraceStatus = "success"
	TraceStatusFailed  TraceStatus = "failed"
	TraceStatusSkipped TraceStatus = "skipped"
	TraceStatusPending TraceStatus = "pending"
)

type TraceEvent struct {
	EventID   string         `json:"event_id"`
	RunID     string         `json:"run_id"`
	FlowID    string         `json:"flow_id"`
	Agent     string         `json:"agent"`
	EventType TraceEventType `json:"event_type"`
	Action    string         `json:"action"`
	Status    TraceStatus    `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
	Details   map[string]any `json:"details,omitempty"`
}

func NewTraceEvent(runID, flowID, agent string, eventType TraceEventType, action string, status TraceStatus) *TraceEvent {
	return &TraceEvent{
		EventID:   newEventID(),
		RunID:     runID,
		FlowID:    flowID,
		Agent:     agent,
		EventType: eventType,
		Action:    action,
		Status:    status,
		Timestamp: time.Now().UTC(),
		Details:   make(map[string]any),
	}
}

func (e *TraceEvent) WithDetail(key string, value any) *TraceEvent {
	e.Details[key] = value
	return e
}

func (e *TraceEvent) WithDetails(details map[string]any) *TraceEvent {
	for k, v := range details {
		e.Details[k] = v
	}
	return e
}

func newEventID() string {
	b := make([]byte, 6)
	rand.Read(b)
	return "evt_" + hex.EncodeToString(b)
}

func hexEncode(b []byte) string {
	return hex.EncodeToString(b)
}
