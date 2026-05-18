package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/packages/shared/types"
)

var (
	flowTableHeaderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true)

	flowTableCellStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	flowTableHeaderBorder = lipgloss.NewStyle().
				Foreground(lipgloss.Color("240"))
)

type FlowStatusModel struct {
	flows    []types.FlowRunState
	selected int
}

func NewFlowStatusModel() *FlowStatusModel {
	return &FlowStatusModel{
		flows:    []types.FlowRunState{},
		selected: 0,
	}
}

func (m *FlowStatusModel) SetFlows(flows []types.FlowRunState) {
	m.flows = flows
	if m.selected >= len(flows) {
		m.selected = 0
	}
}

func (m *FlowStatusModel) Next() {
	if m.selected < len(m.flows)-1 {
		m.selected++
	}
}

func (m *FlowStatusModel) Prev() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *FlowStatusModel) GetSelected() int {
	return m.selected
}

func (m *FlowStatusModel) SetSelected(idx int) {
	if idx >= 0 && idx < len(m.flows) {
		m.selected = idx
	}
}

func (m *FlowStatusModel) View() string {
	if len(m.flows) == 0 {
		return flowTableHeaderStyle.Render("Flow Status") + "\n\n  No flows\n"
	}

	header := fmt.Sprintf("  %-20s %-10s %-12s %-20s", "Flow ID", "Status", "Started", "Duration")
	border := "  " + flowTableHeaderBorder.Render(strings.Repeat("─", 66))

	lines := []string{}
	lines = append(lines, flowTableHeaderStyle.Render("Flow Status"))
	lines = append(lines, "")
	lines = append(lines, flowTableHeaderBorder.Render(header))
	lines = append(lines, border)

	for i, f := range m.flows {
		statusStr := string(f.Status)
		statusColor := statusPending

		switch f.Status {
		case types.FlowStateRunning:
			statusColor = statusRunning
		case types.FlowStatePassed:
			statusColor = statusPassed
		case types.FlowStateFailed:
			statusColor = statusFailed
		case types.FlowStatePaused:
			statusColor = statusPaused
		case types.FlowStateRetrying:
			statusColor = statusPaused
		case types.FlowStateSkippedUpstream, types.FlowStateBlockedConfigError:
			statusColor = statusCancelled
		}

		startedStr := "-"
		if f.StartedAt != nil {
			startedStr = f.StartedAt.Format("15:04:05")
		}

		durationStr := "-"
		if f.StartedAt != nil && f.FinishedAt != nil {
			dur := f.FinishedAt.Sub(*f.StartedAt)
			durationStr = dur.Round(time.Second).String()
		} else if f.StartedAt != nil {
			dur := time.Since(*f.StartedAt)
			durationStr = dur.Round(time.Second).String() + " (running)"
		}

		row := fmt.Sprintf("  %-20s %-10s %-12s %-20s", f.FlowID, statusColor.Render(statusStr), startedStr, durationStr)
		if i == m.selected {
			lines = append(lines, selectedStyle.Render(row))
		} else {
			lines = append(lines, flowTableCellStyle.Render(row))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}
