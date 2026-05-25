package main

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbletea"
	"qa-orchestrator/apps/tui/internal/screens"
	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func createProgram(sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, lifecycleCtrl *runtime.LifecycleController, runCreatedCh chan string, resumeID string, cancelFunc context.CancelFunc) *tea.Program {
	mainScreen := screens.NewMainScreenWithStores(sessionStore, traceStore, artifactStore)
	mainScreen.SetRunCreatedChannel(runCreatedCh)
	mainScreen.SetLifecycleController(lifecycleCtrl)

	if resumeID != "" {
		mainScreen.SetResumeID(resumeID)
	}

	if cancelFunc != nil {
		mainScreen.SetCancelFunc(cancelFunc)
	}

	return tea.NewProgram(mainScreen)
}

func runTUI(p *tea.Program) error {
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("tui run failed: %w", err)
	}
	return nil
}
