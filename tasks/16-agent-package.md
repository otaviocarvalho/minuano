# Task 16 — Agent Package

## Goal

Implement `internal/agent/agent.go` with agent lifecycle management.

## Steps

1. Create `internal/agent/agent.go`.
2. Define the `Agent` struct matching DESIGN.md.
3. Implement:
   - `Spawn(pool, tmuxSession, agentID, claudeMDPath string, env map[string]string) (*Agent, error)`
     - Registers agent in DB.
     - Creates tmux window via `tmux.NewWindow`.
     - Sends the bootstrap command to the window (export vars + `claude --dangerously-skip-permissions -p "$(cat <CLAUDE.md>)"`).
     - Returns immediately (does not wait for task claim).
   - `Kill(pool, tmuxClient, agentID string) error` — kills tmux window, releases claimed task (reset to ready), deletes agent from DB.
   - `KillAll(pool, tmuxClient) error` — kills all agents.
   - `Heartbeat(pool, agentID, status string) error` — updates `last_seen` and `status`.
   - `List(pool) ([]*Agent, error)` — lists all registered agents with their current task.
4. The bootstrap command must set `AGENT_ID`, `DATABASE_URL`, and `PATH` (including scripts dir).

## Acceptance

- `Spawn` creates a tmux window and sends the bootstrap command.
- `Kill` cleans up both tmux and DB state.
- `List` returns agents with their task assignments.

## Phase

3 — Tmux & Agent Infrastructure

## Depends on

- Task 15
- Task 05
