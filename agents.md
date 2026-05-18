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

## Conventions
- Everything you need to know about templates, file naming, log format, and slice strategy lives in `docs/LOG_CONVENTIONS.md`. Read it before writing any log, summary, or updating CURRENT.md.