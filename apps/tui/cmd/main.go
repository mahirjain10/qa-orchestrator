package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbletea"
	"qa-orchestrator/apps/tui/internal/screens"
	"qa-orchestrator/packages/agents/engine"
	"qa-orchestrator/packages/agents/executor"
	"qa-orchestrator/packages/llm"
	"qa-orchestrator/packages/orchestrator/campaign"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

type CLIClient struct {
	client *llm.HTTPClient
}

func (c *CLIClient) Generate(ctx context.Context, prompt string) (string, error) {
	messages := []llm.Message{
		{Role: "user", Content: prompt},
	}
	resp, err := c.client.GenerateWithMessages(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (c *CLIClient) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: user},
	}
	resp, err := c.client.GenerateWithMessages(ctx, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func main() {
	dataDir := "./data"

	sessionStore, err := session.NewSessionStore(dataDir)
	if err != nil {
		panic(fmt.Sprintf("creating session store: %v", err))
	}

	traceStore, err := trace.NewTraceStore(dataDir)
	if err != nil {
		panic(fmt.Sprintf("creating trace store: %v", err))
	}

	artifactStore, err := artifact.NewArtifactStore(dataDir)
	if err != nil {
		panic(fmt.Sprintf("creating artifact store: %v", err))
	}

	campaignPath := ""
	if len(os.Args) > 1 {
		campaignPath = os.Args[1]
	}

	if campaignPath != "" {
		parser := campaign.NewCampaignParser()
		camp, err := parser.ParseFile(campaignPath)
		if err != nil {
			panic(fmt.Sprintf("parsing campaign file: %v", err))
		}

		sess, err := sessionStore.Create(camp)
		if err != nil {
			panic(fmt.Sprintf("creating session: %v", err))
		}

		runID := sess.RunID

		llmClient := createLLMClient()
		var agentEngine *engine.AgentEngine

		if llmClient != nil {
			cliWrapper := &CLIClient{client: llmClient}
			agentEngine = engine.NewAgentEngineWithLLM(
				executor.NewMockToolRegistry(),
				sessionStore,
				cliWrapper,
				nil,
			)
		} else {
			agentEngine = engine.NewAgentEngineWithStores(
				executor.NewMockToolRegistry(),
				sessionStore,
				traceStore,
				artifactStore,
			)
		}

		agentEngine.SetTraceStore(traceStore)
		agentEngine.SetArtifactStore(artifactStore)

		go runCampaign(agentEngine, camp, runID, sessionStore)
	}

	mainScreen := screens.NewMainScreenWithStores(sessionStore, traceStore, artifactStore)
	if campaignPath != "" {
		mainScreen.SetMessage(fmt.Sprintf("Started campaign (run_id initialized)"))
	}

	if _, err := tea.NewProgram(mainScreen).Run(); err != nil {
		os.Stderr.WriteString("Error running TUI: " + err.Error() + "\n")
		os.Exit(1)
	}
}

func createLLMClient() *llm.HTTPClient {
	apiKey := os.Getenv("LLM_API_KEY")
	model := os.Getenv("LLM_MODEL")
	baseURL := os.Getenv("LLM_BASE_URL")

	if apiKey == "" {
		return nil
	}

	cfg := &llm.Config{
		APIKey:     apiKey,
		Model:      model,
		BaseURL:    baseURL,
		MaxRetries: 3,
		Timeout:    120 * time.Second,
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to create LLM client: %v\n", err)
		return nil
	}

	return client
}

func runCampaign(eng *engine.AgentEngine, camp *sharedtypes.Campaign, runID string, sessionStore *session.SessionStore) {
	for _, flow := range camp.Flows {
		result := eng.RunFlow(runID, flow)
		_ = result
	}
}
