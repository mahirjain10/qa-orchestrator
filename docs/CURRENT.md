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

## V4 Phases (TUI Revamp — Current)

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | Design System Unification | ✅ COMPLETE |
| 2 | Utility Functions | ✅ COMPLETE |
| 3 | State Architecture Refactor | ✅ COMPLETE |
| 4 | Async Event System (Replace Polling) | ✅ COMPLETE |
| 5 | Layout System: Sidebar + Main Content | ✅ COMPLETE |
| 6 | Dashboard View | ⏳ Pending |
| 7 | Viewport Integration for Traces | ⏳ Pending |
| 8 | Flows View with Detail Panel | ⏳ Pending |
| 9 | Trace Filtering | ⏳ Pending |
| 10 | Status Bar + Contextual Help | ⏳ Pending |
| 11 | Campaign Selection Modal | ⏳ Pending |
| 12 | Responsive Behavior | ⏳ Pending |
| 13 | Goroutine Shutdown + Clean Exit | ⏳ Pending |
| 14 | Visual Polish + Final Cleanup | ⏳ Pending |

**V4 Phase 1 Completed:**
- Run 040: Created unified design system in `style/theme.go`. Eliminated 136 lines of duplicate style definitions across 5 component files. All colors and Lip Gloss styles now come from single source of truth. Tests pass, binary builds successfully.

**V4 Phase 2 Completed:**
- Run 041: Created `util/truncate.go` with 6 utility functions (Truncate, TruncateStart, TruncateMiddle, SafeWidth, FormatDuration, FormatDurationShort). Added 47 test cases. Replaced all inline truncation logic across 4 component files. Removed `truncatePath()` function. Tests pass, binary builds successfully.

**V4 Phase 3 + 4 Completed:**
- Run 042: Removed `state` package entirely (AppState with mutexes). Merged domain state into `MainScreen` struct. Replaced 1-second polling with async commands (`fetchSessionsCmd`, `fetchRunCmd`, `fetchTracesCmd`, `fetchArtifactsCmd`, `fetchReportCmd`). Added 26 new tests for async message/command architecture. Total 30 tests in screens package. Ticker changed to 2s, only active when run selected. Tests pass, binary builds successfully.
- Run 043: Verified Phase 3+4 implementation integrity. Added 66 new tests covering key handling, steering commands, parsing, View rendering, constructors, edge cases. Total 96 tests in screens package. All passing, build clean, vet clean.

**V4 Phase 5 Completed:**
- Run 046: Replaced 2x2 quadrant layout with sidebar+main content system. Removed `ComponentID` type, `quadrants`, `activeSlot`, `maximized`, `maximizedSlot` fields. Added `View` type enum (`Dashboard`, `Flows`, `Traces`, `Report`) and `sidebarFocus` boolean. Rewrote `View()` method with `renderSidebar()`, `renderMainContent()`, and view-specific renderers. Replaced slot-based key bindings (tab, left/right, 0-3, p, w, m) with view switching (1-4, tab for focus toggle, up/down context-aware). Removed 18 old quadrant/slot tests, added 8 new sidebar/view tests. All 22 packages pass, build clean, vet clean.

### Test Coverage
- `go test ./...` — passing (96 tests in screens package, 47 in util, 12 in style)

## Bug Fixes
- Run 038: Fixed tool registry mismatch — `getDefaultLLMTools()` listed `get_html` and `evaluate` which `MockToolRegistry` didn't have, causing "unknown tool" → "configuration error" failures in autonomous mode.
- Run 039: Fixed autonomous planner infinite loop — LLM generated 20 redundant verification steps because `finish` tool was never exposed in its prompt. Engine already handled `finish` at line 338 but LLM didn't know it existed.

## Last Run
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
- Updated `clean` to be PowerShell-safe on Windows
