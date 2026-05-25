package planner

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"qa-orchestrator/packages/agents/types"
)

func sanitizeDOM(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.ReplaceAll(s, "```", "")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "[", "&#91;")
	s = strings.ReplaceAll(s, "]", "&#93;")
	return s
}

func renderAttr(line *string, m map[string]any, key string, used *map[string]bool) {
	v, ok := m[key]
	if !ok {
		return
	}
	vs := fmt.Sprintf("%v", v)
	if vs == "" || vs == "<nil>" || vs == "false" {
		return
	}
	(*used)[key] = true
	*line += fmt.Sprintf(` %s="%s"`, key, sanitizeDOM(vs))
}

func formatObserveUIObservation(obs types.Observation) string {
	result := "Page observation after last step:\n"
	if obs.LastStep != nil {
		result += fmt.Sprintf("  Last step: %s, Tool: %s, Success: %v\n",
			obs.LastStep.StepID, obs.LastStep.Tool, obs.LastStep.Success)
	}
	if obs.State != nil {
		var parsed map[string]any
		if data, ok := obs.State["data"].(map[string]any); ok {
			parsed = data
		} else if dataStr, ok := obs.State["data"].(string); ok {
			if err := json.Unmarshal([]byte(dataStr), &parsed); err != nil {
				const maxRawDataLen = 2000
				if len(dataStr) > maxRawDataLen {
					dataStr = dataStr[:maxRawDataLen] + "... [truncated]"
				}
				result += fmt.Sprintf("  Raw data: %s\n", sanitizeDOM(dataStr))
			}
		}
		if parsed != nil {
			if warning, ok := parsed["warning"].(string); ok && warning != "" {
				result += fmt.Sprintf("  ⚠ %s\n", sanitizeDOM(warning))
			}
			if pageState, ok := parsed["page_state"].(string); ok {
				result += fmt.Sprintf("  Page state: %s\n", sanitizeDOM(pageState))
			}
			if interactive, ok := parsed["interactive"].([]any); ok {
				totalElems := len(interactive)
				maxElems := 40
				var truncated bool
				if len(interactive) > maxElems {
					interactive = interactive[:maxElems]
					truncated = true
				}
				if truncated {
					result += fmt.Sprintf("  Interactive elements found (%d total, showing %d):\n", totalElems, maxElems)
				} else {
					result += fmt.Sprintf("  Interactive elements found (%d total):\n", totalElems)
				}
				for i, elem := range interactive {
					if elemMap, ok := elem.(map[string]any); ok {
						line := fmt.Sprintf("    %d. <%s>", i+1, sanitizeDOM(fmt.Sprintf("%v", elemMap["tag"])))

						priority := []string{"id", "name", "type", "role", "placeholder", "href",
							"value", "checked", "disabled", "selected", "aria-label"}

						used := map[string]bool{"tag": true, "selector": true}

						for _, k := range priority {
							renderAttr(&line, elemMap, k, &used)
						}

						remaining := make([]string, 0)
						for k := range elemMap {
							if !used[k] {
								remaining = append(remaining, k)
							}
						}
						sort.Strings(remaining)
						for _, k := range remaining {
							renderAttr(&line, elemMap, k, &used)
						}

						renderAttr(&line, elemMap, "selector", &used)

						line += ">"
						if text, ok := elemMap["text"].(string); ok && text != "" {
							line += sanitizeDOM(text)
						}

						if selector, ok := elemMap["selector"].(string); ok && selector != "" {
							line += fmt.Sprintf("  [selector: %s]", sanitizeDOM(selector))
						}
						result += line + "\n"
					}
				}
				if truncated {
					result += fmt.Sprintf("    ... and %d more elements\n", totalElems-maxElems)
				}
			}
		}
	}
	result += "Use the selectors above when generating your next step. Do not invent selectors."
	return result
}
