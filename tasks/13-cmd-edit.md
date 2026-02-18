# Task 13 — `minuano edit`

## Goal

Implement the task body editor command.

## Steps

1. Add `minuano edit <id>` subcommand (supports partial ID).
2. Fetch the current task body from the DB.
3. Write it to a temp file, open `$EDITOR` (fallback to `vi`).
4. On editor exit, read the temp file and update the task body in the DB.
5. Clean up the temp file.

## Acceptance

- `minuano edit <id>` opens the task body in the user's editor.
- Saving and closing the editor updates the DB.
- Closing without changes is a no-op (compare content).

## Phase

2 — Core CLI Commands

## Depends on

- Task 10
