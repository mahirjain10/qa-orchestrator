package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"qa-orchestrator/apps/tui/internal/style"
	"qa-orchestrator/apps/tui/internal/util"
)

type CampaignListModel struct {
	campaigns []string
	selected  int
	height    int
}

func NewCampaignListModel() *CampaignListModel {
	return &CampaignListModel{
		campaigns: []string{},
		selected:  0,
		height:    0,
	}
}

func (m *CampaignListModel) SetCampaigns(names []string) {
	m.campaigns = names
	if m.selected >= len(m.campaigns) {
		m.selected = 0
	}
}

func (m *CampaignListModel) SetSelected(idx int) {
	if idx >= 0 && idx < len(m.campaigns) {
		m.selected = idx
	}
}

func (m *CampaignListModel) View() string {
	if len(m.campaigns) == 0 {
		return style.Header.Render("Campaigns") + "\n\n  No campaigns loaded\n"
	}

	var lines []string
	lines = append(lines, style.Header.Render("Campaigns"))
	lines = append(lines, "")

	for i, name := range m.campaigns {
		if i == m.selected {
			lines = append(lines, style.Selected.Render(fmt.Sprintf("  > %s", name)))
		} else {
			lines = append(lines, style.Normal.Render(fmt.Sprintf("    %s", name)))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *CampaignListModel) ViewWithWidth(width int) string {
	title := style.ViewTitle.Width(width - 2).Render(" Campaigns ")

	if len(m.campaigns) == 0 {
		return title + "\n\n  No campaigns loaded\n"
	}

	var lines []string

	for i, name := range m.campaigns {
		truncated := util.Truncate(name, util.SafeWidth(width-6, 4))
		if i == m.selected {
			lines = append(lines, style.SelectedBold.Render(fmt.Sprintf(" ▶ %s", truncated)))
		} else {
			lines = append(lines, style.Normal.Render(fmt.Sprintf("   %s", truncated)))
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
	return title + "\n" + content
}

func (m *CampaignListModel) Next() {
	if m.selected < len(m.campaigns)-1 {
		m.selected++
	}
}

func (m *CampaignListModel) Prev() {
	if m.selected > 0 {
		m.selected--
	}
}

func (m *CampaignListModel) GetSelected() int {
	return m.selected
}
