package main

import (
	"fmt"
	"os"

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
		if err := startCampaign(campaignPath, sessionStore, traceStore, artifactStore); err != nil {
			panic(err)
		}
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

func startCampaign(campaignPath string, sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) error {
	parser := campaign.NewCampaignParser()
	camp, err := parser.ParseFile(campaignPath)
	if err != nil {
		return fmt.Errorf("parsing campaign file: %w", err)
	}

	llmClient, err := createLLMClientForCampaign(camp)
	if err != nil {
		return err
	}

	sess, err := sessionStore.Create(camp)
	if err != nil {
		return fmt.Errorf("creating session: %w", err)
	}

	agentEngine := createAgentEngine(sessionStore, traceStore, artifactStore, llmClient)
	go runCampaign(agentEngine, camp, sess.RunID, sessionStore)

	return nil
}

func createLLMClientForCampaign(camp *sharedtypes.Campaign) (*llm.HTTPClient, error) {
	if !hasAutonomousFlow(camp) {
		return nil, nil
	}

	llmClient, err := createLLMClient()
	if err != nil {
		return nil, fmt.Errorf("Campaign contains autonomous flows but LLM configuration failed: %w", err)
	}

	return llmClient, nil
}

func createLLMClient() (*llm.HTTPClient, error) {
	cfg, err := llm.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("loading LLM config: %w", err)
	}

	client, err := llm.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("creating LLM client: %w", err)
	}

	return client, nil
}

func createAgentEngine(sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, llmClient *llm.HTTPClient) *engine.AgentEngine {
	var agentEngine *engine.AgentEngine

	if llmClient != nil {
		cliWrapper := llm.NewSimpleClientWithClient(llmClient)
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

	return agentEngine
}

func hasAutonomousFlow(camp *sharedtypes.Campaign) bool {
	for _, flow := range camp.Flows {
		if flow.Mode == sharedtypes.FlowModeAutonomous {
			return true
		}
	}
	return false
}

func runCampaign(eng *engine.AgentEngine, camp *sharedtypes.Campaign, runID string, sessionStore *session.SessionStore) {
	for _, flow := range camp.Flows {
		result := eng.RunFlow(runID, flow)
		_ = result
	}
}
