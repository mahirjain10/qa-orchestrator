package style

import "github.com/charmbracelet/lipgloss"

// Color palette — single source of truth for all TUI colors
const (
	Cyan         = lipgloss.Color("86")
	Green        = lipgloss.Color("76")
	Red          = lipgloss.Color("204")
	Yellow       = lipgloss.Color("228")
	Blue         = lipgloss.Color("75")
	Orange       = lipgloss.Color("208")
	Pink         = lipgloss.Color("205")
	Gray         = lipgloss.Color("245")
	DimGray      = lipgloss.Color("241")
	Border       = lipgloss.Color("240")
	BgDark       = lipgloss.Color("235")
	BgSel        = lipgloss.Color("237")
	Text         = lipgloss.Color("252")
	TextSel      = lipgloss.Color("229")
	BrightGreen  = lipgloss.Color("82")
	BrightYellow = lipgloss.Color("214")
	Green46      = lipgloss.Color("46")
)

// Status indicator styles
var (
	StatusRunning   = lipgloss.NewStyle().Foreground(Blue).Bold(true)
	StatusPassed    = lipgloss.NewStyle().Foreground(Green)
	StatusFailed    = lipgloss.NewStyle().Foreground(Red)
	StatusPaused    = lipgloss.NewStyle().Foreground(Yellow)
	StatusPending   = lipgloss.NewStyle().Foreground(Gray)
	StatusCancelled = lipgloss.NewStyle().Foreground(DimGray)
	StatusRetrying  = lipgloss.NewStyle().Foreground(Yellow)
)

// Layout & typography styles
var (
	Header       = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	ViewTitle    = lipgloss.NewStyle().Foreground(Pink).Bold(true)
	Section      = lipgloss.NewStyle().Foreground(Gray).Bold(true)
	Normal       = lipgloss.NewStyle().Foreground(Text)
	Dim          = lipgloss.NewStyle().Foreground(DimGray)
	Selected     = lipgloss.NewStyle().Foreground(TextSel).Background(BgSel)
	SelectedBold = lipgloss.NewStyle().Foreground(Cyan).Bold(true).Background(BgDark)
	Help         = lipgloss.NewStyle().Foreground(DimGray)
	Msg          = lipgloss.NewStyle().Foreground(Cyan)
	DimmedBg     = lipgloss.NewStyle().Foreground(lipgloss.Color("242"))

	CommandSuggestion      = lipgloss.NewStyle().Foreground(Gray)
	CommandSuggestionMatch = lipgloss.NewStyle().Foreground(Cyan).Bold(true)
	CommandDesc            = lipgloss.NewStyle().Foreground(DimGray)
)

// Border styles
var (
	ActiveBorder         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Cyan).Bold(true)
	InactiveBorder       = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).BorderForeground(Border)
	PanelBorder          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Border)
	FocusBorder          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Cyan).Bold(true)
	ModalBorder          = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Cyan)
	SidebarBorder        = lipgloss.NewStyle().Border(lipgloss.Border{Right: "│"}).BorderForeground(Border)
	CommandPaletteBorder = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Pink)
)

// Trace status helpers
func TraceStatusChar(s string) string {
	switch s {
	case "success":
		return "✓"
	case "failed":
		return "✗"
	case "skipped":
		return "○"
	default:
		return "·"
	}
}

func TraceStatusStyle(s string) lipgloss.Style {
	switch s {
	case "success":
		return lipgloss.NewStyle().Foreground(BrightGreen)
	case "failed":
		return lipgloss.NewStyle().Foreground(Red)
	case "skipped":
		return lipgloss.NewStyle().Foreground(Gray)
	default:
		return lipgloss.NewStyle().Foreground(DimGray)
	}
}
