package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

var (
	panelTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	headerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("205")).
			Bold(true)

	itemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("237"))

	selectedBoldStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("86")).
				Bold(true).
				Background(lipgloss.Color("235"))

	statusPending   = lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	statusRunning   = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	statusPassed    = lipgloss.NewStyle().Foreground(lipgloss.Color("76"))
	statusFailed    = lipgloss.NewStyle().Foreground(lipgloss.Color("204"))
	statusPaused    = lipgloss.NewStyle().Foreground(lipgloss.Color("228"))
	statusCancelled = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
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
		return headerStyle.Render("Campaigns") + "\n\n  No campaigns loaded\n"
	}

	var lines []string
	lines = append(lines, headerStyle.Render("Campaigns"))
	lines = append(lines, "")

	for i, name := range m.campaigns {
		if i == m.selected {
			lines = append(lines, selectedStyle.Render(fmt.Sprintf("  > %s", name)))
		} else {
			lines = append(lines, itemStyle.Render(fmt.Sprintf("    %s", name)))
		}
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *CampaignListModel) ViewWithWidth(width int) string {
	title := panelTitleStyle.Width(width - 2).Render(" Campaigns ")

	if len(m.campaigns) == 0 {
		return title + "\n\n  No campaigns loaded\n"
	}

	var lines []string

	for i, name := range m.campaigns {
		truncated := name
		if len(name) > width-6 {
			truncated = name[:width-9] + "..."
		}
		if i == m.selected {
			lines = append(lines, selectedBoldStyle.Render(fmt.Sprintf(" ▶ %s", truncated)))
		} else {
			lines = append(lines, itemStyle.Render(fmt.Sprintf("   %s", truncated)))
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
