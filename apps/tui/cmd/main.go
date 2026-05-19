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

		llmClient, llmErr := createLLMClient()

		hasAutonomous := false
		for _, f := range camp.Flows {
			if f.Mode == sharedtypes.FlowModeAutonomous {
				hasAutonomous = true
				break
			}
		}

		if hasAutonomous && llmErr != nil {
			panic(fmt.Sprintf("Campaign contains autonomous flows but LLM configuration failed: %v", llmErr))
		}

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

func runCampaign(eng *engine.AgentEngine, camp *sharedtypes.Campaign, runID string, sessionStore *session.SessionStore) {
	for _, flow := range camp.Flows {
		result := eng.RunFlow(runID, flow)
		_ = result
	}
}
