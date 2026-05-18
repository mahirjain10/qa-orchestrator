# Project Phases V2: Hybrid & Autonomous Architecture

This document outlines the implementation phases for the Zenact Major Update, transitioning the system into a hybrid (Guided/Autonomous) orchestrator.

## Phase 1: Hybrid Schema & Validation
**Goal:** Update types and validation to support mode-based execution.
**Tasks:**
- Update `Flow` struct in `packages/shared/types/campaign.go` to include `Mode` and `Priority`.
- Enhance `CampaignParser` in `packages/orchestrator/campaign/parser.go` to validate:
    - `mode` (guided/autonomous).
    - `guided` flows must have `steps`.
    - `autonomous` flows must have `goal`.
    - `id` uniqueness and `depends_on` validity.
- Update `DependencyValidator` to ensure all rules from `agents.md` are enforced.

## Phase 2: LLM Integration Package
**Goal:** Create a shared LLM client for autonomous planning.
**Tasks:**
- Implement `packages/llm` package.
- Support OpenRouter/OpenAI compatible APIs via Environment Variables.
- Implement basic prompt management for the Planner.
- Add error handling and retry logic for LLM calls.

## Phase 3: Iterative Autonomous Planner
**Goal:** Implement the planner logic for generating steps one-by-one.
**Tasks:**
- Update `packages/agents/planner/planner.go` to handle `autonomous` mode.
- Implement iterative step generation (Next Step = LLM(Goal, History, Tools, Observation)).
- Update `types.Plan` to support dynamic step addition.

## Phase 4: Tool Registry & Metadata
**Goal:** Expose tool descriptions for the LLM.
**Tasks:**
- Enhance `ToolRegistry` to include tool descriptions and parameter schemas.
- Provide a `ListToolsWithDocs()` method for the Planner.

## Phase 5: Agent Engine V2 (Mode Routing)
**Goal:** Update the execution engine to route between modes.
**Tasks:**
- Update `packages/agents/engine/engine.go` to check flow `mode`.
- For `guided`: use existing sequential execution.
- For `autonomous`: use iterative planner loop.
- Ensure trace events reflect the mode and LLM decisions.

## Phase 6: TUI & Visibility
**Goal:** Update the TUI to reflect the new architecture.
**Tasks:**
- Update `FlowStatus` component to show mode and priority.
- Add visual indicators for "LLM Planning" status.
- Ensure steering commands (retry/skip) work correctly with autonomous flows.

## Phase 7: Validation & Sample Campaigns
**Goal:** Verify the system with real-world scenarios.
**Tasks:**
- Create `campaigns/sample-autonomous.yaml`.
- Run and verify end-to-end execution.
- Ensure recovery agent can trigger replanning in autonomous mode.
