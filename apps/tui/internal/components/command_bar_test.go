package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestCommandBarInitialization(t *testing.T) {
	cb := NewCommandBarModel()
	if cb.Focused {
		t.Errorf("Expected command bar to be unfocused initially")
	}
	if cb.Input.Placeholder == "" {
		t.Errorf("Expected placeholder to be set")
	}
}

func TestCommandBarFocusBlur(t *testing.T) {
	cb := NewCommandBarModel()
	cb.Focus()
	if !cb.Focused {
		t.Errorf("Expected command bar to be focused")
	}
	if !cb.Input.Focused() {
		t.Errorf("Expected internal input to be focused")
	}

	// Should clear input value on blur
	cb.Input.SetValue("retry flow-1")
	cb.Blur()
	if cb.Focused {
		t.Errorf("Expected command bar to be unfocused")
	}
	if cb.Input.Focused() {
		t.Errorf("Expected internal input to be unfocused")
	}
	if cb.Input.Value() != "" {
		t.Errorf("Expected input to be cleared on blur, got %q", cb.Input.Value())
	}
}

func TestCommandBarUpdateWhenUnfocused(t *testing.T) {
	cb := NewCommandBarModel()
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	_, handled := cb.Update(msg)
	if handled {
		t.Errorf("Expected unhandled message when unfocused")
	}
	if cb.Input.Value() == "a" {
		t.Errorf("Expected input to not update when unfocused")
	}
}

func TestCommandBarUpdateWhenFocused(t *testing.T) {
	cb := NewCommandBarModel()
	cb.Focus()

	// Type "r"
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	_, handled := cb.Update(msg)
	if !handled {
		t.Errorf("Expected handled message when focused")
	}
	if cb.Input.Value() != "r" {
		t.Errorf("Expected input to update, got %q", cb.Input.Value())
	}

	// Press ESC
	escMsg := tea.KeyMsg{Type: tea.KeyEscape}
	_, handled = cb.Update(escMsg)
	if !handled {
		t.Errorf("Expected handled esc message")
	}
	if cb.Focused {
		t.Errorf("Expected command bar to blur on ESC")
	}
}

func TestCommandBarSuggestions(t *testing.T) {
	cb := NewCommandBarModel()
	cb.Focus()
	cb.SetWidth(80)

	// Empty input shows all suggestions
	view := cb.View()
	for _, cmd := range availableCommands {
		if !strings.Contains(view, cmd.Desc) {
			t.Errorf("Expected suggestion view to contain desc for %s", cmd.Name)
		}
	}

	// Filter by prefix "r"
	cb.Input.SetValue("r")
	view = cb.View()

	if !strings.Contains(view, "Retry a specific flow") {
		t.Errorf("Expected 'retry' suggestion")
	}
	if !strings.Contains(view, "Resume a paused run") {
		t.Errorf("Expected 'resume' suggestion")
	}
	if strings.Contains(view, "Pause the current run") {
		t.Errorf("Did not expect 'pause' suggestion")
	}

	// Exact match "skip"
	cb.Input.SetValue("skip ")
	view = cb.View()
	if !strings.Contains(view, "Skip a specific flow") {
		t.Errorf("Expected 'skip' suggestion when full word typed")
	}
	if strings.Contains(view, "Pause the current run") {
		t.Errorf("Did not expect 'pause' suggestion")
	}
}

func TestCommandBarSuggestionsPauseResume(t *testing.T) {
	cb := NewCommandBarModel()
	cb.Focus()
	cb.SetWidth(80)

	// Filter by prefix "p" should show pause
	cb.Input.SetValue("p")
	view := cb.View()

	if !strings.Contains(view, "Pause the current run") {
		t.Errorf("Expected 'pause' suggestion when typing 'p'")
	}

	// Filter by prefix "re" should show resume
	cb.Input.SetValue("re")
	view = cb.View()

	if !strings.Contains(view, "Resume a paused run") {
		t.Errorf("Expected 'resume' suggestion when typing 're'")
	}

	// Exact "pause" should show only pause
	cb.Input.SetValue("pause ")
	view = cb.View()

	if !strings.Contains(view, "Pause the current run") {
		t.Errorf("Expected 'pause' suggestion for exact match")
	}
	if strings.Contains(view, "Resume a paused run") {
		t.Errorf("Did not expect 'resume' suggestion for 'pause' input")
	}
}
