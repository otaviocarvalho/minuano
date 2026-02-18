# Task 04 — Database Connection Layer

## Goal

Implement `internal/db/db.go` with connection pool management and a migration runner.

## Steps

1. Create `internal/db/db.go`.
2. Use `pgx/v5` with `pgxpool.New` for connection pooling.
3. Implement `Connect(databaseURL string) (*pgxpool.Pool, error)` — creates and pings the pool.
4. Implement `RunMigrations(pool *pgxpool.Pool) error` — reads `.sql` files from the embedded `migrations/` directory (use `embed.FS`), tracks applied migrations in a `schema_migrations` table, and applies pending ones in order.
5. Add the `pgx/v5` dependency: `go get github.com/jackc/pgx/v5`.

## Acceptance

- `Connect` returns a working pool when postgres is running.
- `RunMigrations` applies `001_initial.sql` and is idempotent (running twice does not error).
- Migrations are embedded in the binary via `//go:embed`.

## Phase

1 — Foundation

## Depends on

- Task 03
