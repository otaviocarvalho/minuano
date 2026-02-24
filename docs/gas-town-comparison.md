# Gas Town vs Minuano/Tramuntana: Task Isolation, Merge Queues, and Token Efficiency

Analysis comparing Steve Yegge's Gas Town multi-agent orchestration with the
Minuano/Tramuntana Linda-inspired model. Focus on task isolation, merge queue
orchestration, and token savings.

Context: [Our AI Orchestration Frameworks Are Reinventing Linda (1985)](https://otavio.cat/posts/ai-orchestration-reinventing-linda/)

---

## Gas Town's Approach

Gas Town uses a **push-based, role-heavy hierarchy** with 7 agent roles:

| Role | Level | Function | Token Cost |
|------|-------|----------|------------|
| **Mayor** | Town | Chief dispatcher, full workspace context | High (always-on coordinator) |
| **Deacon** | Town | Health daemon, patrol loops | Continuous burn (polling) |
| **Dogs** | Town | Maintenance under Deacon | Spawned by Deacon |
| **Crew** | Rig | Named persistent agents for design/review | Moderate (long-lived) |
| **Polecats** | Rig | Ephemeral workers for specific tasks | Low per-unit |
| **Refinery** | Rig | Merge queue management, rebasing | Active during merges |
| **Witness** | Rig | Supervises Polecats/Refinery, unblocks | Monitoring burn |

**Task isolation**: Git worktree-based. Each Polecat/Crew member gets a worktree.
State persists in Beads (JSONL in `.beads/`) backed by git. The **GUPP** principle
("if there is work on your hook, you MUST run it") drives agent scheduling.

**Merge queue**: The **Refinery** agent manages merges — rebasing, conflict
resolution, sequential validation. The **Witness** supervises and unblocks.
This is an *agent-driven* merge queue: Claude instances actively orchestrating
git operations, which means **every merge decision burns tokens**.

Yegge acknowledges: *"Running 20-30 agent instances requires substantial compute
and API spend."*

---

## Minuano/Tramuntana's Approach

Pull-based, role-flat model built on Linda tuple-space primitives over PostgreSQL:

| Component | Function | Token Cost |
|-----------|----------|------------|
| PostgreSQL | Coordination substrate (tuple space) | Zero (mechanical) |
| DB Trigger | Cascade readiness to dependents | Zero (SQL) |
| `minuano-claim` | Atomic task claim (Linda `in()`) | Zero (bash + psql) |
| `minuano-done` | Test gate + state transition | Zero (bash + psql) |
| `minuano reclaim` | Stale task recovery | Zero (cron + psql) |
| Agents | Identical workers, no roles | Only burn tokens doing real work |

**Task isolation**: Currently **none at the git level**. All agents work in the
same working tree. The DAG serializes dependent work, but concurrent agents
touching different files could still collide.

---

## Linda Primitive Mapping

| Linda Primitive | Gas Town (git-backed) | Minuano (Postgres) |
|---|---|---|
| `in()` atomic claim | Read-then-write on JSONL (race risk) | `FOR UPDATE SKIP LOCKED` (linearizable) |
| `rd()` query work | `bd ready` (file scan) | `SELECT ... WHERE status='ready'` |
| `out()` create task | `bd create` (append JSONL) | `INSERT INTO tasks` |
| `eval()` live tuple | GUPP hook checking (token cost) | DB trigger `refresh_ready_tasks()` (zero cost) |
| Blocking on match | Polling loops (token cost) | `minuano reclaim` + cron (zero cost) |
| Tuple persistence | Git commits (eventual consistency) | PostgreSQL WAL (linearizable) |

Minuano chose **consistency over availability** in the PACELC sense — Postgres
gives linearizable claims at the cost of requiring the DB to be up. Beads chose
availability (works offline with git) at the cost of race conditions on `bd claim`.

---

## The Critical Difference

| Dimension | Gas Town | Minuano |
|-----------|----------|---------|
| Orchestration | Push (Mayor assigns) | Pull (agents claim from queue) |
| Background burn | Mayor + Deacon + Witness polling | **Zero** (DB triggers + cron) |
| Task isolation | Worktrees per agent | Shared working tree (gap) |
| Merge strategy | Refinery agent (token cost) | None (implicit: serial by DAG) |
| State substrate | Git (`.beads/` JSONL) | PostgreSQL |
| Atomic claims | Read-then-write race risk | `FOR UPDATE SKIP LOCKED` (truly atomic) |
| Crash recovery | GUPP + hooks in git | `minuano reclaim` + context in DB |

---

## Proposal: Worktrees for Parallel Isolation

### The Problem Without Worktrees

If Minuano spawns 3 agents and they claim 3 independent tasks, they all edit the
same working tree. Even with DAG serialization of *dependent* tasks, independent
tasks can:

- Overwrite each other's uncommitted changes
- Cause build failures from partial state
- Make test isolation impossible (agent A's test runs see agent B's half-written code)

### The Minuano-Native Worktree Model

Don't copy Gas Town's Refinery agent. Instead, make worktrees a **mechanical
concern** (zero tokens):

```
minuano run --agents 3 --worktrees
```

Each agent gets:

1. A git worktree in `.claude/worktrees/<agent-id>/` (or the project root)
2. Its own branch based on the target branch
3. Isolation: edits + tests run in the worktree, not the main tree

The merge back becomes a **script, not an agent**:

```bash
# scripts/minuano-merge (called by minuano-done after tests pass)
git -C "$WORKTREE" add -A
git -C "$WORKTREE" commit -m "Task $TASK_ID: $SUMMARY"
git checkout main
git merge --no-ff "$BRANCH"
```

If the merge has conflicts, mark the task as `failed` (mechanical, zero tokens)
and let a human or a subsequent agent handle it with the conflict context written
to `task_context`.

### Token Cost Comparison

| Operation | Gas Town | Minuano + Worktrees |
|-----------|----------|---------------------|
| Create worktree | Agent (tokens) | `git worktree add` (bash, 0 tokens) |
| Merge back | Refinery agent (tokens) | `git merge` script (0 tokens) |
| Conflict resolution | Witness + Refinery (tokens) | Mark failed + context in DB (0 tokens until pickup) |
| Supervision | Witness polling (tokens) | DB trigger cascade (0 tokens) |

---

## Proposal: Token-Free Merge Queue

A merge queue as pure SQL + bash — **zero tokens**:

### Schema Addition

```sql
CREATE TABLE merge_queue (
  task_id    TEXT PRIMARY KEY REFERENCES tasks(id),
  branch     TEXT NOT NULL,
  worktree   TEXT NOT NULL,
  status     TEXT NOT NULL DEFAULT 'pending',
    -- pending | merging | merged | conflict
  queued_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  merged_at  TIMESTAMPTZ
);

-- Trigger: when task done, enqueue for merge
CREATE OR REPLACE FUNCTION enqueue_merge()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'done' THEN
    INSERT INTO merge_queue (task_id, branch, worktree)
    SELECT NEW.id, a.branch, a.worktree
    FROM agents a WHERE a.id = NEW.claimed_by
    ON CONFLICT DO NOTHING;
  END IF;
  RETURN NEW;
END;
$$;
```

### Merge Queue Script

```bash
# Process merge queue FIFO, zero tokens
while NEXT=$(psql "$DB" -t -A -c "
  UPDATE merge_queue SET status='merging'
  WHERE task_id = (
    SELECT task_id FROM merge_queue
    WHERE status='pending'
    ORDER BY queued_at LIMIT 1
    FOR UPDATE SKIP LOCKED
  ) RETURNING task_id, branch, worktree
"); [ -n "$NEXT" ]; do
  TASK_ID=$(echo "$NEXT" | cut -d'|' -f1)
  BRANCH=$(echo "$NEXT" | cut -d'|' -f2)
  WORKTREE=$(echo "$NEXT" | cut -d'|' -f3)

  git checkout main && git merge --no-ff "$BRANCH" 2>&1
  if [ $? -eq 0 ]; then
    psql "$DB" -c "UPDATE merge_queue SET status='merged', merged_at=NOW()
                   WHERE task_id='$TASK_ID'"
    git worktree remove "$WORKTREE" 2>/dev/null
  else
    CONFLICT=$(git diff --name-only --diff-filter=U)
    git merge --abort
    psql "$DB" -c "UPDATE merge_queue SET status='conflict'
                   WHERE task_id='$TASK_ID'"
    psql "$DB" -c "INSERT INTO task_context (task_id, kind, content)
      VALUES ('$TASK_ID', 'observation', 'Merge conflict in: $CONFLICT')"
  fi
done
```

This gives GitHub-style merge queue semantics (FIFO, test-then-merge, conflict
ejection) at **zero token cost**.

---

## Token Savings Summary

| Gas Town Cost | Minuano Equivalent | Savings |
|---------------|---------------------|---------|
| Mayor session (always-on coordinator) | DB triggers + `refresh_ready_tasks()` | 100% — no coordinator agent |
| Deacon patrol loops | `minuano reclaim` via cron | 100% — no polling agent |
| Witness supervision | `task_context` + `max_attempts` | 100% — failure context is mechanical |
| Refinery merge queue | SQL queue + bash scripts | 100% — no merge agent |
| GUPP hook checking | `SELECT ... FOR UPDATE SKIP LOCKED` | 100% — DB does it |
| Agent identity/CV chains | `task_context` with `agent_id` | Already have it |

Minuano's design principle — *"Zero background token burn. Every token spent is
an agent doing real work"* — is architecturally sound. Gas Town trades tokens for
sophistication (role specialization, supervision, active merge management).
Minuano trades sophistication for efficiency (all coordination is mechanical).

The missing pieces are **worktrees for parallel isolation** and a **mechanical
merge queue**. Both can be added without any agent involvement — pure bash + SQL
— keeping the zero-background-burn guarantee intact.

---

## Features Gas Town Has That We Don't (Roadmap Candidates)

### 1. Worktree-Based Task Isolation (High Priority)

**Gas Town**: Every Polecat/Crew gets its own git worktree automatically.
**Minuano**: All agents share one working tree.

This is the most impactful gap. Without it, parallel agents on independent tasks
are fundamentally unsafe. Implementation path:

- Add `branch` and `worktree` columns to `agents` table
- `minuano run --worktrees` creates worktrees on spawn
- `minuano-done` commits in the worktree before marking done
- `minuano kill` cleans up worktrees

**Token cost to implement**: Zero ongoing. One-time schema change + bash scripts.

### 2. Merge Queue (High Priority)

**Gas Town**: Refinery agent actively manages rebasing and merge conflicts.
**Minuano**: No merge strategy — tasks complete in isolation but never merge back.

Without this, worktrees are pointless (work stays on orphan branches). See the
SQL + bash merge queue proposal above. Could be:

- `minuano merge-queue` command (run manually or via cron)
- `minuano merge-queue --watch` for continuous processing
- Conflict tasks get `observation` context and can be re-claimed

### 3. Hierarchical Task Decomposition / Dynamic Spawning (Medium Priority)

**Gas Town**: Molecules (workflow graphs) + Protomolecules (templates) + Formulas
(TOML definitions). The Mayor can decompose epics into task DAGs. Agents can
spawn sub-work mid-task.
**Minuano**: Static DAG defined upfront via `minuano add --after`. No dynamic
spawning, no templates.

Roadmap items:
- `minuano add` callable from within agent sessions (agents discover sub-work)
- `minuano template apply <name>` generating task graphs from TOML (already
  noted as future work in DESIGN.md)
- Epic grouping: a parent task that auto-decomposes into sub-tasks

### 4. Multi-Project Orchestration (Medium Priority)

**Gas Town**: Town-level coordination across multiple Rigs (repositories). The
Mayor dispatches work across projects. Cross-project dependencies are first-class.
**Minuano**: Single-project scope. `project_id` exists on tasks but there's no
cross-project dependency resolution or dispatch.

Roadmap items:
- Cross-project task dependencies (`depends_on` referencing tasks in other projects)
- `minuano run --project X` scoping agents to a project
- Tramuntana already has `/project` binding — extend to multi-project views

### 5. Agent Specialization / Capabilities (Low Priority)

**Gas Town**: 7 distinct roles with different prompts, permissions, and
behavioral expectations. Crew for design, Polecats for implementation, Witness
for supervision.
**Minuano**: All agents are identical. The `capability` column exists but is
unused in practice.

This is intentionally low priority — the zero-role model is a feature, not a
bug. Specialization adds token cost (role-specific prompts, supervision loops).
Consider only if specific tasks genuinely need different tool permissions or
system prompts.

Potential light-touch version:
- `minuano run --capability reviewer` spawns agents that only claim `capability='review'` tasks
- No supervision agents — capability is just a filter, not a role

### 6. Agent Mailbox / Inter-Agent Communication (Low Priority)

**Gas Town**: `gt mail check --inject` — agents can send messages to each other.
The Mayor broadcasts instructions.
**Minuano**: No inter-agent communication. Agents communicate indirectly via
`task_context` (observations, handoffs) on shared dependency chains.

The `task_context` model is actually sufficient for most cases — agent B reads
agent A's observations when it inherits context from a dependency. Direct
messaging would only matter for:
- Broadcast alerts ("schema changed, all agents re-read migrations")
- Negotiation ("I need file X, are you editing it?")

If needed, a simple `agent_messages` table + `minuano-mail` script would work
without any polling agent.

### 7. Human Approval Gates (Low Priority)

**Gas Town**: Molecules support approval steps where work pauses for human review.
**Minuano**: No gates. Tasks flow automatically from `pending` → `ready` →
`claimed` → `done`.

Already noted in DESIGN.md as future work: `--gate` flag on `minuano add` that
pauses the task at `ready` until a human runs `minuano approve <id>`.

### 8. Persistent Agent Identity Across Sessions (Not Needed)

**Gas Town**: Crew members have persistent identities, CVs, and reputation.
**Minuano**: Agents are ephemeral — identity is just a tmux window name.

This is a Gas Town feature we should **not** copy. It requires maintaining agent
state across sessions (token cost) and introduces complexity without clear
benefit. The `task_context` table already preserves the history of what each
agent discovered, regardless of agent identity.

---

### Prioritized Roadmap

| Priority | Feature | Token Cost | Complexity |
|----------|---------|------------|------------|
| **P0** | Worktree isolation | Zero ongoing | Medium (schema + scripts) |
| **P0** | Mechanical merge queue | Zero ongoing | Medium (SQL + bash) |
| **P1** | Dynamic task spawning | Zero (agents call `minuano add`) | Low |
| **P1** | Template-based task graphs | Zero | Medium |
| **P2** | Multi-project orchestration | Zero | Medium |
| **P2** | Capability-based agent filtering | Zero | Low |
| **P3** | Human approval gates | Zero | Low |
| **P3** | Agent mailbox | Zero | Low |
| **Skip** | Role-based agents | High (supervision loops) | High |
| **Skip** | Persistent agent identity | Medium (state maintenance) | Medium |

Everything rated P0-P3 can be implemented at **zero ongoing token cost** —
maintaining the core Minuano principle.

---

## Sources

- [Gas Town GitHub Repository](https://github.com/steveyegge/gastown)
- [Welcome to Gas Town — Steve Yegge](https://steve-yegge.medium.com/welcome-to-gas-town-4f25ee16dd04)
- [Gas Town Reading Notes — Torq Software](https://reading.torqsoftware.com/notes/software/ai-ml/agentic-coding/2026-01-15-gas-town-multi-agent-orchestration-framework/)
- [Gas Town Decoded — Andrew Lilley Brinker](https://www.alilleybrinker.com/mini/gas-town-decoded/)
- [GasTown and the Two Kinds of Multi-Agent — Paddo.dev](https://paddo.dev/blog/gastown-two-kinds-of-multi-agent/)
