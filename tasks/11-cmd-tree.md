# Task 11 — `minuano tree`

## Goal

Implement the dependency tree view with status symbols.

## Steps

1. Add `minuano tree` subcommand.
2. Query the full dependency graph.
3. Render a tree view showing parent→child relationships with indentation.
4. Each node shows: status symbol, ID (truncated), title.
5. Handle DAGs (a task with multiple parents should appear under each, or be deduplicated with a reference marker).
6. Optional `--project` filter.

## Acceptance

- `minuano tree` prints a tree matching the example in DESIGN.md.
- Status symbols are correct for each task state.
- Dependencies are visually clear through indentation.

## Phase

2 — Core CLI Commands

## Depends on

- Task 10
