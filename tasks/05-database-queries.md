# Task 05 — Database Queries

## Goal

Implement `internal/db/queries.go` with all SQL operations as typed Go functions. No raw SQL outside this file.

## Steps

1. Create `internal/db/queries.go`.
2. Define Go structs: `Task`, `TaskDep`, `TaskContext`, `Agent` matching the DB schema.
3. Implement these named functions (all take `*pgxpool.Pool` as first arg):
   - `CreateTask(pool, id, title, body, priority, capability, projectID, metadata) error`
   - `AddDependency(pool, taskID, dependsOn) error`
   - `GetTask(pool, id) (*Task, error)` — supports partial ID matching.
   - `ListTasks(pool, projectID) ([]*Task, error)` — ordered by priority/created_at.
   - `GetTaskWithContext(pool, id) (*Task, []TaskContext, error)`
   - `AtomicClaim(pool, agentID, capability) (*Task, error)` — the Linda `in()` from DESIGN.md.
   - `MarkDone(pool, taskID, agentID, summary) error`
   - `RecordFailure(pool, taskID, agentID, output) error`
   - `ReclaimStale(pool, minutes) (int, error)`
   - `AddObservation(pool, taskID, agentID, content) error`
   - `AddHandoff(pool, taskID, agentID, content) error`
   - `GetDependencyTree(pool, projectID) (tree structure, error)` — for `minuano tree`.
   - `SearchContext(pool, query) ([]TaskContext, error)` — full-text search.
   - `RegisterAgent(pool, id, tmuxSession, tmuxWindow) error`
   - `ListAgents(pool) ([]*Agent, error)`
   - `UpdateAgentStatus(pool, id, status) error`
   - `DeleteAgent(pool, id) error`
   - `UpdateTask(pool, id, title, body) error` — for `minuano edit`.
4. Use `pgx` named arguments for multi-param queries.
5. All SQL matches the patterns in DESIGN.md (especially AtomicClaim with CTE + FOR UPDATE SKIP LOCKED).

## Acceptance

- All functions compile.
- `AtomicClaim` uses the exact CTE pattern from DESIGN.md (claim + inherit + agent update in one TX).
- No raw SQL exists outside `queries.go`.

## Phase

1 — Foundation

## Depends on

- Task 04
