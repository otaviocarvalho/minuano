-- NOTIFY triggers for task and planner session status transitions.

-- Task status change notifications.
CREATE OR REPLACE FUNCTION notify_task_status()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
  old_status text;
BEGIN
  IF TG_OP = 'INSERT' THEN
    old_status := 'none';
  ELSE
    old_status := OLD.status;
  END IF;

  -- Only fire when status actually changed (or on insert).
  IF TG_OP = 'INSERT' OR NEW.status != OLD.status THEN
    PERFORM pg_notify(
      'task_events',
      json_build_object(
        'task_id',    NEW.id,
        'title',      NEW.title,
        'status',     NEW.status,
        'old_status', old_status,
        'project_id', COALESCE(NEW.project_id, ''),
        'agent_id',   COALESCE(NEW.claimed_by, ''),
        'ts',         extract(epoch from now())
      )::text
    );
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER on_task_status_notify
AFTER INSERT OR UPDATE OF status ON tasks
FOR EACH ROW
EXECUTE FUNCTION notify_task_status();

-- Planner session status change notifications.
CREATE OR REPLACE FUNCTION notify_planner_status()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
  old_status text;
BEGIN
  IF TG_OP = 'INSERT' THEN
    old_status := 'none';
  ELSE
    old_status := OLD.status;
  END IF;

  IF TG_OP = 'INSERT' OR NEW.status != OLD.status THEN
    PERFORM pg_notify(
      'planner_events',
      json_build_object(
        'session_id',  NEW.id,
        'topic_id',    NEW.topic_id,
        'project_id',  COALESCE(NEW.project_id, ''),
        'status',      NEW.status,
        'old_status',  old_status
      )::text
    );
  END IF;

  RETURN NEW;
END;
$$;

CREATE TRIGGER on_planner_status_notify
AFTER INSERT OR UPDATE OF status ON planner_sessions
FOR EACH ROW
EXECUTE FUNCTION notify_planner_status();
