# Task 02 — Docker Setup

## Goal

Create the `docker/docker-compose.yml` exactly as specified in DESIGN.md, providing PostgreSQL 16 and optional pgAdmin.

## Steps

1. Write `docker/docker-compose.yml` with the `postgres` and `pgadmin` services as defined in DESIGN.md.
2. pgAdmin should only start under the `debug` profile.
3. Verify `docker compose -f docker/docker-compose.yml up -d` starts postgres successfully.
4. Verify the healthcheck passes: `docker compose -f docker/docker-compose.yml ps` shows `healthy`.

## Acceptance

- `docker compose -f docker/docker-compose.yml up -d` starts a healthy postgres container named `minuano-postgres`.
- `psql postgres://minuano:minuano@localhost:5432/minuanodb` connects successfully.
- pgAdmin only starts when `--profile debug` is used.

## Phase

1 — Foundation

## Depends on

- Task 01
