# Task 09 — `minuano add`

## Goal

Implement the task creation command.

## Steps

1. Add `minuano add <title>` subcommand.
2. Flags:
   - `--after <id>` — adds a dependency (partial ID matching). Repeatable for multiple deps.
   - `--priority N` — 0-10, default 5.
   - `--capability X` — required agent capability.
   - `--test-cmd X` — stored in `metadata.test_cmd`.
   - `--project` — project ID (or read from `MINUANO_PROJECT` env).
   - `--body` or read from stdin for the task body.
3. Generate a short, human-friendly ID: slug from title + random suffix (e.g., `design-auth-a1b2c`).
4. If the task has no dependencies (or all deps are `done`), set status to `ready`. Otherwise `pending`.
5. Print the created task ID and title.

## Acceptance

- `minuano add "Design auth system" --priority 8` creates a task and prints its ID.
- `minuano add "Implement endpoints" --after design-auth` creates a dependent task.
- Partial ID matching works for `--after`.
- Tasks without unmet deps are created as `ready`.

## Phase

2 — Core CLI Commands

## Depends on

- Task 08
