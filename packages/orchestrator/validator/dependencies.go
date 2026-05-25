package validator

import (
	"fmt"
	"strings"

	"qa-orchestrator/packages/shared/types"
)

type DependencyValidator struct{}

func NewDependencyValidator() *DependencyValidator {
	return &DependencyValidator{}
}

type ValidationResult struct {
	Valid            bool
	Error            *types.DependencyError
	TopologicalOrder []string
}

func (v *DependencyValidator) Validate(flows []types.Flow) ValidationResult {
	// Build lookup of all flow IDs
	flowMap := make(map[string]types.Flow)
	for _, flow := range flows {
		flowMap[flow.ID] = flow
	}

	// Check for missing dependencies
	for _, flow := range flows {
		var missing []string
		for _, dep := range flow.DependsOn {
			if _, exists := flowMap[dep]; !exists {
				missing = append(missing, dep)
			}
		}
		if len(missing) > 0 {
			return ValidationResult{
				Valid: false,
				Error: &types.DependencyError{
					FlowID:      flow.ID,
					MissingDeps: missing,
				},
			}
		}
	}

	// Check for cycles using Kahn's algorithm
	visited := make(map[string]int) // 0=unvisited, 1=visiting, 2=done
	cycleFound := false
	var cycleDeps []string

	var dfs func(id string, path []string)
	dfs = func(id string, path []string) {
		if visited[id] == 1 {
			cycleFound = true
			// Extract cycle from path
			startIdx := -1
			for i, p := range path {
				if p == id {
					startIdx = i
					break
				}
			}
			if startIdx >= 0 {
				cycleDeps = path[startIdx:]
			}
			return
		}
		if visited[id] == 2 || cycleFound {
			return
		}

		visited[id] = 1
		flow := flowMap[id]
		for _, dep := range flow.DependsOn {
			dfs(dep, append(path, dep))
		}
		visited[id] = 2
	}

	for _, flow := range flows {
		if visited[flow.ID] == 0 {
			dfs(flow.ID, []string{flow.ID})
			if cycleFound {
				break
			}
		}
	}

	if cycleFound {
		return ValidationResult{
			Valid: false,
			Error: &types.DependencyError{
				FlowID:    cycleDeps[0],
				CycleDeps: cycleDeps,
			},
		}
	}

	// Compute topological order
	order := v.topologicalSort(flows)
	if len(order) < len(flows) {
		// topologicalSort detected a cycle not caught by DFS (defense-in-depth).
		// The partial order contains flows before the cycle; report the rest as cycle deps.
		inOrder := make(map[string]bool, len(order))
		for _, id := range order {
			inOrder[id] = true
		}
		var cycleDeps []string
		for _, flow := range flows {
			if !inOrder[flow.ID] {
				cycleDeps = append(cycleDeps, flow.ID)
			}
		}
		return ValidationResult{
			Valid: false,
			Error: &types.DependencyError{
				FlowID:    "campaign",
				CycleDeps: cycleDeps,
			},
		}
	}

	return ValidationResult{
		Valid:            true,
		TopologicalOrder: order,
	}
}

func (v *DependencyValidator) topologicalSort(flows []types.Flow) []string {
	inDegree := make(map[string]int)
	dependents := make(map[string][]string)

	for _, flow := range flows {
		if _, exists := inDegree[flow.ID]; !exists {
			inDegree[flow.ID] = 0
		}
		for _, dep := range flow.DependsOn {
			inDegree[flow.ID]++
			dependents[dep] = append(dependents[dep], flow.ID)
		}
	}

	// Find eligible flows (no dependencies)
	var queue []string
	for _, flow := range flows {
		if inDegree[flow.ID] == 0 {
			queue = append(queue, flow.ID)
		}
	}

	var order []string
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, dependent := range dependents[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Detect cycles: if not all flows are in order, there's a cycle.
	// Return only the partial order up to the cycle. Callers must check
	// the result length against the input length.
	return order
}

func (v *DependencyValidator) GetEligibleFlows(flows []types.Flow, completedFlowIDs map[string]bool) []types.Flow {
	var eligible []types.Flow
	for _, flow := range flows {
		if len(flow.DependsOn) == 0 {
			eligible = append(eligible, flow)
			continue
		}
		allDepsComplete := true
		for _, dep := range flow.DependsOn {
			if !completedFlowIDs[dep] {
				allDepsComplete = false
				break
			}
		}
		if allDepsComplete {
			eligible = append(eligible, flow)
		}
	}
	return eligible
}

func (v *DependencyValidator) FormatError(err *types.DependencyError) string {
	if err == nil {
		return "unknown dependency error"
	}
	var parts []string
	if len(err.MissingDeps) > 0 {
		parts = append(parts, fmt.Sprintf(
			"Flow %q depends on missing flows: %s",
			err.FlowID,
			strings.Join(err.MissingDeps, ", "),
		))
	}
	if len(err.CycleDeps) > 0 {
		parts = append(parts, fmt.Sprintf(
			"Circular dependency detected in: %s",
			strings.Join(err.CycleDeps, " -> "),
		))
	}
	return strings.Join(parts, "; ")
}
