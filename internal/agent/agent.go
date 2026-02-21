package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/otavio/minuano/internal/db"
	"github.com/otavio/minuano/internal/git"
	"github.com/otavio/minuano/internal/tmux"
)

// Agent represents a running agent instance.
type Agent struct {
	ID          string
	TmuxSession string
	TmuxWindow  string
	TaskID      *string
	Status      string
	StartedAt   time.Time
	LastSeen    *time.Time
	WorktreeDir *string
	Branch      *string
}

// Spawn registers an agent in the DB, creates a tmux window, and sends the bootstrap command.
// It returns immediately without waiting for the agent to claim a task.
func Spawn(pool *pgxpool.Pool, tmuxSession, agentID, claudeMDPath string, env map[string]string) (*Agent, error) {
	// Register in DB (no worktree).
	if err := db.RegisterAgent(pool, agentID, tmuxSession, agentID, nil, nil); err != nil {
		return nil, fmt.Errorf("registering agent: %w", err)
	}

	// Create tmux window.
	if err := tmux.NewWindow(tmuxSession, agentID, env); err != nil {
		// Clean up DB on failure.
		db.DeleteAgent(pool, agentID)
		return nil, fmt.Errorf("creating tmux window: %w", err)
	}

	sendBootstrap(tmuxSession, agentID, claudeMDPath, env, nil, nil)

	now := time.Now()
	return &Agent{
		ID:          agentID,
		TmuxSession: tmuxSession,
		TmuxWindow:  agentID,
		Status:      "idle",
		StartedAt:   now,
		LastSeen:    &now,
	}, nil
}

// SpawnWithWorktree registers an agent with an isolated git worktree.
func SpawnWithWorktree(pool *pgxpool.Pool, tmuxSession, agentID, claudeMDPath string, env map[string]string) (*Agent, error) {
	repoRoot, err := git.RepoRoot()
	if err != nil {
		return nil, fmt.Errorf("finding repo root: %w", err)
	}

	worktreeDir := filepath.Join(repoRoot, ".minuano", "worktrees", agentID)
	branch := "minuano/" + agentID

	// Create worktree.
	if err := git.WorktreeAdd(worktreeDir, branch); err != nil {
		return nil, fmt.Errorf("creating worktree: %w", err)
	}

	// Register in DB with worktree info.
	if err := db.RegisterAgent(pool, agentID, tmuxSession, agentID, &worktreeDir, &branch); err != nil {
		git.WorktreeRemove(worktreeDir)
		return nil, fmt.Errorf("registering agent: %w", err)
	}

	// Create tmux window starting in the worktree directory.
	if err := tmux.NewWindowWithDir(tmuxSession, agentID, worktreeDir, env); err != nil {
		db.DeleteAgent(pool, agentID)
		git.WorktreeRemove(worktreeDir)
		return nil, fmt.Errorf("creating tmux window: %w", err)
	}

	sendBootstrap(tmuxSession, agentID, claudeMDPath, env, &worktreeDir, &branch)

	now := time.Now()
	return &Agent{
		ID:          agentID,
		TmuxSession: tmuxSession,
		TmuxWindow:  agentID,
		Status:      "idle",
		StartedAt:   now,
		LastSeen:    &now,
		WorktreeDir: &worktreeDir,
		Branch:      &branch,
	}, nil
}

func sendBootstrap(tmuxSession, agentID, claudeMDPath string, env map[string]string, worktreeDir, branch *string) {
	// Resolve scripts directory path (relative to CLAUDE.md).
	scriptsDir := filepath.Join(filepath.Dir(claudeMDPath), "..", "scripts")
	absScripts, _ := filepath.Abs(scriptsDir)

	bootstrap := []string{
		fmt.Sprintf("export AGENT_ID=%q", agentID),
		fmt.Sprintf("export DATABASE_URL=%q", env["DATABASE_URL"]),
		fmt.Sprintf("export PATH=\"$PATH:%s\"", absScripts),
	}

	if worktreeDir != nil {
		bootstrap = append(bootstrap, fmt.Sprintf("export WORKTREE_DIR=%q", *worktreeDir))
	}
	if branch != nil {
		bootstrap = append(bootstrap, fmt.Sprintf("export BRANCH=%q", *branch))
	}

	// Resolve claudeMD path relative to worktree if applicable.
	claudeMDArg := claudeMDPath
	if worktreeDir != nil {
		// Use the CLAUDE.md from the worktree copy.
		wtClaudeMD := filepath.Join(*worktreeDir, "claude", "CLAUDE.md")
		if _, err := os.Stat(wtClaudeMD); err == nil {
			claudeMDArg = wtClaudeMD
		}
	}

	bootstrap = append(bootstrap, fmt.Sprintf("claude --dangerously-skip-permissions -p \"$(cat %s)\"", claudeMDArg))

	for _, cmd := range bootstrap {
		tmux.SendKeys(tmuxSession, agentID, cmd)
	}
}

// Kill terminates an agent: kills the tmux window, releases claimed tasks, removes from DB.
// If the agent has a worktree with unmerged changes, the worktree is preserved with a warning.
func Kill(pool *pgxpool.Pool, tmuxSession, agentID string) error {
	// Get agent info for worktree cleanup.
	a, err := db.GetAgent(pool, agentID)
	if err != nil {
		return fmt.Errorf("getting agent: %w", err)
	}

	// Kill tmux window (ignore error if already gone).
	tmux.KillWindow(tmuxSession, agentID)

	// Handle worktree cleanup.
	if a != nil && a.WorktreeDir != nil {
		unmerged, err := git.HasUnmergedChanges(*a.Branch, "main")
		if err != nil {
			fmt.Printf("warning: could not check unmerged changes for %s: %v\n", agentID, err)
		} else if unmerged {
			fmt.Printf("warning: preserving worktree %s — branch %s has unmerged changes\n", *a.WorktreeDir, *a.Branch)
		} else {
			if err := git.WorktreeRemove(*a.WorktreeDir); err != nil {
				fmt.Printf("warning: failed to remove worktree %s: %v\n", *a.WorktreeDir, err)
			}
		}
	}

	// Delete from DB (also releases claimed tasks).
	if err := db.DeleteAgent(pool, agentID); err != nil {
		return fmt.Errorf("deleting agent from DB: %w", err)
	}

	return nil
}

// KillAll terminates all registered agents.
func KillAll(pool *pgxpool.Pool, tmuxSession string) error {
	agents, err := db.ListAgents(pool)
	if err != nil {
		return fmt.Errorf("listing agents: %w", err)
	}

	for _, a := range agents {
		if err := Kill(pool, tmuxSession, a.ID); err != nil {
			// Log but continue killing others.
			fmt.Printf("warning: failed to kill agent %s: %v\n", a.ID, err)
		}
	}
	return nil
}

// Heartbeat updates an agent's last_seen and status.
func Heartbeat(pool *pgxpool.Pool, agentID, status string) error {
	return db.UpdateAgentStatus(pool, agentID, status)
}

// List returns all registered agents with their task assignments.
func List(pool *pgxpool.Pool) ([]*db.Agent, error) {
	return db.ListAgents(pool)
}
