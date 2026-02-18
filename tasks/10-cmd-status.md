# Task 10 — `minuano status`

## Goal

Implement the table view of all tasks.

## Steps

1. Add `minuano status` subcommand.
2. Query all tasks (optionally filtered by `--project`).
3. Display a table with columns: status symbol, ID (truncated), title, status text, claimed_by, attempt.
4. Use the status symbols from DESIGN.md: `○ ◎ ● ✓ ✗`.
5. Use `lipgloss` or `tablewriter` for formatting.

## Acceptance

- `minuano status` prints a formatted table of all tasks.
- Status symbols match the DESIGN.md spec.
- Output is aligned and readable.

## Phase

2 — Core CLI Commands

## Depends on

- Task 09
