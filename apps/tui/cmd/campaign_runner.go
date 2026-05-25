package main

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
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

type CampaignConfig struct {
	CampaignPath  string
	ResumeID      string
	BrowserMode   string
	Ctx           context.Context
	SessionStore  *session.SessionStore
	TraceStore    *trace.TraceStore
	ArtifactStore *artifact.ArtifactStore
	RunCreatedCh  chan string
	LifecycleCtrl *runtime.LifecycleController
}

type EngineConfig struct {
	SessionStore  *session.SessionStore
	TraceStore    *trace.TraceStore
	ArtifactStore *artifact.ArtifactStore
	LLMClient     *llm.HTTPClient
	BrowserMode   string
	LifecycleCtrl *runtime.LifecycleController
}

type CampaignRunConfig struct {
	Ctx            context.Context
	Eng            *engine.AgentEngine
	Camp           *sharedtypes.Campaign
	TopoOrder      []string
	RunID          string
	SessionStore   *session.SessionStore
	LifecycleCtrl  *runtime.LifecycleController
	BrowserRuntime *browserruntime.BrowserRuntime
	LLMClient      *llm.HTTPClient
	TraceStore     *trace.TraceStore
	ArtifactStore  *artifact.ArtifactStore
}

func startCampaign(cfg CampaignConfig) error {
	parser := campaign.NewCampaignParser()
	parsed, err := parser.ParseFile(cfg.CampaignPath)
	if err != nil {
		return fmt.Errorf("parsing campaign file %s: %w", cfg.CampaignPath, err)
	}
	camp := parsed.Campaign

	var runID string

	if cfg.ResumeID != "" {
		existingSession, err := cfg.SessionStore.Get(cfg.ResumeID)
		if err != nil {
			return fmt.Errorf("loading session %s for resume: %w", cfg.ResumeID, err)
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
		if err := cfg.SessionStore.Save(existingSession); err != nil {
			return fmt.Errorf("updating resumed session %s: %w", runID, err)
		}
	} else {
		sess, err := cfg.SessionStore.Create(camp)
		if err != nil {
			return fmt.Errorf("creating new session: %w", err)
		}
		runID = sess.RunID
	}

	select {
	case cfg.RunCreatedCh <- runID:
	default:
	}

	if cfg.LifecycleCtrl != nil {
		cfg.LifecycleCtrl.SetRunID(runID)
	}

	llmClient, err := createLLMClientForCampaign(camp)
	if err != nil {
		log.Printf("Warning: %v", err)
		log.Printf("Autonomous flows will fail without LLM client.")
	}

	agentEngine, browserRuntime, err := createAgentEngine(EngineConfig{
		SessionStore:  cfg.SessionStore,
		TraceStore:    cfg.TraceStore,
		ArtifactStore: cfg.ArtifactStore,
		LLMClient:     llmClient,
		BrowserMode:   cfg.BrowserMode,
		LifecycleCtrl: cfg.LifecycleCtrl,
	})
	if err != nil {
		return fmt.Errorf("creating agent engine: %w", err)
	}

	go func() {
		if browserRuntime != nil {
			defer browserRuntime.Stop()
		}
		runCampaignWithContext(CampaignRunConfig{
			Ctx:            cfg.Ctx,
			Eng:            agentEngine,
			Camp:           camp,
			TopoOrder:      parsed.TopologicalOrder,
			RunID:          runID,
			SessionStore:   cfg.SessionStore,
			LifecycleCtrl:  cfg.LifecycleCtrl,
			BrowserRuntime: browserRuntime,
			LLMClient:      llmClient,
			TraceStore:     cfg.TraceStore,
			ArtifactStore:  cfg.ArtifactStore,
		})
	}()

	return nil
}

func createAgentEngine(cfg EngineConfig) (*engine.AgentEngine, *browserruntime.BrowserRuntime, error) {
	var agentEngine *engine.AgentEngine
	var browserRuntime *browserruntime.BrowserRuntime
	var registry executor.ToolRegistry = executor.NewMockToolRegistry()
	var browserTools interface {
		ListToolsWithDocs() []browsertools.ToolInfo
	}

	if cfg.BrowserMode == "real" {
		rt, err := browserruntime.NewBrowserRuntime(nil)
		if err != nil {
			return nil, nil, fmt.Errorf("real browser init failed: %w", err)
		}
		if err := rt.Start(context.Background()); err != nil {
			return nil, nil, fmt.Errorf("real browser start failed: %w", err)
		}
		browserRuntime = rt
		browserRegistry := browsertools.NewToolRegistry(rt)
		registry = browserRegistry
		browserTools = browserRegistry
	}

	if cfg.LLMClient != nil {
		cliWrapper := llm.NewSimpleClientWithClient(cfg.LLMClient)
		agentEngine = engine.NewAgentEngineWithLLM(
			registry,
			cfg.SessionStore,
			cfg.TraceStore,
			cfg.ArtifactStore,
			cliWrapper,
			browserTools,
		)
	} else {
		agentEngine = engine.NewAgentEngineWithStores(
			registry,
			cfg.SessionStore,
			cfg.TraceStore,
			cfg.ArtifactStore,
		)
	}

	if cfg.LifecycleCtrl != nil {
		agentEngine.SetLifecycleController(cfg.LifecycleCtrl)
	}

	return agentEngine, browserRuntime, nil
}

func runCampaignWithContext(cfg CampaignRunConfig) {
	r := &campaignRunner{
		eng:            cfg.Eng,
		camp:           cfg.Camp,
		topoOrder:      cfg.TopoOrder,
		runID:          cfg.RunID,
		sessionStore:   cfg.SessionStore,
		lifecycleCtrl:  cfg.LifecycleCtrl,
		browserRuntime: cfg.BrowserRuntime,
		llmClient:      cfg.LLMClient,
		traceStore:     cfg.TraceStore,
		artifactStore:  cfg.ArtifactStore,
		urlRegex:       regexp.MustCompile(`https?://[^\s,"')\]]+`),
	}
	r.run(cfg.Ctx)
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

	flowMap     map[string]sharedtypes.Flow
	flowDone    map[string]chan struct{}
	flowResults map[string]*engine.ExecutionResult
	flowStates  map[string]*playwright.StorageState
	resultsMu   sync.Mutex
	pauseMu     sync.Mutex
	cancelled   bool
	urlRegex    *regexp.Regexp
}

func (r *campaignRunner) updateRunStatus(_ context.Context, status sharedtypes.RunState) {
	if err := r.sessionStore.UpdateStatus(r.runID, status); err != nil {
		log.Printf("campaignRunner: failed to update run status to %s: %v", status, err)
	}
}

func (r *campaignRunner) updateFlowState(_ context.Context, flowID string, state sharedtypes.FlowState, errMsg string) {
	if err := r.sessionStore.UpdateFlowState(r.runID, flowID, state, errMsg); err != nil {
		log.Printf("campaignRunner: failed to update flow state run=%s flow=%s state=%s: %v", r.runID, flowID, state, err)
	}
}

func (r *campaignRunner) run(ctx context.Context) {
	select {
	case <-ctx.Done():
		r.updateRunStatus(ctx, sharedtypes.RunStateCancelling)
		return
	default:
	}

	r.updateRunStatus(ctx, sharedtypes.RunStateRunning)

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
	r.finalizeRun(ctx)
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

func (r *campaignRunner) waitForPause(ctx context.Context) bool {
	backoff := 200 * time.Millisecond
	const maxBackoff = 2 * time.Second
	for {
		select {
		case <-ctx.Done():
			return true
		default:
		}

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
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		if status.Status == sharedtypes.RunStateResuming || status.Status == sharedtypes.RunStateRunning {
			r.updateRunStatus(ctx, sharedtypes.RunStateRunning)
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
		select {
		case <-depChan:
		case <-ctx.Done():
			return
		}
	}

	if r.checkCancelled(ctx, fid) {
		return
	}

	if r.checkUpstreamFailed(ctx, fid, f) {
		return
	}

	if r.checkContextCancelled(ctx, fid) {
		return
	}

	if r.waitForPause(ctx) {
		r.updateFlowState(ctx, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return
	}

	if r.checkCancelled(ctx, fid) {
		return
	}

	if !r.acquireSemaphore(ctx, semaphore, fid) {
		return
	}
	defer func() { <-semaphore }()

	if r.checkCancelled(ctx, fid) {
		return
	}

	depCtx := r.buildDependencyContext(fid)
	result := r.runFlowEngine(ctx, fid, f, depCtx)

	switch result.Outcome {
	case engine.OutcomePass:
		r.updateFlowState(ctx, fid, sharedtypes.FlowStatePassed, "")
	case engine.OutcomeFail:
		r.markDownstreamSkipped(ctx, fid)
	}
}

func (r *campaignRunner) checkCancelled(ctx context.Context, fid string) bool {
	r.pauseMu.Lock()
	isCancelled := r.cancelled
	r.pauseMu.Unlock()
	if isCancelled {
		r.updateFlowState(ctx, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return true
	}
	return false
}

func (r *campaignRunner) checkUpstreamFailed(ctx context.Context, fid string, f sharedtypes.Flow) bool {
	for _, depID := range f.DependsOn {
		r.resultsMu.Lock()
		depResult, hasResult := r.flowResults[depID]
		r.resultsMu.Unlock()
		if hasResult && (depResult.Outcome == engine.OutcomeSkip || depResult.Outcome == engine.OutcomeFail) {
			r.updateFlowState(ctx, fid, sharedtypes.FlowStateSkippedUpstream, sharedtypes.ErrUpstreamFailed)
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
		r.updateRunStatus(ctx, sharedtypes.RunStateCancelling)
		r.updateFlowState(ctx, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
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
		r.updateRunStatus(ctx, sharedtypes.RunStateCancelling)
		r.updateFlowState(ctx, fid, sharedtypes.FlowStateSkippedUpstream, "cancelled")
		return false
	}
}

func (r *campaignRunner) runFlowEngine(ctx context.Context, fid string, f sharedtypes.Flow, depCtx string) *engine.ExecutionResult {
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
			r.updateFlowState(ctx, fid, sharedtypes.FlowStateFailed, err.Error())
			return result
		}
		flowBrowser = fb
		defer flowBrowser.Close()

		flowRegistry := browsertools.NewToolRegistryWithContext(fb, ctx)
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

func (r *campaignRunner) markDownstreamSkipped(ctx context.Context, fid string) {
	for _, otherFlow := range r.camp.Flows {
		for _, dep := range otherFlow.DependsOn {
			if dep == fid {
				r.updateFlowState(ctx, otherFlow.ID, sharedtypes.FlowStateSkippedUpstream, sharedtypes.ErrUpstreamFailed)
				break
			}
		}
	}
}

func (r *campaignRunner) finalizeRun(_ context.Context) {
	if err := r.sessionStore.FinalizeRun(r.runID); err != nil {
		log.Printf("campaignRunner: failed to finalize run %s: %v", r.runID, err)
	}
}

func hasAutonomousFlow(camp *sharedtypes.Campaign) bool {
	for _, flow := range camp.Flows {
		if flow.Mode == sharedtypes.FlowModeAutonomous {
			return true
		}
	}
	return false
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

	// Capability Check
	hasReasoningSupport := strings.Contains(strings.ToLower(cfg.Model), "o1") ||
		strings.Contains(strings.ToLower(cfg.Model), "o3") ||
		strings.Contains(strings.ToLower(cfg.Model), "deepseek-reasoner") ||
		strings.Contains(strings.ToLower(cfg.Model), "gemini-2.0-flash-thinking")

	log.Printf("=== LLM Configuration ===")
	log.Printf("  Provider:  %s", providerName)
	log.Printf("  Model:     %s", cfg.Model)
	log.Printf("  Endpoint:  %s", providerEndpoint)
	if cfg.ThinkingBudget > 0 && !hasReasoningSupport {
		log.Printf("  ⚠ WARNING: Thinking budget requested but model %s may not support reasoning/thinking features.", cfg.Model)
	}
	log.Printf("========================")

	client, err := llm.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("Campaign contains autonomous flows but LLM configuration failed: %w", err)
	}

	return client, nil
}

func isFlowComplete(status sharedtypes.FlowState) bool {
	return status == sharedtypes.FlowStatePassed ||
		status == sharedtypes.FlowStateFailed ||
		status == sharedtypes.FlowStateSkippedUpstream ||
		status == sharedtypes.FlowStateSkippedUser
}

func findFlowState(sess *sharedtypes.Session, flowID string) *sharedtypes.FlowRunState {
	for i := range sess.Flows {
		if sess.Flows[i].FlowID == flowID {
			return &sess.Flows[i]
		}
	}
	return nil
}
