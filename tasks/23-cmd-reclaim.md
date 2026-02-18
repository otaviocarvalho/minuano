# Task 23 — `minuano reclaim`

## Goal

Implement the stale task recovery command.

## Steps

1. Add `minuano reclaim` subcommand.
2. Flag `--minutes N` — stale threshold (default 30).
3. Run the `ReclaimStale` query from `queries.go`.
4. Print how many tasks were reclaimed.

## Acceptance

- `minuano reclaim --minutes 45` resets tasks claimed more than 45 minutes ago.
- Reclaimed tasks are set back to `ready` with `claimed_by` cleared.
- The count of reclaimed tasks is printed.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 17
