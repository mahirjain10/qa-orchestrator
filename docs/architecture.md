# qa-orchestrator Architecture

This project is a terminal-first TUI application for running AI-assisted QA campaigns against web applications. It is designed as an orchestration layer around test execution: the system accepts a campaign, breaks it into flows, runs those flows through a planner/executor/validator/recovery loop, and stores run state, traces, and artifacts for pause, resume, cancel, and human steering.

## Product goal

The system behaves like a **test manager** rather than a single test runner. A user starts a campaign from the terminal, watches live progress, intervenes when needed, and gets a campaign-level result with evidence such as traces, screenshots, logs, and structured decisions.

## TUI-first experience

The primary interface is a terminal application, not a browser dashboard. The TUI should support campaign submission, live flow status, trace streaming, artifact links/paths, pause, resume, cancel, and steering commands because long-running orchestrated workflows commonly need lifecycle control and human-in-the-loop intervention.

Example user actions from the TUI:

- Start a campaign.
- Pause the active run.
- Resume from last checkpoint.
- Cancel the campaign.
- Steer the run with instructions like "skip this flow" or "retry with different credentials".
- Inspect the latest trace event and artifact path.

## End-to-end flow

1. The user launches the TUI and submits a campaign file in YAML/JSON or natural-language form.
2. The system validates the campaign YAML schema — required fields, mode value, dependency references, and no cycles.
3. The application creates a `run_id` and `session_id` and stores initial run state.
4. The orchestrator selects the next eligible flow whose dependencies are satisfied.
5. The Mode Router checks the flow `mode` field and routes to guided or autonomous execution path.
6. The selected flow enters the agent loop: Planner -> Executor -> Validator -> Recovery.
7. The Executor uses trusted primitive tools implemented with Playwright to interact with the target web app.
8. After each important step, the system writes a checkpoint to the session store, emits trace events, and saves artifacts such as screenshots or logs.
9. The TUI streams current status and accepts pause/resume/cancel/steering commands.
10. When all eligible flows finish, the system generates a campaign summary and report with pass/fail/skip states and linked evidence.

## High-level architecture

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
| Playwright Runtime          |
| browser automation          |
+-------------+---------------+
              |
              v
+-----------------------------+
| Target Web App              |
+-----------------------------+
```

## Campaign schema and YAML validation

Every campaign file must pass schema validation before execution starts. The validator checks required fields, valid mode values, dependency references, and cycle detection.

### Required fields

```yaml
name: string        # required
version: string     # required
config:
  timeout: string   # required, e.g. "300s"
  retry_limit: int  # required
  parallel_limit: int  # required
flows:              # required, at least one flow
  - id: string      # required, unique
    goal: string    # required
    mode: guided | autonomous   # required
    priority: high | medium | low  # required
    depends_on: []  # required, empty list if no deps
```

### Validation rules

- `mode` must be `guided` or `autonomous`.
- `id` must be unique across all flows.
- All `depends_on` references must point to valid flow IDs.
- No circular dependencies allowed.
- If `mode` is `guided`, `steps` must be present and non-empty.
- If `mode` is `autonomous`, `steps` may be omitted — the planner generates them.
- If `steps` is absent and `mode` is `guided`, validation fails with a clear error.

### Guided campaign example

```yaml
name: Login Campaign
version: "1.0"
config:
  timeout: 300s
  retry_limit: 2
  parallel_limit: 1

flows:
  - id: login-flow
    goal: "Test login with invalid credentials"
    mode: guided
    priority: high
    depends_on: []
    steps:
      - id: nav-to-login
        tool: navigate
        params:
          url: "https://example.com/login"
      - id: enter-username
        tool: type_text
        params:
          selector: "#username"
          value: "wronguser"
      - id: verify-error
        tool: assert_text_visible
        params:
          text: "Invalid credentials"
```

### Autonomous campaign example

```yaml
name: Pre-release Campaign
version: "1.0"
config:
  timeout: 300s
  retry_limit: 2
  parallel_limit: 1

flows:
  - id: auth-flow
    goal: "Test login with invalid credentials and verify the error message"
    mode: autonomous
    priority: high
    depends_on: []

  - id: payment-flow
    goal: "Complete checkout under 3G network conditions"
    mode: autonomous
    priority: high
    depends_on: [auth-flow]

  - id: deeplink-flow
    goal: "Verify deep link routing to user profile"
    mode: autonomous
    priority: medium
    depends_on: []
```

## Hybrid execution

The system supports two execution modes per flow. Both modes share the same orchestrator, lifecycle controls, stores, and tool layer.

### Guided mode

- Use when selectors, URLs, and step sequences are stable and well-known.
- The planner follows the predefined step list exactly.
- Failures are deterministic and easy to debug.
- Suitable for regression flows and known UI paths.

### Autonomous mode

- Use when the goal is clear but the step sequence should not be hardcoded.
- The planner receives the flow goal and last observation, then generates a step sequence using the LLM.
- Suitable for exploratory flows, plain-English campaign goals, and adapting to UI changes.
- Recovery agents can replan if the generated steps fail.

## Core runtime layers

### 1. TUI application

The TUI is the operator console for the whole system. It exposes command panels for run lifecycle control, current flow state, recent trace events, and steering input.

Suggested TUI sections:

- Campaign list
- Active run panel
- Flow status table
- Live trace panel
- Artifact panel
- Steering input box
- Controls: pause, resume, cancel, retry, skip

### 2. Runtime control layer

This layer owns interruptibility and human control. It is responsible for pausing safely, resuming from checkpoints, canceling active runs, and handling `WAITING_FOR_INPUT` states.

Primary responsibilities:

- Guard lifecycle transitions
- Expose runtime commands to the orchestrator
- Block or resume execution at safe checkpoints
- Persist steering decisions into trace history

### 3. Campaign YAML validator

The validator runs before any execution starts. It rejects malformed or invalid campaigns early with clear error messages.

Validation checks:

- Required fields present
- `mode` is `guided` or `autonomous`
- Flow IDs are unique
- `depends_on` references exist
- No dependency cycles
- `guided` flows must have non-empty `steps`
- `autonomous` flows must have a non-empty `goal`

### 4. Campaign orchestrator

The orchestrator is the campaign brain. It validates the dependency graph, chooses which flow can execute next, decides what to do after success/failure, and maintains campaign-level state.

Primary responsibilities:

- Select eligible flows
- Track overall campaign progress
- Apply retry policy
- Mark blocked/skipped flows
- Aggregate final release decision

### 5. Mode router

The mode router sits between the orchestrator and the flow execution engine. It reads the `mode` field of the selected flow and routes execution to the correct path.

- `guided` → passes predefined steps directly to the executor.
- `autonomous` → sends the goal to the planner for step generation before execution.

### 6. Flow execution engine

Each flow is executed through an agent loop. The system repeatedly observes, plans, acts, and verifies.

Flow sequence:

- **Planner:** in `guided` mode, reads predefined steps; in `autonomous` mode, generates step sequence from the goal using the LLM and last observation.
- **Executor:** runs tool calls against the browser.
- **Validator:** determines pass/fail/assertion result.
- **Recovery:** decides retry, replan, skip, or human escalation.

### 7. Tool layer

The system uses a small set of reliable primitive tools.

Recommended built-in tools:

- `navigate(url)`
- `observe_ui()`
- `click_element(locator)`
- `type_text(locator, value)`
- `wait_for(locator_or_text)`
- `assert_text_visible(text)`
- `take_screenshot()`
- `set_network_profile(profile)`
- `fetch_test_data()`
- `fetch_otp_mock()`

### 8. Execution substrate

The demo substrate is Playwright for robust browser control, modern locator support, assertions, and trace capture.

## Stores

### Session store

Keeps current run state, flow states, retry counters, checkpoints, and lifecycle state.

Example fields:

- `run_id`
- `session_id`
- `campaign_status`
- `current_flow_id`
- `current_agent`
- `last_completed_step`
- `retry_count`
- `checkpoint_payload`

### Trace store

Keeps machine-readable execution history: step events, agent decisions, tool results, timestamps, and recovery actions.

Example trace event:

```json
{
  "run_id": "run_123",
  "flow_id": "checkout-3g",
  "agent": "executor",
  "action": "click_pay",
  "status": "failed",
  "timestamp": "2026-05-17T18:00:55+05:30",
  "details": { "reason": "network_timeout" }
}
```

### Artifact store

Holds evidence files: screenshots, browser traces, console logs, and HTML reports.

## Lifecycle states

Campaign states:

- `RUNNING`
- `PAUSING`
- `PAUSED`
- `WAITING_FOR_INPUT`
- `RESUMING`
- `CANCELLING`
- `CANCELLED`
- `COMPLETED`
- `FAILED`

Flow states:

- `PENDING`
- `RUNNING`
- `PASSED`
- `FAILED`
- `RETRYING`
- `SKIPPED_UPSTREAM_FAILED`
- `BLOCKED_CONFIG_ERROR`
- `PAUSED`

## Pause, resume, cancel, steering

### Pause

- Accept pause command from TUI
- Mark run as `PAUSING`
- Finish current safe step
- Write checkpoint
- Transition to `PAUSED`

### Resume

- Load latest checkpoint from session store
- Reconstruct flow context
- Transition to `RESUMING` then `RUNNING`
- Continue from next pending step

### Cancel / exit

- Accept cancel command from TUI
- Mark run as `CANCELLING`
- Stop further scheduling
- Gracefully stop active browser task at safe point
- Persist final trace event
- Mark run as `CANCELLED`

### Steering

Steering means the user can intervene with instructions such as:

- retry this flow
- skip this flow
- continue without network throttling
- approve recovery plan
- mark as human review needed

A steering event must always be written to the trace store and applied as an explicit runtime decision.

## Failure handling

- Missing dependency → `BLOCKED_CONFIG_ERROR`
- Dependency failed → `SKIPPED_UPSTREAM_FAILED`
- YAML validation failure → reject campaign before run creation
- Temporary browser or locator problem → retry
- UI mismatch → planner replan
- Persistent failure → recovery escalates to user or marks final failure

## Custom tools

Custom tools are a controlled extension point. For the MVP, built-in tools should do most of the work.

Examples:

- `assert_cart_total`
- `verify_coupon_applied`
- `validate_deeplink_destination`

Suggested custom-tool lifecycle:

1. Register schema and description
2. Attach implementation or composed workflow
3. Run controlled scenario validation
4. Approve and add to tool registry

## Folder structure

```text
qa-orchestrator/
├── apps/
│   └── tui/
│       ├── cmd/
│       │   └── main.go
│       └── internal/
│           ├── screens/
│           ├── components/
│           ├── commands/
│           └── state/
│
├── packages/
│   ├── orchestrator/
│   ├── agents/
│   ├── tools/
│   ├── browser-runtime/
│   ├── storage/
│   ├── reporting/
│   └── shared/
│
├── campaigns/
│   ├── sample-guided.yaml
│   └── sample-autonomous.yaml
│
├── artifacts/
│
├── bin/
│   └── qa-orchestrator
│
├── logs/
│   └── runs/
│       └── 2026-05/
│           ├── run-001.jsonl
│           ├── run-002.jsonl
│           └── ...
│
├── docs/
│   ├── run-summaries/
│   │   ├── run-001.md
│   │   ├── run-002.md
│   │   └── ...
│   ├── architecture.md
│   ├── phases.md
│   ├── CURRENT.md
│   ├── CURRENT_TEMPLATE.md
│   └── LOG_CONVENTIONS.md
│
├── agents.md
├── go.mod
├── go.sum
├── Makefile
├── .gitignore
└── README.md
```

## MVP scope

- TUI with start/pause/resume/cancel/steer
- YAML campaign input with schema validation
- 3 sample flows (mix of guided and autonomous)
- 4 predefined agents
- 8 to 10 built-in tools
- Playwright-backed web execution
- Session store
- Trace store
- Artifact store
- Final Markdown/HTML report

## Build phases

### Phase 1: Core runtime

- Parse campaign file
- Validate YAML schema, fields, dependencies, and mode values
- Create run/session
- Persist state

### Phase 2: TUI shell

- Show campaign list and active run
- Start/pause/resume/cancel commands
- Display basic flow states

### Phase 3: Agent loop

- Planner, Executor, Validator, Recovery agents wired together
- Observe -> Act -> Verify loop working on one flow
- Planner handles both guided and autonomous mode

### Phase 4: Tooling and Playwright integration

- Primitive tools implemented
- Browser automation works on sample app
- Screenshots and assertions available

### Phase 5: Trace and artifact pipeline

- Every major step writes a trace event
- Artifacts stored and visible from TUI
- Resume/checkpointing works

### Phase 6: Steering and lifecycle controls

- Pause, resume, cancel, and steering commands work correctly
- `WAITING_FOR_INPUT` flow supported

### Phase 7: Final reporting

- Campaign summary generated
- Flow-level outcomes + links to artifacts included
- Final user-facing terminal summary available
