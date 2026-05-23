# Agentic Recovery & Semantic UI Healing (Gen 3) - Phased Implementation Plan

> **Status:** Draft
> **Scope:** Add semantic recovery and 404 handling to the autonomous agent loop, simulating Gen-3 agentic QA capabilities.

---run_20260522_be1422   

## Critical Rules for Implementing Agents
*(Derived from `docs/phases/TUI_revamp_phases.md` and `docs/phases/full-audit-findings.md`)*

1. **Read this document fully before starting any phase.**
2. **Each phase must compile successfully before the next begins.** (`go build ./...`)
3. **Run existing tests after each phase. Fix any breakage.** (`go test ./...`)
4. **Do NOT skip phases or merge scopes.**
5. **After completing a phase, update `docs/CURRENT.md` and write a run summary following `docs/LOG_CONVENTIONS.md`.**
6. **Watch out for concurrency:** (As seen in Group A of audit findings), ensure any state changes (like appending observations) are done safely if accessed across goroutines, though the agent loop is mostly sequential per-flow.

---

## Phase 0 — Save Plan to Repository

**Goal:** Persist this plan in the repository for other agents.

**Files affected:**
- CREATE `docs/phases/agentic_recovery.md`

**What to do:**
1. Copy the contents of this plan file into `docs/phases/agentic_recovery.md` so that future agents can read it and execute the phases.

---

## Phase 1 — Update Autonomous Planner Prompts

**Goal:** Authorize the LLM to navigate to the root domain and explore if the initial URL is invalid (404/Not Found).

**Files affected:**
- MODIFY `packages/llm/prompts.go` (or where the autonomous system prompt is defined)

**What to do:**
1. Locate the system prompt for the Autonomous Planner.
2. Add a new explicit rule under the `## URL Context` or `## Goal` section:
   * "If the Current Observation indicates a 404, 'Page Not Found', or an invalid URL, do not immediately fail. You are authorized to extract the root domain from the current URL, use the `navigate(root_domain)` tool, and use `observe_ui()` to find the correct path to achieve your Goal."

**Verification:**
- `go build ./...` succeeds.

---

## Phase 2 — Update Validator to Flag 404s

**Goal:** Ensure the engine flags 404 pages in the observation so the LLM knows the navigation failed semantically, even if the HTTP request technically succeeded.

**Files affected:**
- MODIFY `packages/agents/validator/validator.go`
- MODIFY `packages/browser-runtime/tools/registry.go` (specifically the `observe_ui` script)

**What to do:**
1. In the `observe_ui` JS script or the Validator's post-navigation checks, look for common 404 indicators (e.g., page title contains "404" or "Not Found", or specific known UI elements indicating a missing page).
2. If a 404 is detected, prepend or append a clear warning to the observation string: `⚠️ WARNING: Page appears to be a 404 or error page.`

**Verification:**
- `go build ./...` succeeds.
- `go test ./packages/agents/validator/...` passes.

---

## Phase 3 — Update the Recovery Agent

**Goal:** Intercept invalid URL failures early and issue a Steering Instruction rather than failing the flow immediately.

**Files affected:**
- MODIFY `packages/agents/recovery/recovery.go`

**What to do:**
1. In the `Recover()` (or equivalent) method, add a specific check for when a step fails immediately after a `navigate` tool call due to a missing element or timeout, OR if the validator explicitly flagged a 404.
2. If this condition is met, instead of returning `RecoveryActionFail`, return `RecoveryActionReplan` (or `RecoveryActionRetry` with a steering instruction).
3. Inject the following steering instruction into the flow context (`ctx.SteeringInstructions`):
   * `"The requested URL was invalid or returned a 404. Navigate to the base domain and attempt to reach the goal by clicking through the UI."`

**Verification:**
- `go build ./...` succeeds.
- `go test ./packages/agents/recovery/...` passes.

---

## Phase 4 — Testing & Validation (E2E)

**Goal:** Prove the system can recover from a bad URL in a campaign YAML.

**Test Case:**
1. Run `campaigns/failure-cascade.yaml` (or run a specific flow targeted at `/practice-test-students/` which is known to be broken).
2. **Expected Trace Sequence:**
   - `navigate(https://practicetestautomation.com/practice-test-students/)`
   - `observe_ui()` -> returns 404 warning
   - Planner/Validator detects issue.
   - Recovery Agent intercepts and issues steering instruction.
   - Planner issues `navigate(https://practicetestautomation.com/)`
   - Planner issues `observe_ui()`
   - Planner finds the "Students" link and issues `click(...)`
   - Flow eventually succeeds instead of hard failing.

**Verification:**
- Run the TUI and watch the trace panel for the semantic recovery loop.