# Minuano — Implementation Summary & Validation

## What Was Built

All 26 tasks from `tasks/` have been implemented across 6 phases.

### Phase 1 — Foundation (Tasks 01–05)

| Task | File(s) | Description |
|------|---------|-------------|
| 01 | `go.mod`, `cmd/minuano/main.go`, `.env`, `.gitignore` | Go module init, directory skeleton, env defaults |
| 02 | `docker/docker-compose.yml` | PostgreSQL 16 + optional pgAdmin (debug profile) |
| 03 | `internal/db/migrations/001_initial.sql` | Full schema: `tasks`, `task_deps`, `task_context`, `agents`, indexes, `refresh_ready_tasks()` trigger |
| 04 | `internal/db/db.go` | `pgxpool` connection pool, embedded migration runner with `schema_migrations` tracking |
| 05 | `internal/db/queries.go` | 20+ typed SQL functions: `CreateTask`, `AtomicClaim`, `MarkDone`, `RecordFailure`, `ReclaimStale`, `SearchContext`, agent CRUD, etc. |

### Phase 2 — Core CLI Commands (Tasks 06–14)

| Task | File | Command |
|------|------|---------|
| 06 | `cmd/minuano/main.go` | Root cobra command with `--db` and `--session` flags, `.env` loading |
| 07 | `cmd/minuano/cmd_up.go` | `minuano up` / `minuano down` — Docker lifecycle, auto-detects compose v1/v2 |
| 08 | `cmd/minuano/cmd_migrate.go` | `minuano migrate` — applies pending SQL migrations |
| 09 | `cmd/minuano/cmd_add.go` | `minuano add` — create tasks with `--after`, `--priority`, `--capability`, `--test-cmd` |
| 10 | `cmd/minuano/cmd_status.go` | `minuano status` — table view with status symbols `○ ◎ ● ✓ ✗` |
| 11 | `cmd/minuano/cmd_tree.go` | `minuano tree` — dependency tree with indentation |
| 12 | `cmd/minuano/cmd_show.go` | `minuano show <id>` — task detail + full context log |
| 13 | `cmd/minuano/cmd_edit.go` | `minuano edit <id>` — open task body in `$EDITOR` |
| 14 | `cmd/minuano/cmd_search.go` | `minuano search <query>` — full-text search via GIN index |

### Phase 3 — Tmux & Agent Infrastructure (Tasks 15–16)

| Task | File | Description |
|------|------|-------------|
| 15 | `internal/tmux/tmux.go` | `EnsureSession`, `NewWindow`, `SendKeys`, `CapturePane`, `KillWindow`, `AttachSession`, `SwitchWindow`, auto-detect inside/outside tmux |
| 16 | `internal/agent/agent.go` | `Spawn` (register + tmux window + bootstrap command), `Kill`, `KillAll`, `Heartbeat`, `List` |

### Phase 4 — Agent CLI Commands (Tasks 17–23)

| Task | File | Command |
|------|------|---------|
| 17 | `cmd/minuano/cmd_run.go` | `minuano run` — spawn N agents with `--agents`, `--names`, `--attach` |
| 18 | `cmd/minuano/cmd_spawn.go` | `minuano spawn <name>` — spawn single named agent |
| 19 | `cmd/minuano/cmd_agents.go` | `minuano agents` — agent table, `--watch` launches bubbletea TUI |
| 20 | `cmd/minuano/cmd_attach.go` | `minuano attach [id]` — jump to agent or task owner's tmux window |
| 21 | `cmd/minuano/cmd_logs.go` | `minuano logs <agent-id>` — capture pane output |
| 22 | `cmd/minuano/cmd_kill.go` | `minuano kill [id]` / `--all` — terminate agents, release tasks |
| 23 | `cmd/minuano/cmd_reclaim.go` | `minuano reclaim` — reset stale claimed tasks |

### Phase 5 — Agent Runtime (Tasks 24–25)

| Task | File(s) | Description |
|------|---------|-------------|
| 24 | `scripts/minuano-claim`, `scripts/minuano-done`, `scripts/minuano-observe`, `scripts/minuano-handoff` | Bash scripts for agent task loop, use `quote_literal()` for SQL safety |
| 25 | `claude/CLAUDE.md` | Agent loop instructions: claim → read context → work → observe → handoff → done |

### Phase 6 — Polish (Task 26)

| Task | File | Description |
|------|------|-------------|
| 26 | `internal/tui/tui.go` | Bubbletea live TUI for `minuano agents --watch` — agents table, task summary, 2s refresh |

---

## Validation Steps

### Prerequisites

```bash
# Ensure docker access (one-time)
sudo usermod -aG docker $USER
newgrp docker

# Build the binary
cd /home/otavio/code/minuano
go build ./cmd/minuano
```

### 1. Docker & Database

```bash
# Start postgres
./minuano up
# Expected: "✓ minuano-postgres started (postgres://minuano:minuano@localhost:5432/minuanodb)"

# Run migrations
./minuano migrate
# Expected: "✓ Applied: 001_initial.sql"

# Run again (idempotent)
./minuano migrate
# Expected: "Nothing to apply — all migrations are current."

# Verify tables exist
psql postgres://minuano:minuano@localhost:5432/minuanodb -c "\dt"
# Expected: tasks, task_deps, task_context, agents, schema_migrations
```

### 2. Task Management

```bash
# Create tasks
./minuano add "Design auth system" --priority 8
# Expected: "Created: design-auth-s...  "Design auth system""

./minuano add "Implement auth endpoints" --after design-auth --test-cmd "go test ./internal/auth/..."
# Expected: "Created: implement-au...  "Implement auth endpoints""

./minuano add "Write auth integration tests" --after implement-au
# Expected: "Created: write-auth-i...  "Write auth integration tests""

# View status table
./minuano status
# Expected: table with ◎ (ready) for first task, ○ (pending) for dependent tasks

# View dependency tree
./minuano tree
# Expected: tree showing design-auth → implement-auth → write-auth

# Show task details
./minuano show design-auth
# Expected: full task header, body, and context log

# Edit task body
./minuano edit design-auth
# Expected: opens $EDITOR with task body, saves on close
```

### 3. Agent Lifecycle

```bash
# Spawn agents (requires tmux installed)
./minuano run --agents 2 --names alpha,beta

# Check agents
./minuano agents
# Expected: table showing alpha and beta agents

# Watch mode (bubbletea TUI)
./minuano agents --watch
# Expected: live-updating TUI, press q to quit

# Capture logs from an agent
./minuano logs alpha

# Attach to agent window
./minuano attach alpha

# Kill a single agent
./minuano kill alpha

# Kill all agents
./minuano kill --all
```

### 4. Task Reclaim

```bash
# Reclaim stale tasks (if any were claimed > 30 min ago)
./minuano reclaim --minutes 30
```

### 5. Full-Text Search

```bash
# First add some context manually via psql or scripts, then:
./minuano search "auth middleware"
```

### 6. Agent Scripts (manual test)

```bash
export AGENT_ID=test-agent
export DATABASE_URL=postgres://minuano:minuano@localhost:5432/minuanodb

# Register a test agent in DB first via psql:
psql "$DATABASE_URL" -c "INSERT INTO agents (id, tmux_session, tmux_window) VALUES ('test-agent', 'test', 'test')"

# Claim a task
./scripts/minuano-claim
# Expected: JSON output with task spec, or empty if no ready tasks

# Write an observation
./scripts/minuano-observe <task-id> "found something interesting"

# Write a handoff
./scripts/minuano-handoff <task-id> "completed step 1, next: step 2"

# Mark done (runs tests)
./scripts/minuano-done <task-id> "implemented the feature"
```

### 7. Cleanup

```bash
./minuano kill --all
./minuano down
# Expected: "✓ minuano-postgres stopped"
```

---

## Known Requirements for Full Testing

- **Docker**: user must be in the `docker` group or use `sudo`
- **tmux**: must be installed for agent spawn/attach/logs commands
- **psql**: must be installed for agent bash scripts
- **claude**: Claude Code CLI must be installed for actual agent execution
