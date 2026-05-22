package types

import (
	"time"
)

type SteeringEvent struct {
	RunID       string      `json:"run_id" yaml:"run_id"`
	FlowID      string      `json:"flow_id" yaml:"flow_id"`
	Command     SteeringCmd `json:"command" yaml:"command"`
	Reason      string      `json:"reason,omitempty" yaml:"reason,omitempty"`
	Instruction string      `json:"instruction,omitempty" yaml:"instruction,omitempty"`
	Timestamp   time.Time   `json:"timestamp" yaml:"timestamp"`
}

type SteeringCmd string

const (
	SteerRetry       SteeringCmd = "retry"
	SteerSkip        SteeringCmd = "skip"
	SteerApprove     SteeringCmd = "approve"
	SteerContinue    SteeringCmd = "continue"
	SteerHumanReview SteeringCmd = "human_review"
	SteerInstruction SteeringCmd = "instruction"
)

func NewSteeringEvent(runID, flowID string, cmd SteeringCmd, reason, instruction string) *SteeringEvent {
	return &SteeringEvent{
		RunID:       runID,
		FlowID:      flowID,
		Command:     cmd,
		Reason:      reason,
		Instruction: instruction,
		Timestamp:   time.Now().UTC(),
	}
}

func (e *SteeringEvent) IsRetry() bool       { return e.Command == SteerRetry }
func (e *SteeringEvent) IsSkip() bool        { return e.Command == SteerSkip }
func (e *SteeringEvent) IsApprove() bool     { return e.Command == SteerApprove }
func (e *SteeringEvent) IsContinue() bool    { return e.Command == SteerContinue }
func (e *SteeringEvent) IsHumanReview() bool { return e.Command == SteerHumanReview }
func (e *SteeringEvent) IsInstruction() bool { return e.Command == SteerInstruction }
