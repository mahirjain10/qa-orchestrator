package types

import (
	"qa-orchestrator/packages/shared/types"
)

type AgentType string

const (
	AgentTypePlanner   AgentType = "planner"
	AgentTypeExecutor  AgentType = "executor"
	AgentTypeValidator AgentType = "validator"
	AgentTypeRecovery  AgentType = "recovery"
)

type AgentResult struct {
	Success   bool
	Output    string
	Error     error
	NextAgent AgentType
	StepIndex int
}

type Observation struct {
	State    map[string]any
	LastStep *StepResult
	Error    error
}

type StepResult struct {
	StepID     string
	Tool       string
	Params     map[string]any
	Output     any
	Error      error
	Success    bool
	DurationMs int64
}

type Plan struct {
	FlowID     string
	Steps      []PlanStep
	CurrentIdx int
	Goal       string
}

type PlanStep struct {
	StepIndex int
	StepID    string
	Tool      string
	Params    map[string]any
	Skip      bool
	Reason    string
}

type Assertion = types.Assertion
type Step = types.Step
type Flow = types.Flow

type RecoveryAction string

const (
	RecoveryActionRetry      RecoveryAction = "retry"
	RecoveryActionReplan     RecoveryAction = "replan"
	RecoveryActionSkip       RecoveryAction = "skip"
	RecoveryActionEscalate   RecoveryAction = "escalate"
	RecoveryActionSucceed    RecoveryAction = "succeed"
	RecoveryActionFail       RecoveryAction = "fail"
)

type RecoveryDecision struct {
	Action   RecoveryAction
	Reason   string
	MaxRetries int
}

type ExecutionContext struct {
	RunID       string
	FlowID      string
	Goal        string
	Steps       []types.Step
	Plan        *Plan
	Observations []Observation
}


