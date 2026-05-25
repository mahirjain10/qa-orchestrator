package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sync"

	"qa-orchestrator/apps/tui/internal/logging"
	"qa-orchestrator/packages/runtime"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func main() {
	resumeID := flag.String("resume", "", "resume a session by its Run ID")
	flag.StringVar(resumeID, "r", "", "resume a session by its Run ID (shorthand)")
	browserMode := flag.String("browser", "mock", "browser mode: mock (simulated) or real (Playwright)")
	dataDir := flag.String("data-dir", "./data", "directory for session/trace/artifact data")
	flag.Parse()

	args := flag.Args()

	if *browserMode != "mock" && *browserMode != "real" {
		fmt.Fprintf(os.Stderr, "Error: --browser must be 'mock' or 'real', got %q\n", *browserMode)
		os.Exit(1)
	}

	if err := logging.InitFileOnly("./logs"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logging: %v\n", err)
	}
	defer logging.Close()

	sessionStore, err := session.NewSessionStore(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session store: %v\n", err)
		os.Exit(1)
	}

	traceStore, err := trace.NewTraceStore(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating trace store: %v\n", err)
		os.Exit(1)
	}

	artifactStore, err := artifact.NewArtifactStore(*dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating artifact store: %v\n", err)
		os.Exit(1)
	}

	campaignPath := ""
	if len(args) > 0 {
		campaignPath = args[0]
	}

	if *resumeID != "" && campaignPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --resume requires a campaign YAML path\n")
		os.Exit(1)
	}

	runCreatedCh := make(chan string, 1)
	lifecycleCtrl := runtime.NewLifecycleController("")

	var campaignCtx context.Context
	var campaignCancel context.CancelFunc

	if campaignPath != "" {
		campaignCtx, campaignCancel = context.WithCancel(context.Background())
	}

	p := createProgram(sessionStore, traceStore, artifactStore, lifecycleCtrl, runCreatedCh, *resumeID, campaignCancel)

	var wg sync.WaitGroup

	if campaignPath != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cfg := CampaignConfig{
				CampaignPath:  campaignPath,
				ResumeID:      *resumeID,
				BrowserMode:   *browserMode,
				Ctx:           campaignCtx,
				SessionStore:  sessionStore,
				TraceStore:    traceStore,
				ArtifactStore: artifactStore,
				RunCreatedCh:  runCreatedCh,
				LifecycleCtrl: lifecycleCtrl,
			}
			if err := startCampaign(cfg); err != nil {
				if campaignCancel != nil {
					campaignCancel()
				}
				log.Printf("Error starting campaign: %v", err)
			}
		}()
	}

	if err := runTUI(p); err != nil {
		os.Stderr.WriteString("Error running TUI: " + err.Error() + "\n")
		os.Exit(1)
	}

	if campaignCancel != nil {
		campaignCancel()
	}

	// Wait for campaign runner to finish cleanup
	wg.Wait()
}
