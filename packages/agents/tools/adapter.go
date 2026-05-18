package tools

import (
	browsertools "qa-orchestrator/packages/browser-runtime/tools"
	"qa-orchestrator/packages/llm"
)

func RegistryToLLMTools(registry *browsertools.ToolRegistry) []llm.ToolInfo {
	registryTools := registry.ListToolsWithDocs()

	result := make([]llm.ToolInfo, 0, len(registryTools))
	for _, t := range registryTools {
		llmParams := make(map[string]llm.ParameterInfo)
		for name, p := range t.Parameters {
			llmParams[name] = llm.ParameterInfo{
				Type:        p.Type,
				Description: p.Description,
				Required:    p.Required,
			}
		}
		result = append(result, llm.ToolInfo{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  llmParams,
		})
	}
	return result
}