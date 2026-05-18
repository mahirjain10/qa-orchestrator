package campaign

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qa-orchestrator/packages/shared/types"
)

func validCampaign() *types.Campaign {
	return &types.Campaign{
		Name:    "Test",
		Version: "1.0",
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
    steps:
      - id: step1
        tool: navigate
`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewCampaignParser()
	camp, err := parser.ParseFile(campaignFile)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

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
		"flows": [{"id": "jflow-1", "name": "JSON Flow", "goal": "test", "mode": "guided", "priority": "high", "steps": [{"id": "s1", "tool": "click"}]}]
	}`), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	parser := NewCampaignParser()
	camp, err := parser.ParseFile(campaignFile)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

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
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidate_NoFlows(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows = []types.Flow{}
	err := parser.validate(campaign)
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
		Steps:    []types.Step{{ID: "s2", Tool: "type"}},
	})
	err := parser.validate(campaign)
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
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for invalid mode")
	}
}

func TestValidate_ModeRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = ""
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for empty mode")
	}
}

func TestValidate_GuidedModeRequiresSteps(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Steps = []types.Step{}
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for guided mode with empty steps")
	}
}

func TestValidate_AutonomousModeNoSteps(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = types.FlowModeAutonomous
	campaign.Flows[0].Steps = nil
	err := parser.validate(campaign)
	if err != nil {
		t.Errorf("expected no error for autonomous mode without steps: %v", err)
	}
}

func TestValidate_AutonomousModeWithSteps(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Mode = types.FlowModeAutonomous
	campaign.Flows[0].Steps = []types.Step{{ID: "step1", Tool: "click"}}
	err := parser.validate(campaign)
	if err != nil {
		t.Errorf("expected no error for autonomous mode with steps: %v", err)
	}
}

func TestValidate_InvalidPriority(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Priority = "invalid"
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for invalid priority")
	}
}

func TestValidate_PriorityRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Priority = ""
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for empty priority")
	}
}

func TestValidate_GoalRequired(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].Goal = ""
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for empty goal")
	}
}

func TestValidate_InvalidDependency(t *testing.T) {
	parser := NewCampaignParser()
	campaign := validCampaign()
	campaign.Flows[0].DependsOn = []string{"nonexistent"}
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for invalid dependency")
	}
}

func TestValidate_CircularDependency(t *testing.T) {
	parser := NewCampaignParser()
	campaign := &types.Campaign{
		Name:    "Cycle Campaign",
		Version: "1.0",
		Flows: []types.Flow{
			{
				ID:        "flow-a",
				Name:      "Flow A",
				Goal:      "A",
				Mode:      types.FlowModeAutonomous,
				Priority:  types.FlowPriorityMedium,
				DependsOn: []string{"flow-b"},
			},
			{
				ID:        "flow-b",
				Name:      "Flow B",
				Goal:      "B",
				Mode:      types.FlowModeAutonomous,
				Priority:  types.FlowPriorityMedium,
				DependsOn: []string{"flow-a"},
			},
		},
	}

	err := parser.validate(campaign)
	if err == nil {
		t.Fatal("expected error for circular dependency")
	}
	if !strings.Contains(err.Error(), "Circular dependency detected") {
		t.Fatalf("expected circular dependency error, got: %v", err)
	}
}
