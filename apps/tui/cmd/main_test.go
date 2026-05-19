package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	campaignparser "qa-orchestrator/packages/orchestrator/campaign"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func TestStartCampaignFailsBeforeSessionCreateWhenAutonomousLLMConfigMissing(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	dataDir := t.TempDir()
	campaignPath := filepath.Join(dataDir, "autonomous.yaml")
	writeCampaign(t, campaignPath, "autonomous", "")

	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	err := startCampaign(campaignPath, sessionStore, traceStore, artifactStore)
	if err == nil {
		t.Fatal("expected missing LLM config error")
	}
	if !strings.Contains(err.Error(), "Campaign contains autonomous flows but LLM configuration failed") {
		t.Fatalf("unexpected error: %v", err)
	}

	sessions, err := sessionStore.List()
	if err != nil {
		t.Fatalf("listing sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Fatalf("expected no persisted sessions, got %d", len(sessions))
	}
}

func TestStartCampaignAllowsGuidedWithoutLLMConfig(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	dataDir := t.TempDir()
	campaignPath := filepath.Join(dataDir, "guided.yaml")
	writeCampaign(t, campaignPath, "guided", `
    steps:
      - id: step-1
        tool: echo
        params:
          value: ok
`)

	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	if err := startCampaign(campaignPath, sessionStore, traceStore, artifactStore); err != nil {
		t.Fatalf("startCampaign failed: %v", err)
	}

	sessions, err := sessionStore.List()
	if err != nil {
		t.Fatalf("listing sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one persisted session, got %d", len(sessions))
	}
}

func TestCreateLLMClientForCampaignRequiresModelForAutonomous(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "")

	dataDir := t.TempDir()
	campaignPath := filepath.Join(dataDir, "autonomous.yaml")
	writeCampaign(t, campaignPath, "autonomous", "")

	camp, err := parseCampaign(campaignPath)
	if err != nil {
		t.Fatalf("parseCampaign failed: %v", err)
	}

	_, err = createLLMClientForCampaign(camp)
	if err == nil {
		t.Fatal("expected missing model error")
	}
	if !strings.Contains(err.Error(), "LLM_MODEL environment variable is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func parseCampaign(path string) (*sharedtypes.Campaign, error) {
	parser := campaignparser.NewCampaignParser()
	return parser.ParseFile(path)
}

func createStores(t *testing.T, dataDir string) (*session.SessionStore, *trace.TraceStore, *artifact.ArtifactStore) {
	t.Helper()

	sessionStore, err := session.NewSessionStore(dataDir)
	if err != nil {
		t.Fatalf("creating session store: %v", err)
	}
	traceStore, err := trace.NewTraceStore(dataDir)
	if err != nil {
		t.Fatalf("creating trace store: %v", err)
	}
	artifactStore, err := artifact.NewArtifactStore(dataDir)
	if err != nil {
		t.Fatalf("creating artifact store: %v", err)
	}

	return sessionStore, traceStore, artifactStore
}

func writeCampaign(t *testing.T, path, mode, extra string) {
	t.Helper()

	content := `name: Test Campaign
version: "1.0"
config:
  timeout: 300s
  retry_limit: 1
  parallel_limit: 1
flows:
  - id: flow-1
    name: Flow 1
    goal: Exercise flow
    mode: ` + mode + `
    priority: high
    depends_on: []
` + extra

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing campaign: %v", err)
	}
}
