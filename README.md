# qa-orchestrator

Terminal-first QA campaign orchestrator for guided and autonomous browser testing.

## What this project is

qa-orchestrator is a TUI app that runs QA campaigns as flows, tracks lifecycle state, and stores execution evidence.

- Guided flows: predefined steps are executed.
- Autonomous flows: planner generates steps from flow goal using the LLM.
- Operators can pause, resume, cancel, and steer runs from the terminal.

## Core architecture

High-level runtime path:

1. Campaign parser + validator checks schema, mode, dependencies, and cycles.
2. Orchestrator schedules eligible flows.
3. Mode router sends each flow to guided or autonomous path.
4. Agent loop executes `Planner -> Executor -> Validator -> Recovery`.
5. Tool layer invokes browser/runtime primitives.
6. Stores persist state:
   - Session store: run + flow lifecycle.
   - Trace store: machine-readable execution events.
   - Artifact store: screenshots/logs/reports.
7. TUI renders live run state and accepts operator commands.

Detailed design is in [docs/design/architecture.md](docs/design/architecture.md).

## Repository layout

```text
apps/tui/                        # terminal application (TUI)
packages/orchestrator/           # flow scheduling and execution control
packages/agents/                 # planner/executor/validator/recovery
  ├── engine/                    #   flow execution engine (engine.go, autonomous.go, execution.go, checkpoint.go, lifecycle.go, policy.go)
  ├── executor/                  #   tool execution
  ├── planner/                   #   step planning (guided + autonomous)
  ├── recovery/                  #   failure recovery decisions
  ├── validator/                 #   step validation + observations
  ├── tools/                     #   ToolInfo adapter for LLM prompts
  └── types/                     #   shared agent types
packages/browser-runtime/        # Playwright runtime (runtime.go, navigation.go, flow.go, screenshot.go)
packages/runtime/                # LifecycleController (pause/resume/cancel/steer)
packages/llm/                    # LLM client abstraction (OpenRouter, Gemini)
packages/storage/                # session/trace/artifact stores
packages/reporting/              # campaign summary reporting
packages/shared/                 # shared types (campaign, session, trace)
campaigns/                       # 10 sample campaign YAMLs
logs/                            # app.log + run logs (jsonl)
docs/                            # organized documentation
  ├── design/                    # blueprints and conventions
  ├── status/                    # roadmap and current delivery state
  └── history/                   # run summaries, archived sessions, and phase audits
```

## TUI behavior and controls

Main controls:

- `1-4`: Switch views (Dashboard, Flows, Traces, Report)
- `TAB`: Toggle sidebar/content focus
- `↑/k` `↓/j`: Navigate list items (context-aware)
- `Enter`: Select campaign from list / expand flow detail
- `Left/h`: Collapse flow detail
- `Space`: Pause/resume run
- `x`: Cancel run
- `:`: Enter command mode (type `retry`, `skip`, `continue`, `status`)
- `r`: Manual refresh
- `/`: Filter traces (Traces view only)
- `S`: Toggle failures-only filter (Traces view only)
- `f`: Toggle follow-tail (Traces view only)
- `?`: Toggle help modal overlay
- `Esc`: Dismiss help modal / exit command or filter mode
- `q`: Quit

## Maintenance

To maintain a clean root directory, all transient files (e.g., `MIDWORK.md`, `session-*.md`) are periodically archived to `docs/history/sessions/`. Large execution logs are managed in `logs/` and should be cleared using `make clean` if disk space is a concern.

## Agent Hand-off Experiment (Experimental)

This repository implements an experimental "Run Summary" hand-off mechanism for AI agents. Instead of agents traversing the entire codebase to guess previous progress, we use `docs/history/run-summaries/` to provide high-context, surgical summaries of past work. 

Feedback from participating agents suggests this significantly reduces "hallucination-guessing" and provides a cleaner state transfer than raw log analysis. While experimental, this approach aims to streamline multi-turn agentic workflows.

## Prerequisites

- Go (matching `go.mod`)
- Playwright runtime for browser tools
- For autonomous flows:
  - `LLM_API_KEY`
  - `LLM_MODEL`

## Quick start

```bash
make check-env
make build
make run-guided
```

Run autonomous sample:

```bash
make run-sample
```

Run with a custom campaign:

```bash
make run ARGS="campaigns/sample-autonomous.yaml"
```

## Development workflow

```bash
make test
make verify
```

Useful targets:

- `make lint` (`go fmt` + `go vet`)
- `make test-cover`
- `make deps`
- `make clean`

## Campaign validation rules (enforced)

- Campaign requires: `name`, `version`, `config`, `flows`.
- Every flow requires: `id`, `goal`, `mode`, `priority`, `depends_on`.
- `mode` must be `guided` or `autonomous`.
- Flow IDs must be unique.
- `depends_on` must reference valid flow IDs.
- Circular dependencies are rejected.
- Guided flows must provide non-empty `steps`.

## Current delivery status

See [docs/status/CURRENT.md](docs/status/CURRENT.md) for the latest phase status and run history.

## Performance & Stability Enhancements
- **Dynamic React Sync:** The framework uses Playwright's `WaitForFunction` to dynamically poll UI state updates after interactions, eliminating infinite observation loops on modern frontend frameworks (React, Vue, Angular).
- **High-Performance Deep Cloning:** The session store implements custom native struct copying instead of `json.Marshal`, radically reducing CPU overhead and global lock contention when streaming massive DOM payloads to the TUI.
