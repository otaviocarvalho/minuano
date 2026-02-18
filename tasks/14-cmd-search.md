# Task 14 — `minuano search`

## Goal

Implement full-text search across task context.

## Steps

1. Add `minuano search <query>` subcommand.
2. Use the GIN full-text index on `task_context` (`to_tsvector('english', content)`).
3. Display matching context entries with: task ID, task title, kind, timestamp, and a snippet of the matching content with highlights.
4. Order by relevance (ts_rank).

## Acceptance

- `minuano search "auth middleware"` returns matching context entries.
- Results show which task they belong to.
- Full-text search features work (stemming, ranking).

## Phase

2 — Core CLI Commands

## Depends on

- Task 12
