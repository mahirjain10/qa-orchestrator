package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"
	"qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
)

type TraceFilter struct {
	Text       string
	ShowFailed bool
	FlowID     string
	EventType  string
}

type TracePanelModel struct {
	events      []*types.TraceEvent
	Selected    int
	Viewport    viewport.Model
	FollowTail  bool
	Filter      TraceFilter
	FilterMode  bool
	FilterInput textinput.Model
}

func NewTracePanelModel() *TracePanelModel {
	vp := viewport.New(80, 20)

	ti := textinput.New()
	ti.Placeholder = "Filter traces..."
	ti.Prompt = "/"
	ti.CharLimit = 64
	ti.Width = 30

	return &TracePanelModel{
		events:      []*types.TraceEvent{},
		Selected:    0,
		Viewport:    vp,
		FollowTail:  true,
		FilterInput: ti,
	}
}

func (m *TracePanelModel) SetEvents(events []*types.TraceEvent) {
	m.events = events
	if m.Selected >= len(m.events) {
		m.Selected = max(0, len(m.events)-1)
	}
	m.UpdateViewportContent()
	if m.FollowTail {
		m.Viewport.GotoBottom()
	}
}

func (m *TracePanelModel) FilteredEvents() []*types.TraceEvent {
	if m.Filter.Text == "" && !m.Filter.ShowFailed && m.Filter.FlowID == "" && m.Filter.EventType == "" {
		return m.events
	}

	var filtered []*types.TraceEvent
	for _, e := range m.events {
		if m.Filter.ShowFailed && e.Status != types.TraceStatusFailed {
			continue
		}
		if m.Filter.FlowID != "" && e.FlowID != m.Filter.FlowID {
			continue
		}
		if m.Filter.EventType != "" && string(e.EventType) != m.Filter.EventType {
			continue
		}
		if m.Filter.Text != "" && !strings.Contains(strings.ToLower(e.Action), strings.ToLower(m.Filter.Text)) {
			continue
		}
		filtered = append(filtered, e)
	}
	return filtered
}

func (m *TracePanelModel) AppendEvent(event *types.TraceEvent) {
	m.events = append(m.events, event)
	m.UpdateViewportContent()
	if m.FollowTail {
		m.Viewport.GotoBottom()
	}
}

func (m *TracePanelModel) Next() {
	events := m.FilteredEvents()
	if m.Selected < len(events)-1 {
		m.Selected++
	}
}

func (m *TracePanelModel) Prev() {
	if m.Selected > 0 {
		m.Selected--
	}
}

func (m *TracePanelModel) GetSelected() int {
	return m.Selected
}

func (m *TracePanelModel) SetSize(width, height int) {
	m.Viewport.Width = width
	m.Viewport.Height = height
	m.UpdateViewportContent()
}

func (m *TracePanelModel) UpdateViewportContent() {
	var lines []string

	events := m.FilteredEvents()

	lines = append(lines, style.Section.Render("  TIME     S  TYPE              ACTION"))
	lines = append(lines, style.Dim.Render("  "+strings.Repeat("─", 62)))

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		timeStr := e.Timestamp.Format("15:04:05")
		statusChar := style.TraceStatusChar(string(e.Status))
		statusSt := style.TraceStatusStyle(string(e.Status))
		typeStr := util.Truncate(string(e.EventType), 18)
		actionStr := util.Truncate(e.Action, 40)

		displayIdx := len(events) - 1 - i
		cursor := "  "
		if displayIdx == m.Selected {
			cursor = style.SelectedBold.Render(" ▶ ")
		}

		row := fmt.Sprintf("%s%s  %s  %-18s  %s",
			cursor,
			style.Dim.Render(timeStr),
			statusSt.Render(statusChar),
			typeStr,
			actionStr,
		)
		lines = append(lines, row)
	}

	m.Viewport.SetContent(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (m *TracePanelModel) Update(msg tea.Msg) tea.Cmd {
	if m.FilterMode {
		var cmd tea.Cmd
		m.FilterInput, cmd = m.FilterInput.Update(msg)
		return cmd
	}
	var cmd tea.Cmd
	m.Viewport, cmd = m.Viewport.Update(msg)
	return cmd
}

func (m *TracePanelModel) View() string {
	events := m.FilteredEvents()
	if len(events) == 0 {
		return style.Header.Render("Trace Events") + "\n\n  No trace events\n"
	}

	lines := []string{}
	lines = append(lines, style.Header.Render("Trace Events"))
	lines = append(lines, "")
	lines = append(lines, style.Section.Render("  Time      Agent      Action          Status    Step"))
	lines = append(lines, style.Dim.Render("  "+strings.Repeat("─", 70)))

	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		timeStr := e.Timestamp.Format("15:04:05")

		statusStr := string(e.Status)
		statusStyle := style.TraceStatusStyle(string(e.Status))

		agentStr := e.Agent
		if len(agentStr) > 10 {
			agentStr = agentStr[:10]
		}

		actionStr := e.Action
		if len(actionStr) > 14 {
			actionStr = actionStr[:14]
		}

		stepID := ""
		if step, ok := e.Details["step_id"].(string); ok {
			stepID = step
		}
		if len(stepID) > 6 {
			stepID = stepID[:6]
		}

		row := fmt.Sprintf("  %s  %-10s %-14s %s  %s",
			style.Dim.Render(timeStr),
			agentStr,
			actionStr,
			statusStyle.Render(statusStr),
			stepID,
		)

		displayIdx := len(events) - 1 - i
		if displayIdx == m.Selected {
			lines = append(lines, style.Selected.Render(row))
		} else {
			lines = append(lines, style.Normal.Render(row))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *TracePanelModel) ViewCompact() string {
	title := style.ViewTitle.Render(" Live Traces ")

	events := m.FilteredEvents()
	if len(events) == 0 {
		return title + "\n  No trace events\n"
	}

	var lines []string

	recentEvents := events
	if len(events) > 8 {
		recentEvents = events[len(events)-8:]
	}

	for _, e := range recentEvents {
		timeStr := e.Timestamp.Format("15:04")

		statusChar := style.TraceStatusChar(string(e.Status))
		statusStyle := style.TraceStatusStyle(string(e.Status))

		actionStr := util.Truncate(e.Action, 20)

		row := fmt.Sprintf(" %s %s %s",
			style.Dim.Render(timeStr),
			statusStyle.Render(statusChar),
			actionStr,
		)
		lines = append(lines, style.Normal.Render(row))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return title + "\n" + content
}

type ArtifactPanelModel struct {
	artifacts []*artifact.Artifact
	selected  int
}

func NewArtifactPanelModel() *ArtifactPanelModel {
	return &ArtifactPanelModel{
		artifacts: []*artifact.Artifact{},
		selected:  0,
	}
}

func (m *ArtifactPanelModel) SetArtifacts(artifacts []*artifact.Artifact) {
	m.artifacts = artifacts
	if m.selected >= len(artifacts) {
		m.selected = 0
	}
}

func (m *ArtifactPanelModel) Next() {
	if m.selected < len(m.artifacts)-1 {
		m.selected++
	}
}

func (m *ArtifactPanelModel) Prev() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *ArtifactPanelModel) GetSelected() int {
	return m.selected
}

func (m *ArtifactPanelModel) GetSelectedArtifact() *artifact.Artifact {
	if m.selected >= 0 && m.selected < len(m.artifacts) {
		return m.artifacts[m.selected]
	}
	return nil
}

func (m *ArtifactPanelModel) View() string {
	if len(m.artifacts) == 0 {
		return style.Header.Render("Artifacts") + "\n\n  No artifacts\n"
	}

	lines := []string{}
	lines = append(lines, style.Header.Render("Artifacts"))
	lines = append(lines, "")
	lines = append(lines, style.Section.Render("  ID              Type          Size      Path"))
	lines = append(lines, style.Dim.Render("  "+strings.Repeat("─", 80)))

	for i, a := range m.artifacts {
		sizeStr := formatBytes(a.Size)

		idStr := a.ArtifactID
		if len(idStr) > 12 {
			idStr = idStr[:12]
		}

		row := fmt.Sprintf("  %-16s %-12s %-9s %s",
			idStr,
			a.Type,
			sizeStr,
			util.TruncateStart(a.Path, 40),
		)

		if i == m.selected {
			lines = append(lines, style.Selected.Render(row))
		} else {
			lines = append(lines, style.Normal.Render(row))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	suffix := string("KMGTPE"[exp])
	return fmt.Sprintf("%.1f %sB", float64(bytes)/float64(div), suffix)
}
