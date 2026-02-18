# Task 06 — CLI Skeleton

## Goal

Set up the cobra-based CLI framework with persistent flags and `.env` loading.

## Steps

1. Update `cmd/minuano/main.go` with a cobra root command named `minuano`.
2. Add persistent flags:
   - `--db` — overrides `DATABASE_URL` from env.
   - `--session` — overrides `MINUANO_SESSION` from env (tmux session name, default `minuano`).
3. Load `.env` via `godotenv.Load()` before cobra parses args.
4. Add dependencies: `go get github.com/spf13/cobra github.com/joho/godotenv`.
5. The root command should print usage/help when called with no subcommand.
6. Create a shared `internal/cli/` or use cobra's `PersistentPreRunE` to initialize the DB pool once and pass it to subcommands.

## Acceptance

- `go build ./cmd/minuano && ./minuano --help` prints the command tree.
- `--db` and `--session` flags are recognized.
- `.env` file is loaded automatically.

## Phase

2 — Core CLI Commands

## Depends on

- Task 05
