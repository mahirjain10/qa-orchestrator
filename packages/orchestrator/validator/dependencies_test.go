package validator

import (
	"testing"

	"qa-orchestrator/packages/shared/types"
)

func TestValidate_ValidNoDependencies(t *testing.T) {
	v := NewDependencyValidator()
	flows := []types.Flow{
		{ID: "f1", Name: "Flow 1"},
		{ID: "f2", Name: "Flow 2"},
	}

	result := v.Validate(flows)
	if !result.Valid {
		t.Errorf("expected valid, got error: %v", result.Error)
	}
	if len(result.TopologicalOrder) != 2 {
		t.Errorf("expected 2 flows in order, got %d", len(result.TopologicalOrder))
	}
}

func TestValidate_ValidWithDependencies(t *testing.T) {
	v := NewDependencyValidator()
	flows := []types.Flow{
		{ID: "f1", Name: "Flow 1"},
		{ID: "f2", Name: "Flow 2", DependsOn: []string{"f1"}},
	}

	result := v.Validate(flows)
	if !result.Valid {
		t.Errorf("expected valid, got error: %v", result.Error)
	}
	if result.TopologicalOrder[0] != "f1" {
		t.Errorf("expected f1 first, got %s", result.TopologicalOrder[0])
	}
}

func TestValidate_MissingDependency(t *testing.T) {
	v := NewDependencyValidator()
	flows := []types.Flow{
		{ID: "f1", Name: "Flow 1", DependsOn: []string{"nonexistent"}},
	}

	result := v.Validate(flows)
	if result.Valid {
		t.Error("expected invalid for missing dependency")
	}
	if len(result.Error.MissingDeps) != 1 || result.Error.MissingDeps[0] != "nonexistent" {
		t.Errorf("expected missing dep 'nonexistent', got %v", result.Error.MissingDeps)
	}
}

func TestValidate_CircularDependency(t *testing.T) {
	v := NewDependencyValidator()
	flows := []types.Flow{
		{ID: "f1", Name: "Flow 1", DependsOn: []string{"f2"}},
		{ID: "f2", Name: "Flow 2", DependsOn: []string{"f1"}},
	}

	result := v.Validate(flows)
	if result.Valid {
		t.Error("expected invalid for circular dependency")
	}
	if len(result.Error.CycleDeps) == 0 {
		t.Error("expected cycle detection")
	}
}

func TestGetEligibleFlows(t *testing.T) {
	v := NewDependencyValidator()
	flows := []types.Flow{
		{ID: "f1", Name: "Flow 1"},
		{ID: "f2", Name: "Flow 2", DependsOn: []string{"f1"}},
		{ID: "f3", Name: "Flow 3"},
	}

	// No completed flows — only zero-dependency flows are eligible
	eligible := v.GetEligibleFlows(flows, nil)
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible flows with nil completed, got %d", len(eligible))
	}
	eligible = v.GetEligibleFlows(flows, map[string]bool{})
	if len(eligible) != 2 {
		t.Errorf("expected 2 eligible flows with empty completed, got %d", len(eligible))
	}

	// f1 completed — f2 becomes eligible
	eligible = v.GetEligibleFlows(flows, map[string]bool{"f1": true})
	if len(eligible) != 3 {
		t.Errorf("expected 3 eligible flows after f1 completes, got %d", len(eligible))
	}

	// All completed
	eligible = v.GetEligibleFlows(flows, map[string]bool{"f1": true, "f2": true, "f3": true})
	if len(eligible) != 3 {
		t.Errorf("expected 3 eligible flows when all completed, got %d", len(eligible))
	}
}

func TestFormatError_MissingDeps(t *testing.T) {
	v := NewDependencyValidator()
	err := &types.DependencyError{
		FlowID:      "f1",
		MissingDeps: []string{"missing1", "missing2"},
	}

	msg := v.FormatError(err)
	if msg == "" {
		t.Error("expected formatted error message")
	}
}

func TestFormatError_CycleDeps(t *testing.T) {
	v := NewDependencyValidator()
	err := &types.DependencyError{
		FlowID:    "f1",
		CycleDeps: []string{"a", "b", "c"},
	}

	msg := v.FormatError(err)
	if msg == "" {
		t.Error("expected formatted error message")
	}
}
