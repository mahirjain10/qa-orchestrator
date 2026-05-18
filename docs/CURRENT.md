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
- **Phase 6:** TUI & Visibility — PENDING
- **Phase 7:** Validation & Sample Campaigns — PENDING

### File Structure
```
qa-orchestrator/
├── apps/tui/                      # TUI interface and screens
├── packages/
│   ├── agents/                    # Loop: Engine, Planner, Executor, Validator, Recovery
│   ├── browser-runtime/           # Playwright runtime & ToolRegistry
│   ├── llm/                       # LLM client (OpenRouter/OpenAI)
│   ├── orchestrator/              # Campaign parsing & DAG validation
│   ├── reporting/                 # Markdown and Terminal report generation
│   ├── runtime/                   # LifecycleController (unified state management)
│   ├── shared/types/              # Core types (Session, Trace, Flow, Steering)
│   └── storage/                   # Stores: Session, Trace, Artifact
├── go.mod                         # Root module
└── docs/run-summaries/            # Up to run-021.md
```

### Test Coverage
- `go test ./...` verified successfully across all packages.

## Last Run
- Run 021: 2026-05-19
- Agent: codex
- Status: Hardened Phase 5 autonomous cancellation and tool metadata conversion; tests and compile passed.
