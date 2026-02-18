# Task 08 — `minuano migrate`

## Goal

Implement the migration command that applies pending SQL migrations.

## Steps

1. Add `minuano migrate` subcommand.
2. Connect to the database using `db.Connect()`.
3. Call `db.RunMigrations()`.
4. Print which migrations were applied (e.g., `✓ Applied: 001_initial.sql`).
5. If all migrations are already applied, print a "nothing to apply" message.

## Acceptance

- `minuano migrate` applies `001_initial.sql` to a fresh database.
- Running it again reports nothing to apply.
- Tables from the migration exist in the database.

## Phase

2 — Core CLI Commands

## Depends on

- Task 07
