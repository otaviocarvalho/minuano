-- Remove capability column from tasks (all agents have the same capabilities).
ALTER TABLE tasks DROP COLUMN IF EXISTS capability;
