package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/style"
)

type CommandDef struct {
	Name string
	Desc string
}

var availableCommands = []CommandDef{
	{Name: "retry", Desc: "Retry a specific flow (usage: retry <flow_id>)"},
	{Name: "skip", Desc: "Skip a specific flow (usage: skip <flow_id>)"},
	{Name: "continue", Desc: "Resume run from WAITING_FOR_INPUT state (same as approve)"},
	{Name: "approve", Desc: "Alias for continue"},
	{Name: "status", Desc: "Show current run status"},
	{Name: "pause", Desc: "Pause the current run"},
	{Name: "resume", Desc: "Resume a paused run"},
	{Name: "steer", Desc: "Send instruction to autonomous flow (usage: steer <text>)"},
}

type CommandBarModel struct {
	Input   textinput.Model
	Focused bool
	Width   int
}

func NewCommandBarModel() *CommandBarModel {
	ti := textinput.New()
	ti.Placeholder = "Type command (e.g. retry, skip, pause) or instruction..."
	ti.Prompt = "> "
	ti.CharLimit = 256
	return &CommandBarModel{
		Input: ti,
	}
}

func (m *CommandBarModel) SetWidth(w int) {
	m.Width = w
	m.Input.Width = w - 4 // Account for prompt and padding
}

func (m *CommandBarModel) SuggestionCount() int {
	val := m.Input.Value()
	parts := strings.Fields(val)
	cmdName := ""
	if len(parts) > 0 {
		cmdName = parts[0]
	}
	count := 0
	for _, cmd := range availableCommands {
		if cmdName == "" || strings.HasPrefix(cmd.Name, cmdName) {
			count++
		}
	}
	return count
}

func (m *CommandBarModel) Focus() {
	m.Focused = true
	m.Input.Focus()
}

func (m *CommandBarModel) Blur() {
	m.Focused = false
	m.Input.Blur()
	m.Input.SetValue("")
}

func (m *CommandBarModel) Update(msg tea.Msg) (tea.Cmd, bool) {
	if !m.Focused {
		return nil, false
	}

	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "escape":
			m.Blur()
			return nil, true
		}
	}

	m.Input, cmd = m.Input.Update(msg)
	return cmd, true
}

func (m *CommandBarModel) View() string {
	if !m.Focused {
		return lipgloss.NewStyle().
			Background(style.BgDark).
			Width(m.Width).
			Render(" " + style.Dim.Render(m.Input.Placeholder+"  [: to focus]"))
	}

	val := m.Input.Value()
	parts := strings.Fields(val)
	cmdName := ""
	if len(parts) > 0 {
		cmdName = parts[0]
	}

	var suggestions []string
	for _, cmd := range availableCommands {
		if cmdName == "" || strings.HasPrefix(cmd.Name, cmdName) {
			matchLen := len(cmdName)
			if matchLen > len(cmd.Name) {
				matchLen = len(cmd.Name)
			}

			matchedPart := cmd.Name[:matchLen]
			restPart := cmd.Name[matchLen:]

			styledName := style.CommandSuggestionMatch.Render(matchedPart) + style.CommandSuggestion.Render(restPart)
			suggestions = append(suggestions, fmt.Sprintf("  %s - %s", styledName, style.CommandDesc.Render(cmd.Desc)))
		}
	}

	suggestionsBox := ""
	if len(suggestions) > 0 {
		suggestionsText := lipgloss.JoinVertical(lipgloss.Left, suggestions...)
		suggestionsBox = style.CommandPaletteBorder.Width(m.Width - 2).Render(suggestionsText)
	}

	inputView := lipgloss.NewStyle().
		Foreground(style.Green46).
		Render(m.Input.View())

	inputBar := lipgloss.NewStyle().
		Background(style.BgDark).
		Width(m.Width).
		Render(" " + inputView)

	if suggestionsBox != "" {
		return lipgloss.JoinVertical(lipgloss.Left, suggestionsBox, inputBar)
	}
	return inputBar
}
