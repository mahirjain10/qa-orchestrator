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
| V3 TUI | Runs 031+ | New additions (slot layout, pane management) | 🔄 In Progress |

## V3 Phases (Current)

| Phase | Description | Status |
|-------|-------------|--------|
| 1 | OpenRouter Integration Hardening | ✅ COMPLETE |
| 2 | TUI Rendering Fixes | ✅ COMPLETE |
| 3 | TUI Slot-based Layout Engine (quadrant-based rendering) | ✅ COMPLETE |
| 4 | Interactive Pane Management (swap/maximize panes) | ✅ COMPLETE |
| 5 | Validation & Testing | ✅ COMPLETE |

**V3 Phase 1 Completed:**
- Run 031: Initial LLM fail-fast validation for autonomous campaigns.
- Run 033: Hardened LLM fail-fast validation so both `LLM_API_KEY` and `LLM_MODEL` are required before session creation.

**V3 Phase 2 Completed:**
- Run 032: TUI rendering fixes (Flow Status ANSI corruption, auto-refresh freeze)
- Run 033: Added Flow Status ANSI rendering regression coverage.

**V3 Phase 3 Completed:**
- Run 034: Implemented slot-based layout engine with ComponentID type, quadrants array, activeSlot tracking, maximize mode, renderComponent helper, and new keyboard handlers (TAB/p/m).

**V3 Phase 4 Completed:**
- Run 035: Added left/right arrow keys for slot switching, `w` key to swap with neighbor, number keys (0-3) to jump to specific slot.

**V3 Phase 5 Completed:**
- Run 035: Tests pass, binary builds successfully.
- Run 036: Post-implementation bug hunt fixes for Phase 3/4/5 (steering ESC handling + trace/artifact/report refresh wiring) with regression tests.

### Test Coverage
- `go test ./...` — passing

## Bug Fixes
- Run 038: Fixed tool registry mismatch — `getDefaultLLMTools()` listed `get_html` and `evaluate` which `MockToolRegistry` didn't have, causing "unknown tool" → "configuration error" failures in autonomous mode.
- Run 039: Fixed autonomous planner infinite loop — LLM generated 20 redundant verification steps because `finish` tool was never exposed in its prompt. Engine already handled `finish` at line 338 but LLM didn't know it existed.

## Last Run
- Run 039: 2026-05-19 (Agent: qwen3.6-plus-free)
  - Status: Fixed autonomous planner infinite loop — added `finish` tool to LLM prompt so planner stops after goal is achieved.

## Makefile (Updated)
- Added `run-sample` and `run-guided` targets for quick testing
- Added `help` target for available commands
- Enhanced `run` target documentation
- Added `fmt`, `vet`, `lint`, and `verify` workflow targets
- Updated `clean` to be PowerShell-safe on Windows
