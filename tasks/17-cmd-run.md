# Task 17 — `minuano run`

## Goal

Implement the multi-agent spawn command.

## Steps

1. Add `minuano run` subcommand.
2. Flags:
   - `--agents N` — number of agents to spawn (default 1).
   - `--names a,b,c` — named agents instead of auto-generated IDs.
   - `--capability X` — all spawned agents get this capability.
   - `--attach` — attach to tmux session after spawning.
3. Ensure the tmux session exists (create if needed).
4. Spawn N agents via `agent.Spawn`, each in its own tmux window.
5. Print each spawned agent's ID and tmux window name.
6. If `--attach`, call tmux attach at the end.

## Acceptance

- `minuano run --agents 2` spawns 2 agents in separate tmux windows.
- `minuano run --names alpha,beta` spawns agents with those names.
- `--attach` brings the user into the tmux session.
- Agents are registered in the DB.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 16
- Task 08
