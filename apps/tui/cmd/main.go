package main

import (
	"os"

	"github.com/charmbracelet/bubbletea"
	"qa-orchestrator/apps/tui/internal/screens"
	"qa-orchestrator/packages/storage/session"
)

func main() {
	baseDir := "./data"
	store, err := session.NewSessionStore(baseDir)
	if err != nil {
		panic(err)
	}

	mainScreen := screens.NewMainScreen(store)

	if _, err := tea.NewProgram(mainScreen).Run(); err != nil {
		os.Stderr.WriteString("Error running TUI: " + err.Error() + "\n")
		os.Exit(1)
	}
}
