# Current Project State

## Original MVP Phases (Completed)
- **Phase 1:** Core Runtime (Parsers, Validators, Storage)
- **Phase 2:** TUI Shell (Live UI, Navigation, Lifecycle controls)
- **Phase 3:** Agent Loop (Planner, Executor, Validator, Recovery)
- **Phase 4:** Browser Automation & Tools (Playwright integration, Tool Registry)
- **Phase 5:** Trace and Artifact Pipeline (TraceStore, ArtifactStore, UI panels)
- **Phase 6:** Steering and lifecycle controls (Wait states, Steering commands)
- **Phase 7:** Final reporting (Campaign Summary generation, Markdown/Terminal export)

## V2 Phases (Autonomous Upgrade)
- **Phase 1:** Hybrid Schema & Validation — COMPLETE
- **Phase 2:** LLM Integration Package — COMPLETE
- **Phase 3:** Iterative Autonomous Planner — COMPLETE
- **Phase 4:** Tool Registry & Metadata — COMPLETE (hardened)
- **Phase 5:** Agent Engine V2 (Mode Routing) — COMPLETE (hardened)
- **Phase 6:** TUI & Visibility — COMPLETE (hardened)
- **Phase 7:** Validation & Sample Campaigns — COMPLETE

## TUI Versions

| Version | Runs | Work | Status |
|---------|------|------|--------|
| V1 TUI | MVP Phase 2 | Initial TUI Shell | ✅ Complete |
| V2 TUI | Runs 026-030 | Bug fixes (CLI, visual, navigation) | ✅ Complete |
| V3 TUI | Runs 031-039 | New additions (slot layout, pane management) | ✅ Complete |
| V4 TUI | Runs 040+ | TUI revamp (sidebar+main, async, viewport) | 🔄 In Progress |

## Audit Fix Phases (End-to-End Consistency)

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Runtime Bugs + YAML + Parser + Dead Code + Cleanup (27 issues) | ✅ COMPLETE |
| 2 | Architecture/Docs + State Machine + Hardcoded Values (16 issues) | ✅ COMPLETE |

**Audit Fix Phase 2 Completed (Run 064):**
- Group E: Aligned architecture.md with actual tools (renamed, added, removed). Added missing flow states. Added TODO comments for duplicated structs. Documented SimpleClient adapter pattern.
- Group F: Fixed CanResume() to allow PAUSING. Added double-pause test. Fixed silently swallowed errors in artifact store, trace store, reporter. Added context checks to BrowserRuntime.Start() and Navigate().
- Group H: Extracted 5 hardcoded values into constants/config: MaxAutonomousSteps, retryBackoffBaseMs, resumePollInitialDelay/MaxDelay, defaultRefreshInterval, messageDisplayTimeout.

**Audit Fix Phase 1 Completed (Run 063):**
- Group A: Fixed data corruption in `UpdateFlowState`, added mutex to `Plan`, distinguished skip types, exponential backoff for resume, captured cancelCh reference.
- Group B: Renamed sample-guided.yaml, added mode/priority fields, fixed malformed YAML, deleted empty file.
- Group C: Added config validation, flow config validation, step ID uniqueness, step tool validation. 10 new tests.
- Group D: Removed 10 groups of dead code (30+ functions/methods) across 8 packages.
- Group G: Created 21 placeholder logs, deleted cmd.exe artifacts, removed empty page/ directory.

## V4 Phases (TUI Revamp — Current)

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Design System Unification | ✅ COMPLETE |
| 2 | Utility Functions | ✅ COMPLETE |
| 3 | State Architecture Refactor | ✅ COMPLETE |
| 4 | Async Event System (Replace Polling) | ✅ COMPLETE |
| 5 | Layout System: Sidebar + Main Content | ✅ COMPLETE |
| 6 | Dashboard View | ✅ COMPLETE |
| 7 | Viewport Integration for Traces | ✅ COMPLETE |
| 8 | Flows View with Detail Panel | ✅ COMPLETE |
| 9 | Trace Filtering | ✅ COMPLETE |
| 10 | Status Bar + Contextual Help | ✅ COMPLETE |
| 11 | Campaign Selection Modal | ✅ COMPLETE |
| 12 | Responsive Behavior | ✅ COMPLETE |
| 13 | Goroutine Shutdown + Clean Exit | ✅ COMPLETE |
| 14 | Visual Polish + Final Cleanup | ✅ COMPLETE |

**V4 Phase 1 Completed:**
- Run 040: Created unified design system in `style/theme.go`. Eliminated 136 lines of duplicate style definitions across 5 component files. All colors and Lip Gloss styles now come from single source of truth. Tests pass, binary builds successfully.

**V4 Phase 2 Completed:**
- Run 041: Created `util/truncate.go` with 6 utility functions (Truncate, TruncateStart, TruncateMiddle, SafeWidth, FormatDuration, FormatDurationShort). Added 47 test cases. Replaced all inline truncation logic across 4 component files. Removed `truncatePath()` function. Tests pass, binary builds successfully.

**V4 Phase 3 + 4 Completed:**
- Run 042: Removed `state` package entirely (AppState with mutexes). Merged domain state into `MainScreen` struct. Replaced 1-second polling with async commands (`fetchSessionsCmd`, `fetchRunCmd`, `fetchTracesCmd`, `fetchArtifactsCmd`, `fetchReportCmd`). Added 26 new tests for async message/command architecture. Total 30 tests in screens package. Ticker changed to 2s, only active when run selected. Tests pass, binary builds successfully.
- Run 043: Verified Phase 3+4 implementation integrity. Added 66 new tests covering key handling, steering commands, parsing, View rendering, constructors, edge cases. Total 96 tests in screens package. All passing, build clean, vet clean.

**V4 Phase 5 Completed:**
- Run 046: Replaced 2x2 quadrant layout with sidebar+main content system. Removed `ComponentID` type, `quadrants`, `activeSlot`, `maximized`, `maximizedSlot` fields. Added `View` type enum (`Dashboard`, `Flows`, `Traces`, `Report`) and `sidebarFocus` boolean. Rewrote `View()` method with `renderSidebar()`, `renderMainContent()`, and view-specific renderers. Replaced slot-based key bindings (tab, left/right, 0-3, p, w, m) with view switching (1-4, tab for focus toggle, up/down context-aware). Removed 18 old quadrant/slot tests, added 8 new sidebar/view tests. All 22 packages pass, build clean, vet clean.

**V4 Phase 6 Completed:**
- Run 047: Implemented Dashboard View. Added `util` import, replaced local `utilSafeWidth()` with `util.SafeWidth()`, replaced inline truncation with `util.Truncate()`. Verified `renderDashboardView()`, `renderRunSummary()`, and `renderFlowTimeline()` work correctly. All styles use `style.*`, all truncation uses `util.*`. Added 8 new tests. All packages pass, build clean.

**V4 Phase 7 Completed:**
- Run 048: Integrated `bubbles/viewport` into trace panel. Replaced `maxEvents` truncation with viewport scrolling. Added `Viewport` and `FollowTail` fields to `TracePanelModel`. Added `updateViewportContent()` with column headers and reversed event rows. Added `Update()` method for viewport scroll keys (pgup/pgdown/home/end). Added `f` key to toggle follow-tail mode. Updated `renderTracesView()` to use `Viewport.View()`. All packages pass, build clean.

**V4 Phase 8 Completed:**
- Run 049: Implemented Flows View with expandable detail panel. Added `Expanded` bool and `viewport` to `FlowStatusModel`. Updated `renderFlowsView()` with responsive table using `util.SafeWidth()` and `util.Truncate()`. Added `renderFlowDetail()` showing Started, Finished, Duration, Retries, Error. Added `enter` key to toggle expand/collapse, `left`/`h` to collapse. Added `contentWidth()` helper. Added 18 new tests. All packages pass, build clean.

**V4 Phase 9 Completed:**
- Run 050: Implemented Trace Filtering. Added `TraceFilter` struct with `Text`, `ShowFailed`, `FlowID`, `EventType` fields. Added `Filter`, `FilterMode`, `FilterInput` to `TracePanelModel`. Created `FilteredEvents()` method for text, failed-only, flow ID, and event type filtering. Added `/` key to enter filter mode, `S` key to toggle failures-only filter. ESC cancels filter mode, Enter applies filter text. Updated all view methods to use filtered events. Added 13 new tests. All packages pass, build clean.

**V4 Phase 10 Completed:**
- Run 051: Replaced static footer with contextual status bar. Enhanced `renderStatusBar()` with run status badge + truncated ID on left, contextual keys on right, message line above bar. Added `contextualKeys()` returning view-specific key hints. Hides on terminals < 20 rows. Uses `style.BgDark`, `style.Dim`, `style.Msg`. Added 12 new tests. All packages pass, build clean, vet clean.

**V4 Phase 11 Completed:**
- Run 053: Implemented Campaign Selection Modal. Refactored `renderCampaignSelector()` to use `util.SafeWidth()` for modal width, removed centering logic from selector. Moved centering to `renderDashboardView()` using `m.contentWidth()` for proper content-area centering. Added 10 new tests covering title, empty state, campaign list, selected indicator, width constraints, centering, navigation hints, and modal border. All packages pass, build clean, vet clean.

**V4 Phase 12 Completed:**
- Run 054: Implemented Responsive Behavior. Added hard minimum terminal size checks with styled error messages (`style.StatusFailed`). Added `sidebarWidth()` helper (24/20/16 based on terminal width). Refactored `contentWidth()` to use adaptive sidebar width. Added `contentHeight()` helper (reduced by 2 rows on short terminals). Updated `View()` and `renderTracesView()` to use adaptive helpers. Added 19 new tests covering narrow/short terminals, zero size, adaptive widths, boundary conditions, and error messages. Updated 2 existing tests. All packages pass, build clean, vet clean.

**V4 Phase 13 Completed:**
- Run 055: Implemented Goroutine Shutdown + Clean Exit. Added `cancelFunc context.CancelFunc` field to MainScreen. Added `SetCancelFunc()` method. Updated 'x' key handler to call cancelFunc after CancelRun. Updated `cmd/main.go` to use `context.WithCancel`, pass cancelFunc to mainScreen, and cleanup on TUI exit. Renamed `runCampaign` to `runCampaignWithContext` with `select` on `ctx.Done()`. Updates session status to `Cancelled` on cancellation. Added 5 new tests. All packages pass, build clean, vet clean.

**V4 Phase 14 Completed:**
- Run 056: Visual Polish + Final Cleanup audit. Verified no inline color values outside `style/theme.go`. Confirmed all dead code removed (state package, quadrant fields, old footer). Verified consistent padding/borders across all panels. Vet clean, no unused imports, no formatting issues. All 22 packages pass, 170 tests in screens package. V4 TUI Revamp COMPLETE.

### Test Coverage
- `go test ./...` — passing (170 tests in screens package, 47 in util, 12 in style)

## Bug Fixes
- Run 072: Fixed autonomous planner selector hallucination — 4-layer defense implemented: (1) Code-level enforce_observe_ui after every navigate() and before every retry, injected as real steps in plan history. (2) Recovery agent now distinguishes Playwright selector timeouts (→ Replan) from generic/network timeouts (→ Retry). (3) Pre-execution selector existence check via fast JS querySelector() — turns 30s timeouts into sub-second failures for nonexistent selectors. (4) User prompt reformatted with observation data at bottom under aggressive "USE ONLY THESE SELECTORS" header. Added observation_context trace event before each planner call to verify observation reaches LLM. 5 files modified, 2 new tests. 22 packages pass, build clean.
- Run 071: Fixed 5 bugs from code review: critical type mismatch in formatObserveUIObservation (map[string]any vs string), test-production gap in mock return type, autoObserve called for guided flows (unnecessary overhead), observe_ui error silently swallowed, unbounded observation growth (capped at 10).

## V3 Phases (Steering + TUI State Sync)

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | TUI State Sync Fixes (4 bugs) | ✅ COMPLETE |
| 2 | Steering Instructions for Autonomous Flows | ✅ COMPLETE |

**V3 Phase 1 Completed:**
- **1.1** — New sessions auto-select via `runCreatedMsg` + channel bridge from `startCampaign` goroutine to TUI. Fixed bug where TUI showed old session state after starting new campaign.
- **1.2** — Steering commands (`retry`, `skip`, `pause`, `resume`, `approve`, `continue`) now return `fetchRunCmd` for instant display refresh instead of waiting for 2s ticker.
- **1.3** — Spacebar pause/resume returns refresh Cmd.
- **1.4** — Cancel key (`x`) returns refresh Cmd.

**V3 Phase 2 Completed:**
- **2.1-2.2** — Added `SteerInstruction` type + `Instruction` field to `SteeringEvent` (`packages/shared/types/steering.go`).
- **2.3** — Added `SteeringInstructions []string` to `ExecutionContext` (`packages/agents/types/agent.go`).
- **2.4** — TUI `steer <text>` command submits to lifecycle controller (`apps/tui/internal/screens/main.go`).
- **2.5** — Lifecycle controller wired through `cmd/main.go` → engine → TUI.
- **2.6** — Autonomous loop drains steering events between steps, caps at 5 most recent (`packages/agents/engine/engine.go`).
- **2.7** — Steering instructions appended to LLM user prompt with `IMPORTANT` label (`packages/agents/planner/planner.go`).
- **2.8** — `steer` added to command bar auto-discovery + help overlay.

## Autonomous URL Context Fixes (Run 073)

**Goal:** Fix 5 systematic failures in autonomous flow execution caused by missing URL context and failure invisibility to the LLM.

**Approach:** ChatGPT-recommended `start_url` architecture — deterministic config field separate from natural-language goal.

### Changes (6 phases, build+test after each)

| Phase | What | Files |
|-------|------|-------|
| 1 | Schema + Types — `StartURL` on Flow + ExecutionContext, `CurrentURL` tracked after navigate | `campaign.go`, `agent.go`, `engine.go` |
| 2 | Prompt Injection — URL context + failure context (`⚠ RECENT FAILURE`) in LLM user prompt | `prompts.go`, `planner.go` |
| 3 | YAML Updates — `start_url` added to 3 campaigns (6 autonomous flows) | `large-parallel.yaml`, `mixed-mode-campaign.yaml`, `sample-autonomous.yaml` |
| 4 | Safety-Nets — repeat detection + observe_ui loop detection with steering instruction injection | `engine.go`, `agent.go` |
| 5 | Campaign Test Runner — parse+validate all 10 campaigns, validate start_url format | `campaign_integration_test.go` |
| 6 | Documentation — run log, summary, CURRENT.md | `run-073.jsonl`, `run-073.md` |

**Test Results:** `go test ./...` — all test suites PASS. All 10 campaign YAMLs parse and validate.

### Code Review (Run 073)

Adversarial code review of all changed files found and fixed 4 bugs:

| Severity | Bug | Fix |
|----------|-----|-----|
| 🟡 Logic | Redundant failure context in LLM prompt (duplicate error text) | Removed `lastObs.Error` append from observation — only `scanForRecentFailure` prepend remains |
| 🟡 Logic | Double `Advance(plan)` + `stepCount++` for injected observe steps | Removed Advance from `injectObserveStep` caller — main loop handles it |
| 🟡 Logic | Empty tool name (`tool=`) in failure message when `LastStep` is nil | Changed to `tool=?` |
| 🤦 Stupid | Missing `fmt` import in planner test file | Added `"fmt"` to imports |

**12 new tests added:**
- 7 `TestScanForRecentFailure_*` — nil observations, success-only, error with tool, error with nil LastStep, step failure, step failure no error, finds most recent, multiple failures
- 5 `TestStepSignature_*` — deterministic, order-independent, empty params, nil step, different tools

## Autonomous Engine Hardening (Run 074)

**Goal:** Fix two bugs from real browser run — `max_steps_reached` not setting `OutcomeFail`, and repeat detection lacking hard-break.

### Bugs Fixed

| Bug | Root Cause | Fix |
|-----|-----------|-----|
| max_steps_reached → PASSED | Line 561 only emitted trace, never set `result.Outcome` or `result.Errors`. Default `OutcomePass` was never overridden. | Set `OutcomeFail` + error string in the `stepCount >= maxSteps` block |
| Repeat detection ∞ | Always `continue`d with steering — no counter, no threshold, no hard-break. LLM regenerated same step 18+ times. | Added `consecutiveRepeats` counter. Hard-break (`OutcomeFail` + `goto done`) at 3 repeats |

### Changes (8 lines in `engine.go`, 2 new tests in `engine_test.go`)
- `consecutiveRepeats` counter declared and reset on non-repeat steps
- Hard-break block: `if consecutiveRepeats >= 3 { OutcomeFail; goto done }`
- `max_steps_reached` block: now sets `OutcomeFail` + `"max autonomous steps reached without finishing"`
- 2 new tests verify both failure modes produce `OutcomeFail` with distinct errors
- `go test ./...` — all test suites PASS, build clean

## Agentic Recovery & Semantic UI Healing (Gen 3) — NEW

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Update Autonomous Planner Prompts (system prompt 404 rule) | ✅ COMPLETE |
| 2 | Update Observer to Flag 404s (formatObserveUIObservation warning) | ✅ COMPLETE |
| 3 | Update Recovery Agent (has404Warning + 404-first check in Decide) | ✅ COMPLETE |
| 4 | Testing & Validation (6 new tests) | ✅ COMPLETE |

**Run 077 completed:** Added 404 recovery rule to system prompt URL RULES. Fixed formatObserveUIObservation to surface 404 warnings to the LLM. Added has404Warning() detection in recovery.Decide() — 404 observations cause RecoveryActionRetry + steering instruction to navigate to root domain, prioritized before all other error checks. 6 new tests added. All 22 packages pass, build clean, vet clean.

## Performance & Stability Enhancements (Run 078)

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | High-Performance Deep Cloning (Replace JSON) | ✅ COMPLETE |
| 2 | Dynamic React Sync (WaitForFunction instead of static sleep) | ✅ COMPLETE |
| 3 | Password Redaction Bug (Empty field observation fix) | ✅ COMPLETE |

**Run 078 completed:** Identified severe UI freezing caused by TUI polling `SessionStore.List()` which used `json.Marshal` inside a global lock to deep clone large historical sessions. Built a custom native Go struct memory cloner `Session.Clone()`. Removed static 250ms sleep in `Fill()` and replaced it with a dynamic Playwright `WaitForFunction` to perfectly sync with React/Vue DOM reconciliations. Fixed `observeUIJS` to report empty strings for empty password fields instead of always `********`. All tests pass cleanly.

## Last Run
- Run 109: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed Makefile: removed undefined $(BROWSER_MODE) variable from run-real target, made clean target cross-platform (Windows cmd vs Unix rm -f). All 24 test suites pass, build clean, vet clean.
- Run 108: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed T6 (2 GetRunStatus error discards in processSteeringCommand continue/approve — already in working tree, added tests) and T11 (filter textinput stale focus state — added Blur() calls in enter/escape branches). 4 new tests, 3 updated. All 22 suites pass.
- Run 107: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed 4 bugs: (L10) fallback dedup — filter req.Model out of FallbackModels. (L11) DefaultRetryConfig converted from mutable var to value-returning function; CalculateDelay to value receiver. (L13) Gemini BuildRequest now checks ReasoningEffort and maps to reasoningConfig. (T9) cancelled re-checked after semaphore acquire to close TOCTOU race window. All 22 suites pass.
- Run 106: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed 4 browser-runtime bugs: (S8) `withPageRecreation` added to `BrowserRuntime` for all 8 ops. (S9) `Stop()`/`Close()` nil out fields. (S11) `observe_ui` Evaluate errors captured. (S12) `json.Number` in `matchesType`. Added 5 new tests: Stop nil-out, Close nil-out, all ops fail-when-not-running, matchesType with json.Number (9 sub-cases), 2 Evaluate error sub-cases in 404 detection. All 22 test suites pass.
- Run 105: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed TUI bugs T11 and T12 from bug-report-sunday.md: (T11) stale `cmd` from `FilterInput.Update()` captured before `SetValue("")` — restructured to call Update once after all state mutations. (T12) `handleContentUp`/`handleContentDown` changed selection but never called `UpdateViewportContent()` or scrolled viewport — added both, selection stays centered. All 22 test suites pass, build clean.
- Run 104: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed TUI bugs T4 and T5 from bug-report-sunday.md: (T4) ANSI-styled status column misaligned in Flows view — `lipgloss.Width` + manual padding replaces raw `%-*s` Sprintf width. (T5) Context leak on campaign startup error — `campaignCancel()` now called when `startCampaign` returns error. All 22 test suites pass, TUI builds clean.
- Run 103: 2026-05-25 (Agent: opencode)
  - Status: COMPLETE. Fixed 2 bugs from bug-report-sunday.md: (Bug A) Engine BUG 5 — pause-before-steering drain order in `runAutonomousFlow` and `runGuidedFlow` — `drainSteeringEvents` now called before `handlePauseResume`. (Bug B) config.go L19 — `envAllowFallbacks` constant alignment. Added `TestGuidedFlow_SteeringSkipBeforePause_ProcessedFirst` verifying steering events are consumed before pause blocks (skip step, flow completes with `OutcomePass`). All 18 test suites pass, build clean.
- Run 102: 2026-05-24 (Agent: opencode)
  - Status: COMPLETE. Fixed 7 Storage/Runtime/Browser bugs: S1 (sanitizeID unified), S2 (ListRunIDs mutex), S3 (artifact Save rollback), S4 (removed overflowDropped), S5 (runWithContext drain goroutine removed), S6 (ensurePage isRunning check), S7 (FlowBrowserRuntime page recovery). All affected packages pass, build clean.
- Run 101: 2026-05-24 (Agent: opencode)
  - Status: COMPLETE. Fixed 5 LLM package bugs: (1) removed parseTimeout wrapper, (2) MaxRetries capped at 20, (3) ApplyProviderSettings merges instead of overwrites, (4) renamed LLM_PROVIDER_ALLOW → LLM_ALLOW_FALLBACKS, (5) body read error now retryable. 7 new tests. All llm tests pass, build clean, vet clean.
- Run 100: 2026-05-24 (Agent: opencode)
  - Status: COMPLETE. Fixed 4 LLM package bugs: (B1) Auto BaseURL resolved after provider detection, no longer defaults to OpenRouter URL when model is Gemini; (B2) ThinkingBudget parse error no longer silently ignored — returns error like maxRetries; (B3) Gemini BuildRequest now passes TopP and StopSequences from request; (B4) NewSimpleClient now uses apiKey parameter even when LoadConfig succeeds. 24/24 test suites pass, build clean.
- Run 099: 2026-05-24 (Agent: opencode)
  - Status: COMPLETE. Fixed 5 orchestrator bugs: (O1) flow timeout validation changed to `< 0` (0 = inherit from campaign); (O2) topologicalSort length check in Validate; (O3) FormatError nil guard; (O4) DependencyError.Error() includes flow ID and detail; (O5) config-block-existence check before field-level errors. 24/24 test suites pass, build clean.
  - Status: COMPLETE. Fixed 7 code-review bugs: (E1) cross-flow steering infinite loop — already had filter, verified by test. (E2) handlePauseResume session deletion — returns pauseFail. (E3) finalizeRunResult no longer demotes OutcomePass on errors. (E4) autonomousLLMContext goroutine leak — added `lifecycleCancel` field, `Close()` method, `defer llmCancel()`. (P5-7) Planner locking and dangling pointer — all properly locked, value copy safe. 24/24 test suites pass (0 pre-existing failures), build clean.
- Run 094: 2026-05-24 (Agent: opencode)
  - Status: COMPLETE. Fixed `runWithContext` goroutine leak — added channel-drain goroutine on `ctx.Done()` path so background `fn()` goroutine completes cleanly instead of remaining in-flight. 24/24 test suites pass, build clean.
- Run 093: 2026-05-24 (Agent: opencode)
  - Status: Bug Fixes COMPLETE. Fixed 11 code-review bugs across 3 packages: (B1) `runWithContext` goroutine leak fixed via context-aware `fn` signature + channel drain. (B2-3) `checkSelectorExists` returns real errors; `wait_for` no longer defeats dynamic element detection. (B4-6) Sanitization hardened — `sanitizeGoal` escapes `</`, `sanitizeDOM` escapes `"`, `\`, `\n`, `[`, `]`, steering instructions now sanitized. (B7) `BaseURL` wired into all providers. (B8) `IsNetworkError`/`IsAPIError` unwrap `*RetryableError`. (B9-11) Fallback models route to correct provider endpoint; `Generate` clones request to prevent mutation; dead `retryConfig` removed. 23/24 test suites pass (1 pre-existing engine test failure out of scope), build clean.
- Run 092: 2026-05-23 (Agent: Gemini CLI)
  - Status: Reliability & Edge Cases COMPLETE. Fixed 5 critical campaign reliability failures (API timeouts, broken retry logic, uncontrolled step repetition, JSON parsing failures, state staleness) and 2 edge cases regarding the LLM "thinking" budget configuration (Gemini gate and MaxTokens cap). All 24 test suites pass, build clean.
- Run 091: 2026-05-23 (Agent: Gemini CLI)
  - Status: DeepSeek/GPT-5 Integration COMPLETE. Added `reasoning_effort` and `thinking` parameters to LLM payload, configured OpenRouter pass-through, and repaired provider test suite. All tests pass, build clean.
- Run 062: 2026-05-20 (Agent: Gemini CLI)
  - Status: TUI Bug Fixes COMPLETE. Wired `pause`, `resume`, `approve` to steering handlers. Fixed spacebar handling for `WAITING_FOR_INPUT`. Removed dead code (`renderHelpModal`). All tests passing.
- Run 061: 2026-05-20 (Agent: Gemini CLI)
  - Status: TUI Incremental Fixes COMPLETE. Replaced legacy input with `CommandBarModel` for command auto-discovery, overhauled polling loop with `time.Ticker` in `cmd/main.go`, updated tests. All packages pass, build clean.
- Run 060: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: Phase 1 — CLI and Logging Infrastructure COMPLETE. Log redirection to logs/app.log, --resume/-r flag, session resumption logic, permanent command bar replacing steering modal. All 22 packages pass, build clean, vet clean.
- Run 059: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Debug Fix Phase 3 COMPLETE — Real-Time Ticker Updates. Fixed startRefreshTicker() to always return tea.Cmd (never nil), self-perpetuating ticker. Added fetch-on-select trigger. 8 new tests. All 22 packages pass, build clean, vet clean. ALL THREE DEBUG FIX PHASES COMPLETE.
- Run 058: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Debug Fix Phase 2 COMPLETE — Session Age Distinction. Added formatSessionAge(), current run highlighted with green dot and appears first, previous sessions sorted newest-to-oldest with separator line. 13 new tests. All 22 packages pass, build clean, vet clean.
- Run 057: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Debug Fix Phase 1 COMPLETE — Keyboard Interactivity. Added global key pass-through (q/ctrl+c/?), implemented ? help modal overlay with contextual keys per view. 13 new tests. All 22 packages pass, build clean, vet clean.
- Run 056: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 14 COMPLETE — Visual Polish + Final Cleanup. Audit passed, no changes needed. All 22 packages pass, 170 tests. V4 TUI Revamp COMPLETE.
- Run 055: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 13 COMPLETE — Goroutine Shutdown + Clean Exit. Added context cancellation, updated cmd/main.go. 5 new tests added. All 22 packages pass.
- Run 054: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 12 COMPLETE — Responsive Behavior. Added adaptive sidebar width, content height, hard minimum size checks. 19 new tests added. All 22 packages pass.
- Run 053: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 11 COMPLETE — Campaign Selection Modal. Refactored selector to use util.SafeWidth, moved centering to dashboard view. 10 new tests added. All 22 packages pass.
- Run 052: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: Code review of Phases 5-10 — 3 bugs found and fixed (nil pointer panic in renderFlowDetail, double-width paused character, content width inconsistency). 3 new tests added. All 22 packages pass.
- Run 051: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 10 COMPLETE — Status Bar + Contextual Help. All packages pass, build clean, vet clean.
- Run 050: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 9 COMPLETE — Trace Filtering. All packages pass, build clean.
- Run 049: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 8 COMPLETE — Flows View with expandable detail panel. All packages pass, build clean.
- Run 048: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 7 COMPLETE — Viewport Integration for Traces. All packages pass, build clean.
- Run 047: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 6 COMPLETE — Dashboard View implemented. All packages pass, build clean.
- Run 046: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 5 COMPLETE — Replaced 2x2 quadrant layout with sidebar+main content. All 22 packages pass, build clean, vet clean.
- Run 045: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: All 4 flagged issues fixed — RunFlow result used, View() side effect removed, space bar gap closed, custom itoa replaced. All 22 packages pass.
- Run 044: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: Windows TempDir race fixed, code review completed. 3 bugs fixed, 4 flagged. All 22 packages pass, build clean, vet clean.
- Run 043: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 3+4 VERIFIED — 96 tests in screens package (was 30), all passing. Build clean, vet clean.
- Run 042: 2026-05-20 (Agent: qwen3.6-plus-free)
  - Status: V4 Phase 3+4 COMPLETE — Removed `state` package, merged state into `MainScreen`, replaced polling with async commands. 30 tests in screens package, all passing.

## Makefile (Updated)
- Added `run-sample` and `run-guided` targets for quick testing
- Added `help` target for available commands
- Enhanced `run` target documentation
- Added `fmt`, `vet`, `lint`, and `verify` workflow targets
- Updated `clean` to be cross-platform (Windows `del /Q`, Unix `rm -f`)
- Removed undefined `$(BROWSER_MODE)` variable from `run-real` target
- Added reasoning/thinking env vars to `check-env` (`LLM_REASONING_EFFORT`, `LLM_THINKING_TYPE/BUDGET`, `LLM_HTTP_REFERER`, `LLM_APP_TITLE`)
- Added provider routing env vars to `check-env` (`LLM_PROVIDER_PRIORITY`, `LLM_PROVIDER_ONLY`, `LLM_ALLOW_FALLBACKS`)
- Updated usage examples with DeepSeek V4 and GPT-5-Mini configuration patterns
