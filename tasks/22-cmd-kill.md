# Task 22 — `minuano kill`

## Goal

Implement the agent termination command.

## Steps

1. Add `minuano kill [agent-id]` subcommand.
2. Flag `--all` — kill all agents.
3. When killing an agent:
   - Release any claimed task (reset to `ready`).
   - Kill the tmux window.
   - Remove the agent from the DB.
4. Print confirmation for each killed agent.

## Acceptance

- `minuano kill agent-1` terminates a single agent and releases its task.
- `minuano kill --all` terminates all agents.
- After killing, `minuano agents` no longer shows the killed agent(s).
- Previously claimed tasks are back in `ready` state.

## Phase

4 — Agent CLI Commands

## Depends on

- Task 19
