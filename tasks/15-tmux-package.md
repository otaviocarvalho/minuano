# Task 15 — Tmux Package

## Goal

Implement `internal/tmux/tmux.go` with all tmux management functions.

## Steps

1. Create `internal/tmux/tmux.go`.
2. Implement all functions from DESIGN.md:
   - `SessionExists(name string) bool`
   - `EnsureSession(name string) error` — creates if missing, no-op if exists.
   - `WindowExists(session, window string) bool`
   - `NewWindow(session, window string, env map[string]string) error` — creates window with env vars set.
   - `SendKeys(session, window, keys string) error`
   - `CapturePane(session, window string, lines int) (string, error)`
   - `KillWindow(session, window string) error`
   - `AttachSession(session, window string) error` — uses `execlp` replacement (syscall.Exec).
   - `SwitchWindow(session, window string) error` — when already inside tmux.
3. Detect inside-tmux via `os.Getenv("TMUX") != ""` to choose between attach and switch.
4. All functions shell out to the `tmux` binary. Handle errors from tmux cleanly.

## Acceptance

- All functions compile.
- `EnsureSession` + `NewWindow` + `SendKeys` can create a session, add a window, and send a command to it.
- `CapturePane` returns the window's visible output.
- `AttachSession` vs `SwitchWindow` is chosen based on TMUX env var.

## Phase

3 — Tmux & Agent Infrastructure

## Depends on

- Task 06
