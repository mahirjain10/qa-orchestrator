package validator

import (
	"fmt"
	"strings"

	"qa-orchestrator/packages/agents/types"
)

type ValidationResult struct {
	Passed     bool
	Errors     []string
	Assertions []AssertionResult
}

type AssertionResult struct {
	Assertion types.Assertion
	Passed    bool
	Message   string
}

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) ValidateStep(step *types.Step, result *types.StepResult) *ValidationResult {
	validation := &ValidationResult{
		Passed:     true,
		Errors:     []string{},
		Assertions: []AssertionResult{},
	}

	if !result.Success {
		validation.Passed = false
		validation.Errors = append(validation.Errors, fmt.Sprintf("step %s failed: %v", step.ID, result.Error))
		return validation
	}

	for _, assertion := range step.Assertions {
		assertionResult := v.validateAssertion(assertion, result.Output)
		validation.Assertions = append(validation.Assertions, assertionResult)
		if !assertionResult.Passed {
			validation.Passed = false
			validation.Errors = append(validation.Errors, assertionResult.Message)
		}
	}

	return validation
}

func (v *Validator) validateAssertion(assertion types.Assertion, output any) AssertionResult {
	result := AssertionResult{
		Assertion: assertion,
		Passed:    true,
	}

	switch assertion.Type {
	case "equals":
		if !equals(output, assertion.Value) {
			result.Passed = false
			result.Message = fmt.Sprintf("expected %v, got %v", assertion.Value, output)
		}
	case "contains":
		outputStr, ok := output.(string)
		if !ok {
			result.Passed = false
			result.Message = "output is not a string"
		} else {
			valStr, valOk := assertion.Value.(string)
			if !valOk {
				result.Passed = false
				result.Message = "assertion value is not a string"
			} else if !strings.Contains(outputStr, valStr) {
				result.Passed = false
				result.Message = fmt.Sprintf("output does not contain '%v'", valStr)
			}
		}
	case "not_empty":
		if output == nil || output == "" {
			result.Passed = false
			result.Message = "output is empty"
		}
	case "is_true":
		if b, ok := output.(bool); !ok || !b {
			result.Passed = false
			result.Message = "output is not true"
		}
	case "is_false":
		if b, ok := output.(bool); !ok || b {
			result.Passed = false
			result.Message = "output is not false"
		}
	default:
		result.Passed = true
	}

	return result
}

func equals(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func (v *Validator) ValidateResults(results []*types.StepResult, steps []types.Step) (bool, []string) {
	allPassed := true
	var errors []string

	for i, result := range results {
		if i < len(steps) {
			validation := v.ValidateStep(&steps[i], result)
			if !validation.Passed {
				allPassed = false
				errors = append(errors, validation.Errors...)
			}
		}

		if !result.Success {
			allPassed = false
			errors = append(errors, fmt.Sprintf("step %s failed", result.StepID))
		}
	}

	return allPassed, errors
}

func (v *Validator) CreateObservation(result *types.StepResult) *types.Observation {
	return &types.Observation{
		State: map[string]any{
			"last_step_id":      result.StepID,
			"last_step_success": result.Success,
			"last_output":       result.Output,
		},
		LastStep: result,
	}
}
