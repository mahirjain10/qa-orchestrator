package components

import (
	"testing"

	"qa-orchestrator/packages/shared/types"
)

func TestCampaignListModelCreation(t *testing.T) {
	model := NewCampaignListModel()

	if model.campaigns == nil {
		t.Error("Campaigns should be initialized as empty slice")
	}

	if model.selected != 0 {
		t.Error("Selected should default to 0")
	}
}

func TestCampaignListModelSetCampaigns(t *testing.T) {
	model := NewCampaignListModel()
	campaigns := []string{"Campaign A", "Campaign B", "Campaign C"}
	model.SetCampaigns(campaigns)

	if len(model.campaigns) != 3 {
		t.Error("SetCampaigns not working correctly")
	}

	if model.campaigns[0] != "Campaign A" {
		t.Error("Campaign names not set correctly")
	}
}

func TestCampaignListModelNavigation(t *testing.T) {
	model := NewCampaignListModel()
	model.SetCampaigns([]string{"A", "B", "C"})

	model.Next()
	if model.GetSelected() != 1 {
		t.Error("Next navigation not working")
	}

	model.Next()
	if model.GetSelected() != 2 {
		t.Error("Next navigation at limit not working")
	}

	model.Next() // Should stay at 2
	if model.GetSelected() != 2 {
		t.Error("Should not go beyond last item")
	}

	model.Prev()
	if model.GetSelected() != 1 {
		t.Error("Prev navigation not working")
	}

	model.Prev()
	if model.GetSelected() != 0 {
		t.Error("Prev navigation at start not working")
	}

	model.Prev() // Should stay at 0
	if model.GetSelected() != 0 {
		t.Error("Should not go below 0")
	}
}

func TestCampaignListModelSetSelected(t *testing.T) {
	model := NewCampaignListModel()
	model.SetCampaigns([]string{"A", "B", "C"})

	model.SetSelected(1)
	if model.GetSelected() != 1 {
		t.Error("SetSelected not working")
	}

	model.SetSelected(5) // Should be ignored
	if model.GetSelected() != 1 {
		t.Error("SetSelected should ignore out of bounds")
	}

	model.SetSelected(-1) // Should be ignored
	if model.GetSelected() != 1 {
		t.Error("SetSelected should ignore negative values")
	}
}

func TestRunPanelModelCreation(t *testing.T) {
	model := NewRunPanelModel()

	if model.session != nil {
		t.Error("Session should be nil initially")
	}

	if model.width != 60 {
		t.Error("Default width should be 60")
	}
}

func TestRunPanelModelSetSession(t *testing.T) {
	model := NewRunPanelModel()
	session := &types.Session{
		RunID:        "run_test_123",
		CampaignName: "Test Campaign",
		Status:       types.RunStateRunning,
	}

	model.SetSession(session)

	if model.session == nil {
		t.Error("Session should be set")
	}

	if model.session.RunID != "run_test_123" {
		t.Error("Session not set correctly")
	}
}

func TestFlowStatusModelCreation(t *testing.T) {
	model := NewFlowStatusModel()

	if len(model.flows) != 0 {
		t.Error("Flows should be empty initially")
	}

	if model.selected != 0 {
		t.Error("Selected should default to 0")
	}
}

func TestFlowStatusModelSetFlows(t *testing.T) {
	model := NewFlowStatusModel()
	flows := []types.FlowRunState{
		{FlowID: "flow1", Status: types.FlowStatePending},
		{FlowID: "flow2", Status: types.FlowStateRunning},
	}

	model.SetFlows(flows)

	if len(model.flows) != 2 {
		t.Error("Flows not set correctly")
	}
}

func TestFlowStatusModelNavigation(t *testing.T) {
	model := NewFlowStatusModel()
	flows := []types.FlowRunState{
		{FlowID: "flow1"},
		{FlowID: "flow2"},
		{FlowID: "flow3"},
	}
	model.SetFlows(flows)

	model.Next()
	if model.GetSelected() != 1 {
		t.Error("Next not working")
	}

	model.Prev()
	if model.GetSelected() != 0 {
		t.Error("Prev not working")
	}

	model.SetSelected(1)
	model.Prev()
	if model.GetSelected() != 0 {
		t.Error("Prev at boundary not working")
	}

	model.SetSelected(1)
	model.Next()
	if model.GetSelected() != 2 {
		t.Error("Next at boundary not working")
	}
}

func TestFlowStatusModelBounds(t *testing.T) {
	model := NewFlowStatusModel()
	model.SetFlows([]types.FlowRunState{{FlowID: "only_one"}})

	model.Next() // Should stay at 0
	if model.GetSelected() != 0 {
		t.Error("Should not exceed bounds")
	}

	model.Prev() // Should stay at 0
	if model.GetSelected() != 0 {
		t.Error("Should not go below 0")
	}
}
