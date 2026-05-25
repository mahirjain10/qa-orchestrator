package screens

import (
	"testing"

	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func newScreenWithRun(t *testing.T) (*MainScreen, string) {
	t.Helper()

	baseDir := t.TempDir()
	sessionStore, err := session.NewSessionStore(baseDir)
	if err != nil {
		t.Fatalf("new session store: %v", err)
	}
	traceStore, err := trace.NewTraceStore(baseDir)
	if err != nil {
		t.Fatalf("new trace store: %v", err)
	}
	artifactStore, err := artifact.NewArtifactStore(baseDir)
	if err != nil {
		t.Fatalf("new artifact store: %v", err)
	}

	campaign := &types.Campaign{
		Name: "test-campaign",
		Flows: []types.Flow{
			{
				ID:       "flow-1",
				Name:     "Flow 1",
				Goal:     "goal",
				Mode:     types.FlowModeGuided,
				Priority: types.FlowPriorityHigh,
			},
		},
	}

	sess, err := sessionStore.Create(campaign)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	screen := NewMainScreenWithStores(sessionStore, traceStore, artifactStore)
	return screen, sess.RunID
}
