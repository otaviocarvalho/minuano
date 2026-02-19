# Launching Minuano + Tramuntana

This guide explains how **Minuano** (task coordination) and **Tramuntana** (Telegram bridge) work together, what to start first, and how to run agent workflows end-to-end.

---

## Architecture Overview

```
Telegram Group Forum
        │
        ▼
   Tramuntana (Go bot)
        │
        │  CLI exec (minuano status --json, minuano prompt, etc.)
        ▼
   Minuano (Go CLI)
        │
        │  SQL (pgx/v5)
        ▼
   PostgreSQL (Docker)
        │
        ▲
   Agent bash scripts (minuano-claim, minuano-done, etc.)
        │
   Claude Code (inside tmux windows)
```

**Minuano** is the task coordination layer. It manages a PostgreSQL database of tasks, dependencies, and agent state. Agents (Claude Code processes) run in tmux windows and interact with the database through bash scripts.

**Tramuntana** is the Telegram interface. It maps Telegram group forum topics to tmux windows running Claude Code. It talks to Minuano exclusively via CLI commands — no shared DB, no library imports, no IPC.

---

## Startup Order

### 1. Start PostgreSQL (via Minuano)

```bash
cd /path/to/minuano
minuano up
```

This runs `docker compose up -d` using `docker/docker-compose.yml`. PostgreSQL will be available on port 5432.

### 2. Run Migrations

```bash
minuano migrate
```

Creates the `tasks`, `task_deps`, `task_context`, and `agents` tables.

### 3. Add Tasks

```bash
# Add a standalone task
minuano add "Set up auth module" --project auth --priority 8 \
  --body "Implement OAuth2 with JWT tokens" \
  --test-cmd "go test ./internal/auth/..."

# Add a task that depends on another
minuano add "Write auth middleware" --project auth --priority 6 \
  --after set-up-auth \
  --body "HTTP middleware that validates JWT tokens"

# Check what you've added
minuano status --project auth
#   ◎  set-up-auth-a1b    Set up auth module    ready    —    0/3
#   ○  write-auth-mid-c2d  Write auth middleware  pending  —    0/3
```

Tasks start as `pending`. A task with all dependencies met becomes `ready`. The cascade trigger handles this automatically.

### 4. Start Tramuntana (optional — only if using Telegram)

Create a `.env` file for Tramuntana:

```bash
# /path/to/tramuntana/.env
TELEGRAM_BOT_TOKEN=123456:ABC-...
ALLOWED_USERS=your_telegram_user_id
MINUANO_BIN=/path/to/minuano/minuano
MINUANO_DB=postgres://minuano:minuano@localhost:5432/minuanodb?sslmode=disable
TMUX_SESSION_NAME=tramuntana
```

Then start:

```bash
cd /path/to/tramuntana
tramuntana serve
```

---

## Running Agents

### Without Tramuntana (Minuano standalone)

```bash
# Spawn an agent that works through a project autonomously
minuano run --project auth

# Or spawn multiple agents
minuano spawn --project auth --count 3

# Monitor them
minuano agents
minuano watch    # TUI dashboard

# Attach to an agent's tmux window
minuano attach <agent-id>

# View agent output
minuano logs <agent-id>
```

### With Tramuntana (via Telegram)

From a Telegram group forum topic:

```
/project auth              # Bind this topic to the "auth" project
/tasks                     # List current tasks and their status
/pick set-up-auth-a1b      # Send one specific task to Claude
/auto                      # Let Claude work through all ready tasks
/batch task-a task-b       # Send multiple specific tasks in order
```

Behind the scenes, Tramuntana:
1. Calls `minuano prompt single|auto|batch ...` to generate a self-contained prompt
2. Writes the prompt to a temp file (`/tmp/tramuntana-task-XXXX.md`)
3. Sends a reference to the Claude Code process in the topic's tmux window
4. Claude reads the prompt, claims tasks via `minuano-pick` or `minuano-claim`, and works

---

## Agent Task Flow

Regardless of how the agent is started, the workflow is the same:

```
Agent claims task (minuano-claim or minuano-pick)
  │
  ├─ Reads inherited context from dependency tasks
  ├─ Works on the task
  ├─ Records observations: minuano-observe <id> "found X"
  ├─ Records handoffs before risky ops: minuano-handoff <id> "about to refactor Y"
  │
  └─ Completes: minuano-done <id> "implemented feature Z"
       │
       ├─ Tests pass → status = done, cascade triggers dependents to ready
       └─ Tests fail → status = failed, failure context recorded, attempt incremented
```

### Three Agent Modes

| Mode | Trigger | Behavior |
|------|---------|----------|
| **Pick** | `minuano-pick <id>` or `/pick <id>` | Work one specific task, then stop |
| **Auto** | `minuano-claim --project X` or `/auto` | Loop: claim next ready → work → done → repeat until empty |
| **Batch** | `/batch <id1> <id2> ...` | Work through specific tasks in order, then stop |

---

## Environment Variables

### Minuano

| Variable | Default | Description |
|----------|---------|-------------|
| `DATABASE_URL` | (from `--db` flag) | PostgreSQL connection string |
| `MINUANO_SESSION` | `minuano` | tmux session name |

### Agent Scripts (set automatically by `minuano run/spawn`)

| Variable | Description |
|----------|-------------|
| `AGENT_ID` | Unique agent identifier |
| `DATABASE_URL` | PostgreSQL connection string |
| `PATH` | Includes `scripts/` directory |
| `CAPABILITY` | Optional agent capability filter |

### Tramuntana

| Variable | Description |
|----------|-------------|
| `TELEGRAM_BOT_TOKEN` | Bot token from @BotFather |
| `ALLOWED_USERS` | Comma-separated Telegram user IDs |
| `MINUANO_BIN` | Path to the minuano binary |
| `MINUANO_DB` | PostgreSQL connection string for minuano |
| `TMUX_SESSION_NAME` | tmux session name (default: `tramuntana`) |

---

## Example: Full Session

```bash
# Terminal 1 — Start infrastructure
minuano up
minuano migrate

# Add tasks for a project
minuano add "Design database schema" --project api --priority 10 \
  --body "Create PostgreSQL schema for users, sessions, and permissions" \
  --test-cmd "go test ./internal/db/..."

minuano add "Implement user CRUD" --project api --priority 8 \
  --after design-database \
  --body "Create, read, update, delete for users table" \
  --test-cmd "go test ./internal/api/users/..."

minuano add "Add session management" --project api --priority 7 \
  --after design-database \
  --body "JWT session creation, validation, and refresh" \
  --test-cmd "go test ./internal/api/sessions/..."

# Check the tree
minuano tree --project api
#   ◎  design-database-x1    Design database schema (ready)
#   ├── ○  implement-user-y2  Implement user CRUD (pending)
#   └── ○  add-session-z3     Add session management (pending)

# Terminal 2 — Run an agent
minuano run --project api

# Terminal 3 — Monitor
minuano watch

# Or via Telegram:
#   /project api
#   /auto
```

---

## Shutdown

```bash
# Kill all agents
minuano kill --all

# Stop PostgreSQL
minuano down
```

---

## Troubleshooting

- **`DATABASE_URL not set`**: Make sure `minuano up` succeeded and you have a `.env` file or pass `--db`.
- **Tasks stuck in `pending`**: Check dependencies with `minuano tree`. A task only becomes `ready` when all its `--after` dependencies are `done`.
- **Agent not claiming tasks**: Verify the project name matches (`minuano status --project X`). Check capability filters.
- **Tramuntana can't reach Minuano**: Ensure `MINUANO_BIN` points to the built binary and `MINUANO_DB` is correct.
