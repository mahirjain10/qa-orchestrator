# TUI Debug & Fix Phases (V4)

> **Status:** Draft — awaiting approval
> **Created:** 2026-05-20
> **Scope:** Fix three categories of bugs discovered in the current TUI implementation
> **Source:** User-reported issues + codebase analysis

---

## Problem Summary

Three distinct bug categories were identified:

| # | Symptom | Root Cause |
|---|---------|------------|
| 1 | Keyboard shortcuts (help, steering, etc.) don't work — no interactivity regardless of workflow state | handleMainKey swallows keys in steering/filter mode; no help modal exists; keys are view-scoped but the code doesn't route them correctly when content is focused |
| 2 | No visual distinction between new (current session) and old (previous sessions) runs — all appear identical in the campaign selector | renderCampaignSelector and campaignNames() render all sessions uniformly; Session.StartedAt exists but is never used for visual differentiation |
| 3 | Reports, flows, and traces don't update in real-time — require manual refresh (r) | startRefreshTicker returns nil when runID is empty at init time; ticker never re-queues after the first tick in some code paths; tickMsg handler doesn't always re-schedule the ticker |

---

## Phase 1 — Fix Keyboard Interactivity & Global Key Routing

### Problem

The TUI has three mutually exclusive key-handling modes:
1. **Filter mode** (tracePanel.FilterMode) — only filter input receives keys
2. **Steering mode** (steeringMode) — only steering input receives keys
3. **Main mode** (handleMainKey) — all other keys

The problem: when in filter or steering mode, **global keys** like help, quit, view switch, and TAB (focus toggle) are completely blocked. The user is trapped in that mode with no escape except ESC.

Additionally, the help key is referenced in contextualKeys() but has no handler anywhere in the codebase.

### Root Cause Analysis

**File:** apps/tui/internal/screens/main.go

Lines 180-195 (Update method):
- handleFilterKey (lines 222-244) and handleSteeringKey (lines 203-220) only handle enter and escape. They do NOT pass through global keys.
- handleMainKey (lines 246-382) has no case for the help key.

### Fix

1. **Add a global key pass-through** before mode-specific handlers. Keys like quit and help should always work regardless of mode.

2. **Implement help modal** that overlays the current view and shows all available keys for the current context.

3. **Allow view switch keys to work** even in steering/filter mode (exit the mode first, then switch).

### Implementation Plan

**Add to MainScreen struct:**
- showHelp bool

**In Update(), BEFORE mode checks:**
- Check for global keys (quit, help) first
- Then proceed to mode-specific handlers

**Add help modal rendering:**
- Contextual help based on activeView and current mode
- Render as centered overlay

**Update View()** to render help modal when showHelp is true

**Update handleFilterKey and handleSteeringKey** to allow quit keys to pass through

### Verification
- Help toggles help overlay from any view, any mode
- Quit works from any mode
- View switch keys work when not in steering/filter mode
- Steering and filter modes still work correctly

---

## Phase 2 — Session Age Distinction (New vs Previous)

### Problem

When the campaign selector renders, all sessions appear identical. There is no visual indicator of which session is from the **current run** vs which are **historical/previous runs**.

### Root Cause Analysis

**File:** apps/tui/internal/screens/main.go — renderCampaignSelector()

The selector iterates over m.sessions and renders each with only the campaign name and RunID. The Session struct has StartedAt and UpdatedAt fields, but they are never used for rendering.

### Fix

1. **Highlight the current run** in the campaign selector with a distinct indicator.
2. **Show session age** (e.g., "just now", "2m ago", "1h ago", "yesterday") next to each session.
3. **Add a visual separator** between current-session runs and historical runs.

### Implementation Plan

**Add utility function** for human-readable time formatting

**Update renderCampaignSelector():**
- Sort sessions: current run first, then by StartedAt descending
- Render current session first with special indicator (bright green dot)
- Add separator line between current and previous
- Render previous sessions with age labels

**Update campaignNames()** to include age info and [CURRENT] marker

### Verification
- Current session is highlighted and appears first
- Previous sessions are separated by a divider line
- Each session shows relative age
- Sessions are sorted: current first, then newest-to-oldest

---

## Phase 3 — Fix Real-Time Updates for Traces, Flows, and Reports

### Problem

Traces, flows, and reports do not update automatically during campaign execution. The user must press 'r' to manually refresh.

### Root Cause Analysis

**File:** apps/tui/internal/screens/commands.go — startRefreshTicker()

The bug chain:
1. At startup, m.currentRun is nil, so currentRunID() returns empty string.
2. The first tickMsg fires after 2 seconds.
3. runID is empty, so startRefreshTicker returns nil.
4. **The ticker is never re-scheduled.** No more ticks will ever fire.
5. Even when a run is later selected, the ticker is already dead.

### Fix

1. **Always re-schedule the ticker** regardless of runID. The ticker should be self-perpetuating.
2. **Only fetch data when runID is valid** — but always keep the ticker alive.
3. **Add a fetch-on-select trigger** — when a session is selected from the campaign selector, immediately trigger a refresh.

### Implementation Plan

**In commands.go:**
- Replace startRefreshTicker to not require runID parameter — always returns a tea.Cmd

**In main.go Init():**
- Call startRefreshTicker() without runID

**In tickMsg handler:**
- Always re-schedule the ticker after processing

**In enter key handler (campaign selection):**
- Immediately trigger a refresh for the selected run

### Verification
- Ticker fires every 2 seconds regardless of run state
- Traces update automatically during campaign execution
- Flows update automatically during campaign execution
- Reports update automatically
- Selecting a session triggers immediate data refresh
- No infinite loops or memory leaks from ticker

---

## Implementation Dependency Graph

`
Phase 1 (Keyboard Interactivity) — independent
Phase 2 (Session Age Distinction) — independent
Phase 3 (Real-Time Updates) — independent

All three phases can be implemented in parallel.
`

---

## Testing Checklist

- [ ] go build ./... succeeds
- [ ] go test ./apps/tui/... passes
- [ ] go vet ./... passes
- [ ] Help toggles help overlay from any view
- [ ] Quit works from any mode (steering, filter, main)
- [ ] Help modal shows contextual keys for current view
- [ ] Current session highlighted in campaign selector
- [ ] Session age displayed
- [ ] Previous sessions separated by divider line
- [ ] Traces auto-update every 2 seconds during campaign run
- [ ] Flows auto-update every 2 seconds during campaign run
- [ ] Reports auto-update every 2 seconds
- [ ] Selecting a session triggers immediate data refresh
- [ ] No memory leaks after extended run

---

## Files to Modify

| File | Phase 1 | Phase 2 | Phase 3 |
|------|---------|---------|---------|
| apps/tui/internal/screens/main.go | Yes | Yes | Yes |
| apps/tui/internal/screens/commands.go | | | Yes |

No new files need to be created. All changes are modifications to existing files.
