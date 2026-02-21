# LISTEN/NOTIFY Plan: Event-Driven Coordination

Bring Minuano closer to Linda's blocking `in()` primitive by replacing polling
loops with Postgres LISTEN/NOTIFY. Propagate events through a new `minuano watch`
command to Tramuntana and Telegram topics.

Context: [AI Orchestration: Reinventing Linda](https://otavio.cat/posts/ai-orchestration-reinventing-linda/)

---

## Current state: polling everywhere

| Component | What it polls | Interval | Token cost |
|-----------|--------------|----------|------------|
| `minuano merge --watch` | `merge_queue` table | 5s | Zero (SQL) |
| `minuano agents --watch` | `agents` + `tasks` tables | 2s | Zero (SQL) |
| `minuano-claim` | One-shot query, no poll | — | Zero (SQL) |
| Tramuntana session monitor | JSONL file mtimes | 2s | Zero (file I/O) |
| Tramuntana status poller | tmux pane capture | 1s | Zero (tmux) |

No token waste — all polling is mechanical (SQL, file I/O, tmux). But it adds
latency (up to 5s for merges) and prevents true event-driven flows. The biggest
gap is that `/auto` mode in Tramuntana is fire-and-forget: it sends a prompt and
has no way to know when a task completes, what unblocked, or when to claim next.

---

## Linda primitive gap

| Linda | Current Minuano | With LISTEN/NOTIFY |
|-------|----------------|-------------------|
| `in(template)` blocks until match | Returns empty if nothing ready | Blocks until `task_ready` fires, then claims atomically |
| `eval(t)` completion propagates | Trigger sets dependents to `ready` (but nobody knows) | `task_ready` notification wakes listeners immediately |
| Event subscription | Not supported | `LISTEN` on typed channels |

---

## Phase 1: Postgres triggers (Minuano)

Add NOTIFY calls to existing triggers and create new ones. No schema changes
needed — just trigger modifications.

### 1.1 Task readiness notification

The `refresh_ready_tasks()` trigger already runs when a task is marked done and
sets dependents to `ready`. Add a NOTIFY:

```sql
-- Inside refresh_ready_tasks(), after UPDATE ... SET status = 'ready'
PERFORM pg_notify('task_ready', pending_task.id || '|' || COALESCE(pending_task.project_id, ''));
```

**Channel**: `task_ready`
**Payload**: `<task_id>|<project_id>`

Also fire on direct `INSERT INTO tasks ... status = 'ready'` (tasks with no
dependencies start as ready):

```sql
CREATE OR REPLACE FUNCTION notify_task_ready()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'ready' AND (OLD IS NULL OR OLD.status != 'ready') THEN
    PERFORM pg_notify('task_ready', NEW.id || '|' || COALESCE(NEW.project_id, ''));
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER task_ready_notify
  AFTER INSERT OR UPDATE OF status ON tasks
  FOR EACH ROW EXECUTE FUNCTION notify_task_ready();
```

### 1.2 Task done notification

```sql
CREATE OR REPLACE FUNCTION notify_task_done()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'done' AND OLD.status != 'done' THEN
    PERFORM pg_notify('task_done', NEW.id || '|' || COALESCE(NEW.project_id, '') || '|' || COALESCE(NEW.claimed_by, ''));
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER task_done_notify
  AFTER UPDATE OF status ON tasks
  FOR EACH ROW EXECUTE FUNCTION notify_task_done();
```

**Channel**: `task_done`
**Payload**: `<task_id>|<project_id>|<agent_id>`

### 1.3 Task failed notification

```sql
CREATE OR REPLACE FUNCTION notify_task_failed()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'failed' AND OLD.status != 'failed' THEN
    PERFORM pg_notify('task_failed', NEW.id || '|' || COALESCE(NEW.project_id, ''));
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER task_failed_notify
  AFTER UPDATE OF status ON tasks
  FOR EACH ROW EXECUTE FUNCTION notify_task_failed();
```

**Channel**: `task_failed`
**Payload**: `<task_id>|<project_id>`

### 1.4 Merge queue notification

```sql
CREATE OR REPLACE FUNCTION notify_merge_queue()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF TG_OP = 'INSERT' THEN
    PERFORM pg_notify('merge_ready', NEW.id::text || '|' || NEW.task_id);
  ELSIF NEW.status != OLD.status THEN
    PERFORM pg_notify('merge_' || NEW.status, NEW.id::text || '|' || NEW.task_id);
  END IF;
  RETURN NEW;
END;
$$;

CREATE TRIGGER merge_queue_notify
  AFTER INSERT OR UPDATE OF status ON merge_queue
  FOR EACH ROW EXECUTE FUNCTION notify_merge_queue();
```

**Channels**: `merge_ready`, `merge_merged`, `merge_conflict`, `merge_failed`
**Payload**: `<queue_id>|<task_id>`

### Migration file

All triggers go in `003_listen_notify.sql`. Pure additive — no table changes,
fully backward compatible. Existing polling continues to work.

---

## Phase 2: `minuano watch` command

New streaming command that holds a LISTEN connection and emits JSON events to
stdout. This is the bridge between Postgres events and external consumers
(Tramuntana, scripts, dashboards).

### Interface

```bash
minuano watch [--project <id>] [--channels task_ready,task_done,merge_merged]
```

**Default channels**: `task_ready`, `task_done`, `task_failed`, `merge_ready`,
`merge_merged`, `merge_conflict`, `merge_failed`

### Output format

One JSON object per line (JSONL):

```jsonl
{"channel":"task_ready","task_id":"design-auth","project_id":"backend","ts":"2026-02-21T14:30:00Z"}
{"channel":"task_done","task_id":"design-auth","project_id":"backend","agent_id":"agent-1","ts":"2026-02-21T14:35:00Z"}
{"channel":"task_ready","task_id":"impl-endpoints","project_id":"backend","ts":"2026-02-21T14:35:00Z"}
{"channel":"merge_merged","queue_id":"42","task_id":"design-auth","ts":"2026-02-21T14:35:05Z"}
```

### Implementation

```go
// cmd/minuano/cmd_watch.go
func runWatch(cmd *cobra.Command, args []string) error {
    conn, _ := pgx.Connect(ctx, dbURL)
    defer conn.Close(ctx)

    for _, ch := range channels {
        conn.Exec(ctx, "LISTEN "+ch)
    }

    enc := json.NewEncoder(os.Stdout)
    for {
        notification, _ := conn.WaitForNotification(ctx)
        event := parsePayload(notification.Channel, notification.Payload)
        if projectFilter != "" && event.ProjectID != projectFilter {
            continue
        }
        enc.Encode(event)
    }
}
```

Key properties:
- Long-lived connection, blocks on `WaitForNotification` (no polling)
- Filters by project client-side (LISTEN is per-channel, not per-payload)
- JSONL output for easy piping and subprocess consumption
- Exits cleanly on SIGINT/context cancellation

### Update `minuano merge --watch`

Replace the 5s poll loop with LISTEN internally:

```go
// Instead of time.Sleep(5 * time.Second)
conn.Exec(ctx, "LISTEN merge_ready")
for {
    conn.WaitForNotification(ctx)
    processMergeQueue(ctx, db)
}
```

---

## Phase 3: Tramuntana integration

Tramuntana spawns `minuano watch` as a long-running subprocess and routes events
to Telegram topics.

### Architecture

```
Postgres ──NOTIFY──> minuano watch ──stdout/JSONL──> Tramuntana ──Telegram API──> Topics
```

This keeps the clean CLI boundary. Tramuntana doesn't need a direct Postgres
connection or knowledge of the schema. The existing CLI bridge (`exec.Command`)
stays for request/response operations (`status`, `show`, `prompt`).

### Watcher goroutine

```go
// internal/minuano/watcher.go
type Watcher struct {
    bridge  *Bridge
    cmd     *exec.Cmd
    events  chan Event
}

func (w *Watcher) Start(ctx context.Context, project string) error {
    args := []string{"watch", "--project", project}
    w.cmd = exec.CommandContext(ctx, w.bridge.bin, args...)
    stdout, _ := w.cmd.StdoutPipe()
    w.cmd.Start()

    go func() {
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            var event Event
            json.Unmarshal(scanner.Bytes(), &event)
            w.events <- event
        }
    }()
    return nil
}
```

### Event routing to topics

The bot registers a handler that reads from the watcher's event channel and
routes to the appropriate topic based on project bindings:

```go
// internal/bot/events.go
func (b *Bot) handleMinuanoEvent(event minuano.Event) {
    // Find all topics bound to this project
    topics := b.state.TopicsForProject(event.ProjectID)

    for _, topic := range topics {
        switch event.Channel {
        case "task_done":
            b.sendToTopic(topic, formatTaskDone(event))
        case "task_ready":
            b.sendToTopic(topic, formatTaskReady(event))
        case "task_failed":
            b.sendToTopic(topic, formatTaskFailed(event))
        case "merge_merged":
            b.sendToTopic(topic, formatMergeDone(event))
        case "merge_conflict":
            b.handleMergeConflict(topic, event)
        }
    }
}
```

### Watcher lifecycle

- **Start**: When first `/project` binding is created, or on `tramuntana serve`
  startup if any project bindings exist in state
- **Restart**: If the subprocess dies, respawn with backoff
- **Stop**: When last project binding is removed, or on graceful shutdown
- **Multiple projects**: One `minuano watch` per project (filtered), or one
  global watcher that routes by project_id

One global watcher is simpler. Start it unconditionally on `tramuntana serve` if
`MINUANO_BIN` is configured. Filter events to topics with active project
bindings. Ignore events for unbound projects.

---

## Phase 4: Event-driven auto mode

The current `/auto` mode sends a loop prompt to Claude and hopes for the best.
With events, Tramuntana can orchestrate the loop itself — claiming tasks,
waiting for completion, reacting to merges and conflicts, all driven by
Postgres notifications.

### Current flow (fire-and-forget)

```
User: /auto
Bot:  sends loop prompt to Claude
      ... no visibility until Claude happens to write output ...
```

### New flow (event-driven)

Tramuntana drives the loop. Claude gets single-task prompts. Each step is
triggered by a Postgres notification, not polling or hope.

```go
// internal/bot/auto.go
func (b *Bot) runAutoMode(ctx context.Context, topic TopicInfo, project string) {
    for {
        // Claim next task (existing bridge call)
        tasks, _ := b.bridge.Status(project)
        ready := filterReady(tasks)
        if len(ready) == 0 {
            b.sendToTopic(topic, "Queue empty. Auto mode finished.")
            return
        }

        taskID := ready[0].ID
        prompt, _ := b.bridge.PromptSingle(taskID)
        b.sendPromptToWindow(topic, prompt)
        b.sendToTopic(topic, fmt.Sprintf("Claimed: %s", taskID))

        // Wait for completion event (blocking, no polling)
        select {
        case event := <-b.taskDoneForTopic(topic):
            b.sendToTopic(topic, formatCompletion(event))
        case event := <-b.taskFailedForTopic(topic):
            b.sendToTopic(topic, formatFailure(event))
        case <-ctx.Done():
            return
        }
    }
}
```

The key change: `select` on event channels replaces hope. Each step is visible
in the Telegram topic. The user can `/esc` to cancel, or watch the autonomous
loop progress in real time.

### Interactive control during auto mode

- Send a message → forwarded to Claude as usual (interrupt/guide)
- `/esc` → interrupt current work, auto mode pauses
- `/tasks` → show current queue state
- `/stop` (new command) → cancel auto mode, leave current task claimed

---

## Telegram UX: three topic levels

With worktree auto mode, there are three levels of Telegram topics. Each has
a distinct role and receives different events.

### Topic hierarchy

```
Project topic "backend"          ← orchestrator, sees all events, drives the loop
  ├── Task topic "design-auth [backend]"     ← isolated Claude session in worktree
  ├── Task topic "impl-endpoints [backend]"  ← isolated Claude session in worktree
  ├── Merge topic "merge-impl-endpoints [backend]"  ← only on conflict
  └── Task topic "write-tests [backend]"     ← isolated Claude session in worktree
```

### Example: full auto mode cycle with worktrees

Given a project `backend` with 3 tasks:
- `design-auth` (priority 8, no deps)
- `impl-endpoints` (priority 7, depends on design-auth)
- `write-tests` (priority 5, depends on design-auth)

#### Project topic "backend"

This is where the user runs `/auto --worktrees`. It orchestrates the full cycle
and receives all notifications. The user watches progress here.

```
User:     /auto --worktrees
Bot:      Auto mode started for project backend
          1 task ready, 2 blocked

          ── Task 1/3 ──────────────────────────────────

          Claimed: design-auth (priority 8)
          Created topic: "design-auth [backend]"
          Branch: minuano/backend-design-auth

                    [Claude works in the task topic]
                    [NOTIFY task_done ← minuano-done]

Bot:      ✓ design-auth completed: "Implemented JWT auth with refresh tokens"
          Merging minuano/backend-design-auth → main...

                    [minuano merge processes queue]
                    [NOTIFY merge_merged]

Bot:      ✓ Merged → main (a1b2c3d)
          Worktree cleaned up.

                    [trigger cascades: impl-endpoints, write-tests → ready]
                    [NOTIFY task_ready × 2]

Bot:      → 2 tasks now ready: impl-endpoints, write-tests

          ── Task 2/3 ──────────────────────────────────

          Claimed: impl-endpoints (priority 7)
          Created topic: "impl-endpoints [backend]"
          Branch: minuano/backend-impl-endpoints

                    [Claude works in the task topic]
                    [NOTIFY task_done]

Bot:      ✓ impl-endpoints completed: "Added REST endpoints with validation"
          Merging minuano/backend-impl-endpoints → main...

                    [merge hits conflict]
                    [NOTIFY merge_conflict]

Bot:      ⚠ Merge conflict on minuano/backend-impl-endpoints
          Conflicted files:
            - internal/auth/handler.go
            - internal/auth/middleware.go
          Created topic: "merge-impl-endpoints [backend]"

                    [Claude resolves conflicts in merge topic]
                    [NOTIFY merge_merged]

Bot:      ✓ Conflict resolved → main (e5f6g7h)
          Worktree cleaned up.

          ── Task 3/3 ──────────────────────────────────

          Claimed: write-tests (priority 5)
          Created topic: "write-tests [backend]"
          Branch: minuano/backend-write-tests

                    [Claude works in the task topic]
                    [NOTIFY task_done]

Bot:      ✓ write-tests completed: "Added 47 tests, all passing"
          Merging minuano/backend-write-tests → main...

                    [NOTIFY merge_merged]

Bot:      ✓ Merged → main (i9j0k1l)
          Worktree cleaned up.
          → Queue empty. Auto mode finished. 3/3 tasks done.
```

#### Task topic "design-auth [backend]"

Created automatically by the auto loop. Contains one Claude Code session
working in an isolated worktree. Sees only its own streaming output — normal
tool results, status line, interactive prompts. Does not see other tasks or
merge events.

```
Bot:      Working on: design-auth
          Branch: minuano/backend-design-auth
          [task prompt sent to Claude]

          ... normal Claude streaming output ...
          **Read**(internal/auth/handler.go) → Read 45 lines
          **Write**(internal/auth/jwt.go) → Wrote 120 lines
          **Bash**(go test ./internal/auth/...) → Output 8 lines
          ...

Bot:      ✓ Tests passed. Task marked done.
```

The topic persists after completion. The user can scroll back to review what
Claude did, or close it to clean up.

#### Merge topic "merge-impl-endpoints [backend]"

Only created when a merge has conflicts. Contains a Claude session with the
conflict resolution prompt. Sees only the merge work.

```
Bot:      Merge conflict: minuano/backend-impl-endpoints → main
          Conflicted files:
            - internal/auth/handler.go
            - internal/auth/middleware.go

          [conflict resolution prompt sent to Claude]

          ... Claude reads conflict markers, resolves ...
          **Read**(internal/auth/handler.go) → Read 89 lines
          **Edit**(internal/auth/handler.go) → Added 3, removed 8
          **Bash**(go test ./...) → Output 4 lines
          ...

Bot:      ✓ Conflicts resolved. Merged → main (e5f6g7h)
```

### Example: auto mode without worktrees (single topic)

Simpler variant — everything happens in one topic, one tmux window. No merge
queue involved since there are no branches to merge.

```
Project topic "backend":

User:     /auto
Bot:      Auto mode started for project backend
          1 task ready, 2 blocked
          Claimed: design-auth (priority 8)

          [task prompt sent to Claude in THIS topic's window]

          ... Claude streaming output ...
          **Read**(internal/auth/handler.go) → Read 45 lines
          **Write**(internal/auth/jwt.go) → Wrote 120 lines
          **Bash**(go test ./internal/auth/...) → Output 8 lines

                    [NOTIFY task_done]

Bot:      ✓ design-auth completed: "Implemented JWT auth with refresh tokens"
          → 2 tasks now ready: impl-endpoints, write-tests
          Claimed: impl-endpoints (priority 7)

          [new single-task prompt sent to same window]

          ... Claude streaming output ...

                    [NOTIFY task_done]

Bot:      ✓ impl-endpoints completed: "Added REST endpoints with validation"
          Claimed: write-tests (priority 5)

          ... Claude streaming output ...

                    [NOTIFY task_done]

Bot:      ✓ write-tests completed: "Added 47 tests, all passing"
          → Queue empty. Auto mode finished. 3/3 tasks done.
```

### Example: passive notifications (no auto mode)

Any topic bound to a project via `/project backend` receives state-change
notifications even if it's not running auto mode. Pure observability — the
user can watch progress while agents work elsewhere (via `minuano run` or
another Tramuntana topic).

```
Topic bound to project "backend" (not running /auto):

Bot:      → design-auth claimed by agent-1
Bot:      ✓ design-auth completed by agent-1
Bot:      ✓ Merged minuano/backend-design-auth → main (a1b2c3d)
Bot:      → 2 tasks now ready: impl-endpoints, write-tests
Bot:      → impl-endpoints claimed by agent-2
Bot:      → write-tests claimed by agent-1
Bot:      ✓ impl-endpoints completed by agent-2
Bot:      ⚠ write-tests failed (attempt 2/3): test timeout
Bot:      → write-tests back to ready (will retry)
Bot:      ✓ write-tests completed by agent-1
Bot:      → Queue empty. All 3 tasks done.
```

### Event routing summary

| Event | Project topic | Task topic | Merge topic |
|-------|:------------:|:----------:|:-----------:|
| `task_ready` | ✓ (+ claims next in auto) | — | — |
| `task_done` | ✓ (+ triggers merge) | ✓ (own task only) | — |
| `task_failed` | ✓ (+ decides retry/stop) | ✓ (own task only) | — |
| `merge_merged` | ✓ (+ cleans worktree) | — | ✓ (own merge only) |
| `merge_conflict` | ✓ (+ creates merge topic) | — | — |
| `merge_failed` | ✓ (+ pauses auto) | — | ✓ (own merge only) |
| Claude streaming | — | ✓ | ✓ |
| Status line | — | ✓ | ✓ |
| Interactive UI | — | ✓ | ✓ |

The project topic never shows Claude streaming output — it's the control plane.
Task and merge topics show Claude output — they're the data plane.

---

## Implementation order

| Phase | Scope | Effort | Depends on |
|-------|-------|--------|------------|
| 1 | Postgres triggers (`003_listen_notify.sql`) | Hours | Nothing |
| 2a | `minuano watch` command | 1 day | Phase 1 |
| 2b | Update `minuano merge --watch` to use LISTEN | Hours | Phase 1 |
| 3a | Tramuntana watcher goroutine | 1 day | Phase 2a |
| 3b | Event routing to topics (passive notifications) | 1 day | Phase 3a |
| 4a | Event-driven `/auto` mode | 1-2 days | Phase 3b |
| 4b | Auto mode + worktrees integration | 1 day | Phase 4a |

Phases 1 and 2b are fully backward compatible — existing polling continues to
work, just faster. Phase 2a is additive. Phases 3-4 are Tramuntana-only changes.

### What stays as polling

- **Tramuntana session monitor** (JSONL files) — no Postgres event for "Claude
  wrote output". Would need inotify or Claude Code hook changes. Not worth it;
  2s polling is fine for file I/O.
- **Tramuntana status poller** (tmux pane) — terminal state isn't in Postgres.
  1s polling is fine.
- **`minuano agents --watch`** (TUI) — pure display, low priority. Could use
  LISTEN but 2s refresh is acceptable.

---

## What this achieves relative to Linda

After all phases:

| Linda primitive | Implementation |
|-----------------|---------------|
| `out(t)` — put | `minuano add` → INSERT + NOTIFY `task_ready` |
| `in(template)` — blocking take | `LISTEN task_ready` + `AtomicClaim` (true blocking `in()`) |
| `rd(template)` — non-blocking read | `minuano show` / `minuano status` (unchanged) |
| `eval(t)` — live tuple → data | Agent works → `minuano-done` → NOTIFY `task_done` (completion propagates instantly) |
| Event subscription | `minuano watch` (spatial + temporal decoupling via typed channels) |

The remaining gap vs pure Linda: no arbitrary pattern matching on tuple fields.
But `project + capability + priority` covers the real use cases. The coordination
semantics — atomic take, blocking wait, event propagation — are complete.
