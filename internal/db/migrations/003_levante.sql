-- Levante: draft/pending_approval statuses, approval columns, schedules, planner sessions.

-- 1. New columns on tasks for approval workflow.
ALTER TABLE tasks ADD COLUMN requires_approval BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE tasks ADD COLUMN approved_by       TEXT;
ALTER TABLE tasks ADD COLUMN approved_at       TIMESTAMPTZ;
ALTER TABLE tasks ADD COLUMN rejection_reason  TEXT;

-- 2. Schedules table for recurring jobs.
CREATE TABLE schedules (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  name        TEXT        NOT NULL UNIQUE,
  description TEXT,
  cron        TEXT        NOT NULL,
  template    JSONB       NOT NULL,
  project_id  TEXT,
  enabled     BOOLEAN     NOT NULL DEFAULT true,
  last_run    TIMESTAMPTZ,
  next_run    TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 3. Planner sessions table.
CREATE TABLE planner_sessions (
  id          UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  topic_id    BIGINT      NOT NULL UNIQUE,
  project_id  TEXT,
  tmux_window TEXT,
  status      TEXT        NOT NULL DEFAULT 'stopped',
  started_at  TIMESTAMPTZ,
  stopped_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 4. Update the cascade trigger to handle requires_approval.
--    When a task becomes done, dependents that have requires_approval=true
--    transition to pending_approval instead of ready.
CREATE OR REPLACE FUNCTION refresh_ready_tasks()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.status = 'done' THEN
    -- Dependents WITHOUT requires_approval → ready
    UPDATE tasks
    SET status = 'ready'
    WHERE status = 'pending'
      AND requires_approval = false
      AND id IN (
        SELECT task_id FROM task_deps WHERE depends_on = NEW.id
      )
      AND id NOT IN (
        SELECT td.task_id
        FROM task_deps td
        JOIN tasks t ON t.id = td.depends_on
        WHERE t.status != 'done'
      );

    -- Dependents WITH requires_approval → pending_approval
    UPDATE tasks
    SET status = 'pending_approval'
    WHERE status = 'pending'
      AND requires_approval = true
      AND id IN (
        SELECT task_id FROM task_deps WHERE depends_on = NEW.id
      )
      AND id NOT IN (
        SELECT td.task_id
        FROM task_deps td
        JOIN tasks t ON t.id = td.depends_on
        WHERE t.status != 'done'
      );
  END IF;
  RETURN NEW;
END;
$$;
