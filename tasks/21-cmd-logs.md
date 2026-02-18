# Task 21 — `minuano logs`

## Goal

Implement the agent log capture command.

## Steps

1. Add `minuano logs <agent-id>` subcommand.
2. Flag `--lines N` — number of lines to capture (default 50).
3. Use `tmux.CapturePane` to grab the last N lines from the agent's tmux window.
4. Print the captured output to stdout.

## Acceptance

- `minuano logs agent-1` prints the last 50 lines from the agent's tmux pane.
- `--lines 100` captures 100 lines.
- Errors gracefully if the agent or window doesn't exist.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 19
