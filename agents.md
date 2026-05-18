# AGENTS.md

This is a terminal-based AI coding/orchestration project.

## Core workflow
- Read `docs/architecture.md` whenever architecture context is needed.
- Create a short plan.
- Ask for approval before implementing anything.
- If approved, implement only the approved scope.
- Add tests if required. Do not take shortcuts writing them.
- Run the relevant test suite.
- Compile successfully. If it fails, fix and compile again.
- Stop and wait for the next instruction.

## Required output order
1. Make changes.
2. Compile successfully. If it fails, fix and compile again.
3. Add tests if needed or improve current tests if necessary. Do not take shortcuts writing them.
4. Run the relevant test suite.
5. Compile successfully. If it fails, fix and compile again.
6. Stop and wait for the next instruction.

## Rules
- Do not skip tests.
- Do not claim completion without successful compile.
- Do not modify scope without approval.
- Prefer small, focused changes.