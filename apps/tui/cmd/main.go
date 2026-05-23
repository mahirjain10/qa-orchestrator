package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/playwright-community/playwright-go"
	"qa-orchestrator/apps/tui/internal/logging"
	"qa-orchestrator/apps/tui/internal/screens"
	"qa-orchestrator/packages/agents/engine"
	"qa-orchestrator/packages/agents/executor"
	browserruntime "qa-orchestrator/packages/browser-runtime"
	browsertools "qa-orchestrator/packages/browser-runtime/tools"
	"qa-orchestrator/packages/llm"
	"qa-orchestrator/packages/orchestrator/campaign"
	"qa-orchestrator/packages/runtime"
	sharedtypes "qa-orchestrator/packages/shared/types"
	"qa-orchestrator/packages/storage/artifact"
	"qa-orchestrator/packages/storage/session"
	"qa-orchestrator/packages/storage/trace"
)

func main() {
	resumeID := flag.String("resume", "", "resume a session by its Run ID")
	flag.StringVar(resumeID, "r", "", "resume a session by its Run ID (shorthand)")
	browserMode := flag.String("browser", "mock", "browser mode: mock (simulated) or real (Playwright)")
	flag.Parse()

	args := flag.Args()

	if *browserMode != "mock" && *browserMode != "real" {
		fmt.Fprintf(os.Stderr, "Error: --browser must be 'mock' or 'real', got %q\n", *browserMode)
		os.Exit(1)
	}

	if err := logging.InitFileOnly("./logs"); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not initialize logging: %v\n", err)
	}

	dataDir := "./data"

	sessionStore, err := session.NewSessionStore(dataDir)
	if err != nil {
		panic(fmt.Sprintf("creating session store: %v", err))
	}

	traceStore, err := trace.NewTraceStore(dataDir)
	if err != nil {
		panic(fmt.Sprintf("creating trace store: %v", err))
	}

	artifactStore, err := artifact.NewArtifactStore(dataDir)
	if err != nil {
		panic(fmt.Sprintf("creating artifact store: %v", err))
	}

	campaignPath := ""
	if len(args) > 0 {
		campaignPath = args[0]
	}

	if *resumeID != "" && campaignPath == "" {
		fmt.Fprintf(os.Stderr, "Error: --resume requires a campaign YAML path\n")
		os.Exit(1)
	}

	mainScreen := screens.NewMainScreenWithStores(sessionStore, traceStore, artifactStore)

	runCreatedCh := make(chan string, 1)
	mainScreen.SetRunCreatedChannel(runCreatedCh)

	lifecycleCtrl := runtime.NewLifecycleController("")
	mainScreen.SetLifecycleController(lifecycleCtrl)

	if *resumeID != "" {
		mainScreen.SetResumeID(*resumeID)
	}

	var campaignCtx context.Context
	var campaignCancel context.CancelFunc

	if campaignPath != "" {
		campaignCtx, campaignCancel = context.WithCancel(context.Background())
		mainScreen.SetCancelFunc(campaignCancel)
	}

	p := tea.NewProgram(mainScreen)

	if campaignPath != "" {
		go func() {
			if err := startCampaign(campaignPath, *resumeID, *browserMode, campaignCtx, sessionStore, traceStore, artifactStore, runCreatedCh, lifecycleCtrl); err != nil {
				log.Printf("Error starting campaign: %v", err)
			}
		}()
	}

	if _, err := p.Run(); err != nil {
		os.Stderr.WriteString("Error running TUI: " + err.Error() + "\n")
		os.Exit(1)
	}

	if campaignCancel != nil {
		campaignCancel()
	}
}

func startCampaign(campaignPath string, resumeID string, browserMode string, ctx context.Context, sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, runCreatedCh chan string, lifecycleCtrl *runtime.LifecycleController) error {
	parser := campaign.NewCampaignParser()
	parsed, err := parser.ParseFile(campaignPath)
	if err != nil {
		return fmt.Errorf("parsing campaign file: %w", err)
	}
	camp := parsed.Campaign

	var runID string

	if resumeID != "" {
		existingSession, err := sessionStore.Get(resumeID)
		if err != nil {
			return fmt.Errorf("loading session %s: %w", resumeID, err)
		}

		runID = existingSession.RunID

		for _, flow := range camp.Flows {
			existingFlow := findFlowState(existingSession, flow.ID)
			if existingFlow == nil {
				existingSession.AddFlowState(sharedtypes.FlowRunState{
					FlowID:   flow.ID,
					Name:     flow.Name,
					Mode:     flow.Mode,
					Priority: flow.Priority,
					Status:   sharedtypes.FlowStatePending,
				})
			}
		}

		existingSession.Status = sharedtypes.RunStateRunning
		if err := sessionStore.Save(existingSession); err != nil {
			return fmt.Errorf("updating resumed session: %w", err)
		}
	} else {
		sess, err := sessionStore.Create(camp)
		if err != nil {
			return fmt.Errorf("creating session: %w", err)
		}
		runID = sess.RunID
	}

	select {
	case runCreatedCh <- runID:
	default:
	}

	if lifecycleCtrl != nil {
		lifecycleCtrl.SetRunID(runID)
	}

	llmClient, err := createLLMClientForCampaign(camp)
	if err != nil {
		log.Printf("Warning: %v", err)
		log.Printf("Autonomous flows will fail without LLM client.")
	}

	agentEngine, browserRuntime := createAgentEngine(sessionStore, traceStore, artifactStore, llmClient, browserMode, lifecycleCtrl)
	go func() {
		if browserRuntime != nil {
			defer browserRuntime.Stop()
		}
		runCampaignWithContext(ctx, agentEngine, camp, parsed.TopologicalOrder, runID, sessionStore, lifecycleCtrl, browserRuntime, llmClient, traceStore, artifactStore)
	}()

	return nil
}

func findFlowState(sess *sharedtypes.Session, flowID string) *sharedtypes.FlowRunState {
	for i := range sess.Flows {
		if sess.Flows[i].FlowID == flowID {
			return &sess.Flows[i]
		}
	}
	return nil
}

func isFlowComplete(status sharedtypes.FlowState) bool {
	return status == sharedtypes.FlowStatePassed ||
		status == sharedtypes.FlowStateFailed ||
		status == sharedtypes.FlowStateSkippedUpstream ||
		status == sharedtypes.FlowStateSkippedUser
}

func createLLMClientForCampaign(camp *sharedtypes.Campaign) (*llm.HTTPClient, error) {
	if !hasAutonomousFlow(camp) {
		return nil, nil
	}

	cfg, err := llm.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("Campaign contains autonomous flows but LLM configuration failed: %w", err)
	}

	provider, _ := cfg.GetProvider()
	log.Printf("=== LLM Configuration ===")
	log.Printf("  Provider:  %s", provider.Name())
	log.Printf("  Model:     %s", cfg.Model)
	log.Printf("  Endpoint:  %s", provider.Endpoint(cfg.Model))
	log.Printf("========================")

	client, err := llm.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Campaign contains autonomous flows but LLM configuration failed: %w", err)
	}

	return client, nil
}

func createAgentEngine(sessionStore *session.SessionStore, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore, llmClient *llm.HTTPClient, browserMode string, lifecycleCtrl *runtime.LifecycleController) (*engine.AgentEngine, *browserruntime.BrowserRuntime) {
	var agentEngine *engine.AgentEngine
	var browserRuntime *browserruntime.BrowserRuntime
	var registry executor.ToolRegistry = executor.NewMockToolRegistry()
	var browserTools interface {
		ListToolsWithDocs() []browsertools.ToolInfo
	}

	if browserMode == "real" {
		rt, err := browserruntime.NewBrowserRuntime(nil)
		if err == nil {
			if err := rt.Start(context.Background()); err == nil {
				browserRuntime = rt
				browserRegistry := browsertools.NewToolRegistry(rt)
				registry = browserRegistry
				browserTools = browserRegistry
			}
		}
	}

	if llmClient != nil {
		cliWrapper := llm.NewSimpleClientWithClient(llmClient)
		agentEngine = engine.NewAgentEngineWithLLM(
			registry,
			sessionStore,
			traceStore,
			artifactStore,
			cliWrapper,
			browserTools,
		)
	} else {
		agentEngine = engine.NewAgentEngineWithStores(
			registry,
			sessionStore,
			traceStore,
			artifactStore,
		)
	}

	if lifecycleCtrl != nil {
		agentEngine.SetLifecycleController(lifecycleCtrl)
	}

	return agentEngine, browserRuntime
}

func hasAutonomousFlow(camp *sharedtypes.Campaign) bool {
	for _, flow := range camp.Flows {
		if flow.Mode == sharedtypes.FlowModeAutonomous {
			return true
		}
	}
	return false
}

func runCampaignWithContext(ctx context.Context, eng *engine.AgentEngine, camp *sharedtypes.Campaign, topoOrder []string, runID string, sessionStore *session.SessionStore, lifecycleCtrl *runtime.LifecycleController, browserRuntime *browserruntime.BrowserRuntime, llmClient *llm.HTTPClient, traceStore *trace.TraceStore, artifactStore *artifact.ArtifactStore) {
	select {
	case <-ctx.Done():
		_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateCancelling)
		return
	default:
	}

	_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)

	flowMap := make(map[string]sharedtypes.Flow)
	for _, flow := range camp.Flows {
		flowMap[flow.ID] = flow
	}

	parallelLimit := camp.Config.ParallelLimit
	if parallelLimit <= 0 {
		parallelLimit = 1
	}

	flowDone := make(map[string]chan struct{})
	for _, flowID := range topoOrder {
		flowDone[flowID] = make(chan struct{})
	}

	flowResults := make(map[string]*engine.ExecutionResult)
	flowStorageStates := make(map[string]*playwright.StorageState)
	var resultsMu sync.Mutex

	if existingSess, err := sessionStore.Get(runID); err == nil {
		for _, fs := range existingSess.Flows {
			if isFlowComplete(fs.Status) {
				var outcome engine.FlowOutcome
				switch fs.Status {
				case sharedtypes.FlowStatePassed:
					outcome = engine.OutcomePass
				case sharedtypes.FlowStateFailed:
					outcome = engine.OutcomeFail
				default:
					outcome = engine.OutcomeSkip
				}
				flowResults[fs.FlowID] = &engine.ExecutionResult{FlowID: fs.FlowID, Outcome: outcome}
			}
		}
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, parallelLimit)

	var pauseMu sync.Mutex
	var cancelled bool

	waitForPause := func() bool {
		backoff := 200 * time.Millisecond
		const maxBackoff = 2 * time.Second
		for {
			status, _ := sessionStore.Get(runID)
			if status == nil {
				return true
			}
			if status.Status == sharedtypes.RunStateCancelling || status.Status == sharedtypes.RunStateCancelled {
				pauseMu.Lock()
				cancelled = true
				pauseMu.Unlock()
				return true
			}
			if status.Status == sharedtypes.RunStatePausing || status.Status == sharedtypes.RunStatePaused {
				if status.Status == sharedtypes.RunStatePausing {
					_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStatePaused)
				}
				time.Sleep(backoff)
				backoff *= 2
				if backoff > maxBackoff {
					backoff = maxBackoff
				}
				continue
			}
			if status.Status == sharedtypes.RunStateResuming || status.Status == sharedtypes.RunStateRunning {
				_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateRunning)
				return false
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}

	buildDependencyContext := func(flowID string) string {
		flow, exists := flowMap[flowID]
		if !exists || len(flow.DependsOn) == 0 {
			return ""
		}

		resultsMu.Lock()
		defer resultsMu.Unlock()

		var parts []string
		urlRegex := regexp.MustCompile(`https?://[^\s,"')\]]+`)

		for _, depID := range flow.DependsOn {
			depFlow, depExists := flowMap[depID]
			if !depExists {
				continue
			}
			depResult, hasResult := flowResults[depID]
			if !hasResult || depResult.Outcome != engine.OutcomePass {
				continue
			}

			urls := urlRegex.FindAllString(depFlow.Goal, -1)
			if len(urls) > 0 {
				parts = append(parts, fmt.Sprintf("Upstream flow '%s' navigated to %s", depID, strings.Join(urls, ", ")))
			}
		}

		if len(parts) == 0 {
			return ""
		}
		return strings.Join(parts, "\n")
	}

	for _, flowID := range topoOrder {
		flow, exists := flowMap[flowID]
		if !exists {
			close(flowDone[flowID])
			continue
		}

		if _, alreadyDone := flowResults[flowID]; alreadyDone {
			close(flowDone[flowID])
			continue
		}

		wg.Add(1)
		go func(fid string, f sharedtypes.Flow) {
			defer wg.Done()
			defer close(flowDone[fid])

			for _, depID := range f.DependsOn {
				depChan, depExists := flowDone[depID]
				if !depExists {
					continue
				}
				<-depChan
			}

			pauseMu.Lock()
			isCancelled := cancelled
			pauseMu.Unlock()
			if isCancelled {
				_ = sessionStore.UpdateFlowState(runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
				return
			}

			upstreamFailed := false
			for _, depID := range f.DependsOn {
				resultsMu.Lock()
				depResult, hasResult := flowResults[depID]
				resultsMu.Unlock()
				if hasResult && (depResult.Outcome == engine.OutcomeSkip || depResult.Outcome == engine.OutcomeFail) {
					upstreamFailed = true
					break
				}
			}
			if upstreamFailed {
				_ = sessionStore.UpdateFlowState(runID, fid, sharedtypes.FlowStateSkippedUpstream, sharedtypes.ErrUpstreamFailed)
				resultsMu.Lock()
				flowResults[fid] = &engine.ExecutionResult{FlowID: fid, Outcome: engine.OutcomeSkip}
				resultsMu.Unlock()
				return
			}

			select {
			case <-ctx.Done():
				pauseMu.Lock()
				cancelled = true
				pauseMu.Unlock()
				_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateCancelling)
				_ = sessionStore.UpdateFlowState(runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
				return
			default:
			}

			if waitForPause() {
				_ = sessionStore.UpdateFlowState(runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
				return
			}

			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				pauseMu.Lock()
				cancelled = true
				pauseMu.Unlock()
				_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateCancelling)
				_ = sessionStore.UpdateFlowState(runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
				return
			}
			defer func() { <-semaphore }()

			depCtx := buildDependencyContext(fid)

			var flowEngine *engine.AgentEngine
			var flowBrowser *browserruntime.FlowBrowserRuntime

			if browserRuntime != nil {
				// Inherit storage state from upstream completed flows
				var upstreamState *playwright.StorageState
				for _, depID := range f.DependsOn {
					resultsMu.Lock()
					state, hasState := flowStorageStates[depID]
					resultsMu.Unlock()
					if hasState {
						upstreamState = state
						break
					}
				}

				fb, err := browserRuntime.NewFlowRuntime(upstreamState)
				if err != nil {
					resultsMu.Lock()
					flowResults[fid] = &engine.ExecutionResult{FlowID: fid, Outcome: engine.OutcomeFail, Errors: []string{fmt.Sprintf("failed to create flow browser: %v", err)}}
					resultsMu.Unlock()
					_ = sessionStore.UpdateFlowState(runID, fid, sharedtypes.FlowStateFailed, err.Error())
					return
				}
				flowBrowser = fb
				defer flowBrowser.Close()

				flowRegistry := browsertools.NewToolRegistryWithContext(fb, context.Background())
				flowEngine = engine.NewAgentEngineWithStores(
					flowRegistry,
					sessionStore,
					traceStore,
					artifactStore,
				)
				if llmClient != nil {
					cliWrapper := llm.NewSimpleClientWithClient(llmClient)
					flowEngine.SetLLMClient(cliWrapper)
					flowEngine.SetBrowserTools(flowRegistry)
				}
				flowEngine.SetLifecycleController(lifecycleCtrl)
			} else {
				flowEngine = eng
			}

			flowEngine.SetDependencyContext(depCtx)
			result := flowEngine.RunFlow(runID, f)

			// Save storage state from this flow for downstream dependents
			if flowBrowser != nil {
				if state, err := flowBrowser.StorageState(); err == nil && state != nil {
					resultsMu.Lock()
					flowStorageStates[fid] = state
					resultsMu.Unlock()
				}
			}

			resultsMu.Lock()
			flowResults[fid] = result
			resultsMu.Unlock()

			if result.Outcome == engine.OutcomeFail {
				for _, otherFlow := range camp.Flows {
					for _, dep := range otherFlow.DependsOn {
						if dep == fid {
							_ = sessionStore.UpdateFlowState(runID, otherFlow.ID, sharedtypes.FlowStateSkippedUpstream, sharedtypes.ErrUpstreamFailed)
							break
						}
					}
				}
			}
		}(flowID, flow)
	}

	wg.Wait()

	sess, err := sessionStore.Get(runID)
	if err != nil || sess == nil {
		_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateFailed)
		return
	}

	if sess.Status == sharedtypes.RunStateCancelling || sess.Status == sharedtypes.RunStateCancelled {
		_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateCancelled)
		return
	}

	hasFailure := false
	for _, f := range sess.Flows {
		if f.Status == sharedtypes.FlowStateFailed {
			hasFailure = true
			break
		}
	}

	if hasFailure {
		_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateFailed)
	} else {
		_ = sessionStore.UpdateStatus(runID, sharedtypes.RunStateCompleted)
	}
}
