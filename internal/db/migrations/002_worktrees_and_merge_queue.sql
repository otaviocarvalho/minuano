-- Worktree isolation + merge queue support.

ALTER TABLE agents ADD COLUMN worktree_dir TEXT;
ALTER TABLE agents ADD COLUMN branch TEXT;

CREATE TABLE merge_queue (
  id            BIGSERIAL PRIMARY KEY,
  task_id       TEXT NOT NULL REFERENCES tasks(id),
  agent_id      TEXT NOT NULL,
  branch        TEXT NOT NULL,
  worktree_dir  TEXT NOT NULL,
  base_branch   TEXT NOT NULL DEFAULT 'main',
  status        TEXT NOT NULL DEFAULT 'pending',
  commit_sha    TEXT,
  merge_sha     TEXT,
  conflict_files TEXT[],
  error_msg     TEXT,
  enqueued_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  started_at    TIMESTAMPTZ,
  completed_at  TIMESTAMPTZ
);
CREATE INDEX idx_merge_queue_status ON merge_queue(status, enqueued_at ASC);
