# Minuano — TODO

## Missing Features

### `minuano dep` — Add/remove dependencies on existing tasks

**Problem:** Dependencies (`--after`) can only be set at task creation time via `minuano add --after <id>`.
There is no way to add or remove dependency edges after a task has been created.

**Impact:** When planning tasks with cross-cutting dependencies (e.g., task 7.2 depends on
both task 2.4 AND task 7.1), the planner must either:
1. Carefully order `minuano add` calls so all `--after` flags are known upfront, or
2. Delete and re-create the task to fix a missing dependency

Neither is practical during iterative planning sessions.

**Proposed CLI:**
```
# Add a dependency (task B depends on task A)
minuano dep add <task-B> --after <task-A>

# Remove a dependency
minuano dep rm <task-B> --after <task-A>

# List dependencies for a task
minuano dep list <task-id>
```

**Alternative:** Extend `minuano edit` beyond body-only editing:
```
minuano edit <id> --add-after <dep-id>
minuano edit <id> --rm-after <dep-id>
minuano edit <id> --priority 8
minuano edit <id> --status draft
```

This would make `edit` a general-purpose task mutation command rather than just a body editor.
