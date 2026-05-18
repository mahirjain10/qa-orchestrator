package state

import (
	"testing"

	"qa-orchestrator/packages/storage/session"
)

func TestAppStateCreation(t *testing.T) {
	store, err := session.NewSessionStore("./test_data")
	if err != nil {
		t.Fatalf("Failed to create session store: %v", err)
	}
	defer session.NewSessionStore("./test_data") // cleanup

	appState := NewAppState(store)

	if appState.SessionStore != store {
		t.Error("SessionStore not set correctly")
	}

	if appState.GetView() != ViewCampaignList {
		t.Error("Default view should be ViewCampaignList")
	}

	if appState.GetCurrentRunID() != "" {
		t.Error("Default run ID should be empty")
	}

	if appState.GetSelectedIdx() != 0 {
		t.Error("Default selected index should be 0")
	}
}

func TestAppStateNavigation(t *testing.T) {
	store, _ := session.NewSessionStore("./test_data_state")
	defer session.NewSessionStore("./test_data_state")

	appState := NewAppState(store)

	appState.SetView(ViewActiveRun)
	if appState.GetView() != ViewActiveRun {
		t.Error("View not set correctly")
	}

	appState.SetCurrentRunID("run_123")
	if appState.GetCurrentRunID() != "run_123" {
		t.Error("CurrentRunID not set correctly")
	}

	appState.SetSelectedIdx(5)
	if appState.GetSelectedIdx() != 5 {
		t.Error("SelectedIdx not set correctly")
	}
}

func TestAppStateIncrementDecrement(t *testing.T) {
	store, _ := session.NewSessionStore("./test_data_nav")
	appState := NewAppState(store)

	appState.SetSelectedIdx(0)
	appState.DecrementSelected()
	if appState.GetSelectedIdx() != 0 {
		t.Error("Decrement should not go below 0")
	}

	appState.SetSelectedIdx(5)
	appState.IncrementSelected()
	if appState.GetSelectedIdx() != 6 {
		t.Error("Increment not working")
	}

	appState.DecrementSelected()
	if appState.GetSelectedIdx() != 5 {
		t.Error("Decrement not working")
	}
}

func TestViewConstants(t *testing.T) {
	if ViewCampaignList != "campaign_list" {
		t.Error("ViewCampaignList value incorrect")
	}
	if ViewActiveRun != "active_run" {
		t.Error("ViewActiveRun value incorrect")
	}
	if ViewFlowStatus != "flow_status" {
		t.Error("ViewFlowStatus value incorrect")
	}
}
