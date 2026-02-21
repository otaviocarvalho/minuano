package db

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestTaskJSONTags(t *testing.T) {
	task := Task{
		ID:        "test-123",
		Title:     "Test Task",
		Body:      "Do something",
		Status:    "ready",
		Priority:  5,
		CreatedAt: time.Now(),
		Attempt:   0,
		MaxAttempts: 3,
	}

	data, err := json.Marshal(task)
	if err != nil {
		t.Fatalf("failed to marshal Task: %v", err)
	}

	s := string(data)
	requiredFields := []string{
		`"id"`, `"title"`, `"body"`, `"status"`, `"priority"`,
		`"created_at"`, `"attempt"`, `"max_attempts"`,
	}
	for _, f := range requiredFields {
		if !strings.Contains(s, f) {
			t.Errorf("expected %s in JSON, got: %s", f, s)
		}
	}

	// Omitempty fields should not appear when nil/zero.
	omittedFields := []string{`"capability"`, `"claimed_by"`, `"claimed_at"`, `"done_at"`, `"project_id"`}
	for _, f := range omittedFields {
		if strings.Contains(s, f) {
			t.Errorf("expected %s to be omitted from JSON, got: %s", f, s)
		}
	}
}

func TestTaskContextJSONTags(t *testing.T) {
	ctx := TaskContext{
		ID:        1,
		TaskID:    "test-123",
		Kind:      "observation",
		Content:   "Found something",
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(ctx)
	if err != nil {
		t.Fatalf("failed to marshal TaskContext: %v", err)
	}

	s := string(data)
	requiredFields := []string{`"id"`, `"task_id"`, `"kind"`, `"content"`, `"created_at"`}
	for _, f := range requiredFields {
		if !strings.Contains(s, f) {
			t.Errorf("expected %s in JSON, got: %s", f, s)
		}
	}
}

func TestAgentJSONTags(t *testing.T) {
	agent := Agent{
		ID:          "agent-1",
		TmuxSession: "minuano",
		TmuxWindow:  "agent-1",
		Status:      "idle",
		StartedAt:   time.Now(),
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("failed to marshal Agent: %v", err)
	}

	s := string(data)
	requiredFields := []string{`"id"`, `"tmux_session"`, `"tmux_window"`, `"status"`, `"started_at"`}
	for _, f := range requiredFields {
		if !strings.Contains(s, f) {
			t.Errorf("expected %s in JSON, got: %s", f, s)
		}
	}

	// Omitempty fields should not appear when nil.
	omittedFields := []string{`"worktree_dir"`, `"branch"`}
	for _, f := range omittedFields {
		if strings.Contains(s, f) {
			t.Errorf("expected %s to be omitted from JSON, got: %s", f, s)
		}
	}

	// With worktree fields set.
	wt := "/tmp/worktree"
	br := "minuano/agent-1"
	agentWT := Agent{
		ID:          "agent-2",
		TmuxSession: "minuano",
		TmuxWindow:  "agent-2",
		Status:      "idle",
		StartedAt:   time.Now(),
		WorktreeDir: &wt,
		Branch:      &br,
	}
	data, err = json.Marshal(agentWT)
	if err != nil {
		t.Fatalf("failed to marshal Agent with worktree: %v", err)
	}
	s = string(data)
	for _, f := range []string{`"worktree_dir"`, `"branch"`} {
		if !strings.Contains(s, f) {
			t.Errorf("expected %s in JSON, got: %s", f, s)
		}
	}
}
