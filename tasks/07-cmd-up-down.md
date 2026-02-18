# Task 07 — `minuano up` / `minuano down`

## Goal

Implement the Docker container lifecycle commands.

## Steps

1. Add `minuano up` subcommand:
   - Runs `docker compose -f docker/docker-compose.yml up -d`.
   - Waits for the healthcheck to pass.
   - Prints the connection URL on success.
2. Add `minuano down` subcommand:
   - Runs `docker compose -f docker/docker-compose.yml down`.
   - Prints confirmation.
3. Both commands should resolve the `docker-compose.yml` path relative to the binary or a known project root.

## Acceptance

- `minuano up` starts postgres and prints `✓ minuano-postgres started (postgres://...)`.
- `minuano down` stops postgres cleanly.
- Commands are idempotent (running `up` twice doesn't error).

## Phase

2 — Core CLI Commands

## Depends on

- Task 06
- Task 02
