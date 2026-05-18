package llm

const (
	SystemPromptTemplate = `You are an autonomous planning agent for a QA orchestration system.

Your role is to generate a sequence of steps to achieve a given goal.

## Available Tools
{{.Tools}}

## Constraints
- Each step must use one of the available tools
- Steps should be specific and executable
- Include necessary parameters for each tool
- Output in JSON format as specified

## Response Format
Respond with a JSON array of steps, each containing:
- "tool": tool name
- "params": object with tool parameters
- "reason": brief explanation of why this step`

	UserPromptTemplate = `## Goal
{{.Goal}}

## History
{{.History}}

## Current Observation
{{.Observation}}

## Task
Generate the next step to progress toward the goal.
Output ONLY a JSON array with one step in this format:
[{"tool": "tool_name", "params": {...}, "reason": "explanation"}]`
)

type ToolInfo struct {
	Name        string
	Description string
	Parameters  map[string]ParameterInfo
}

type ParameterInfo struct {
	Type        string
	Description string
	Required    bool
}

func FormatTools(tools []ToolInfo) string {
	result := ""
	for _, tool := range tools {
		result += "- **" + tool.Name + "**: " + tool.Description + "\n"
		result += "  Parameters:\n"
		for param, info := range tool.Parameters {
			required := ""
			if info.Required {
				required = " (required)"
			}
			result += "    - " + param + ": " + info.Description + required + "\n"
		}
		result += "\n"
	}
	return result
}

type PlannerPromptData struct {
	Goal        string
	History     string
	Observation string
	Tools       []ToolInfo
}

func BuildSystemPrompt(tools []ToolInfo) string {
	prompt := SystemPromptTemplate
	toolsStr := FormatTools(tools)
	prompt = replacePlaceholder(prompt, "Tools", toolsStr)
	return prompt
}

func BuildUserPrompt(data PlannerPromptData) string {
	prompt := UserPromptTemplate
	prompt = replacePlaceholder(prompt, "Goal", data.Goal)
	prompt = replacePlaceholder(prompt, "History", data.History)
	prompt = replacePlaceholder(prompt, "Observation", data.Observation)
	return prompt
}

func replacePlaceholder(template, placeholder, value string) string {
	return replaceAll(template, "{{."+placeholder+"}}", value)
}

func replaceAll(s, old, new string) string {
	result := s
	for {
		if len(result) == 0 {
			break
		}
		i := indexOf(result, old)
		if i < 0 {
			break
		}
		result = result[:i] + new + result[i+len(old):]
	}
	return result
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}