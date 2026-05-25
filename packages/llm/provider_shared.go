package llm

import (
	"context"
	"fmt"
)

func splitSystemMessage(messages []Message) (systemPrompt string, rest []Message) {
	if len(messages) > 0 && messages[0].Role == RoleSystem {
		return messages[0].Content, messages[1:]
	}
	return "", messages
}

func endpoint(baseURL, defaultURL, path string) string {
	if baseURL != "" {
		return baseURL + path
	}
	return defaultURL + path
}

func checkContext(ctx context.Context) error {
	if ctx.Err() != nil {
		return fmt.Errorf("context cancelled before building request: %w", ctx.Err())
	}
	return nil
}

func validateModel(model, providerName string) error {
	if model == "" {
		return fmt.Errorf("model is required for %s provider", providerName)
	}
	return nil
}
