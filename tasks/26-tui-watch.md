# Task 26 — TUI Watch View

## Goal

Implement an optional bubbletea-based TUI for `minuano agents --watch` and potentially `minuano status --watch`.

## Steps

1. Create `internal/tui/tui.go`.
2. Use `bubbletea` and `lipgloss` for a live-updating terminal UI.
3. The TUI should:
   - Poll the DB every 2 seconds.
   - Show agents table with status, current task, elapsed time.
   - Show task status summary (counts by status).
   - Support keyboard navigation (q to quit).
4. Wire this into `minuano agents --watch` as a replacement for the simple poll loop.
5. Add dependencies: `go get github.com/charmbracelet/bubbletea github.com/charmbracelet/lipgloss`.

## Acceptance

- `minuano agents --watch` launches a live TUI.
- Agent status updates appear within 2 seconds.
- `q` exits the TUI cleanly.
- The display is well-formatted with colors/styles.

## Phase

6 — Polish

## Depends on

- Task 19
