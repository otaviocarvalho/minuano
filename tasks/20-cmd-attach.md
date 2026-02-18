# Task 20 — `minuano attach`

## Goal

Implement the tmux attach/switch command.

## Steps

1. Add `minuano attach [id]` subcommand.
2. The `id` can be:
   - An agent ID → jump to that agent's tmux window.
   - A task ID (partial) → find which agent owns it, jump to that window.
   - Empty → attach to the tmux session's first window.
3. Use `tmux.AttachSession` if outside tmux, `tmux.SwitchWindow` if inside tmux.

## Acceptance

- `minuano attach agent-1` switches to agent-1's tmux window.
- `minuano attach design-auth` finds the agent working on that task and switches to its window.
- Works correctly both inside and outside tmux.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 19
