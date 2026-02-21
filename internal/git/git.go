package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// ConflictError is returned when a merge has conflicts.
type ConflictError struct {
	Files []string
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("merge conflict in %d file(s): %s", len(e.Files), strings.Join(e.Files, ", "))
}

// RepoRoot returns the root directory of the current git repository.
func RepoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// CurrentBranch returns the current branch name.
func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("getting current branch: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// HasUncommittedChanges returns true if there are uncommitted changes in the working tree.
func HasUncommittedChanges() (bool, error) {
	err := exec.Command("git", "diff", "--quiet", "HEAD").Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return true, nil
			}
		}
		return false, fmt.Errorf("checking uncommitted changes: %w", err)
	}
	return false, nil
}

// WorktreeAdd creates a new worktree at the given directory on a new branch.
func WorktreeAdd(dir, branch string) error {
	out, err := exec.Command("git", "worktree", "add", "-b", branch, dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("adding worktree: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// WorktreeRemove removes a worktree at the given directory.
func WorktreeRemove(dir string) error {
	out, err := exec.Command("git", "worktree", "remove", "--force", dir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("removing worktree: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// HasUnmergedChanges returns true if the branch has commits not in baseBranch.
func HasUnmergedChanges(branch, baseBranch string) (bool, error) {
	out, err := exec.Command("git", "log", "--oneline", baseBranch+".."+branch).Output()
	if err != nil {
		return false, fmt.Errorf("checking unmerged changes: %w", err)
	}
	return strings.TrimSpace(string(out)) != "", nil
}

// AddAndCommit stages all changes and commits in the given worktree directory.
// Returns the commit SHA.
func AddAndCommit(worktreeDir, message string) (string, error) {
	addCmd := exec.Command("git", "-C", worktreeDir, "add", "-A")
	if out, err := addCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Check if there's anything to commit.
	if err := exec.Command("git", "-C", worktreeDir, "diff", "--cached", "--quiet").Run(); err == nil {
		return "", nil // Nothing to commit.
	}

	commitCmd := exec.Command("git", "-C", worktreeDir, "commit", "-m", message)
	if out, err := commitCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit: %s: %w", strings.TrimSpace(string(out)), err)
	}

	shaCmd := exec.Command("git", "-C", worktreeDir, "rev-parse", "HEAD")
	out, err := shaCmd.Output()
	if err != nil {
		return "", fmt.Errorf("getting commit sha: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// MergeNoFF performs a no-fast-forward merge of branch into baseBranch.
// Returns the merge commit SHA on success, or a *ConflictError on conflict.
func MergeNoFF(branch, baseBranch, message string) (string, error) {
	// Checkout base branch.
	if out, err := exec.Command("git", "checkout", baseBranch).CombinedOutput(); err != nil {
		return "", fmt.Errorf("checkout %s: %s: %w", baseBranch, strings.TrimSpace(string(out)), err)
	}

	// Attempt merge.
	mergeCmd := exec.Command("git", "merge", "--no-ff", "-m", message, branch)
	out, err := mergeCmd.CombinedOutput()
	if err != nil {
		// Check for conflict.
		conflictFiles, conflictErr := getConflictFiles()
		if conflictErr == nil && len(conflictFiles) > 0 {
			return "", &ConflictError{Files: conflictFiles}
		}
		return "", fmt.Errorf("merge failed: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Get merge commit SHA.
	shaOut, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("getting merge sha: %w", err)
	}
	return strings.TrimSpace(string(shaOut)), nil
}

// AbortMerge aborts an in-progress merge.
func AbortMerge() error {
	out, err := exec.Command("git", "merge", "--abort").CombinedOutput()
	if err != nil {
		return fmt.Errorf("aborting merge: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func getConflictFiles() ([]string, error) {
	out, err := exec.Command("git", "diff", "--name-only", "--diff-filter=U").Output()
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}
