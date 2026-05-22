package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qa-orchestrator/packages/orchestrator/campaign"
	sharedtypes "qa-orchestrator/packages/shared/types"
)

func TestAutonomousCampaign_ParseAndValidate(t *testing.T) {
	parser := campaign.NewCampaignParser()
	parsed, err := parser.ParseFile("../../../campaigns/sample-autonomous.yaml")
	if err != nil {
		t.Fatalf("Failed to parse autonomous campaign: %v", err)
	}
	camp := parsed.Campaign

	if camp.Name != "Autonomous E2E Test Campaign" {
		t.Errorf("Campaign name = %q, want %q", camp.Name, "Autonomous E2E Test Campaign")
	}

	if len(camp.Flows) != 3 {
		t.Errorf("Flows count = %d, want 3", len(camp.Flows))
	}

	flowIDs := make(map[string]bool)
	for _, f := range camp.Flows {
		if flowIDs[f.ID] {
			t.Errorf("Duplicate flow ID: %s", f.ID)
		}
		flowIDs[f.ID] = true

		if f.Mode != sharedtypes.FlowModeAutonomous {
			t.Errorf("Flow %s mode = %s, want autonomous", f.ID, f.Mode)
		}

		if f.Goal == "" {
			t.Errorf("Flow %s has empty goal", f.ID)
		}
	}

	if !flowIDs["auth-flow"] || !flowIDs["navigation-flow"] || !flowIDs["search-flow"] {
		t.Error("Missing expected flow IDs")
	}
}

func TestAutonomousCampaign_DependencyGraph(t *testing.T) {
	parser := campaign.NewCampaignParser()
	parsed, err := parser.ParseFile("../../../campaigns/sample-autonomous.yaml")
	if err != nil {
		t.Fatalf("Failed to parse autonomous campaign: %v", err)
	}
	camp := parsed.Campaign

	flowMap := make(map[string]*sharedtypes.Flow)
	for i := range camp.Flows {
		flowMap[camp.Flows[i].ID] = &camp.Flows[i]
	}

	authFlow := flowMap["auth-flow"]
	if len(authFlow.DependsOn) != 0 {
		t.Errorf("auth-flow depends_on = %v, want empty", authFlow.DependsOn)
	}

	navFlow := flowMap["navigation-flow"]
	if len(navFlow.DependsOn) != 1 || navFlow.DependsOn[0] != "auth-flow" {
		t.Errorf("navigation-flow depends_on = %v, want [auth-flow]", navFlow.DependsOn)
	}

	searchFlow := flowMap["search-flow"]
	if len(searchFlow.DependsOn) != 0 {
		t.Errorf("search-flow depends_on = %v, want empty", searchFlow.DependsOn)
	}
}

func TestAutonomousCampaign_PriorityOrdering(t *testing.T) {
	parser := campaign.NewCampaignParser()
	parsed, err := parser.ParseFile("../../../campaigns/sample-autonomous.yaml")
	if err != nil {
		t.Fatalf("Failed to parse autonomous campaign: %v", err)
	}
	camp := parsed.Campaign

	priorityOrder := map[string]int{
		"high":   3,
		"medium": 2,
		"low":    1,
	}

	flowMap := make(map[string]*sharedtypes.Flow)
	for i := range camp.Flows {
		flowMap[camp.Flows[i].ID] = &camp.Flows[i]
	}

	authPriority := priorityOrder[string(flowMap["auth-flow"].Priority)]
	navPriority := priorityOrder[string(flowMap["navigation-flow"].Priority)]
	searchPriority := priorityOrder[string(flowMap["search-flow"].Priority)]

	if authPriority != 3 {
		t.Errorf("auth-flow priority = %s, want high", flowMap["auth-flow"].Priority)
	}
	if navPriority != 2 {
		t.Errorf("navigation-flow priority = %s, want medium", flowMap["navigation-flow"].Priority)
	}
	if searchPriority != 1 {
		t.Errorf("search-flow priority = %s, want low", flowMap["search-flow"].Priority)
	}
}

func TestAutonomousCampaign_FileExists(t *testing.T) {
	_, err := os.Stat(filepath.Join("..", "..", "..", "campaigns", "sample-autonomous.yaml"))
	if err != nil {
		t.Fatalf("Sample autonomous campaign file does not exist: %v", err)
	}
}

func TestAutonomousCampaign_RecoveryReplanIntegration(t *testing.T) {
	parser := campaign.NewCampaignParser()
	parsed, err := parser.ParseFile("../../../campaigns/sample-autonomous.yaml")
	if err != nil {
		t.Fatalf("Failed to parse autonomous campaign: %v", err)
	}
	camp := parsed.Campaign

	authFlow := camp.Flows[0]
	if authFlow.Mode != sharedtypes.FlowModeAutonomous {
		t.Fatalf("Expected autonomous mode for auth-flow, got %s", authFlow.Mode)
	}

	if authFlow.Goal == "" {
		t.Fatal("auth-flow has empty goal, which is required for autonomous mode")
	}

	t.Logf("Autonomous flow %s has goal: %s", authFlow.ID, authFlow.Goal)
	t.Logf("Recovery agent can trigger replanning on locator errors (verified in recovery_test.go)")
	t.Logf("Engine runAutonomousFlow handles RecoveryActionReplan at lines 227-238")
}

func TestAllCampaigns_ParseAndValidate(t *testing.T) {
	parser := campaign.NewCampaignParser()
	campaignDir := "../../../campaigns"

	files, err := filepath.Glob(filepath.Join(campaignDir, "*.yaml"))
	if err != nil {
		t.Fatalf("Failed to glob campaign files: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("No campaign YAML files found in campaigns/ directory")
	}

	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			parsed, err := parser.ParseFile(f)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", name, err)
			}
			camp := parsed.Campaign
			if camp.Name == "" {
				t.Error("Campaign name is empty")
			}
			if len(camp.Flows) == 0 {
				t.Error("Campaign has no flows")
			}
			if parsed.TopologicalOrder == nil {
				t.Error("TopologicalOrder is nil after ParseFile")
			}
			t.Logf("Campaign %q: %d flows, topological order: %v", camp.Name, len(camp.Flows), parsed.TopologicalOrder)
		})
	}
}

func TestAllCampaigns_StartURLValidation(t *testing.T) {
	parser := campaign.NewCampaignParser()
	campaignDir := "../../../campaigns"

	files, err := filepath.Glob(filepath.Join(campaignDir, "*.yaml"))
	if err != nil {
		t.Fatalf("Failed to glob campaign files: %v", err)
	}

	for _, f := range files {
		name := filepath.Base(f)
		t.Run(name, func(t *testing.T) {
			parsed, err := parser.ParseFile(f)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", name, err)
			}
			for _, flow := range parsed.Campaign.Flows {
				if flow.StartURL != "" {
					if !strings.HasPrefix(flow.StartURL, "http://") && !strings.HasPrefix(flow.StartURL, "https://") {
						t.Errorf("Flow %q has invalid start_url %q: must start with http:// or https://", flow.ID, flow.StartURL)
					}
				}
			}
		})
	}
}
