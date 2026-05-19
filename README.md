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

Detailed design is in [docs/architecture.md](docs/architecture.md).

## Repository layout

```text
apps/tui/                   # terminal application
packages/orchestrator/      # flow scheduling and execution control
packages/agents/            # planner/executor/validator/recovery
packages/browser-runtime/   # browser execution substrate
packages/storage/           # session/trace/artifact stores
packages/reporting/         # terminal/summary reporting
campaigns/                  # sample campaigns
docs/                       # phases, architecture, current state, summaries
logs/runs/                  # run logs (jsonl)
```

## TUI behavior and controls

Main controls:

- `TAB` / `←` / `→`: switch active slot
- `0-3`: jump to slot
- `p`: cycle component in active slot
- `w`: swap active slot with neighbor
- `m`: maximize/restore active slot
- `↑` / `↓` or `j` / `k`: navigate list items
- `Enter`: select run from campaign list
- `Space`: pause/resume run
- `x`: cancel run
- `s`: steering mode (retry/skip/continue/status)
- `esc`: exit steering or restore from maximize
- `q`: quit

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

See [docs/CURRENT.md](docs/CURRENT.md) for the latest phase status and run history.
