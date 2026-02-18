# Task 19 — `minuano agents`

## Goal

Implement the agent status display command.

## Steps

1. Add `minuano agents` subcommand.
2. Display a table with columns: status symbol, agent ID, status, task ID, task title, last seen (relative time).
3. Use `● ○` for working/idle agents.
4. Flag `--watch` — refresh every 2s (clear + reprint, or use bubbletea from Task 26).
5. For now, `--watch` can be a simple poll loop with terminal clear.

## Acceptance

- `minuano agents` shows a table of all registered agents.
- Each agent shows its current task (if any).
- `--watch` refreshes the display every 2 seconds.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 17
