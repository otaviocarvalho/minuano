# Task 12 — `minuano show`

## Goal

Implement the task detail view with full context log.

## Steps

1. Add `minuano show <id>` subcommand (supports partial ID).
2. Display:
   - Task header: ID, title, status, attempt count, priority, capability.
   - Body: the full task specification.
   - Context log: all `task_context` entries ordered by `created_at`, showing timestamp, kind, agent_id, source_task (if inherited), and content.
3. Format the output like the example in DESIGN.md (boxed sections with headers).

## Acceptance

- `minuano show <partial-id>` displays the task with all its context.
- Output matches the format shown in DESIGN.md's session example.
- Partial ID matching works.

## Phase

2 — Core CLI Commands

## Depends on

- Task 10
