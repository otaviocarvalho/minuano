# Task 24 — Agent Scripts

## Goal

Create the bash scripts in `scripts/` that agents call from inside their CLAUDE.md loop.

## Steps

1. Write `scripts/minuano-claim` — atomic task claim via psql. Prints JSON or empty on no work. Exactly as in DESIGN.md.
2. Write `scripts/minuano-done` — runs tests, marks done or records failure. Exactly as in DESIGN.md.
3. Write `scripts/minuano-observe` — writes observation to task_context. Exactly as in DESIGN.md.
4. Write `scripts/minuano-handoff` — writes handoff note to task_context. Exactly as in DESIGN.md.
5. Make all scripts executable (`chmod +x`).
6. All scripts use `$AGENT_ID`, `$DATABASE_URL` from environment.
7. Review the SQL injection risk in the scripts (the `$SUMMARY`, `$NOTE` values are interpolated into SQL). Consider using psql's `-v` variable binding or `quote_literal()` for safety.

## Acceptance

- All four scripts exist and are executable.
- `minuano-claim` returns valid JSON when a ready task exists.
- `minuano-done` runs the test command and correctly marks done or records failure.
- `minuano-observe` and `minuano-handoff` insert context entries.
- Scripts work when called with the right env vars set.

## Phase

5 — Agent Runtime

## Depends on

- Task 08
