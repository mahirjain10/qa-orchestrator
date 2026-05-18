package campaign

import (
	"os"
	"path/filepath"
	"testing"

	"qa-orchestrator/packages/shared/types"
)

func TestParseFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	campaignFile := filepath.Join(tmpDir, "test.yaml")
	err := os.WriteFile(campaignFile, []byte(`
name: Test Campaign
description: A test campaign
version: "1.0"
flows:
  - id: flow-1
    name: First Flow
    goal: Do something
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
		"flows": [{"id": "jflow-1", "name": "JSON Flow", "goal": "test"}]
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
	campaign := &types.Campaign{Name: "", Flows: []types.Flow{{ID: "f1"}}}
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestValidate_NoFlows(t *testing.T) {
	parser := NewCampaignParser()
	campaign := &types.Campaign{Name: "Test", Flows: []types.Flow{}}
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for no flows")
	}
}

func TestValidate_DuplicateFlowID(t *testing.T) {
	parser := NewCampaignParser()
	campaign := &types.Campaign{
		Name: "Test",
		Flows: []types.Flow{
			{ID: "f1", Name: "Flow 1"},
			{ID: "f1", Name: "Flow 2"},
		},
	}
	err := parser.validate(campaign)
	if err == nil {
		t.Error("expected error for duplicate flow ID")
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
	if camp.Flows[0].ID != "auto-flow-1" {
		t.Errorf("expected auto ID 'auto-flow-1', got %q", camp.Flows[0].ID)
	}
}
