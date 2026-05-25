package screens

import (
	"fmt"

	"qa-orchestrator/packages/shared/types"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *MainScreen) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		inputVal := m.commandBar.Input.Value()
		if inputVal != "" {
			cmd := m.processSteeringCommand(inputVal)
			m.commandBar.Blur()
			return m, cmd
		}
		return m, nil
	}

	cmd, handled := m.commandBar.Update(msg)
	if handled {
		return m, cmd
	}
	return m, nil
}

func (m *MainScreen) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "enter" {
		m.tracePanel.Filter.Text = m.tracePanel.FilterInput.Value()
		m.tracePanel.FilterMode = false
		m.tracePanel.FilterInput.Blur()
		m.tracePanel.FilterInput.SetValue("")
		m.tracePanel.Selected = 0
		m.tracePanel.UpdateViewportContent()
		if m.tracePanel.Filter.Text != "" {
			m.setMsg(fmt.Sprintf("Filter: \"%s\"", m.tracePanel.Filter.Text))
		} else {
			m.setMsg("Filter cleared")
		}
		return m, nil
	}

	if msg.String() == "escape" || msg.String() == "esc" {
		m.tracePanel.FilterMode = false
		m.tracePanel.FilterInput.Blur()
		m.tracePanel.FilterInput.SetValue("")
		m.setMsg("Filter cancelled")
		return m, nil
	}

	var cmd tea.Cmd
	m.tracePanel.FilterInput, cmd = m.tracePanel.FilterInput.Update(msg)
	return m, cmd
}

func (m *MainScreen) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "1":
		m.activeView = ViewDashboard
		m.setMsg("Dashboard view")

	case "2":
		m.activeView = ViewFlows
		m.setMsg("Flows view")

	case "3":
		m.activeView = ViewTraces
		m.setMsg("Traces view")

	case "4":
		m.activeView = ViewReport
		m.setMsg("Report view")

	case "tab":
		m.sidebarFocus = !m.sidebarFocus
		m.setMsg(map[bool]string{true: "Sidebar focused", false: "Content focused"}[m.sidebarFocus])

	case "up", "k":
		if m.sidebarFocus {
			m.cycleView(-1)
		} else {
			m.handleContentUp()
		}

	case "down", "j":
		if m.sidebarFocus {
			m.cycleView(1)
		} else {
			m.handleContentDown()
		}

	case "enter":
		if m.activeView == ViewFlows && !m.sidebarFocus {
			m.flowStatus.Expanded = !m.flowStatus.Expanded
		} else if m.currentRun == nil {
			idx := m.campaignList.GetSelected()
			visSessions := m.visualSessions()
			if idx >= 0 && idx < len(visSessions) {
				runID := visSessions[idx].RunID
				m.currentRun = visSessions[idx]
				m.setMsg(fmt.Sprintf("Selected run: %s", runID))
				return m, tea.Batch(
					refreshAllCmd(runID, m.sessionStore, m.traceStore, m.artifactStore, m.reportGenerator),
				)
			}
		}

	case "left", "h":
		if m.activeView == ViewFlows && m.flowStatus.Expanded && !m.sidebarFocus {
			m.flowStatus.Expanded = false
		}

	case " ":
		runID := m.currentRunID()
		if runID != "" {
			sess, err := m.handlers.GetRunStatus(runID)
			if err != nil {
				m.setMsg(fmt.Sprintf("Error getting run status: %v", err))
			} else {
				switch sess.Status {
				case types.RunStatePending, types.RunStateRunning:
					err = m.handlers.PauseRun(runID)
					if err == nil {
						m.setMsg("Run pausing...")
						return m, fetchRunCmd(m.sessionStore, runID)
					}
				case types.RunStatePaused:
					err = m.handlers.ResumeRun(runID)
					if err == nil {
						m.setMsg("Run resuming...")
						return m, fetchRunCmd(m.sessionStore, runID)
					}
				case types.RunStatePausing:
					m.setMsg("Run is pausing, please wait")
				case types.RunStateResuming:
					m.setMsg("Run is resuming, please wait")
				case types.RunStateCancelling:
					m.setMsg("Run is cancelling, please wait")
				case types.RunStateWaitingInput:
					err = m.handlers.AcknowledgeInputAndResume(runID)
					if err == nil {
						m.setMsg("Run resumed from WAITING_FOR_INPUT")
						return m, fetchRunCmd(m.sessionStore, runID)
					}
				}
				if err != nil {
					m.setMsg(fmt.Sprintf("Error: %v", err))
				}
			}
		}

	case "x":
		runID := m.currentRunID()
		if runID != "" {
			err := m.handlers.CancelRun(runID)
			if err != nil {
				m.setMsg(fmt.Sprintf("Error cancelling: %v", err))
			} else {
				if m.lifecycle != nil {
					m.lifecycle.RequestCancel()
				}
				if m.cancelFunc != nil {
					m.cancelFunc()
				}
				m.setMsg("Run cancelled")
				return m, fetchRunCmd(m.sessionStore, runID)
			}
		}

	case "r":
		runID := m.currentRunID()
		return m, tea.Batch(
			refreshAllCmd(runID, m.sessionStore, m.traceStore, m.artifactStore, m.reportGenerator),
		)

	case ":":
		if m.currentRunID() != "" {
			m.commandBar.Focus()
			m.setMsg("Command mode: type and press ENTER. ESC to cancel.")
		} else {
			m.setMsg("Select a run first before using commands")
		}

	case "f":
		if m.activeView == ViewTraces {
			m.tracePanel.FollowTail = !m.tracePanel.FollowTail
			m.setMsg(fmt.Sprintf("Follow tail: %v", m.tracePanel.FollowTail))
		}

	case "/":
		if m.activeView == ViewTraces && !m.sidebarFocus {
			m.tracePanel.FilterMode = true
			m.tracePanel.FilterInput.Focus()
			m.setMsg("Filter traces (ESC to cancel)")
		}

	case "S":
		if m.activeView == ViewTraces && !m.sidebarFocus {
			m.tracePanel.Filter.ShowFailed = !m.tracePanel.Filter.ShowFailed
			m.tracePanel.Selected = 0
			m.tracePanel.UpdateViewportContent()
			m.setMsg(fmt.Sprintf("Show failed only: %v", m.tracePanel.Filter.ShowFailed))
		}
	}
	return m, nil
}

func (m *MainScreen) cycleView(dir int) {
	views := []View{ViewDashboard, ViewFlows, ViewTraces, ViewReport}
	idx := 0
	for i, v := range views {
		if v == m.activeView {
			idx = i
			break
		}
	}
	idx = (idx + dir + len(views)) % len(views)
	m.activeView = views[idx]
}

func (m *MainScreen) handleContentUp() {
	switch m.activeView {
	case ViewDashboard:
		m.campaignList.Prev()
	case ViewFlows:
		m.flowStatus.Prev()
	case ViewTraces:
		m.tracePanel.Prev()
		m.tracePanel.UpdateViewportContent()
		m.scrollTraceToSelection()
	}
}

func (m *MainScreen) handleContentDown() {
	switch m.activeView {
	case ViewDashboard:
		m.campaignList.Next()
	case ViewFlows:
		m.flowStatus.Next()
	case ViewTraces:
		m.tracePanel.Next()
		m.tracePanel.UpdateViewportContent()
		m.scrollTraceToSelection()
	}
}

func (m *MainScreen) scrollTraceToSelection() {
	events := m.tracePanel.FilteredEvents()
	if len(events) == 0 {
		return
	}
	selectedRow := 2 + (len(events) - 1 - m.tracePanel.Selected)
	vp := &m.tracePanel.Viewport
	half := vp.Height / 2
	if vp.Height <= 0 {
		half = 5
	}
	target := selectedRow - half
	if target < 0 {
		target = 0
	}
	vp.SetYOffset(target)
}
