# Task 25 — Agent CLAUDE.md

## Goal

Write `claude/CLAUDE.md` — the instructions that each Claude Code agent receives when spawned.

## Steps

1. Create `claude/CLAUDE.md` with the agent loop instructions exactly as described in DESIGN.md.
2. Sections:
   - **Setup**: PATH configuration, env vars.
   - **Loop**: claim → read context → work → observe → handoff → done.
   - **Rules**: never mark done without minuano-done, fix only what broke on test_failure, etc.
3. The CLAUDE.md must be self-contained — an agent reading only this file should know exactly how to operate.
4. Reference the scripts by name (they'll be on PATH).

## Acceptance

- `claude/CLAUDE.md` exists and matches the DESIGN.md specification.
- An agent spawned with this file as its prompt would know how to claim tasks, do work, and report results.

## Phase

5 — Agent Runtime

## Depends on

- Task 24
