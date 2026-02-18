# Minuano — Agent Task Coordination

A Go CLI that coordinates Claude Code agents via tmux, using PostgreSQL (Docker) as the
coordination substrate. Inspired by Beads/Gas Town but built around a pull-based Linda
tuple-space model: agents atomically claim tasks from a shared queue, inherit context
from dependency tasks, run tests locally as the completion gate, and write persistent
observations back to the database across context resets.

---

## Principles

- **Pull, never push.** Agents claim work themselves. No orchestrator, no nudge system,
  no GUPP violations. The database drives the loop.
- **Tests are the gate.** A task is only `done` when tests pass locally. The agent owns
  the full cycle: write code → run tests → fix failures → repeat.
- **Context survives resets.** Agents write observations and handoff notes to the DB
  incrementally. A new session picking up a stale task reads the full history of what
  was tried and what broke.
- **Zero background token burn.** No patrol agents, no Mayor session, no heartbeat
  Claude invocations. Every token spent is an agent doing real work.
- **One binary, one command.** The entire system is operated through a single `minuano` CLI.

---

## Repository Layout

```
minuano/
├── cmd/
│   └── minuano/
│       └── main.go              # entry point, cobra root command (minuano)
├── internal/
│   ├── db/
│   │   ├── db.go                # connection pool, migrations runner
│   │   ├── queries.go           # all SQL (no ORM)
│   │   └── migrations/
│   │       └── 001_initial.sql
│   ├── tmux/
│   │   └── tmux.go              # tmux session/window management
│   ├── agent/
│   │   └── agent.go             # agent lifecycle: spawn, heartbeat, kill
│   └── tui/
│       └── tui.go               # optional: bubbletea watch view
├── scripts/
│   ├── minuano-claim               # bash: called by CLAUDE.md loop
│   ├── minuano-done                # bash: runs tests, marks done or resets
│   ├── minuano-observe             # bash: writes observation to DB
│   └── minuano-handoff             # bash: writes handoff note to DB
├── claude/
│   └── CLAUDE.md                # agent loop instructions
├── docker/
│   └── docker-compose.yml       # postgres + optional pgAdmin
├── go.mod
├── go.sum
└── DESIGN.md                    # this file
```

---

## Docker Setup

### `docker/docker-compose.yml`

```yaml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    container_name: minuano-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB:       minuanodb
      POSTGRES_USER:     minuano
      POSTGRES_PASSWORD: minuano
    ports:
      - "5432:5432"
    volumes:
      - minuano-pgdata:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U minuano -d minuanodb"]
      interval: 5s
      timeout: 3s
      retries: 10

  pgadmin:
    image: dpage/pgadmin4:latest
    container_name: minuano-pgadmin
    restart: unless-stopped
    profiles: ["debug"]           # only starts with: docker compose --profile debug up
    environment:
      PGADMIN_DEFAULT_EMAIL:    admin@minuano.local
      PGADMIN_DEFAULT_PASSWORD: admin
    ports:
      - "5050:80"
    depends_on:
      postgres:
        condition: service_healthy

volumes:
  minuano-pgdata:
```

### Environment

```bash
# .env  (gitignored)
DATABASE_URL=postgres://minuano:minuano@localhost:5432/minuanodb?sslmode=disable
MINUANO_SESSION=minuano
```

The CLI reads `DATABASE_URL` from environment or `--db` flag. All commands work if the
container is running; `minuano up` / `minuano down` manage the container lifecycle.

---

## Database Schema

### `internal/db/migrations/001_initial.sql`

```sql
-- Tasks: the work units
CREATE TABLE tasks (
  id           TEXT PRIMARY KEY,
  title        TEXT        NOT NULL,
  body         TEXT        NOT NULL DEFAULT '',
  status       TEXT        NOT NULL DEFAULT 'pending',
    -- pending | ready | claimed | done | failed
  priority     INTEGER     NOT NULL DEFAULT 5,
  capability   TEXT,                          -- NULL = any agent
  claimed_by   TEXT,
  claimed_at   TIMESTAMPTZ,
  done_at      TIMESTAMPTZ,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  attempt      INTEGER     NOT NULL DEFAULT 0,
  max_attempts INTEGER     NOT NULL DEFAULT 3,
  project_id   TEXT,
  metadata     JSONB                          -- e.g. {"test_cmd": "go test ./..."}
);

CREATE INDEX idx_tasks_status   ON tasks(status, priority DESC, created_at ASC);
CREATE INDEX idx_tasks_project  ON tasks(project_id) WHERE project_id IS NOT NULL;

-- Dependencies: the DAG
CREATE TABLE task_deps (
  task_id    TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  depends_on TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  PRIMARY KEY (task_id, depends_on)
);

CREATE INDEX idx_deps_depends_on ON task_deps(depends_on);

-- Context: persistent agent memory
CREATE TABLE task_context (
  id          BIGSERIAL   PRIMARY KEY,
  task_id     TEXT        NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  agent_id    TEXT,
  kind        TEXT        NOT NULL,
    -- observation | result | handoff | inherited | test_failure
  content     TEXT        NOT NULL,
  source_task TEXT        REFERENCES tasks(id),   -- set for kind=inherited
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_context_task ON task_context(task_id, created_at);
CREATE INDEX idx_context_fts  ON task_context
  USING gin(to_tsvector('english', content));

-- Agents: running instance registry
CREATE TABLE agents (
  id           TEXT        PRIMARY KEY,
  tmux_session TEXT        NOT NULL,
  tmux_window  TEXT        NOT NULL,
  task_id      TEXT        REFERENCES tasks(id),
  status       TEXT        NOT NULL DEFAULT 'idle',
    -- idle | working | dead
  started_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen    TIMESTAMPTZ
);

-- Trigger: when a task is marked done, cascade readiness to dependents
CREATE OR REPLACE FUNCTION refresh_ready_tasks()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'done' THEN
    UPDATE tasks
    SET status = 'ready'
    WHERE status = 'pending'
      AND id IN (
        SELECT task_id FROM task_deps WHERE depends_on = NEW.id
      )
      AND id NOT IN (
        SELECT td.task_id
        FROM task_deps td
        JOIN tasks t ON t.id = td.depends_on
        WHERE t.status != 'done'
      );
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER on_task_done
AFTER UPDATE OF status ON tasks
FOR EACH ROW
WHEN (NEW.status = 'done')
EXECUTE FUNCTION refresh_ready_tasks();
```

---

## Core Operations (SQL)

These live in `internal/db/queries.go` as named functions called by CLI commands.

### Atomic Claim (Linda `in()`)

```sql
-- AtomicClaim: claim one ready task and inject inherited context in one transaction
WITH claimed AS (
  UPDATE tasks
  SET    status     = 'claimed',
         claimed_by = $1,           -- agent_id
         claimed_at = NOW(),
         attempt    = attempt + 1
  WHERE  id = (
    SELECT id FROM tasks
    WHERE  status = 'ready'
      AND  (capability IS NULL OR capability = $2)   -- $2 = agent capability
      AND  attempt < max_attempts
    ORDER  BY priority DESC, created_at ASC
    LIMIT  1
    FOR UPDATE SKIP LOCKED
  )
  RETURNING *
),
inherited AS (
  INSERT INTO task_context (task_id, agent_id, kind, content, source_task)
  SELECT c.id, $1, 'inherited', tc.content, tc.task_id
  FROM   claimed c
  JOIN   task_deps td    ON td.task_id    = c.id
  JOIN   task_context tc ON tc.task_id   = td.depends_on
                        AND tc.kind IN ('result', 'observation', 'handoff', 'test_failure')
  JOIN   tasks dep       ON dep.id        = td.depends_on
                        AND dep.status    = 'done'
  RETURNING task_id
),
agent_update AS (
  UPDATE agents
  SET    task_id   = (SELECT id FROM claimed),
         status    = 'working',
         last_seen = NOW()
  WHERE  id = $1
)
SELECT row_to_json(t) FROM (
  SELECT c.*,
    (SELECT json_agg(tc ORDER BY tc.created_at)
     FROM   task_context tc
     WHERE  tc.task_id = c.id) AS context
  FROM claimed c
) t;
```

### Complete Task (tests passed)

```sql
-- MarkDone: agent calls this after tests pass
WITH ctx AS (
  INSERT INTO task_context (task_id, agent_id, kind, content)
  VALUES ($1, $2, 'result', $3)     -- $3 = result summary
)
UPDATE tasks
SET    status     = 'done',
       done_at    = NOW(),
       claimed_by = NULL,
       claimed_at = NULL
WHERE  id         = $1
  AND  claimed_by = $2;
-- Trigger fires here: cascades readiness to dependents
```

### Record Test Failure (reset to ready)

```sql
-- RecordFailure: called by minuano-done script when tests fail
WITH ctx AS (
  INSERT INTO task_context (task_id, agent_id, kind, content)
  VALUES ($1, $2, 'test_failure', $3)   -- $3 = truncated test output
)
UPDATE tasks
SET    status     = 'ready',
       claimed_by = NULL,
       claimed_at = NULL
WHERE  id         = $1
  AND  claimed_by = $2
  AND  attempt    < max_attempts;

-- If attempt >= max_attempts, mark failed instead:
UPDATE tasks SET status = 'failed'
WHERE  id = $1 AND attempt >= max_attempts;
```

### Stale Claim Recovery

```sql
-- Reclaim: run by `minuano reclaim` or a cron
UPDATE tasks
SET    status     = 'ready',
       claimed_by = NULL,
       claimed_at = NULL
WHERE  status     = 'claimed'
  AND  claimed_at < NOW() - make_interval(mins => $1);  -- $1 = timeout minutes
```

---

## Go CLI Commands

### Command Tree

```
minuano
├── up                    Start Docker postgres container
├── down                  Stop Docker postgres container
├── migrate               Run pending migrations
│
├── add <title>           Create a task
│   ├── --after <id>      Add dependency (partial id ok)
│   ├── --priority N      0-10, default 5
│   ├── --capability X    Required agent capability
│   └── --test-cmd X      Test command override (default: from metadata)
│
├── edit <id>             Open task body in $EDITOR
├── show <id>             Print task spec + full context log
├── tree                  Print dependency tree with status symbols
├── status                Table view of all tasks
├── search <query>        Full-text search across task context
│
├── run                   Spawn agents in tmux
│   ├── --agents N        Number of agents (default: 1)
│   ├── --names a,b,c     Named agents instead of auto-generated IDs
│   ├── --capability X    All spawned agents get this capability
│   └── --attach          Attach to tmux session after spawning
│
├── agents                Show running agents + what they own
│   └── --watch           Refresh every 2s
│
├── attach [id]           Attach to tmux session or jump to agent/task window
├── logs <agent-id>       Capture last N lines from agent's tmux window
│   └── --lines N         Default: 50
│
├── spawn <name>          Spawn a single named agent
│   └── --capability X
│
├── kill [agent-id]       Kill agent(s), release their claimed tasks
│   └── --all
│
└── reclaim               Reset stale claimed tasks back to ready
    └── --minutes N       Stale threshold (default: 30)
```

### Key Packages

```
cobra                    github.com/spf13/cobra       CLI framework
pgx/v5                   github.com/jackc/pgx/v5      Postgres driver (no ORM)
bubbletea                github.com/charmbracelet/bubbletea   TUI watch views
lipgloss                 github.com/charmbracelet/lipgloss    Terminal styling
tablewriter              github.com/olekuznetsov/go-table     Status tables
godotenv                 github.com/joho/godotenv     .env loading
```

---

## Agent Scripts

These live in `scripts/` and are called from `CLAUDE.md`. They are thin wrappers
over `psql` so they work without the Go binary running inside the agent session.

### `scripts/minuano-claim`

```bash
#!/usr/bin/env bash
# Atomically claim one ready task. Prints JSON or exits 0 with no output.
set -euo pipefail

AGENT_ID="${AGENT_ID:?AGENT_ID not set}"
CAPABILITY="${CAPABILITY:-}"
DB="${DATABASE_URL:?DATABASE_URL not set}"

psql "$DB" -t -A -c "
  WITH claimed AS (
    UPDATE tasks
    SET status='claimed', claimed_by='$AGENT_ID', claimed_at=NOW(),
        attempt = attempt + 1
    WHERE id=(
      SELECT id FROM tasks
      WHERE status='ready'
        AND (capability IS NULL OR capability='$CAPABILITY')
        AND attempt < max_attempts
      ORDER BY priority DESC, created_at ASC
      LIMIT 1 FOR UPDATE SKIP LOCKED
    )
    RETURNING *
  ),
  inherited AS (
    INSERT INTO task_context (task_id, agent_id, kind, content, source_task)
    SELECT c.id, '$AGENT_ID', 'inherited', tc.content, tc.task_id
    FROM claimed c
    JOIN task_deps td ON td.task_id = c.id
    JOIN task_context tc ON tc.task_id = td.depends_on
      AND tc.kind IN ('result','observation','handoff','test_failure')
    JOIN tasks dep ON dep.id = td.depends_on AND dep.status='done'
    RETURNING task_id
  ),
  agent_upd AS (
    UPDATE agents SET task_id=(SELECT id FROM claimed),
      status='working', last_seen=NOW() WHERE id='$AGENT_ID'
  )
  SELECT row_to_json(t) FROM (
    SELECT c.*,
      (SELECT json_agg(tc ORDER BY tc.created_at)
       FROM task_context tc WHERE tc.task_id = c.id) AS context
    FROM claimed c
  ) t;
"
```

### `scripts/minuano-done`

```bash
#!/usr/bin/env bash
# Run tests. On pass: mark done. On fail: write failure to context, reset to ready.
set -euo pipefail

TASK_ID="${1:?Usage: minuano-done <id> <summary>}"
SUMMARY="${2:?Usage: minuano-done <id> <summary>}"
AGENT_ID="${AGENT_ID:?AGENT_ID not set}"
DB="${DATABASE_URL:?DATABASE_URL not set}"

# Resolve test command: from task metadata, env, or default
TEST_CMD=$(psql "$DB" -t -A -c \
  "SELECT COALESCE(metadata->>'test_cmd', 'go test ./...') FROM tasks WHERE id='$TASK_ID'")
TEST_CMD="${MINUANO_TEST_CMD:-$TEST_CMD}"

echo "▶ Running: $TEST_CMD"
TEST_OUTPUT=$($TEST_CMD 2>&1) && EXIT_CODE=0 || EXIT_CODE=$?

if [ "$EXIT_CODE" -eq 0 ]; then
  psql "$DB" -c "
    WITH ctx AS (
      INSERT INTO task_context (task_id, agent_id, kind, content)
      VALUES ('$TASK_ID', '$AGENT_ID', 'result',
              $(printf '%s' "$SUMMARY" | psql "$DB" -c "SELECT quote_literal('\$1')" -t -A))
    )
    UPDATE tasks SET status='done', done_at=NOW(), claimed_by=NULL, claimed_at=NULL
    WHERE id='$TASK_ID' AND claimed_by='$AGENT_ID';

    UPDATE agents SET task_id=NULL, status='idle', last_seen=NOW()
    WHERE id='$AGENT_ID';
  "
  echo "✓ Done: $TASK_ID"
else
  ATTEMPT=$(psql "$DB" -t -A -c "SELECT attempt FROM tasks WHERE id='$TASK_ID'")
  MAX=$(psql "$DB" -t -A -c "SELECT max_attempts FROM tasks WHERE id='$TASK_ID'")
  TRUNCATED=$(echo "$TEST_OUTPUT" | tail -80)

  psql "$DB" -c "
    INSERT INTO task_context (task_id, agent_id, kind, content)
    VALUES ('$TASK_ID', '$AGENT_ID', 'test_failure',
      'Attempt $ATTEMPT/$MAX failed. Command: $TEST_CMD

$TRUNCATED');
  "

  if [ "$ATTEMPT" -ge "$MAX" ]; then
    psql "$DB" -c "UPDATE tasks SET status='failed' WHERE id='$TASK_ID';"
    echo "✗ Failed after $ATTEMPT attempts: $TASK_ID"
    exit 1
  else
    psql "$DB" -c "
      UPDATE tasks SET status='ready', claimed_by=NULL, claimed_at=NULL
      WHERE id='$TASK_ID' AND claimed_by='$AGENT_ID';

      UPDATE agents SET task_id=NULL, status='idle', last_seen=NOW()
      WHERE id='$AGENT_ID';
    "
    echo "⚠ Tests failed (attempt $ATTEMPT/$MAX). Task reset to ready."
    exit 1
  fi
fi
```

### `scripts/minuano-observe`

```bash
#!/usr/bin/env bash
TASK_ID="${1:?Usage: minuano-observe <id> <note>}"
NOTE="${2:?Usage: minuano-observe <id> <note>}"
psql "${DATABASE_URL:?}" -c "
  INSERT INTO task_context (task_id, agent_id, kind, content)
  VALUES ('$TASK_ID', '${AGENT_ID:-unknown}', 'observation', $(printf '%s' "$NOTE" | psql "$DATABASE_URL" -c "SELECT quote_literal('\$1')" -t -A));
"
```

### `scripts/minuano-handoff`

```bash
#!/usr/bin/env bash
TASK_ID="${1:?Usage: minuano-handoff <id> <note>}"
NOTE="${2:?Usage: minuano-handoff <id> <note>}"
psql "${DATABASE_URL:?}" -c "
  INSERT INTO task_context (task_id, agent_id, kind, content)
  VALUES ('$TASK_ID', '${AGENT_ID:-unknown}', 'handoff', $(printf '%s' "$NOTE" | psql "$DATABASE_URL" -c "SELECT quote_literal('\$1')" -t -A));
"
```

---

## CLAUDE.md (Agent Loop)

```markdown
# Agent Task Loop

You are an autonomous coding agent. Work through the task queue without waiting
for human input. Exit cleanly when the queue is empty.

## Setup

Scripts are in ./scripts/ relative to this file. Add to PATH for this session:
export PATH="$PATH:$(cd "$(dirname "$0")/.." && pwd)/scripts"

Your agent ID is in $AGENT_ID. Your database is in $DATABASE_URL.

## Loop

1. **Claim**: Run `minuano-claim`
   - Prints JSON: your task spec + context array.
   - No output: queue empty or nothing ready. Run `minuano heartbeat idle` then exit.

2. **Read context** from the JSON:
   - `body`: your complete specification
   - `context[].kind == "inherited"`: findings from dependency tasks — read these first
   - `context[].kind == "handoff"`: where a previous attempt left off on this task
   - `context[].kind == "test_failure"`: what broke last time — fix exactly this

3. **Write observations** as you discover things:
   `minuano-observe <id> "auth middleware expects Bearer not X-Auth-Token"`

4. **Write a handoff** before any long operation or risky change:
   `minuano-handoff <id> "completed token generation at src/auth/token.go, next: refresh endpoint"`

5. **Submit**: When you believe the work is complete:
   `minuano-done <id> "brief summary of what was built"`
   - Tests pass → task marked done, loop back to step 1
   - Tests fail → failure written to context, task reset to ready, loop back to step 1
     (you or another agent will fix it with the failure logs as context)

## Rules

- Never mark a task done without calling `minuano-done`. It runs the tests.
- If you see a `test_failure` context entry: fix only what broke. Do not rewrite unrelated code.
- If after reading failure logs you genuinely cannot determine the fix, write a detailed
  handoff note explaining what you tried, then call `minuano-done` anyway to record the
  failure cleanly. A human will triage tasks that reach max_attempts.
- One task per outer loop iteration. Exit after `minuano-done` returns success.
- Do not ask for clarification. Make a reasonable interpretation, note it as an observation,
  proceed.
```

---

## Go Implementation Notes

### `internal/db/db.go`

- Use `pgx/v5` with a connection pool (`pgxpool.New`)
- Run migrations on `minuano migrate` or automatically on `minuano up`
- All queries in `queries.go` as typed functions returning structs — no raw SQL in
  command files
- Use `pgx` named arguments (`@agent_id`) for clarity on multi-param queries

### `internal/tmux/tmux.go`

```go
// Key functions
func SessionExists(name string) bool
func EnsureSession(name string) error       // creates if missing, no-op if exists
func WindowExists(session, window string) bool
func NewWindow(session, window string, env map[string]string) error
func SendKeys(session, window, keys string) error
func CapturePane(session, window string, lines int) (string, error)
func KillWindow(session, window string) error
func AttachSession(session, window string) error  // execlp replacement
func SwitchWindow(session, window string) error   // when already inside tmux
```

Detect whether the CLI is running inside tmux via `os.Getenv("TMUX") != ""` to decide
between `attach-session` and `switch-window`.

### `internal/agent/agent.go`

```go
type Agent struct {
  ID          string
  TmuxSession string
  TmuxWindow  string
  TaskID      *string
  Status      string
  StartedAt   time.Time
  LastSeen    *time.Time
}

func Spawn(db *pgxpool.Pool, tmuxSession, agentID, claudeMD string,
           env map[string]string) (*Agent, error)
func Kill(db *pgxpool.Pool, tmux *tmux.Client, agentID string) error
func KillAll(db *pgxpool.Pool, tmux *tmux.Client) error
func Heartbeat(db *pgxpool.Pool, agentID, status string) error
func List(db *pgxpool.Pool) ([]*Agent, error)
```

`Spawn` registers the agent in the DB, creates the tmux window, sends the bootstrap
command, and returns. It does not wait for the agent to claim a task.

The bootstrap command sent to the tmux window:

```bash
export AGENT_ID="<id>"
export DATABASE_URL="<url>"
export PATH="$PATH:<abs-path-to-scripts>"
claude --dangerously-skip-permissions -p "$(cat <abs-path-to-CLAUDE.md>)"
```

### `cmd/minuano/main.go`

Use `cobra` with persistent flags for `--db` (DATABASE_URL override) and
`--session` (tmux session name override). Load `.env` via `godotenv.Load()` before
cobra parses args.

---

## Status Symbols (used in `minuano status` and `minuano tree`)

```
○  pending      deps not yet done
◎  ready        available to claim
●  claimed      agent is working (shows agent name + elapsed time)
✓  done         tests passed, merged
✗  failed       hit max_attempts, needs human
```

---

## Full CLI Session Example

```bash
# Start postgres
minuano up
# ✓ minuano-postgres started (postgres://minuano:minuano@localhost:5432/minuanodb)

# Run migrations
minuano migrate
# ✓ Applied: 001_initial.sql

# Describe a project
export MINUANO_PROJECT=auth-system

minuano add "Design auth system" --priority 8
# Created: design-auth-a1b2c  "Design auth system"

minuano add "Implement auth endpoints" --after design-auth --test-cmd "go test ./internal/auth/..."
# Created: implement-en-d3e4f  "Implement auth endpoints"

minuano add "Write auth integration tests" --after implement-en
# Created: write-auth-g5h6i  "Write auth integration tests"

minuano edit design-auth      # opens $EDITOR with the task body

minuano tree
#   ◎  design-auth-a1b2c    Design auth system
#     ○  implement-en-d3e4f  Implement auth endpoints
#       ○  write-auth-g5h6i  Write auth integration tests

# Launch 2 agents
minuano run --agents 2 --attach
# Spawned: agent-291847-1  →  minuano:agent-291847-1
# Spawned: agent-291847-2  →  minuano:agent-291847-2
# [tmux attaches]

# Inside tmux — your windows:
# control | agent-291847-1 | agent-291847-2

# In the control window:
minuano agents --watch
#  AGENT               STATUS    TASK                  TITLE                          LAST SEEN
#  ────────────────────────────────────────────────────────────────────────────────────────────
#  ● agent-291847-1    working   design-auth-a1b2c     Design auth system             2s ago
#  ○ agent-291847-2    idle      —                     —                              5s ago

# Jump to agent-1's window to watch
minuano attach agent-291847-1

# Or jump to whichever agent owns a task
minuano attach design-auth

# Back in control — agent-1 called minuano-done, tests passed
minuano agents
#  ✓ design-auth done → implement-en now ready
#  ● agent-291847-1    working   implement-en-d3e4f    Implement auth endpoints       1s ago
#  ● agent-291847-2    working   —                     —                              3s ago

# Something running 45 minutes — reclaim
minuano reclaim --minutes 45
# Reclaimed 1 stale task: implement-en-d3e4f

# Read what happened before the stall
minuano show implement-en
# ── Task: implement-en-d3e4f ──────────────────────────────────────────────
# Title:  Implement auth endpoints
# Status: ready (attempt 1/3)
#
# Body:
# Implement JWT auth endpoints...
#
# ── Context ───────────────────────────────────────────────────────────────
# [14:23:11] INHERITED  (agent-291847-2)  from: design-auth-a1b2c
#   Auth system: JWT HS256, expiry 3600000ms, refresh at /api/auth/refresh
#
# [14:31:44] OBSERVATION  (agent-291847-2)
#   Middleware reads Authorization header, expects "Bearer <token>" format
#
# [14:38:02] HANDOFF  (agent-291847-2)
#   Token generation done at internal/auth/token.go. Next: wire refresh
#   endpoint. Schema in db/migrations/003_tokens.sql.

# Agents auto-pick it up, read the handoff, continue

minuano status
#   ✓  design-auth-a1b2c    Design auth system              done
#   ✓  implement-en-d3e4f   Implement auth endpoints         done
#   ✓  write-auth-g5h6i     Write auth integration tests     done

# Clean up
minuano kill --all
minuano down
```

---

## What This Does Not Include (Intentional)

| Missing | Why |
|---|---|
| Remote CI / PR lifecycle | Tests run locally; no GitHub dependency |
| Mayor / orchestrator agent | The DB trigger does this mechanically |
| Patrol agents (Witness, Deacon) | `minuano reclaim` + cron replaces them at zero token cost |
| Agent CV / identity chains | `task_context` gives per-task history; cross-task identity is `--names` |
| Formula templates | Add later: `minuano template apply <name>` generating a task graph from TOML |
| Dynamic task spawning mid-work | Add later: agents call `minuano add` if they discover sub-work |
| Gates / human approval steps | Add later: `--gate` flag on `minuano add` pauses at that node |
| Multi-machine distribution | SQLite + `BEGIN IMMEDIATE` for single machine; this design is Postgres-only |

---

## Getting Started for Claude Code

1. Read this file fully before writing any code.
2. Initialize the Go module: `go mod init github.com/yourname/minuano`
3. Start with `docker/docker-compose.yml` and `internal/db/migrations/001_initial.sql`.
4. Build `minuano up`, `minuano migrate`, `minuano add`, `minuano status`, `minuano tree` first —
   verify the DB layer works before touching tmux.
5. Build `internal/tmux/tmux.go` next, test with `minuano run --agents 1`.
6. Build `minuano attach`, `minuano agents`, `minuano logs`, `minuano kill` last.
7. The `scripts/` bash files should be written and tested manually before wiring them
   into `CLAUDE.md`.
8. Do not add an ORM. All SQL lives in `internal/db/queries.go` as named functions.
9. Do not add runtime dependencies beyond the listed packages without a clear reason.
