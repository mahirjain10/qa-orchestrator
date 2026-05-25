package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	campaignparser "qa-orchestrator/packages/orchestrator/campaign"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func TestStartCampaignCreatesSessionBeforeLLMCheck(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	dataDir := makeTestDir(t)
	campaignPath := filepath.Join(dataDir, "autonomous.yaml")
	writeCampaign(t, campaignPath, "autonomous", "")

	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	err := startCampaign(campaignPath, "", "mock", context.Background(), sessionStore, traceStore, artifactStore, make(chan string, 1), nil)
	if err != nil {
		t.Fatalf("startCampaign should not error when LLM is missing, got: %v", err)
	}

	sessions, err := sessionStore.List()
	if err != nil {
		t.Fatalf("listing sessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected one persisted session, got %d", len(sessions))
	}
}

func TestStartCampaignAllowsGuidedWithoutLLMConfig(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	dataDir := makeTestDir(t)
	campaignPath := filepath.Join(dataDir, "guided.yaml")
	writeCampaign(t, campaignPath, "guided", `
    steps:
      - id: step-1
        tool: echo
        params:
          value: ok
`)

	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	if err := startCampaign(campaignPath, "", "mock", context.Background(), sessionStore, traceStore, artifactStore, make(chan string, 1), nil); err != nil {
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

func TestCreateLLMClientForCampaignUsesDefaultModel(t *testing.T) {
	t.Setenv("LLM_API_KEY", "test-key")
	t.Setenv("LLM_MODEL", "")

	dataDir := makeTestDir(t)
	campaignPath := filepath.Join(dataDir, "autonomous.yaml")
	writeCampaign(t, campaignPath, "autonomous", "")

	camp, err := parseCampaign(campaignPath)
	if err != nil {
		t.Fatalf("parseCampaign failed: %v", err)
	}

	client, err := createLLMClientForCampaign(camp)
	if err != nil {
		t.Fatalf("expected default model to be used, got error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client with default model")
	}
}

func parseCampaign(path string) (*sharedtypes.Campaign, error) {
	parser := campaignparser.NewCampaignParser()
	parsed, err := parser.ParseFile(path)
	if err != nil {
		return nil, err
	}
	return parsed.Campaign, nil
}

func makeTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "qa-orchestrator-test-*")
	if err != nil {
		t.Fatalf("creating temp dir: %v", err)
	}
	t.Cleanup(func() {
		for i := 0; i < 5; i++ {
			err := os.RemoveAll(dir)
			if err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	})
	return dir
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

func writeMultiFlowCampaign(t *testing.T, path string) {
	t.Helper()

	content := `name: Test Campaign
version: "1.0"
config:
  timeout: 300s
  retry_limit: 0
  parallel_limit: 1
flows:
  - id: flow-pass
    name: Pass Flow
    goal: Pass
    mode: guided
    priority: high
    depends_on: []
    steps:
      - id: step-1
        tool: echo
        params:
          value: ok
  - id: flow-fail
    name: Fail Flow
    goal: Fail
    mode: guided
    priority: high
    depends_on: []
    steps:
      - id: step-1
        tool: nonexistent_tool
        params:
          value: fail
`

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("writing campaign: %v", err)
	}
}

func TestRunCampaignWithContext_SetsFailedStatusOnFlowFailure(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	dataDir := makeTestDir(t)
	campaignPath := filepath.Join(dataDir, "campaign.yaml")
	writeMultiFlowCampaign(t, campaignPath)

	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	if err := startCampaign(campaignPath, "", "mock", context.Background(), sessionStore, traceStore, artifactStore, make(chan string, 1), nil); err != nil {
		t.Fatalf("startCampaign failed: %v", err)
	}

	var sess *sharedtypes.Session
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		sessions, err := sessionStore.List()
		if err != nil {
			t.Fatalf("listing sessions: %v", err)
		}
		if len(sessions) == 0 {
			continue
		}
		sess = sessions[0]
		if sess.Status != sharedtypes.RunStatePending && sess.Status != sharedtypes.RunStateRunning {
			break
		}
	}

	if sess == nil {
		t.Fatal("expected session to be created")
	}
	if sess.Status != sharedtypes.RunStateFailed {
		t.Fatalf("expected run status FAILED, got %s", sess.Status)
	}

	hasFailed := false
	hasPassed := false
	for _, f := range sess.Flows {
		if f.Status == sharedtypes.FlowStateFailed {
			hasFailed = true
		}
		if f.Status == sharedtypes.FlowStatePassed {
			hasPassed = true
		}
	}
	if !hasFailed {
		t.Fatal("expected at least one flow to be FAILED")
	}
	if !hasPassed {
		t.Fatal("expected at least one flow to be PASSED")
	}
}

func TestRunCampaignWithContext_SetsCompletedStatusOnAllPass(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("LLM_MODEL", "")

	dataDir := makeTestDir(t)
	campaignPath := filepath.Join(dataDir, "campaign.yaml")
	writeCampaign(t, campaignPath, "guided", `
    steps:
      - id: step-1
        tool: echo
        params:
          value: ok
`)

	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	if err := startCampaign(campaignPath, "", "mock", context.Background(), sessionStore, traceStore, artifactStore, make(chan string, 1), nil); err != nil {
		t.Fatalf("startCampaign failed: %v", err)
	}

	var sess *sharedtypes.Session
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		sessions, err := sessionStore.List()
		if err != nil {
			t.Fatalf("listing sessions: %v", err)
		}
		if len(sessions) == 0 {
			continue
		}
		sess = sessions[0]
		if sess.Status != sharedtypes.RunStatePending && sess.Status != sharedtypes.RunStateRunning {
			break
		}
	}

	if sess == nil {
		t.Fatal("expected session to be created")
	}
	if sess.Status != sharedtypes.RunStateCompleted {
		t.Fatalf("expected run status COMPLETED, got %s", sess.Status)
	}

	for _, f := range sess.Flows {
		if f.Status != sharedtypes.FlowStatePassed {
			t.Fatalf("expected flow %s to be PASSED, got %s", f.FlowID, f.Status)
		}
	}
}

func TestCreateAgentEngineMockModeReturnsNilBrowser(t *testing.T) {
	dataDir := makeTestDir(t)
	sessionStore, traceStore, artifactStore := createStores(t, dataDir)

	_, browserRuntime := createAgentEngine(sessionStore, traceStore, artifactStore, nil, "mock", nil)
	if browserRuntime != nil {
		t.Fatal("expected nil browser runtime in mock mode")
	}
}
