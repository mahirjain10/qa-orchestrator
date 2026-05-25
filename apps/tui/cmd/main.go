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
	defer logging.Close()

	dataDir := "./data"

	sessionStore, err := session.NewSessionStore(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating session store: %v\n", err)
		os.Exit(1)
	}

	traceStore, err := trace.NewTraceStore(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating trace store: %v\n", err)
		os.Exit(1)
	}

	artifactStore, err := artifact.NewArtifactStore(dataDir)
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
				campaignCancel()
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

	provider, err := cfg.GetProvider()
	providerName := "unknown"
	providerEndpoint := "unknown"
	if err == nil && provider != nil {
		providerName = provider.Name()
		providerEndpoint = provider.Endpoint(cfg.Model)
	}
	log.Printf("=== LLM Configuration ===")
	log.Printf("  Provider:  %s", providerName)
	log.Printf("  Model:     %s", cfg.Model)
	log.Printf("  Endpoint:  %s", providerEndpoint)
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
		if err != nil {
			log.Printf("Warning: failed to create browser runtime: %v", err)
		} else if err := rt.Start(context.Background()); err != nil {
			log.Printf("Warning: failed to start browser runtime: %v", err)
		} else {
			browserRuntime = rt
			browserRegistry := browsertools.NewToolRegistry(rt)
			registry = browserRegistry
			browserTools = browserRegistry
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
	r := &campaignRunner{
		eng:            eng,
		camp:           camp,
		topoOrder:      topoOrder,
		runID:          runID,
		sessionStore:   sessionStore,
		lifecycleCtrl:  lifecycleCtrl,
		browserRuntime: browserRuntime,
		llmClient:      llmClient,
		traceStore:     traceStore,
		artifactStore:  artifactStore,
		urlRegex:       regexp.MustCompile(`https?://[^\s,"')\]]+`),
	}
	r.run(ctx)
}

type campaignRunner struct {
	eng            *engine.AgentEngine
	camp           *sharedtypes.Campaign
	topoOrder      []string
	runID          string
	sessionStore   *session.SessionStore
	lifecycleCtrl  *runtime.LifecycleController
	browserRuntime *browserruntime.BrowserRuntime
	llmClient      *llm.HTTPClient
	traceStore     *trace.TraceStore
	artifactStore  *artifact.ArtifactStore

	flowMap    map[string]sharedtypes.Flow
	flowDone   map[string]chan struct{}
	flowResults map[string]*engine.ExecutionResult
	flowStates map[string]*playwright.StorageState
	resultsMu  sync.Mutex
	pauseMu    sync.Mutex
	cancelled  bool
	urlRegex   *regexp.Regexp
}

func (r *campaignRunner) run(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateCancelling)
		return
	default:
	}

	_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateRunning)

	r.flowMap = make(map[string]sharedtypes.Flow)
	for _, flow := range r.camp.Flows {
		r.flowMap[flow.ID] = flow
	}

	parallelLimit := r.camp.Config.ParallelLimit
	if parallelLimit <= 0 {
		parallelLimit = 1
	}

	r.flowDone = make(map[string]chan struct{})
	for _, flowID := range r.topoOrder {
		r.flowDone[flowID] = make(chan struct{})
	}

	r.flowResults = make(map[string]*engine.ExecutionResult)
	r.flowStates = make(map[string]*playwright.StorageState)

	r.resumeCompletedFlows()

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, parallelLimit)

	for _, flowID := range r.topoOrder {
		flow, exists := r.flowMap[flowID]
		if !exists {
			close(r.flowDone[flowID])
			continue
		}
		r.resultsMu.Lock()
		_, alreadyDone := r.flowResults[flowID]
		r.resultsMu.Unlock()
		if alreadyDone {
			close(r.flowDone[flowID])
			continue
		}
		wg.Add(1)
		go r.executeFlow(ctx, &wg, semaphore, flowID, flow)
	}

	wg.Wait()
	r.finalizeRun()
}

func (r *campaignRunner) resumeCompletedFlows() {
	existingSess, err := r.sessionStore.Get(r.runID)
	if err != nil {
		return
	}
	for _, fs := range existingSess.Flows {
		if !isFlowComplete(fs.Status) {
			continue
		}
		var outcome engine.FlowOutcome
		switch fs.Status {
		case sharedtypes.FlowStatePassed:
			outcome = engine.OutcomePass
		case sharedtypes.FlowStateFailed:
			outcome = engine.OutcomeFail
		default:
			outcome = engine.OutcomeSkip
		}
		r.resultsMu.Lock()
		r.flowResults[fs.FlowID] = &engine.ExecutionResult{FlowID: fs.FlowID, Outcome: outcome}
		r.resultsMu.Unlock()
	}
}

func (r *campaignRunner) waitForPause() bool {
	backoff := 200 * time.Millisecond
	const maxBackoff = 2 * time.Second
	for {
		status, _ := r.sessionStore.Get(r.runID)
		if status == nil {
			return true
		}
		if status.Status == sharedtypes.RunStateCancelling || status.Status == sharedtypes.RunStateCancelled {
			r.pauseMu.Lock()
			r.cancelled = true
			r.pauseMu.Unlock()
			return true
		}
		if status.Status == sharedtypes.RunStatePausing || status.Status == sharedtypes.RunStatePaused {
			if status.Status == sharedtypes.RunStatePausing {
				_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStatePaused)
			}
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		if status.Status == sharedtypes.RunStateResuming || status.Status == sharedtypes.RunStateRunning {
			_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateRunning)
			return false
		}
		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (r *campaignRunner) buildDependencyContext(flowID string) string {
	flow, exists := r.flowMap[flowID]
	if !exists || len(flow.DependsOn) == 0 {
		return ""
	}
	var parts []string
	for _, depID := range flow.DependsOn {
		depFlow, depExists := r.flowMap[depID]
		if !depExists {
			continue
		}
		r.resultsMu.Lock()
		depResult, hasResult := r.flowResults[depID]
		r.resultsMu.Unlock()
		if !hasResult || depResult.Outcome != engine.OutcomePass {
			continue
		}
		urls := r.urlRegex.FindAllString(depFlow.Goal, -1)
		if len(urls) > 0 {
			parts = append(parts, fmt.Sprintf("Upstream flow '%s' navigated to %s", depID, strings.Join(urls, ", ")))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func (r *campaignRunner) executeFlow(ctx context.Context, wg *sync.WaitGroup, semaphore chan struct{}, fid string, f sharedtypes.Flow) {
	defer wg.Done()
	defer close(r.flowDone[fid])

	for _, depID := range f.DependsOn {
		depChan, depExists := r.flowDone[depID]
		if !depExists {
			continue
		}
		<-depChan
	}

	if r.checkCancelled(fid) {
		return
	}

	if r.checkUpstreamFailed(fid, f) {
		return
	}

	if r.checkContextCancelled(ctx, fid) {
		return
	}

	if r.waitForPause() {
		_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return
	}

	if r.checkCancelled(fid) {
		return
	}

	if !r.acquireSemaphore(ctx, semaphore, fid) {
		return
	}
	defer func() { <-semaphore }()

	if r.checkCancelled(fid) {
		<-semaphore
		return
	}

	depCtx := r.buildDependencyContext(fid)
	result := r.runFlowEngine(fid, f, depCtx)

	if result.Outcome == engine.OutcomePass {
		_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStatePassed, "")
	} else if result.Outcome == engine.OutcomeFail {
		r.markDownstreamSkipped(fid)
	}
}

func (r *campaignRunner) checkCancelled(fid string) bool {
	r.pauseMu.Lock()
	isCancelled := r.cancelled
	r.pauseMu.Unlock()
	if isCancelled {
		_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return true
	}
	return false
}

func (r *campaignRunner) checkUpstreamFailed(fid string, f sharedtypes.Flow) bool {
	for _, depID := range f.DependsOn {
		r.resultsMu.Lock()
		depResult, hasResult := r.flowResults[depID]
		r.resultsMu.Unlock()
		if hasResult && (depResult.Outcome == engine.OutcomeSkip || depResult.Outcome == engine.OutcomeFail) {
			_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStateSkippedUpstream, sharedtypes.ErrUpstreamFailed)
			r.resultsMu.Lock()
			r.flowResults[fid] = &engine.ExecutionResult{FlowID: fid, Outcome: engine.OutcomeSkip}
			r.resultsMu.Unlock()
			return true
		}
	}
	return false
}

func (r *campaignRunner) checkContextCancelled(ctx context.Context, fid string) bool {
	select {
	case <-ctx.Done():
		r.pauseMu.Lock()
		r.cancelled = true
		r.pauseMu.Unlock()
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateCancelling)
		_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return true
	default:
		return false
	}
}

func (r *campaignRunner) acquireSemaphore(ctx context.Context, semaphore chan struct{}, fid string) bool {
	select {
	case semaphore <- struct{}{}:
		return true
	case <-ctx.Done():
		r.pauseMu.Lock()
		r.cancelled = true
		r.pauseMu.Unlock()
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateCancelling)
		_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return false
	}
}

func (r *campaignRunner) runFlowEngine(fid string, f sharedtypes.Flow, depCtx string) *engine.ExecutionResult {
	var flowEngine *engine.AgentEngine
	var flowBrowser *browserruntime.FlowBrowserRuntime

	if r.browserRuntime != nil {
		var upstreamState *playwright.StorageState
		for _, depID := range f.DependsOn {
			r.resultsMu.Lock()
			state, hasState := r.flowStates[depID]
			r.resultsMu.Unlock()
			if hasState {
				upstreamState = state
				break
			}
		}

		fb, err := r.browserRuntime.NewFlowRuntime(upstreamState)
		if err != nil {
			result := &engine.ExecutionResult{FlowID: fid, Outcome: engine.OutcomeFail, Errors: []string{fmt.Sprintf("failed to create flow browser: %v", err)}}
			r.resultsMu.Lock()
			r.flowResults[fid] = result
			r.resultsMu.Unlock()
			_ = r.sessionStore.UpdateFlowState(r.runID, fid, sharedtypes.FlowStateFailed, err.Error())
			return result
		}
		flowBrowser = fb
		defer flowBrowser.Close()

		flowRegistry := browsertools.NewToolRegistryWithContext(fb, context.Background())
		flowEngine = engine.NewAgentEngineWithStores(flowRegistry, r.sessionStore, r.traceStore, r.artifactStore)
		if r.llmClient != nil {
			cliWrapper := llm.NewSimpleClientWithClient(r.llmClient)
			flowEngine.SetLLMClient(cliWrapper)
			flowEngine.SetBrowserTools(flowRegistry)
		}
		flowEngine.SetLifecycleController(r.lifecycleCtrl)
	} else {
		flowEngine = r.eng
	}

	flowEngine.SetDependencyContext(depCtx)
	retryLimit := f.Config.RetryLimit
	if retryLimit == 0 {
		retryLimit = r.camp.Config.RetryLimit
	}
	result := flowEngine.RunFlowWithRetry(r.runID, f, retryLimit)

	if flowBrowser != nil {
		if state, err := flowBrowser.StorageState(); err == nil && state != nil {
			r.resultsMu.Lock()
			r.flowStates[fid] = state
			r.resultsMu.Unlock()
		} else if err != nil {
			log.Printf("Failed to capture storage state for flow %s: %v", fid, err)
		}
	}

	r.resultsMu.Lock()
	r.flowResults[fid] = result
	r.resultsMu.Unlock()

	return result
}

func (r *campaignRunner) markDownstreamSkipped(fid string) {
	for _, otherFlow := range r.camp.Flows {
		for _, dep := range otherFlow.DependsOn {
			if dep == fid {
				_ = r.sessionStore.UpdateFlowState(r.runID, otherFlow.ID, sharedtypes.FlowStateSkippedUpstream, sharedtypes.ErrUpstreamFailed)
				break
			}
		}
	}
}

func (r *campaignRunner) finalizeRun() {
	sess, err := r.sessionStore.Get(r.runID)
	if err != nil || sess == nil {
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateFailed)
		return
	}

	if sess.Status == sharedtypes.RunStateCancelling || sess.Status == sharedtypes.RunStateCancelled {
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateCancelled)
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
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateFailed)
	} else {
		_ = r.sessionStore.UpdateStatus(r.runID, sharedtypes.RunStateCompleted)
	}
}
