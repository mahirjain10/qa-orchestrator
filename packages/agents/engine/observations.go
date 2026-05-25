package engine

import (
	"fmt"
	"regexp"

	agentstypes "qa-orchestrator/packages/agents/types"
	"qa-orchestrator/packages/shared"
)

func buildObservationSummary(ctx *agentstypes.ExecutionContext) string {
	if len(ctx.Observations) == 0 {
		return "no observations yet"
	}
	lastObs := ctx.Observations[len(ctx.Observations)-1]
	if lastObs.LastStep == nil {
		return "observation with no last step"
	}
	summary := fmt.Sprintf("last_obs_tool=%s success=%v", lastObs.LastStep.Tool, lastObs.LastStep.Success)
	if lastObs.LastStep.Tool == "observe_ui" {
		if data, ok := lastObs.State["data"].(map[string]any); ok {
			if pageState, ok := data["page_state"].(string); ok {
				summary += fmt.Sprintf(" page_state=%s", pageState)
			}
			if interactive, ok := data["interactive"].([]any); ok {
				summary += fmt.Sprintf(" elements=%d", len(interactive))
			}
			if warning, ok := data["warning"].(string); ok && warning != "" {
				summary += fmt.Sprintf(" warning=%s", warning)
			}
		}
	}
	if lastObs.Error != nil {
		summary += fmt.Sprintf(" error=%v", lastObs.Error)
	}
	return summary
}

// observedSelectors extracts valid selectors from the most recent observe_ui
// observation. Returns nil if no observe_ui data is available.
func observedSelectors(observations []agentstypes.Observation) []string {
	for i := len(observations) - 1; i >= 0; i-- {
		obs := observations[i]
		if obs.LastStep == nil || obs.LastStep.Tool != "observe_ui" {
			continue
		}
		data, ok := obs.State["data"].(map[string]any)
		if !ok {
			continue
		}
		interactive, ok := data["interactive"].([]any)
		if !ok {
			continue
		}
		result := make([]string, 0, len(interactive))
		for _, elem := range interactive {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			sel, ok := elemMap["selector"].(string)
			if !ok || sel == "" {
				continue
			}
			result = append(result, sel)
		}
		return result
	}
	return nil
}

var safeGenericSelectors = map[string]bool{
	"body":     true,
	"html":     true,
	"*":        true,
	"document": true,
	":root":    true,
}

func isSafeGenericSelector(selector string) bool {
	return safeGenericSelectors[selector]
}

func containsSelector(list []string, target string) bool {
	return shared.Contains(list, target)
}

var hasTextRE = regexp.MustCompile(`:has-text\("([^"]*)"\)`)

// extractTextFromSelector parses `tag:has-text("Some text")` and returns the text.
// Returns "" if the selector does not use has-text.
func extractTextFromSelector(selector string) string {
	match := hasTextRE.FindStringSubmatch(selector)
	if len(match) >= 2 {
		return match[1]
	}
	return ""
}

// observedElements returns the full interactive element data from the most recent
// observe_ui observation. Each element is a map with keys: tag, text, id, name,
// placeholder, selector. Returns nil if no observe_ui data is available.
func observedElements(observations []agentstypes.Observation) []map[string]any {
	for i := len(observations) - 1; i >= 0; i-- {
		obs := observations[i]
		if obs.LastStep == nil || obs.LastStep.Tool != "observe_ui" {
			continue
		}
		data, ok := obs.State["data"].(map[string]any)
		if !ok {
			continue
		}
		interactive, ok := data["interactive"].([]any)
		if !ok {
			continue
		}
		result := make([]map[string]any, 0, len(interactive))
		for _, elem := range interactive {
			elemMap, ok := elem.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, elemMap)
		}
		return result
	}
	return nil
}
