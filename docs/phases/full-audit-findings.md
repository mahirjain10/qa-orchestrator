# Full Audit Findings — End-to-End Inconsistency Report

**Date:** 2026-05-21
**Scope:** Entire codebase — source files, campaign YAMLs, docs, logs, build artifacts
**Total Issues:** 43

---

## Parallel Execution Groups

Issues are grouped into **8 independent work streams**. Each group can be assigned to a different agent and executed in parallel — no group depends on another. Within each group, issues are ordered by severity.

---

## Group A — Data Corruption & Runtime Bugs

*Agents: 1 | Risk: HIGH | Can run in parallel with: ALL other groups*

These are bugs that cause incorrect behavior at runtime. Fixing them does not require changes to any other group.

### A1. `UpdateFlowState` modifies a copy, not the slice (CRITICAL)
- **Location:** `packages/shared/types/session.go:48-55`
- **Problem:** `s.Flows[i]` is a value copy in Go. Setting `s.Flows[i].StartedAt = &now` modifies the copy, not the original slice element. Flow state timestamps are never actually persisted.
- **Fix:** Use a pointer or reassign: `flow := &s.Flows[i]; flow.StartedAt = &now`
- **Files to change:** `packages/shared/types/session.go`
- **Tests to update:** `packages/shared/types/session_test.go`

### A2. `toolRegistry` nil in LLM engine constructor (CRITICAL)
- **Location:** `packages/agents/engine/engine.go:92-108`
- **Problem:** `NewAgentEngineWithLLM()` sets `registry` as a constructor parameter but never assigns it to `e.toolRegistry`. The field remains nil.
- **Fix:** Add `toolRegistry: registry,` to the struct literal.
- **Files to change:** `packages/agents/engine/engine.go`
- **Tests to update:** `packages/agents/engine/engine_test.go`

### A3. `Plan` type has data race (HIGH)
- **Location:** `packages/agents/types/agent.go:54-105`
- **Problem:** `historyCache`, `historyDirty`, `historyBuilt` are accessed without any mutex. Concurrent planner/engine access causes data race.
- **Fix:** Add `sync.RWMutex` to `Plan` struct; lock in `AddStep()`, `GetHistory()`, `Advance()`.
- **Files to change:** `packages/agents/types/agent.go`
- **Tests to update:** `packages/agents/types/agent_test.go`

### A4. `finalizeFlowState` doesn't distinguish skip types (HIGH)
- **Location:** `packages/agents/engine/engine.go:759-778`
- **Problem:** All skips map to `FlowStateSkippedUser`. No logic for `SKIPPED_UPSTREAM_FAILED` when upstream dependency fails.
- **Fix:** Add a parameter or context flag to distinguish user-skip vs upstream-fail-skip.
- **Files to change:** `packages/agents/engine/engine.go`

### A5. `waitForResume` busy-polls without debounce (HIGH)
- **Location:** `packages/agents/engine/engine.go:592-606`
- **Problem:** 500ms sleep loop hammering session store. No exponential backoff.
- **Fix:** Add backoff: start at 200ms, double each iteration up to 2s max.
- **Files to change:** `packages/agents/engine/engine.go`

### A6. `autonomousLLMContext` race on lifecycle replacement (HIGH)
- **Location:** `packages/agents/engine/engine.go:676-682`
- **Problem:** Goroutine reads `e.lifecycle.CancelCh()`. If `SetLifecycleController` replaces the lifecycle while goroutine runs, race occurs.
- **Fix:** Capture `cancelCh` reference before spawning goroutine, or use `sync.RWMutex` on `lifecycle` field.
- **Files to change:** `packages/agents/engine/engine.go`

---

## Group B — Campaign YAML Fixes

*Agents: 1 | Risk: HIGH | Can run in parallel with: ALL other groups*

Fixes to campaign YAML files and the parser. Independent of all code changes.

### B1. Missing `sample-guided.yaml` (CRITICAL)
- **Location:** `Makefile:18`, `README.md:77`
- **Problem:** `make run-guided` references `campaigns/sample-guided.yaml` which doesn't exist. The guided campaign is named `sample-campaign.yaml`.
- **Fix:** Rename `campaigns/sample-campaign.yaml` → `campaigns/sample-guided.yaml`. Update `Makefile` line 18 and `README.md` line 77 to reference the new name.
- **Files to change:** `campaigns/sample-campaign.yaml` (rename), `Makefile`, `README.md`

### B2. `sample-guided.yaml` missing `mode` + `priority` fields (CRITICAL)
- **Location:** `campaigns/sample-campaign.yaml` (will be `sample-guided.yaml` after B1)
- **Problem:** All 3 flows lack required `mode` and `priority` fields. Parser would reject this file.
- **Fix:** Add `mode: guided` and `priority: high` to `login-flow`, `priority: medium` to `dashboard-flow`, `priority: low` to `logout-flow`.
- **Files to change:** `campaigns/sample-guided.yaml`

### B3. Malformed YAML in `autonomous-campaign-01.yaml` (HIGH)
- **Location:** `campaigns/autonomous-campaign-01.yaml:12-14`
- **Problem:** Stray closing `"` on goal field, inconsistent indentation on multi-line value.
- **Fix:** Remove stray `"`, fix indentation to consistent 2-space continuation.
- **Files to change:** `campaigns/autonomous-campaign-01.yaml`

### B4. Empty `test-camgain-1.yaml` (MEDIUM)
- **Location:** `campaigns/test-camgain-1.yaml`
- **Problem:** Typo in filename ("camgain") + file is completely empty.
- **Fix:** Delete the file.
- **Files to change:** `campaigns/test-camgain-1.yaml` (delete)

---

## Group C — Parser & Validation Gaps

*Agents: 1 | Risk: HIGH | Can run in parallel with: ALL groups except B*

Adds missing validation to the campaign parser. Does not change any types.

### C1. Config fields never validated (HIGH)
- **Location:** `packages/orchestrator/campaign/parser.go:55-110`
- **Problem:** `config.timeout`, `config.retry_limit`, `config.parallel_limit` are required by architecture but parser never checks them.
- **Fix:** Add validation: timeout > 0, retry_limit >= 0, parallel_limit >= 1.
- **Files to change:** `packages/orchestrator/campaign/parser.go`
- **Tests to update:** `packages/orchestrator/campaign/parser_test.go`

### C2. `Flow.Config` never validated (HIGH)
- **Location:** `packages/shared/types/campaign.go:45-51`, `packages/orchestrator/campaign/parser.go`
- **Problem:** Per-flow `timeout` and `retry_limit` exist in type but parser ignores them.
- **Fix:** Validate per-flow config if present: timeout > 0, retry_limit >= 0.
- **Files to change:** `packages/orchestrator/campaign/parser.go`
- **Tests to update:** `packages/orchestrator/campaign/parser_test.go`

### C3. `Step.ID` uniqueness not validated (MEDIUM)
- **Location:** `packages/orchestrator/campaign/parser.go`
- **Problem:** Duplicate step IDs within a flow are not detected.
- **Fix:** Add step ID uniqueness check within each flow's step list.
- **Files to change:** `packages/orchestrator/campaign/parser.go`
- **Tests to update:** `packages/orchestrator/campaign/parser_test.go`

### C4. `Step.Tool` validity not validated (MEDIUM)
- **Location:** `packages/orchestrator/campaign/parser.go`
- **Problem:** No check that step tool names are non-empty.
- **Fix:** Validate `step.Tool != ""` for each step in guided flows.
- **Files to change:** `packages/orchestrator/campaign/parser.go`
- **Tests to update:** `packages/orchestrator/campaign/parser_test.go`

---

## Group D — Dead Code Removal

*Agents: 1 | Risk: LOW | Can run in parallel with: ALL other groups*

Remove unused functions, methods, and fields. Each deletion is independent.

### D1. `hexEncode` duplicated and unused
- **Location:** `packages/shared/types/trace.go:75` + `packages/storage/artifact/store.go:279`
- **Fix:** Delete both functions.
- **Files to change:** `packages/shared/types/trace.go`, `packages/storage/artifact/store.go`

### D2. `parseErrorResponse` never called
- **Location:** `packages/llm/client.go:208`
- **Fix:** Delete function.
- **Files to change:** `packages/llm/client.go`

### D3. Planner dead methods: `PlanFromFlow`, `Observe`, `GetPendingSteps`
- **Location:** `packages/agents/planner/planner.go:85,115,125`
- **Fix:** Delete all three methods.
- **Files to change:** `packages/agents/planner/planner.go`
- **Tests to update:** `packages/agents/planner/planner_test.go` (remove any tests for these)

### D4. `handleSteeringEvent` never called
- **Location:** `packages/agents/engine/engine.go:609`
- **Fix:** Delete function.
- **Files to change:** `packages/agents/engine/engine.go`

### D5. `RegisterTool` only works with MockToolRegistry
- **Location:** `packages/agents/engine/engine.go:522`
- **Fix:** Delete function.
- **Files to change:** `packages/agents/engine/engine.go`

### D6. Lifecycle dead methods: `WaitForPause/Resume/Cancel`, `SteerCh`, `RequestInput`, `InputCh`, `SetCompleted`, `SetFailed`
- **Location:** `packages/runtime/lifecycle.go:144-231`
- **Fix:** Delete all eight methods.
- **Files to change:** `packages/runtime/lifecycle.go`
- **Tests to update:** `packages/runtime/lifecycle_test.go`

### D7. `Context()` never called
- **Location:** `packages/browser-runtime/runtime.go:149`
- **Fix:** Delete method.
- **Files to change:** `packages/browser-runtime/runtime.go`

### D8. Artifact store dead methods: `GetRecentPaths`, `GetArtifactPaths`, `SaveArtifact[T]`
- **Location:** `packages/storage/artifact/store.go:202,283,301`
- **Fix:** Delete all three methods.
- **Files to change:** `packages/storage/artifact/store.go`
- **Tests to update:** `packages/storage/artifact/store_test.go`

### D9. `GetReportPath` never called
- **Location:** `packages/reporting/reporter.go:252`
- **Fix:** Delete function.
- **Files to change:** `packages/reporting/reporter.go`

### D10. Guided mode replan path is dead code
- **Location:** `packages/agents/engine/engine.go:237-242`
- **Problem:** `RecoveryActionReplan` case downgrades to retry with a comment. The replan path is unreachable for guided flows.
- **Fix:** Remove the `RecoveryActionReplan` case block entirely (it's a no-op that falls through to retry).
- **Files to change:** `packages/agents/engine/engine.go`

---

## Group E — Architecture & Docs Alignment

*Agents: 1 | Risk: LOW | Can run in parallel with: ALL other groups*

Update documentation to match reality, or vice versa. No code logic changes.

### E1. Tool name mismatches with architecture.md
- **Location:** `docs/architecture.md:312-323`
- **Problem:** Architecture says `click_element(locator)`, code uses `click(selector)`. Architecture says `take_screenshot()`, code uses `screenshot`.
- **Fix:** Update architecture.md to match actual tool names: `click`, `screenshot`.
- **Files to change:** `docs/architecture.md`

### E2. Tools in code but not in architecture
- **Location:** `docs/architecture.md:312-323`
- **Problem:** `get_html`, `evaluate`, `finish`, `log`, `delay`, `assert_true`, `echo` exist in code but not documented.
- **Fix:** Add missing tools to architecture.md tool list with descriptions.
- **Files to change:** `docs/architecture.md`

### E3. Tools in architecture but not in code
- **Location:** `docs/architecture.md:312-323`
- **Problem:** `observe_ui`, `set_network_profile`, `fetch_test_data`, `fetch_otp_mock` listed in architecture but not implemented.
- **Fix:** Remove from architecture.md (or mark as "planned" in a future tools section).
- **Files to change:** `docs/architecture.md`

### E4. `FlowStateSkippedUser` + `FlowStateWaitingInput` not in architecture
- **Location:** `docs/architecture.md:383-391`, `packages/shared/types/campaign.go:92,95`
- **Problem:** Two flow states exist in code but not documented in architecture.md flow states list.
- **Fix:** Add `SKIPPED_USER` and `WAITING_FOR_INPUT` to architecture.md flow states section.
- **Files to change:** `docs/architecture.md`

### E5. `Flow.Name` vs `Flow.ID` confusion
- **Location:** `packages/shared/types/campaign.go:37-38`
- **Problem:** Both `ID` and `Name` exist on `Flow`, but only `ID` is used/populated by the parser and engine.
- **Fix:** Either remove `Name` field from `Flow` struct, or document that it's optional metadata. Recommended: keep it but add a comment `// optional display name, not used for identification`.
- **Files to change:** `packages/shared/types/campaign.go`, `docs/architecture.md`

### E6. `ToolInfo`/`ParameterInfo` duplicated across packages
- **Location:** `packages/llm/prompts.go:43,49` + `packages/browser-runtime/tools/registry.go:12,18`
- **Problem:** Same structs defined in two packages. Adapter at `packages/agents/tools/adapter.go` converts between them.
- **Fix:** Move both structs to `packages/shared/types/` and have both packages import from there. Remove duplicates.
- **Files to change:** `packages/shared/types/tools.go` (new), `packages/llm/prompts.go`, `packages/browser-runtime/tools/registry.go`, `packages/agents/tools/adapter.go`

### E7. `LLMClient` vs `Client` interface mismatch
- **Location:** `packages/agents/planner/planner.go:12` vs `packages/llm/client.go:16`
- **Problem:** Completely different signatures, bridged by undocumented `SimpleClient` adapter.
- **Fix:** Add documentation comment to `SimpleClient` explaining the adapter pattern. Consider consolidating interfaces in a future refactor.
- **Files to change:** `packages/llm/client.go` (add docs)

---

## Group F — State Machine & Lifecycle Consistency

*Agents: 1 | Risk: MEDIUM | Can run in parallel with: ALL groups except A*

Fixes to state transitions and lifecycle logic.

### F1. `CanResume()` inconsistent with handler
- **Location:** `packages/runtime/lifecycle.go:70` vs `apps/tui/internal/screens/handlers.go:44`
- **Problem:** Lifecycle `CanResume()` only allows transition from `PAUSED`, but handler allows `PAUSED` OR `PAUSING`.
- **Fix:** Update `CanResume()` to also allow `PAUSING` state.
- **Files to change:** `packages/runtime/lifecycle.go`
- **Tests to update:** `packages/runtime/lifecycle_test.go`

### F2. `CanPause()` allows double-pause
- **Location:** `packages/runtime/lifecycle.go:64`
- **Problem:** `CanPause()` allows `RUNNING` or `PENDING`, but no guard prevents pause from `PAUSING` state (double-pause).
- **Fix:** Add `PAUSING` to the exclusion list in `CanPause()`.
- **Files to change:** `packages/runtime/lifecycle.go`
- **Tests to update:** `packages/runtime/lifecycle_test.go`

### F3. Errors silently swallowed in stores
- **Location:** `packages/storage/artifact/store.go:194,258`, `packages/storage/trace/store.go:191`, `packages/reporting/reporter.go:68-69`
- **Problem:** Delete errors, rename errors, unmarshal errors, and store lookup errors are all silently discarded.
- **Fix:** Return errors or log them properly with zerolog.
- **Files to change:** `packages/storage/artifact/store.go`, `packages/storage/trace/store.go`, `packages/reporting/reporter.go`

### F4. `BrowserRuntime.Start` ignores context
- **Location:** `packages/browser-runtime/runtime.go:58`
- **Problem:** Accepts `ctx` but never uses it for cancellation.
- **Fix:** Use `ctx` for playwright launch timeout or pass to relevant operations.
- **Files to change:** `packages/browser-runtime/runtime.go`

### F5. `Navigate` has no context
- **Location:** `packages/browser-runtime/runtime.go:175`
- **Problem:** Can't be cancelled mid-navigation.
- **Fix:** Add `ctx context.Context` parameter to `Navigate()` and pass to playwright.
- **Files to change:** `packages/browser-runtime/runtime.go`, `packages/browser-runtime/tools/registry.go`

---

## Group G — Cleanup & Housekeeping

*Agents: 1 | Risk: LOW | Can run in parallel with: ALL other groups*

File system cleanup, no code logic changes.

### G1. Run log gaps
- **Location:** `logs/runs/2026-05/`
- **Problem:** Missing logs for runs 012, 014, 035, 040-056, 060 — 41 logs vs 62 summaries.
- **Fix:** Create placeholder log entries for missing runs explaining the gap, OR delete orphaned summaries. Recommended: create minimal placeholder logs.
- **Files to change:** `logs/runs/2026-05/run-012.jsonl`, `logs/runs/2026-05/run-014.jsonl`, etc.

### G2. `cmd.exe` artifacts in root
- **Location:** Project root
- **Problem:** Stray Windows executables, not in `.gitignore`.
- **Fix:** Delete `cmd.exe` and `cmd.exe~`. Add `*.exe` to `.gitignore` if not present.
- **Files to change:** `cmd.exe` (delete), `cmd.exe~` (delete), `.gitignore`

### G3. Empty `page/` directory
- **Location:** `packages/browser-runtime/page/`
- **Problem:** Placeholder directory with no implementation.
- **Fix:** Delete the directory.
- **Files to change:** `packages/browser-runtime/page/` (delete directory)

---

## Group H — Hardcoded Values & Config

*Agents: 1 | Risk: LOW | Can run in parallel with: ALL other groups*

Make hardcoded values configurable. Each fix is independent.

### H1. `maxAutonomousSteps` hardcoded
- **Location:** `packages/agents/engine/engine.go:304`
- **Problem:** `maxAutonomousSteps := 20` — not configurable.
- **Fix:** Add to `FlowConfig` or `CampaignConfig`, with default of 20.
- **Files to change:** `packages/agents/engine/engine.go`, `packages/shared/types/campaign.go`

### H2. Retry backoff hardcoded
- **Location:** `packages/agents/engine/engine.go:499`
- **Problem:** `time.Sleep(time.Duration(100*(attempt+1)) * time.Millisecond)` — hardcoded.
- **Fix:** Use `recovery.DefaultPolicy.RetryDelayMs` or add configurable backoff constant.
- **Files to change:** `packages/agents/engine/engine.go`

### H3. Resume poll interval hardcoded
- **Location:** `packages/agents/engine/engine.go:597`
- **Problem:** `time.Sleep(500 * time.Millisecond)` — hardcoded.
- **Fix:** Extract to a constant `resumePollInterval = 500 * time.Millisecond` at package level.
- **Files to change:** `packages/agents/engine/engine.go`

### H4. TUI refresh ticker hardcoded
- **Location:** `apps/tui/internal/screens/commands.go:94`
- **Problem:** `2*time.Second` refresh ticker — hardcoded.
- **Fix:** Extract to constant `defaultRefreshInterval = 2 * time.Second`.
- **Files to change:** `apps/tui/internal/screens/commands.go`

### H5. TUI message timeout hardcoded
- **Location:** `apps/tui/internal/screens/main.go:617`
- **Problem:** `5*time.Second` message display timeout — hardcoded.
- **Fix:** Extract to constant `messageTimeout = 5 * time.Second`.
- **Files to change:** `apps/tui/internal/screens/main.go`

---

## Execution Order Recommendation

```
Phase 1 (Parallel): Groups A, B, C, D, G
  → Fix runtime bugs, YAML issues, validation gaps, dead code, and cleanup

Phase 2 (Parallel): Groups E, F, H
  → Align docs, fix state machine, extract hardcoded values

Phase 3: Full test suite + build verification
  → go test ./... && go build ./...
```

## Verification Checklist

After all groups complete:

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./...` all passing
- [ ] `make run-guided` works (campaign loads and runs)
- [ ] `make run-sample` works (autonomous campaign loads)
- [ ] No inline color values outside `style/theme.go`
- [ ] No dead code remains (run `go vet -unused` or manual check)
- [ ] `docs/architecture.md` matches actual implementation
- [ ] `docs/CURRENT.md` updated with audit fix summary
