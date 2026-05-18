package validator

import (
	"errors"
	"testing"

	"qa-orchestrator/packages/agents/types"
)

func TestValidatorValidateStepSuccess(t *testing.T) {
	v := NewValidator()

	step := &types.Step{
		ID:   "test-step",
		Tool: "log",
	}

	result := &types.StepResult{
		StepID:  "test-step",
		Tool:    "log",
		Success: true,
		Output:  "logged: hello",
	}

	validation := v.ValidateStep(step, result)

	if !validation.Passed {
		t.Errorf("validation.Passed = false, want true")
	}
}

func TestValidatorValidateStepFailure(t *testing.T) {
	v := NewValidator()

	step := &types.Step{
		ID:   "test-step",
		Tool: "log",
	}

	result := &types.StepResult{
		StepID:  "test-step",
		Tool:    "log",
		Success: false,
		Error:   errors.New("tool execution failed"),
	}

	validation := v.ValidateStep(step, result)

	if validation.Passed {
		t.Errorf("validation.Passed = true, want false")
	}

	if len(validation.Errors) == 0 {
		t.Error("validation.Errors should not be empty")
	}
}

func TestValidatorAssertEquals(t *testing.T) {
	v := NewValidator()

	step := &types.Step{
		ID:   "test",
		Tool: "echo",
		Assertions: []types.Assertion{
			{Type: "equals", Value: "hello"},
		},
	}

	result := &types.StepResult{Success: true, Output: "hello"}
	validation := v.ValidateStep(step, result)

	if !validation.Passed {
		t.Errorf("validation.Passed = false, want true: %v", validation.Errors)
	}
}

func TestValidatorAssertContains(t *testing.T) {
	v := NewValidator()

	step := &types.Step{
		ID:   "test",
		Tool: "log",
		Assertions: []types.Assertion{
			{Type: "contains", Value: "world"},
		},
	}

	result := &types.StepResult{Success: true, Output: "hello world"}
	validation := v.ValidateStep(step, result)

	if !validation.Passed {
		t.Errorf("validation.Passed = false, want true: %v", validation.Errors)
	}
}

func TestValidatorAssertNotEmpty(t *testing.T) {
	v := NewValidator()

	step := &types.Step{
		ID:   "test",
		Tool: "log",
		Assertions: []types.Assertion{
			{Type: "not_empty"},
		},
	}

	result := &types.StepResult{Success: true, Output: "some value"}
	validation := v.ValidateStep(step, result)

	if !validation.Passed {
		t.Errorf("validation.Passed = false, want true: %v", validation.Errors)
	}
}

func TestValidatorCreateObservation(t *testing.T) {
	v := NewValidator()

	stepResult := &types.StepResult{
		StepID:  "test-step",
		Success: true,
		Output:  "result",
	}

	obs := v.CreateObservation(stepResult)

	if obs.State["last_step_id"] != "test-step" {
		t.Errorf("last_step_id = %v, want 'test-step'", obs.State["last_step_id"])
	}

	if obs.LastStep != stepResult {
		t.Error("LastStep should reference the step result")
	}
}
