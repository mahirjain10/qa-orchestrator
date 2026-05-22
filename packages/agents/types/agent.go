package types

import (
	"encoding/json"
	"fmt"
	"sync"

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
	mu           sync.RWMutex
	FlowID       string
	Steps        []PlanStep
	CurrentIdx   int
	Goal         string
	IsAutonomous bool
	historyCache string
	historyBuilt int
	historyDirty bool
}

func (p *Plan) AddStep(step PlanStep) {
	p.mu.Lock()
	defer p.mu.Unlock()
	step.StepIndex = len(p.Steps)
	if len(step.Params) > 0 {
		if b, err := json.Marshal(step.Params); err == nil {
			step.paramsJSON = string(b)
		}
	}
	p.Steps = append(p.Steps, step)
	p.historyDirty = true
}

func (p *Plan) InvalidateHistoryCache() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.historyDirty = true
}

func (p *Plan) GetHistory() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.CurrentIdx == 0 {
		return "No steps executed yet."
	}
	if p.historyDirty || p.historyBuilt > p.CurrentIdx {
		p.historyCache = ""
		p.historyBuilt = 0
		p.historyDirty = false
	}

	history := p.historyCache
	for i := p.historyBuilt; i < p.CurrentIdx && i < len(p.Steps); i++ {
		step := &p.Steps[i]
		history += fmt.Sprintf("Step %d (%s): tool=%s", i+1, step.StepID, step.Tool)
		if step.paramsJSON == "" && len(step.Params) > 0 {
			if b, err := json.Marshal(step.Params); err == nil {
				step.paramsJSON = string(b)
			}
		}
		if step.paramsJSON != "" {
			history += fmt.Sprintf(", params=%s", step.paramsJSON)
		}
		if step.Skip {
			history += " [SKIPPED: " + step.Reason + "]"
		} else if step.Reason != "" {
			history += " - " + step.Reason
		}
		history += "\n"
	}
	p.historyCache = history
	if p.CurrentIdx < len(p.Steps) {
		p.historyBuilt = p.CurrentIdx
	} else {
		p.historyBuilt = len(p.Steps)
	}
	return history
}

type PlanStep struct {
	StepIndex  int
	StepID     string
	Tool       string
	Params     map[string]any
	paramsJSON string
	Skip       bool
	Reason     string
}

type Assertion = types.Assertion
type Step = types.Step
type Flow = types.Flow

type RecoveryAction string

const (
	RecoveryActionRetry    RecoveryAction = "retry"
	RecoveryActionReplan   RecoveryAction = "replan" // semantically retry-with-observation, not a true replan
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
	RunID                   string
	FlowID                  string
	Goal                    string
	StartURL                string
	CurrentURL              string
	Mode                    types.FlowMode
	Steps                   []types.Step
	Plan                    *Plan
	Observations            []Observation
	SteeringInstructions    []string
	DependencyContext       string
	LastStepSignature       string
	ConsecutiveObserveCount int
}
