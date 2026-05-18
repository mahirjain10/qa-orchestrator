package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/shared/types"
)

var (
	tracePanelHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("226")).
				Bold(true)

	tracePanelCellStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	traceEventTimeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245"))

	traceEventSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82"))

	traceEventFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("196"))

	traceEventPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("241"))

	tracePanelHeaderBorder = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

type TracePanelModel struct {
	events    []*types.TraceEvent
	selected  int
	maxEvents int
}

func NewTracePanelModel() *TracePanelModel {
	return &TracePanelModel{
		events:    []*types.TraceEvent{},
		selected:  0,
		maxEvents: 50,
	}
}

func (m *TracePanelModel) SetEvents(events []*types.TraceEvent) {
	if len(events) > m.maxEvents {
		m.events = events[len(events)-m.maxEvents:]
	} else {
		m.events = events
	}
	if m.selected >= len(m.events) {
		m.selected = 0
	}
}

func (m *TracePanelModel) AppendEvent(event *types.TraceEvent) {
	m.events = append(m.events, event)
	if len(m.events) > m.maxEvents {
		m.events = m.events[1:]
	}
}

func (m *TracePanelModel) Next() {
	if m.selected < len(m.events)-1 {
		m.selected++
	}
}

func (m *TracePanelModel) Prev() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *TracePanelModel) GetSelected() int {
	return m.selected
}

func (m *TracePanelModel) View() string {
	if len(m.events) == 0 {
		return tracePanelHeaderStyle.Render("Trace Events") + "\n\n  No trace events\n"
	}

	lines := []string{}
	lines = append(lines, tracePanelHeaderStyle.Render("Trace Events"))
	lines = append(lines, "")
	lines = append(lines, tracePanelHeaderBorder.Render("  Time      Agent      Action          Status    Step"))
	lines = append(lines, tracePanelHeaderBorder.Render(strings.Repeat("─", 70)))

	for i := len(m.events) - 1; i >= 0; i-- {
		e := m.events[i]
		timeStr := e.Timestamp.Format("15:04:05")

		statusStr := string(e.Status)
		statusStyle := traceEventPendingStyle
		switch e.Status {
		case types.TraceStatusSuccess:
			statusStyle = traceEventSuccessStyle
		case types.TraceStatusFailed:
			statusStyle = traceEventFailedStyle
		}

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
			traceEventTimeStyle.Render(timeStr),
			agentStr,
			actionStr,
			statusStyle.Render(statusStr),
			stepID,
		)

		if i == m.selected {
			lines = append(lines, selectedStyle.Render(row))
		} else {
			lines = append(lines, tracePanelCellStyle.Render(row))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

type ArtifactPanelModel struct {
	artifacts []*artifact.Artifact
	selected  int
}

func NewArtifactPanelModel() *ArtifactPanelModel {
	return &ArtifactPanelModel{
		artifacts: []*artifact.Artifact{},
		selected: 0,
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
		return artifactPanelHeaderStyle.Render("Artifacts") + "\n\n  No artifacts\n"
	}

	lines := []string{}
	lines = append(lines, artifactPanelHeaderStyle.Render("Artifacts"))
	lines = append(lines, "")
	lines = append(lines, artifactPanelBorder.Render("  ID              Type          Size      Path"))
	lines = append(lines, artifactPanelBorder.Render(strings.Repeat("─", 80)))

	for i, a := range m.artifacts {
		sizeStr := formatBytes(a.Size)

		row := fmt.Sprintf("  %-16s %-12s %-9s %s",
			a.ArtifactID[:12],
			a.Type,
			sizeStr,
			truncatePath(a.Path),
		)

		if i == m.selected {
			lines = append(lines, selectedStyle.Render(row))
		} else {
			lines = append(lines, artifactPanelCellStyle.Render(row))
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
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func truncatePath(path string) string {
	if len(path) <= 40 {
		return path
	}
	return "..." + path[len(path)-37:]
}

var (
	artifactPanelHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("82")).
				Bold(true)

	artifactPanelCellStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	artifactPanelBorder = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)