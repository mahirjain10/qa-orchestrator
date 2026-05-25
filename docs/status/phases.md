# Project Phases and Agent Implementation Guide

This document outlines the implementation phases for the Zenact TUI POC. AI agents should use this file to understand the project roadmap and what is expected to be implemented in each phase.

## Core Agent Workflow
When working on any phase or task, agents MUST strictly adhere to the rules in `AGENTS.md` and the architecture in `docs/architecture.md`:
1. Create a plan and ask for human approval before implementing anything.
2. Implement *only* the approved scope.
3. Add necessary tests and ensure compilation succeeds. (Do not take shortcuts).

---

## Phase 1: Core runtime 
**Focus:** Foundation and Orchestration
**Implementation Expected:**
- Parse campaign input files (YAML/JSON/natural-language).
- Validate dependencies and dependency graphs.
- Create `run_id` and `session_id` logic.
- Implement the Session Store to persist run state.

## Phase 2: TUI shell
**Focus:** The Terminal User Interface
**Implementation Expected:**
- Set up the main TUI framework (terminal-first experience).
- Display the campaign list and the active run panel.
- Display basic flow states (e.g., PENDING, RUNNING, PASSED, FAILED).
- Wire UI controls for Start, Pause, Resume, and Cancel commands.
> **Example Constraints (e.g., `PHASE-2-TASK-4: persistent run state storage`):**
> - State must survive crashes and support resume.
> - No in-memory-only state.
> - Avoid tight coupling (respect orchestrator interfaces).
> - Prevent partial writes, race conditions, and corrupted state.

## Phase 3: Agent loop
**Focus:** Flow Execution Engine
**Implementation Expected:**
- Wire together the Planner, Executor, Validator, and Recovery agents.
- Implement the "Observe -> Act -> Verify -> Recover" continuous loop.
- Guarantee the agent loop works end-to-end for at least one sample flow.

## Phase 4: Tooling and Playwright integration
**Focus:** Browser Automation and Tools
**Implementation Expected:**
- Implement trusted primitive tools (e.g., `navigate`, `observe_ui`, `click_element`, `wait_for`, `take_screenshot`).
- Integrate the Playwright browser automation framework as the execution substrate.
- Ensure locators, screenshots, and assertions work reliably against a sample target web app.

## Phase 5: Trace and artifact pipeline
**Focus:** Observability and Evidence
**Implementation Expected:**
- Implement the Trace Store to record machine-readable step events, decisions, and tool results.
- Implement the Artifact Store to save screenshots, logs, and HTML reports safely.
- Expose live trace events and artifact links/paths in the TUI.
- Solidify safe checkpointing and state-resume capabilities.

## Phase 6: Steering and lifecycle controls
**Focus:** Human-in-the-loop (HITL) Intervention
**Implementation Expected:**
- Implement and finalize `PAUSING`, `PAUSED`, `RESUMING`, and `CANCELLING` lifecycle transitions.
- Implement the `WAITING_FOR_INPUT` state.
- Support explicit steering commands (e.g., retry this flow, skip this flow) initiated by the user via the TUI.

## Phase 7: Final reporting
**Focus:** Results Output
**Implementation Expected:**
- Generate an aggregated campaign-level summary report when all flows finish.
- Map all flow-level outcomes (pass/fail/skip) to their respective linked evidence.
- Render the final Markdown/HTML report and present the final completion status gracefully in the terminal.

---

## Active Task Handling
When an agent is assigned a specific task ID, it must carefully review the associated task definition and adhere strictly to the following sections:

### Goal
Understand the primary objective of the task.

### Constraints
Adhere strictly to architectural and design constraints.

### Files allowed
Limit modifications **only** to the allowed files or directories (e.g., `internal/state/*`, `internal/store/*`).

### Forbidden
Strictly avoid forbidden actions (e.g., changing orchestrator interfaces).

### Acceptance Criteria
Ensure all criteria are met before considering the task complete.

### Failure Modes To Avoid
Proactively guard against documented failure modes (e.g., race conditions, partial writes).

### Example Task Definition

```text
# TASK

ID: PHASE-2-TASK-4

Goal:
Implement persistent run state storage.

Constraints:
- Must survive crashes
- Must support resume
- No in-memory-only state
- Avoid tight coupling

Files allowed:
- internal/state/*
- internal/store/*

Forbidden:
- changing orchestrator interfaces

Acceptance Criteria:
- Run resumes after restart
- State persists to disk
- Validator passes integration tests

Failure Modes To Avoid:
- partial writes
- race conditions
- corrupted state
```
