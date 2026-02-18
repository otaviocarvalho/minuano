# Task 18 — `minuano spawn`

## Goal

Implement the single-agent spawn command.

## Steps

1. Add `minuano spawn <name>` subcommand.
2. Flags:
   - `--capability X` — agent capability.
3. Ensure the tmux session exists.
4. Spawn one agent via `agent.Spawn` with the given name.
5. Print the agent ID and tmux window.

## Acceptance

- `minuano spawn myagent` creates a single named agent.
- The agent appears in `minuano agents` output.
- The tmux window is created and the bootstrap command is sent.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 17
