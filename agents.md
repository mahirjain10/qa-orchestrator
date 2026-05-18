# AGENTS.md

This is a terminal-based AI coding/orchestration project.

## Core workflow
- Read `docs/CURRENT.md` first.
- Read the latest run summary in `docs/run-summaries/`.
- Read `docs/architecture.md` whenever architecture context is needed.
- Read runtime logs only if needed, and only a relevant slice.
- Create a short plan.
- Ask for approval before implementing anything.
- If approved, implement only the approved scope.
- Add tests if required. Do not take shortcuts writing them.
- Run the relevant test suite.
- Compile successfully. If it fails, fix and compile again.
- Write the runtime log entry.
- Write the run summary.
- Update `docs/CURRENT.md`.
- Stop and wait for the next instruction.

## Execution mode

Before executing any flow, check the `mode` field in the campaign YAML.

- `guided` — follow the predefined steps exactly. Do not generate steps.
- `autonomous` — the planner generates step sequence from the flow goal using the LLM.
- If a flow has a goal but no steps, treat it as `autonomous`.
- If a flow is `guided` and has no steps, reject it — it is an invalid campaign.

## YAML validation rules

Before any run is created, validate the campaign file:

- `name`, `version`, `config`, and `flows` are required.
- Each flow must have `id`, `goal`, `mode`, `priority`, and `depends_on`.
- `mode` must be `guided` or `autonomous`.
- `id` must be unique across all flows.
- All `depends_on` references must point to existing flow IDs.
- No circular dependencies allowed.
- `guided` flows must have a non-empty `steps` list.
- `autonomous` flows may omit `steps`.
- Reject the campaign with a clear error message if any rule fails.

## Required output order
1. Make changes.
2. Compile successfully. If it fails, fix and compile again.
3. Add tests if needed or improve current tests if necessary. Do not take shortcuts writing them.
4. Run the relevant test suite.
5. Compile successfully. If it fails, fix and compile again.
6. Write logs.
7. Write summary.
8. Update `CURRENT.md`.
9. Stop and wait for the next instruction.

## Rules
- Do not skip tests.
- Do not claim completion without successful compile.
- Do not modify scope without approval.
- Prefer small, focused changes.

## Summary Edit Protocol
- Only write/edit to your own current session's summary/log.
- Never touch another agent's summaries (past or present) without explicit permission.
- Request permission before any edit to someone else's summary.
- Show exact changes before touching it.
- Wait for user confirmation before proceeding.

## Conventions
- Everything you need to know about templates, file naming, log format, and slice strategy lives in `docs/LOG_CONVENTIONS.md`. Read it before writing any log, summary, or updating CURRENT.md.

## Template files
- Run log template: `logs/runs/run-log_TEMPLATE.jsonl`
- Run summary template: `docs/run-summaries/run_summary_TEMPLATE.md`
- Current file template: `docs/CURRENT_TEMPLATE.md`
