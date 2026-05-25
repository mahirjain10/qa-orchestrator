# Log Convention

## Purpose
Defines how run logs and summaries are named and stored.

## Template files
- Log template: `logs/runs/run-log_TEMPLATE.jsonl`
- Summary template: `docs/run-summaries/run_summary_TEMPLATE.md`
- Current template: `docs/CURRENT_TEMPLATE.md`

## Folder structure
- Run logs: `logs/runs/2026-05/run-001.jsonl`
- Run summaries: `docs/run-summaries/run-001.md`
- Current file: `docs/CURRENT.md`

## Rules
- Use sequential run numbers with leading zeros.
- Keep log and summary numbers matched.
- Follow the templates exactly.

## Log behavior
- Run logs are append-only JSONL files.
- New entries must be added at the end of the file.
- Do not rewrite, reorder, or delete previous entries.
- If a correction is needed, write a new log entry that explains the correction.

## Slice strategy
- Do not dump the entire log file into the prompt.
- For context, read the latest run summary in `docs/run-summaries/` first.
- If the summary is insufficient, ask for permission before reading logs.
- When approved, read only a relevant slice:
  - Last N entries for recent context.
  - Filter by `run_id` or `flow_id` when debugging a specific run.
  - First few entries only if run origin or initial state is needed.
- Never load more than 20 entries at once unless explicitly instructed.