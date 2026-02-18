# Task 03 — Database Migration File

## Goal

Write the initial SQL migration that creates all tables, indexes, and the readiness trigger.

## Steps

1. Create `internal/db/migrations/001_initial.sql` with the exact schema from DESIGN.md:
   - `tasks` table with all columns and indexes.
   - `task_deps` table with composite primary key and index.
   - `task_context` table with indexes (including GIN full-text).
   - `agents` table.
   - `refresh_ready_tasks()` trigger function and `on_task_done` trigger.
2. The migration should be idempotent or at least safe to apply once on a fresh DB.

## Acceptance

- Applying the SQL against a fresh `minuanodb` succeeds without errors.
- All tables, indexes, and the trigger exist after applying.

## Phase

1 — Foundation

## Depends on

- Task 01
