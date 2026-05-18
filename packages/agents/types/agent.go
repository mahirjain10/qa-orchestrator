package types

import (
	"encoding/json"
	"fmt"

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
	FlowID       string
	Steps        []PlanStep
	CurrentIdx   int
	Goal         string
	IsAutonomous bool
}

func (p *Plan) AddStep(step PlanStep) {
	step.StepIndex = len(p.Steps)
	p.Steps = append(p.Steps, step)
}

func (p *Plan) GetHistory() string {
	if p.CurrentIdx == 0 {
		return "No steps executed yet."
	}
	var history string
	for i := 0; i < p.CurrentIdx && i < len(p.Steps); i++ {
		step := p.Steps[i]
		history += fmt.Sprintf("Step %d (%s): tool=%s", i+1, step.StepID, step.Tool)
		if len(step.Params) > 0 {
			if b, err := json.Marshal(step.Params); err == nil {
				history += fmt.Sprintf(", params=%s", string(b))
			}
		}
		if step.Skip {
			history += " [SKIPPED: " + step.Reason + "]"
		} else if step.Reason != "" {
			history += " - " + step.Reason
		}
		history += "\n"
	}
	return history
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
	RecoveryActionRetry    RecoveryAction = "retry"
	RecoveryActionReplan   RecoveryAction = "replan"
	RecoveryActionSkip     RecoveryAction = "skip"
	RecoveryActionEscalate RecoveryAction = "escalate"
	RecoveryActionSucceed  RecoveryAction = "succeed"
	RecoveryActionFail     RecoveryAction = "fail"
)

type RecoveryDecision struct {
	Action     RecoveryAction
	Reason     string
	MaxRetries int
}

type ExecutionContext struct {
	RunID        string
	FlowID       string
	Goal         string
	Mode         types.FlowMode
	Steps        []types.Step
	Plan         *Plan
	Observations []Observation
}
