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

```text
+-----------------------------+
|      User / QA Engineer     |
|  uses terminal TUI          |
+-------------+---------------+
              |
              v
+-----------------------------+
|      TUI Application        |
| start/pause/resume/cancel   |
| steer/show traces/status    |
+-------------+---------------+
              |
              v
+-----------------------------+
|   Runtime Control Layer     |
| lifecycle + HITL gate       |
+-------------+---------------+
              |
              v
+-----------------------------+
|   Campaign YAML Validator   |
| schema, fields, deps, mode  |
+-------------+---------------+
              |
              v
+-----------------------------+
|  Campaign Orchestrator      |
| graph, eligibility, retry   |
| session management          |
| worker pool + semaphore     |
+------+------------+---------+
       |            |
       v            v
+------------+  +-------------+
| Session    |  | Trace Store |
| Store      |  | events/logs |
+------------+  +-------------+
       |
       v
+---------------+
| Artifact Store|
| screenshots   |
| logs/reports  |
+-------+-------+
        |
        v
+-----------------------------+
|       Mode Router           |
| reads flow `mode` field     |
| guided | autonomous         |
+------+---------------+------+
       |               |
       v               v
+-----------+   +------------------+
| Guided    |   | Autonomous       |
| Execution |   | Execution        |
| follows   |   | Planner generates|
| predefined|   | steps from goal  |
| steps     |   | using LLM        |
+-----+-----+   +--------+---------+
      |                  |
      +--------+---------+
               |
               v
+-----------------------------+
| Flow Execution Engine       |
| Planner -> Executor         |
| -> Validator -> Recovery    |
| + dependency context        |
+-------------+---------------+
              |
              v
+-----------------------------+
| Tool Registry               |
| trusted primitives          |
+-------------+---------------+
              |
              v
+-----------------------------+
| Browser Runtime             |
+-------------+---------------+
              |
              v
+-----------------------------+
| Playwright Runtime          |
+-------------+---------------+
              |
              v
+-----------------------------+
| Target Web App              |
+-----------------------------+
```

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

## Core Features

### 1. High-Performance Terminal Interface (TUI)
- **Live Dashboard:** Real-time visualization of campaign progress and metrics.
- **Multi-View Navigation:** Specialized screens for Dashboard, Flows, Traces, and Reports.
- **Trace Streaming:** Live, filterable logs with "follow-tail" mode for instant debugging.

### 2. Advanced Orchestration
- **Parallel Execution:** Worker pool and semaphore system for concurrent flow execution.
- **Dependency Graph:** Topological scheduling ensuring upstream requirements are met.
- **Session Inheritance:** Persistence of cookies/localStorage across dependent flows.

### 3. Agentic Execution Engine
- **Hybrid Modes:** Supports both `guided` (deterministic) and `autonomous` (LLM-powered) flows.
- **Grounded Observations:** Custom `observe_ui` tool providing exact selectors to the LLM.
- **Safety Nets:** Intelligent loop detection, 404 interception, and automated selector repair.

### 4. Reliability
- **Checkpoint/Resume:** Pause and resume any run from the last successful step.
- **LLM Fallback:** Automatic model switching during provider downtime.
- **Dynamic Sync:** Intelligent waiting for modern frontend state updates (React/Vue).

## Current Limitations & Known Issues (The "Why")

While the system is robust, several areas are still in "experimental" or "evolving" states:

| Issue | Description | The "Why" |
|-------|-------------|-----------|
| **TUI Stability** | The TUI can be "a bit buggy" with occasional rendering glitches or input lag. | Concurrent updates from multiple background workers can overwhelm the Bubble Tea event loop, leading to flickering or state desync in high-concurrency runs. |
| **LLM Latency** | Autonomous planning steps can take 5–15 seconds. | Caused by high-token "Thinking" modes and network round-trips to providers like OpenRouter/Gemini. |
| **Complex Iframes** | Interactions inside nested iframes may occasionally fail. | The `observe_ui` crawler currently prioritizes the top-level document; deep cross-origin iframe traversal is computationally expensive. |
| **TUI Memory Pressure** | TUI may slow down during massive campaigns (50+ parallel flows). | Radical DOM payload streaming to the terminal puts pressure on the TUI's refresh loop despite clone optimizations. |
| **Fuzzy Selector Reliability** | Automated selector repair is "best-effort". | UI elements with identical text but different functional roles can confuse the fuzzy matcher. |
| **Shadow DOM Support** | Elements inside closed Shadow Roots are currently invisible. | Standard `TreeWalker` and `querySelector` APIs used in `observe_ui` cannot penetrate closed shadow boundaries without specialized handling. |

## Agent Hand-off Experiment (Experimental)

This repository implements an experimental "Run Summary" hand-off mechanism for AI agents. Instead of agents traversing the entire codebase to guess previous progress, we use `docs/history/run-summaries/` to provide high-context, surgical summaries of past work. 

### Example: Run Summary (`docs/history/run_summaries/run_001.md`)

```markdown
# Run Summary — 001

## Goal
Initial project setup and Phase 1 implementation.

## What happened
1. Created project structure with root module.
2. Implemented campaign parser (YAML/JSON/natural-language).
3. Implemented dependency validator.
4. Implemented run/session ID generation.
5. Implemented session store with atomic writes.

## Files created/modified
- `packages/shared/types/campaign.go`
- `packages/orchestrator/campaign/parser.go`
- `packages/storage/session/store.go`

## Status
- Tests: PASSED
- Phase: COMPLETE

## Next
- Phase 2: TUI Shell.
```

### Example: Execution Logs (`logs/app.log` style)

```text
2026-05-25 14:02:10 [INFO] Orchestrator: starting campaign "Login Campaign" (v1.0)
2026-05-25 14:02:11 [INFO] Flow: starting flow "auth-flow" (mode: autonomous, priority: high)
2026-05-25 14:02:15 [DEBUG] Agent[auth-flow]: planning next step for goal "Test login with invalid credentials"
2026-05-25 14:02:18 [INFO] Executor[auth-flow]: tool=navigate params={"url": "https://example.com/login"}
2026-05-25 14:02:22 [INFO] Validator[auth-flow]: step PASSED (observation: page loaded, login form visible)
2026-05-25 14:02:30 [INFO] Flow: flow "auth-flow" COMPLETED successfully
```

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
