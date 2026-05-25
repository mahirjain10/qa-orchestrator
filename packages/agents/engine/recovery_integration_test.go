package engine

import (
	"context"
	"testing"

	"qa-orchestrator/packages/agents/executor"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

type recoveryMockLLM struct {
	responses []string
	calls     int
}

func (m *recoveryMockLLM) Generate(ctx context.Context, prompt string) (string, error) {
	if m.calls >= len(m.responses) {
		return `[{"tool": "finish", "params": {"status": "fail"}, "reason": "no more mock responses"}]`, nil
	}
	resp := m.responses[m.calls]
	m.calls++
	return resp, nil
}

func (m *recoveryMockLLM) GenerateWithSystem(ctx context.Context, system, user string) (string, error) {
	return m.Generate(ctx, user)
}

func TestRecovery_404InterceptIntegration(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sessStore, _ := session.NewSessionStore(tmpDir)
	traceStore, _ := trace.NewTraceStore(tmpDir)
	
	registry := executor.NewMockToolRegistry()
	
	// Create a stateful mock tool registry to return 404 only on the first call
	observeCalls := 0
	registry.Register("observe_ui", func(params map[string]any) (any, error) {
		observeCalls++
		if observeCalls == 1 {
			return map[string]any{
				"page_state": "loaded",
				"warning":    "404 Not Found",
				"interactive": []any{},
			}, nil
		}
		return map[string]any{
			"page_state":  "loaded",
			"interactive": []any{map[string]any{"tag": "button", "selector": "#login", "text": "Login"}},
		}, nil
	})

	llm := &recoveryMockLLM{
		responses: []string{
			`[{"tool": "navigate", "params": {"url": "http://example.com/bad"}, "reason": "test"}]`,
			`[{"tool": "finish", "params": {"status": "success"}, "reason": "done"}]`,
		},
	}

	eng := NewAgentEngine(
		WithToolRegistry(registry),
		WithSessionStore(sessStore),
		WithTraceStore(traceStore),
		WithLLMClient(llm),
	)

	camp := &sharedtypes.Campaign{Name: "404-test-camp"}
	sess, err := sessStore.Create(camp)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	runID := sess.RunID

	flow := sharedtypes.Flow{
		ID:   "404-test",
		Goal: "Recover from 404",
		Mode: sharedtypes.FlowModeAutonomous,
	}

	// This should trigger handle404Intercept which calls performRootNav
	result := eng.RunFlow(runID, flow)

	t.Logf("Result steps: %d", len(result.Steps))
	for _, step := range result.Steps {
		t.Logf("Step: %s (ID: %s) Success: %v", step.Tool, step.StepID, step.Success)
	}

	// Verify
	foundRootNav := false
	foundRootObserve := false
	for _, step := range result.Steps {
		if step.StepID == "engine-root-nav" {
			foundRootNav = true
		}
		if step.StepID == "engine-root-observe" {
			foundRootObserve = true
		}
	}

	if !foundRootNav {
		t.Error("Engine failed to trigger root navigation recovery after 404 warning")
	}
	if !foundRootObserve {
		t.Error("Engine failed to trigger root observation after recovery navigation")
	}
	
	if result.Outcome != OutcomePass {
		t.Errorf("Expected OutcomePass after recovery, got %v", result.Outcome)
	}
}

func TestRecovery_SelectorHallucinationRepair(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	sessStore, _ := session.NewSessionStore(tmpDir)
	traceStore, _ := trace.NewTraceStore(tmpDir)
	
	registry := executor.NewMockToolRegistry()
	
	// Mock observe_ui to return a valid selector
	registry.Register("observe_ui", func(params map[string]any) (any, error) {
		return map[string]any{
			"page_state": "loaded",
			"interactive": []any{
				map[string]any{"tag": "button", "selector": "#real-id", "text": "Submit Now"},
			},
		}, nil
	})

	llm := &recoveryMockLLM{
		responses: []string{
			// LLM tries a hallucinated selector but uses the correct text in reasoning
			`[{"tool": "click", "params": {"selector": "button:has-text(\"Submit Now\")"}, "reason": "I want to click the Submit Now button"}]`,
			`[{"tool": "finish", "params": {"status": "success"}, "reason": "done"}]`,
		},
	}

	eng := NewAgentEngine(
		WithToolRegistry(registry),
		WithSessionStore(sessStore),
		WithTraceStore(traceStore),
		WithLLMClient(llm),
	)

	camp := &sharedtypes.Campaign{Name: "repair-test-camp"}
	sess, err := sessStore.Create(camp)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	runID := sess.RunID

	// Manually add an observation so the engine has something to validate against
	// In a real run, this would come from a previous observe_ui call.
	// But wait, handleSelectorValidation only works if there's a recent observation.

	flow := sharedtypes.Flow{
		ID:   "repair-test",
		Goal: "Repair hallucinated selector",
		Mode: sharedtypes.FlowModeAutonomous,
	}

	// We need to trigger an initial observation for the repair logic to find the best match
	flow.StartURL = "http://example.com"

	result := eng.RunFlow(runID, flow)

	// Verify
	repaired := false
	for _, step := range result.Steps {
		if step.Tool == "click" {
			params := step.Params
			if sel, ok := params["selector"].(string); ok && sel == "#real-id" {
				repaired = true
			}
		}
	}

	if !repaired {
		t.Error("Engine failed to repair hallucinated selector using fuzzy matching")
	}
}
