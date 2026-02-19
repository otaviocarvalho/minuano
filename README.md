# Minuano

> Minuano is a wind that cleans the sky and the pocket

A Go CLI that coordinates [Claude Code](https://docs.anthropic.com/en/docs/claude-code) agents via tmux, using PostgreSQL as the coordination substrate.

Agents atomically claim tasks from a shared queue, inherit context from dependency tasks, run tests locally as the completion gate, and write persistent observations back to the database across context resets. No orchestrator, no background token burn — the database drives the loop.

## How it works

1. You describe work as tasks with dependencies (a DAG).
2. `minuano run` spawns Claude Code agents in tmux windows.
3. Each agent pulls a ready task, reads inherited context, writes code, runs tests, and marks it done.
4. When a task completes, its dependents become ready automatically (via a Postgres trigger).
5. Agents loop until the queue is empty.

## Quick start

```bash
go build ./cmd/minuano

minuano up                # start postgres (Docker)
minuano migrate           # apply schema

minuano add "Design auth system" --priority 8
minuano add "Implement auth endpoints" --after design-auth
minuano tree              # visualize the DAG

minuano run --agents 2    # spawn agents in tmux
minuano agents --watch    # monitor progress
```

## Commands

```
minuano up              Start Docker postgres container
minuano down            Stop Docker postgres container
minuano migrate         Run pending migrations

minuano add <title>     Create a task (--after, --priority, --capability, --test-cmd)
minuano edit <id>       Open task body in $EDITOR
minuano show <id>       Print task spec + full context log
minuano tree            Print dependency tree with status symbols
minuano status          Table view of all tasks
minuano search <query>  Full-text search across task context

minuano run             Spawn agents in tmux (--agents N, --attach)
minuano spawn <name>    Spawn a single named agent
minuano agents          Show running agents (--watch)
minuano attach [id]     Attach to tmux session or jump to agent window
minuano logs <id>       Capture lines from agent's tmux pane
minuano kill [id]       Kill agent(s), release claimed tasks (--all)
minuano reclaim         Reset stale claimed tasks back to ready
```

## Requirements

- Go 1.24+
- Docker (for PostgreSQL)
- tmux
- [Claude Code](https://docs.anthropic.com/en/docs/claude-code) CLI

## License

MIT
