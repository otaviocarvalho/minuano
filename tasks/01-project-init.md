# Task 01 — Project Initialization

## Goal

Bootstrap the Go module and create the full directory skeleton described in DESIGN.md.

## Steps

1. Run `go mod init github.com/otavio/minuano` (or the correct module path).
2. Create every directory in the repository layout:
   - `cmd/minuano/`
   - `internal/db/migrations/`
   - `internal/tmux/`
   - `internal/agent/`
   - `internal/tui/`
   - `scripts/`
   - `claude/`
   - `docker/`
3. Create placeholder `main.go` at `cmd/minuano/main.go` with `package main` and an empty `func main()`.
4. Create a `.env` file with the default values from DESIGN.md and add `.env` to `.gitignore`.
5. Create a `.gitignore` with sensible Go defaults (binary, `.env`, vendor, etc).

## Acceptance

- `go build ./cmd/minuano` succeeds (even if the binary does nothing).
- Directory structure matches DESIGN.md layout.
- `.env` is gitignored.

## Phase

1 — Foundation
