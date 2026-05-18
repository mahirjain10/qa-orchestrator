# Zenact-Style QA Campaign Orchestrator Architecture

This project is a terminal-first TUI application for running AI-assisted QA campaigns against web applications. It is designed as an orchestration layer around test execution: the system accepts a campaign, breaks it into flows, runs those flows through a planner/executor/validator/recovery loop, and stores run state, traces, and artifacts for pause, resume, cancel, and human steering.[cite:74][cite:58][cite:253]

## Product goal

The system behaves like a **test manager** rather than a single test runner. A user starts a campaign from the terminal, watches live progress, intervenes when needed, and gets a campaign-level result with evidence such as traces, screenshots, logs, and structured decisions.[cite:74][cite:76][cite:161]

## TUI-first experience

The primary interface is a terminal application, not a browser dashboard. The TUI should support campaign submission, live flow status, trace streaming, artifact links/paths, pause, resume, cancel, and steering commands because long-running orchestrated workflows commonly need lifecycle control and human-in-the-loop intervention.[cite:253][cite:254][cite:258]

Example user actions from the TUI:

- Start a campaign.
- Pause the active run.
- Resume from last checkpoint.
- Cancel the campaign.
- Steer the run with instructions like "skip this flow" or "retry with different credentials".
- Inspect the latest trace event and artifact path.

## End-to-end flow

1. The user launches the TUI and submits a campaign file in YAML/JSON or natural-language form.[cite:74]
2. The application creates a `run_id` and `session_id`, validates dependency references, and stores initial run state.[cite:192][cite:197]
3. The orchestrator selects the next eligible flow whose dependencies are satisfied.[cite:192][cite:215]
4. The selected flow enters the agent loop: Planner -> Executor -> Validator -> Recovery.[cite:170][cite:185]
5. The Executor uses trusted primitive tools implemented with Playwright to interact with the target web app.[cite:58][cite:167]
6. After each important step, the system writes a checkpoint to the session store, emits trace events, and saves artifacts such as screenshots or logs.[cite:198][cite:199][cite:202]
7. The TUI streams current status and accepts pause/resume/cancel/steering commands.[cite:253][cite:258][cite:264]
8. When all eligible flows finish, the system generates a campaign summary and report with pass/fail/skip states and linked evidence.[cite:74][cite:161]

## High-level architecture

```text
+---------------------------+
|      User / QA Engineer   |
|  uses terminal TUI        |
+------------+--------------+
             |
             v
+---------------------------+
|      TUI Application      |
| start/pause/resume/cancel |
| steer/show traces/status  |
+------------+--------------+
             |
             v
+---------------------------+
|   Runtime Control Layer   |
| lifecycle + HITL gate     |
+------------+--------------+
             |
             v
+---------------------------+
|  Campaign Orchestrator    |
| graph, eligibility, retry |
| session management        |
+-----+-----------+---------+
      |           | 
      |           +------------------+
      |                              |
      v                              v
+-------------+               +-------------+
| Session     |               | Trace Store |
| Store       |               | events/logs |
+-------------+               +-------------+
      |
      +------------------+
                         |
                         v
                 +---------------+
                 | Artifact Store |
                 | screenshots    |
                 | logs / reports |
                 +-------+-------+
                         |
                         v
                +--------------------+
                | Flow Execution     |
                | Planner -> Executor|
                | -> Validator       |
                | -> Recovery        |
                +---------+----------+
                          |
                          v
                +--------------------+
                | Tool Registry      |
                | trusted primitives |
                +---------+----------+
                          |
                          v
                +--------------------+
                | Playwright Runtime |
                | browser automation |
                +---------+----------+
                          |
                          v
                +--------------------+
                | Target Web App     |
                +--------------------+
```

## Core runtime layers

### 1. TUI application

The TUI is the operator console for the whole system. It should expose command panels or keybindings for run lifecycle control, current flow state, recent trace events, and steering input because the project is explicitly terminal-first.[cite:253][cite:258]

Suggested TUI sections:

- Campaign list
- Active run panel
- Flow status table
- Live trace panel
- Artifact panel
- Steering input box
- Controls: pause, resume, cancel, retry, skip

### 2. Runtime control layer

This layer owns interruptibility and human control. It is responsible for pausing safely, resuming from checkpoints, canceling active runs, and handling `WAITING_FOR_INPUT` states when the system needs human steering.[cite:253][cite:254][cite:263]

Primary responsibilities:

- Guard lifecycle transitions
- Expose runtime commands to the orchestrator
- Block or resume execution at safe checkpoints
- Persist steering decisions into trace history

### 3. Campaign orchestrator

The orchestrator is the campaign brain. It validates the dependency graph, chooses which flow can execute next, decides what to do after success/failure, and maintains campaign-level state for long-running runs.[cite:192][cite:197][cite:215]

Primary responsibilities:

- Validate missing dependencies and cycles
- Select eligible flows
- Track overall campaign progress
- Apply retry policy
- Mark blocked/skipped flows
- Aggregate final release decision

### 4. Flow execution engine

Each flow is executed through an agent loop rather than a single blind step. The system repeatedly observes, plans, acts, and verifies, which is a common pattern for agentic task execution and browser automation.[cite:170][cite:185][cite:245]

Flow sequence:

- Planner receives flow goal + last observation
- Executor runs tool calls
- Validator determines pass/fail/assertion result
- Recovery decides retry, replan, skip, or human escalation

### 5. Tool layer

The system should use a small set of reliable primitive tools rather than many large tools. Browser automation frameworks like Playwright support locators, assertions, navigation, screenshots, and waiting primitives that can be composed into complete test workflows.[cite:58][cite:167][cite:165]

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

### 6. Execution substrate

The demo substrate should be Playwright because it offers robust browser control, modern locator support, assertions, trace capture, and web-app automation capabilities suitable for a terminal-driven testing POC.[cite:58][cite:167]

## Stores

### Session store

The session store keeps current run state, flow states, retry counters, checkpoints, and lifecycle state such as `RUNNING`, `PAUSED`, or `WAITING_FOR_INPUT`. Workflow orchestration systems rely on persistent state to resume or cancel long-running processes safely.[cite:5][cite:197][cite:258]

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

The trace store keeps machine-readable execution history: step events, agent decisions, tool results, timestamps, and recovery actions. Traces are part of observability and are useful for reconstructing what happened during long-running automated workflows.[cite:200][cite:203][cite:206]

Example trace event:

```json
{
  "run_id": "run_123",
  "flow_id": "checkout-3g",
  "agent": "executor",
  "action": "click_pay",
  "status": "failed",
  "timestamp": "2026-05-17T18:00:55Z",
  "details": { "reason": "network_timeout" }
}
```

### Artifact store

The artifact store holds the evidence files produced during execution, such as screenshots, browser traces, console logs, and HTML reports. Test-automation systems commonly separate artifacts from structured state because artifacts are large files used for debugging and sharing outcomes.[cite:199][cite:202][cite:208]

## Lifecycle states

Recommended campaign states:

- `RUNNING`
- `PAUSING`
- `PAUSED`
- `WAITING_FOR_INPUT`
- `RESUMING`
- `CANCELLING`
- `CANCELLED`
- `COMPLETED`
- `FAILED`

Recommended flow states:

- `PENDING`
- `RUNNING`
- `PASSED`
- `FAILED`
- `RETRYING`
- `SKIPPED_UPSTREAM_FAILED`
- `BLOCKED_CONFIG_ERROR`
- `PAUSED`

These states make lifecycle and dependency handling explicit, which is important for orchestration clarity.[cite:258][cite:260][cite:264]

## Pause, resume, cancel, steering

The architecture must support more than just start-and-forget execution. Human-in-the-loop workflow systems commonly allow pause, resume, cancel, and manual approval or intervention in the middle of execution.[cite:253][cite:254][cite:255]

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

A steering event should always be written to the trace store and applied as an explicit runtime decision.[cite:253][cite:256]

## Failure handling

Failure handling should be deterministic and explicit.

- Missing dependency -> `BLOCKED_CONFIG_ERROR`
- Dependency failed -> `SKIPPED_UPSTREAM_FAILED`
- Temporary browser or locator problem -> retry
- UI mismatch -> planner replan
- Persistent failure -> recovery escalates to user or marks final failure

This makes the system safer and easier to reason about than a fully open-ended agent loop.[cite:197][cite:215]

## Custom tools

Custom tools should exist, but only as a controlled extension point. For the MVP, built-in trusted tools should do most of the work, while custom tools should be limited to QA-specific validators or composed workflows rather than arbitrary raw browser code.[cite:143][cite:155]

Examples:

- `assert_cart_total`
- `verify_coupon_applied`
- `validate_deeplink_destination`

Suggested custom-tool lifecycle:

1. Register schema and description
2. Attach implementation or composed workflow
3. Run controlled scenario validation
4. Approve and add to tool registry

## MVP scope

The smallest strong version of this project should include:

- TUI with start/pause/resume/cancel/steer
- YAML campaign input
- 3 sample flows
- 4 predefined agents
- 8 to 10 built-in tools
- Playwright-backed web execution
- session store
- trace store
- artifact store
- final Markdown/HTML report

## Build phases

### Phase 1: Core runtime

Expectation:

- Parse campaign file
- Validate dependencies
- Create run/session
- Persist state

### Phase 2: TUI shell

Expectation:

- Show campaign list and active run
- Start/pause/resume/cancel commands
- Display basic flow states

### Phase 3: Agent loop

Expectation:

- Planner, Executor, Validator, Recovery agents wired together
- Observe -> Act -> Verify loop working on one flow

### Phase 4: Tooling and Playwright integration

Expectation:

- Primitive tools implemented
- Browser automation works on sample app
- Screenshots and assertions available

### Phase 5: Trace and artifact pipeline

Expectation:

- Every major step writes a trace event
- Artifacts stored and visible from TUI
- Resume/checkpointing works

### Phase 6: Steering and lifecycle controls

Expectation:

- Pause, resume, cancel, and steering commands work correctly
- `WAITING_FOR_INPUT` flow supported

### Phase 7: Final reporting

Expectation:

- Campaign summary generated
- Flow-level outcomes + links to artifacts included
- Final user-facing terminal summary available
