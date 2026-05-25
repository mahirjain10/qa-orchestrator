package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qa-orchestrator/packages/shared/types"
)

func validCampaign() *types.Campaign {
	return &types.Campaign{
		Name:    "Test",
		Version: "1.0",
		Config: types.CampaignConfig{
			Timeout:       300 * time.Second,
			RetryLimit:    2,
			ParallelLimit: 1,
		},
		Flows: []types.Flow{
			{
				ID:        "f1",
				Name:      "Flow 1",
				Goal:      "test goal",
				Mode:      types.FlowModeGuided,
				Priority:  types.FlowPriorityHigh,
				Steps:     []types.Step{{ID: "s1", Tool: "click"}},
				DependsOn: []string{},
			},
		},
	}
}

func TestParseFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	campaignFile := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(campaignFile, []byte(`
name: Test Campaign
description: A test campaign
version: "1.0"
config:
  timeout: 300s
  retry_limit: 2
  parallel_limit: 1
flows:
  - id: flow-1
    name: First Flow
    goal: Do something
    mode: guided
    priority: high
    depends_on: []
    steps:
      - id: step1
        tool: navigate
`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewCampaignParser()
	parsed, err := parser.ParseFile(campaignFile)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	camp := parsed.Campaign

	if camp.Name != "Test Campaign" {
		t.Errorf("expected name 'Test Campaign', got %q", camp.Name)
	}
	if len(camp.Flows) != 1 {
		t.Errorf("expected 1 flow, got %d", len(camp.Flows))
	}
	if camp.Flows[0].ID != "flow-1" {
		t.Errorf("expected flow ID 'flow-1', got %q", camp.Flows[0].ID)
	}
}

func TestParseFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	campaignFile := filepath.Join(tmpDir, "test.json")
	err := os.WriteFile(campaignFile, []byte(`{
		"name": "JSON Campaign",
		"version": "1.0",
		"config": {
			"timeout": 300000000000,
			"retry_limit": 2,
			"parallel_limit": 1
		},
		"flows": [{"id": "jflow-1", "name": "JSON Flow", "goal": "test", "mode": "guided", "priority": "high", "depends_on": [], "steps": [{"id": "s1", "tool": "click"}]}]
	}`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewCampaignParser()
	parsed, err := parser.ParseFile(campaignFile)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	camp := parsed.Campaign

	if camp.Name != "JSON Campaign" {
		t.Errorf("expected name 'JSON Campaign', got %q", camp.Name)
	}
}

func TestParseFile_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	campaignFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(campaignFile, []byte("some content"), 0644)

	parser := NewCampaignParser()
	_, err := parser.ParseFile(campaignFile)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestValidate_CampaignNameRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Name = ""
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidate_NoFlows(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows = []types.Flow{}
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for no flows")
	}
}

func TestValidate_DuplicateFlowID(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows = append(campaign.Flows, types.Flow{
		ID:       "f1",
		Name:     "Flow 2",
		Goal:     "another goal",
		Mode:     types.FlowModeGuided,
		Priority: types.FlowPriorityMedium,
		Steps:    []types.Step{{ID: "s2", Tool: "click"}},
	})
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for duplicate flow ID")
	}
	if !strings.Contains(err.Error(), `duplicate flow ID "f1"`) {
		t.Fatalf("expected duplicate flow ID error, got: %v", err)
	}
}

func TestParseNaturalLanguage(t *testing.T) {
	parser := NewCampaignParser()
	text := "Buy item\nPlace order in cart"
	camp, err := parser.ParseNaturalLanguage(text)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	if camp.Name != "Natural Campaign" {
		t.Errorf("expected name 'Natural Campaign', got %q", camp.Name)
	}
	if len(camp.Flows) != 1 {
		t.Errorf("expected 1 flow, got %d", len(camp.Flows))
	}
	if !strings.HasPrefix(camp.Flows[0].ID, "auto-flow-") {
		t.Errorf("expected auto-generated ID with prefix 'auto-flow-', got %q", camp.Flows[0].ID)
	}
	if camp.Version != "1.0" {
		t.Errorf("expected version '1.0', got %q", camp.Version)
	}
	if camp.Flows[0].Mode != types.FlowModeAutonomous {
		t.Errorf("expected autonomous mode, got %q", camp.Flows[0].Mode)
	}
	if camp.Flows[0].Priority != types.FlowPriorityMedium {
		t.Errorf("expected medium priority, got %q", camp.Flows[0].Priority)
	}
}

func TestValidate_InvalidMode(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = "invalid"
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestValidate_ModeRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = ""
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for empty mode")
	}
}

func TestValidate_GuidedModeRequiresSteps(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{}
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for guided mode with empty steps")
	}
}

func TestValidate_AutonomousModeNoSteps(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = types.FlowModeAutonomous
	campaign.Flows[0].Steps = nil
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Errorf("expected no error for autonomous mode without steps: %v", err)
	}
}

func TestValidate_AutonomousModeWithSteps(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = types.FlowModeAutonomous
	campaign.Flows[0].Steps = []types.Step{{ID: "step1", Tool: "click"}}
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Errorf("expected no error for autonomous mode with steps: %v", err)
	}
}

func TestValidate_InvalidPriority(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Priority = "invalid"
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestValidate_PriorityRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Priority = ""
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for empty priority")
	}
}

func TestValidate_GoalRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Goal = ""
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Error("expected error for empty goal")
	}
}

func TestValidate_InvalidDependency(t *testing.T) {
	tmpDir := t.TempDir()
	campaignFile := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(campaignFile, []byte(`
name: Test Campaign
version: "1.0"
config:
  timeout: 300000000000
  retry_limit: 2
  parallel_limit: 1
flows:
  - id: f1
    goal: test
    mode: autonomous
    priority: high
    depends_on: [nonexistent]
`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewCampaignParser()
	_, err = parser.ParseFile(campaignFile)
	if err == nil {
		t.Error("expected error for invalid dependency")
	}
}

func TestValidate_CircularDependency(t *testing.T) {
	tmpDir := t.TempDir()
	campaignFile := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(campaignFile, []byte(`
name: Cycle Campaign
version: "1.0"
config:
  timeout: 300000000000
  retry_limit: 2
  parallel_limit: 1
flows:
  - id: flow-a
    goal: A
    mode: autonomous
    priority: high
    depends_on: [flow-b]
  - id: flow-b
    goal: B
    mode: autonomous
    priority: high
    depends_on: [flow-a]
`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewCampaignParser()
	_, err = parser.ParseFile(campaignFile)
	if err == nil {
		t.Error("expected error for circular dependency")
	}
}

func TestValidate_ConfigZeroTimeout(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Config.Timeout = 0
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for zero timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected timeout error, got: %v", err)
	}
}

func TestValidate_ConfigNegativeRetryLimit(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Config.RetryLimit = -1
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for negative retry_limit")
	}
	if !strings.Contains(err.Error(), "retry_limit") {
		t.Fatalf("expected retry_limit error, got: %v", err)
	}
}

func TestValidate_ConfigZeroParallelLimit(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Config.ParallelLimit = 0
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for zero parallel_limit")
	}
	if !strings.Contains(err.Error(), "parallel_limit") {
		t.Fatalf("expected parallel_limit error, got: %v", err)
	}
}

func TestValidate_FlowConfigNegativeTimeout(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Config.Timeout = -5 * time.Second
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for negative flow timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("expected flow timeout error, got: %v", err)
	}
}

func TestValidate_FlowConfigNegativeRetryLimit(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Config.RetryLimit = -1
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for negative flow retry_limit")
	}
	if !strings.Contains(err.Error(), "retry_limit") {
		t.Fatalf("expected flow retry_limit error, got: %v", err)
	}
}

func TestValidate_FlowConfigValidValues(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Config.Timeout = 60 * time.Second
	campaign.Flows[0].Config.RetryLimit = 3
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Fatalf("expected no error for valid flow config: %v", err)
	}
}

func TestValidate_DuplicateStepID(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{
		{ID: "step1", Tool: "click"},
		{ID: "step1", Tool: "type"},
	}
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for duplicate step ID")
	}
	if !strings.Contains(err.Error(), `duplicate step ID "step1"`) {
		t.Fatalf("expected duplicate step ID error, got: %v", err)
	}
}

func TestValidate_EmptyStepTool(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{
		{ID: "step1", Tool: "click"},
		{ID: "step2", Tool: ""},
	}
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for empty step tool")
	}
	if !strings.Contains(err.Error(), `'tool' is required`) {
		t.Fatalf("expected tool required error, got: %v", err)
	}
}

func TestValidate_StepToolEmptyInAutonomousAllowed(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = types.FlowModeAutonomous
	campaign.Flows[0].Steps = []types.Step{
		{ID: "step1", Tool: ""},
	}
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Fatalf("expected no error for empty tool in autonomous mode: %v", err)
	}
}

func TestValidate_DependsOnRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].DependsOn = nil
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for nil depends_on")
	}
	if !strings.Contains(err.Error(), "depends_on") {
		t.Fatalf("expected depends_on error, got: %v", err)
	}
}

func TestValidate_InvalidStartURL(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].StartURL = "not-a-url"
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for invalid start_url")
	}
	if !strings.Contains(err.Error(), "start_url") {
		t.Fatalf("expected start_url error, got: %v", err)
	}
}

func TestValidate_InvalidStartURLScheme(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].StartURL = "ftp://example.com/page"
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for invalid start_url scheme")
	}
	if !strings.Contains(err.Error(), "start_url") {
		t.Fatalf("expected start_url error, got: %v", err)
	}
}

func TestValidate_ValidStartURL(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].StartURL = "https://example.com/page"
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Fatalf("expected no error for valid start_url: %v", err)
	}
}

func TestValidate_UnknownTool(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{
		{ID: "step1", Tool: "clik"},
	}
	err := parser.validateSchema(campaign)
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
	if !strings.Contains(err.Error(), "unknown tool") {
		t.Fatalf("expected unknown tool error, got: %v", err)
	}
}

func TestValidate_ValidTool(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{
		{ID: "step1", Tool: "navigate"},
	}
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Fatalf("expected no error for valid tool: %v", err)
	}
}

func TestValidate_SelectOptionToolAllowed(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{
		{ID: "step1", Tool: "select_option"},
	}
	err := parser.validateSchema(campaign)
	if err != nil {
		t.Fatalf("expected no error for select_option tool: %v", err)
	}
}
