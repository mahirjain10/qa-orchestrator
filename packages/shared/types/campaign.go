package types

import (
	"time"
)

type Campaign struct {
	Name        string         `json:"name" yaml:"name"`
	Description string         `json:"description" yaml:"description"`
	Version     string         `json:"version" yaml:"version"`
	Flows       []Flow         `json:"flows" yaml:"flows"`
	Config      CampaignConfig `json:"config" yaml:"config"`
}

type CampaignConfig struct {
	Timeout       time.Duration `json:"timeout" yaml:"timeout"`
	RetryLimit    int           `json:"retry_limit" yaml:"retry_limit"`
	ParallelLimit int           `json:"parallel_limit" yaml:"parallel_limit"`
}

type FlowMode string

const (
	FlowModeGuided     FlowMode = "guided"
	FlowModeAutonomous FlowMode = "autonomous"
)

type FlowPriority string

const (
	FlowPriorityHigh   FlowPriority = "high"
	FlowPriorityMedium FlowPriority = "medium"
	FlowPriorityLow    FlowPriority = "low"
)

type Flow struct {
	ID          string       `json:"id" yaml:"id"`
	Name        string       `json:"name" yaml:"name"`
	Description string       `json:"description" yaml:"description"`
	Goal        string       `json:"goal" yaml:"goal"`
	Mode        FlowMode     `json:"mode" yaml:"mode"`
	Priority    FlowPriority `json:"priority" yaml:"priority"`
	DependsOn   []string     `json:"depends_on" yaml:"depends_on"`
	Steps       []Step       `json:"steps" yaml:"steps"`
	Config      FlowConfig   `json:"config" yaml:"config"`
}

type FlowConfig struct {
	Timeout    time.Duration `json:"timeout" yaml:"timeout"`
	RetryLimit int           `json:"retry_limit" yaml:"retry_limit"`
}

type Step struct {
	ID         string         `json:"id" yaml:"id"`
	Name       string         `json:"name" yaml:"name"`
	Tool       string         `json:"tool" yaml:"tool"`
	Params     map[string]any `json:"params" yaml:"params"`
	Assertions []Assertion    `json:"assertions" yaml:"assertions"`
}

type Assertion struct {
	Type      string `json:"type" yaml:"type"`
	Target    string `json:"target" yaml:"target"`
	Condition string `json:"condition" yaml:"condition"`
	Value     any    `json:"value" yaml:"value"`
}

type RunState string

const (
	RunStatePending      RunState = "PENDING"
	RunStateRunning      RunState = "RUNNING"
	RunStatePausing      RunState = "PAUSING"
	RunStatePaused       RunState = "PAUSED"
	RunStateResuming     RunState = "RESUMING"
	RunStateCancelling   RunState = "CANCELLING"
	RunStateCancelled    RunState = "CANCELLED"
	RunStateCompleted    RunState = "COMPLETED"
	RunStateFailed       RunState = "FAILED"
	RunStateWaitingInput RunState = "WAITING_FOR_INPUT"
)

type FlowState string

const (
	FlowStatePending            FlowState = "PENDING"
	FlowStateRunning            FlowState = "RUNNING"
	FlowStatePassed             FlowState = "PASSED"
	FlowStateFailed             FlowState = "FAILED"
	FlowStateRetrying           FlowState = "RETRYING"
	FlowStateSkippedUpstream    FlowState = "SKIPPED_UPSTREAM_FAILED"
	FlowStateSkippedUser        FlowState = "SKIPPED_USER"
	FlowStateBlockedConfigError FlowState = "BLOCKED_CONFIG_ERROR"
	FlowStatePaused             FlowState = "PAUSED"
	FlowStateWaitingInput       FlowState = "WAITING_FOR_INPUT"
)

type Session struct {
	RunID         string         `json:"run_id" yaml:"run_id"`
	SessionID     string         `json:"session_id" yaml:"session_id"`
	CampaignName  string         `json:"campaign_name" yaml:"campaign_name"`
	Status        RunState       `json:"status" yaml:"status"`
	CurrentFlowID string         `json:"current_flow_id" yaml:"current_flow_id"`
	CurrentAgent  string         `json:"current_agent" yaml:"current_agent"`
	StartedAt     time.Time      `json:"started_at" yaml:"started_at"`
	UpdatedAt     time.Time      `json:"updated_at" yaml:"updated_at"`
	CompletedAt   *time.Time     `json:"completed_at,omitempty" yaml:"completed_at,omitempty"`
	Flows         []FlowRunState `json:"flows" yaml:"flows"`
	RetryCount    int            `json:"retry_count" yaml:"retry_count"`
	Checkpoint    *Checkpoint    `json:"checkpoint,omitempty" yaml:"checkpoint,omitempty"`
}

type FlowRunState struct {
	FlowID     string       `json:"flow_id" yaml:"flow_id"`
	Name       string       `json:"name" yaml:"name"`
	Mode       FlowMode     `json:"mode" yaml:"mode"`
	Priority   FlowPriority `json:"priority" yaml:"priority"`
	Status     FlowState    `json:"status" yaml:"status"`
	StartedAt  *time.Time   `json:"started_at,omitempty" yaml:"started_at,omitempty"`
	FinishedAt *time.Time   `json:"finished_at,omitempty" yaml:"finished_at,omitempty"`
	RetryCount int          `json:"retry_count" yaml:"retry_count"`
	Error      string       `json:"error,omitempty" yaml:"error,omitempty"`
}

type Checkpoint struct {
	FlowID    string         `json:"flow_id" yaml:"flow_id"`
	StepIndex int            `json:"step_index" yaml:"step_index"`
	StepID    string         `json:"step_id" yaml:"step_id"`
	Payload   map[string]any `json:"payload" yaml:"payload"`
	Timestamp time.Time      `json:"timestamp" yaml:"timestamp"`
}

type DependencyError struct {
	FlowID      string
	MissingDeps []string
	CycleDeps   []string
}

func (e *DependencyError) Error() string {
	if len(e.CycleDeps) > 0 {
		return "circular dependency detected"
	}
	return "missing dependencies"
}
