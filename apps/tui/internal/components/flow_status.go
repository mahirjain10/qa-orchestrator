package components

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"
	"qa-orchestrator/packages/shared/types"
)

type FlowStatusModel struct {
	flows    []types.FlowRunState
	selected int
	Expanded bool
	viewport viewport.Model
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

func (m *FlowStatusModel) SyncFlows(flows []types.FlowRunState) {
	var selectedID string
	if len(m.flows) > 0 && m.selected < len(m.flows) {
		selectedID = m.flows[m.selected].FlowID
	}
	m.flows = flows
	if selectedID != "" {
		for i, f := range m.flows {
			if f.FlowID == selectedID {
				m.selected = i
				return
			}
		}
	}
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
		return style.Header.Render("Flow Status") + "\n\n  No flows\n"
	}

	header := fmt.Sprintf("  %-20s %-12s %-10s %-10s %-10s %-12s", "Flow ID", "Mode", "Priority", "Status", "Started", "Duration")
	lines := []string{}
	lines = append(lines, style.Header.Render("Flow Status"))
	lines = append(lines, "")
	lines = append(lines, style.Section.Render(header))
	lines = append(lines, style.Dim.Render("  "+strings.Repeat("─", 80)))

	for i, f := range m.flows {
		statusStr := string(f.Status)
		statusColor := style.StatusPending

		switch f.Status {
		case types.FlowStateRunning:
			statusColor = style.StatusRunning
		case types.FlowStatePassed:
			statusColor = style.StatusPassed
		case types.FlowStateFailed:
			statusColor = style.StatusFailed
		case types.FlowStatePaused:
			statusColor = style.StatusPaused
		case types.FlowStatePending:
			statusColor = style.StatusPending
		case types.FlowStateRetrying:
			statusColor = style.StatusRetrying
		case types.FlowStateSkippedUpstream, types.FlowStateSkippedUser, types.FlowStateBlockedConfigError:
			statusColor = style.StatusCancelled
		case types.FlowStateWaitingInput:
			statusColor = style.StatusPaused
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

		modeStr := string(f.Mode)
		priorityStr := string(f.Priority)

		row := fmt.Sprintf("  %-20s %-12s %-10s %-10s %-10s %-12s",
			f.FlowID,
			modeStr,
			priorityStr,
			statusColor.Render(statusStr),
			startedStr,
			durationStr,
		)
		if i == m.selected {
			lines = append(lines, style.Selected.Render(row))
		} else {
			lines = append(lines, style.Normal.Render(row))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *FlowStatusModel) ViewWithWidth(width int) string {
	title := style.ViewTitle.Width(width - 2).Render(" Flow Status ")

	if len(m.flows) == 0 {
		return title + "\n\n  No flows\n"
	}

	colFlow := width / 3
	colMode := 8
	colPriority := 8
	colStatus := 10

	lines := []string{}
	headerFmt := fmt.Sprintf(" %%-%ds %%-%ds %%-%ds %%-%ds", colFlow, colMode, colPriority, colStatus)
	lines = append(lines, fmt.Sprintf(headerFmt, "Flow", "Mode", "Priority", "Status"))
	lines = append(lines, style.Dim.Render(strings.Repeat("─", width-2)))

	for i, f := range m.flows {
		statusStr := string(f.Status)
		statusColor := style.StatusPending

		switch f.Status {
		case types.FlowStateRunning:
			statusColor = style.StatusRunning
		case types.FlowStatePassed:
			statusColor = style.StatusPassed
		case types.FlowStateFailed:
			statusColor = style.StatusFailed
		case types.FlowStatePaused:
			statusColor = style.StatusPaused
		case types.FlowStatePending:
			statusColor = style.StatusPending
		case types.FlowStateRetrying:
			statusColor = style.StatusRetrying
		case types.FlowStateSkippedUpstream, types.FlowStateSkippedUser, types.FlowStateBlockedConfigError:
			statusColor = style.StatusCancelled
		case types.FlowStateWaitingInput:
			statusColor = style.StatusPaused
		}

		flowID := util.Truncate(f.FlowID, util.SafeWidth(colFlow-2, 4))

		flowFmt := fmt.Sprintf("%%-%ds", colFlow)
		modeFmt := fmt.Sprintf("%%-%ds", colMode)
		priorityFmt := fmt.Sprintf("%%-%ds", colPriority)
		statusFmt := fmt.Sprintf("%%-%ds", colStatus)

		flowStr := fmt.Sprintf(flowFmt, flowID)
		modeStr := fmt.Sprintf(modeFmt, string(f.Mode))
		priorityStr := fmt.Sprintf(priorityFmt, string(f.Priority))
		statusPadded := fmt.Sprintf(statusFmt, statusStr)

		row := fmt.Sprintf(" %s %s %s %s", flowStr, modeStr, priorityStr, statusColor.Render(statusPadded))
		if i == m.selected {
			lines = append(lines, style.Selected.Render(row))
		} else {
			lines = append(lines, style.Normal.Render(row))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return title + "\n" + content
}
