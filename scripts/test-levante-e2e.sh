#!/usr/bin/env bash
# Levante E2E test: full pipeline with schedule, draft, release, approval, execution.
set -euo pipefail

DB="${DATABASE_URL:?DATABASE_URL not set}"
M="${MINUANO_BIN:-minuano}"
PASS=0
FAIL=0
TEST_PROJECT="e2e-levante-$$"

pass() { echo "  PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

cleanup() {
  echo "Cleaning up test project $TEST_PROJECT..."
  psql "$DB" -q -c "DELETE FROM task_context WHERE task_id IN (SELECT id FROM tasks WHERE project_id='$TEST_PROJECT');" 2>/dev/null || true
  psql "$DB" -q -c "DELETE FROM task_deps WHERE task_id IN (SELECT id FROM tasks WHERE project_id='$TEST_PROJECT');" 2>/dev/null || true
  psql "$DB" -q -c "DELETE FROM tasks WHERE project_id='$TEST_PROJECT';" 2>/dev/null || true
  psql "$DB" -q -c "DELETE FROM schedules WHERE project_id='$TEST_PROJECT';" 2>/dev/null || true
  # Register cleanup agent
  psql "$DB" -q -c "DELETE FROM agents WHERE id='e2e-agent-$$';" 2>/dev/null || true
}
trap cleanup EXIT

# Register test agent.
psql "$DB" -q -c "
  INSERT INTO agents (id, tmux_session, tmux_window, status, started_at, last_seen)
  VALUES ('e2e-agent-$$', 'test', 'test', 'idle', NOW(), NOW())
  ON CONFLICT (id) DO UPDATE SET status='idle', last_seen=NOW();
"

echo "=== Step 1: Create schedule with template ==="

# Write template file.
TEMPLATE=$(mktemp /tmp/blog-template-XXXXXX.json)
cat > "$TEMPLATE" <<'TMPL'
[
  {"ref": "research",   "title": "Research topic",        "body": "Research the blog topic", "priority": 5, "after": []},
  {"ref": "draft-post", "title": "Draft blog post",       "body": "Write the draft",        "priority": 5, "after": ["research"]},
  {"ref": "gen-images", "title": "Generate images",       "body": "Create illustrations",   "priority": 5, "after": ["draft-post"]},
  {"ref": "review",     "title": "Review blog post",      "body": "Human review gate",      "priority": 5, "requires_approval": true, "after": ["gen-images"]},
  {"ref": "publish",    "title": "Publish blog post",     "body": "Deploy to production",   "priority": 5, "after": ["review"]}
]
TMPL

$M schedule add "blog-e2e-$$" --cron "* * * * *" --template "$TEMPLATE" --project "$TEST_PROJECT" 2>&1
rm -f "$TEMPLATE"
pass "Schedule created"

echo ""
echo "=== Step 2: Instantiate template (using schedule run, not waiting for cron) ==="

$M schedule run "blog-e2e-$$" 2>&1
DRAFT_COUNT=$(psql "$DB" -t -A -c "SELECT COUNT(*) FROM tasks WHERE project_id='$TEST_PROJECT' AND status='draft';")

if [ "$DRAFT_COUNT" -eq 5 ]; then
  pass "5 draft tasks created"
else
  fail "Expected 5 draft tasks, got $DRAFT_COUNT"
fi

echo ""
echo "=== Step 3: Release drafts ==="

$M draft-release --all --project "$TEST_PROJECT" 2>&1

# research should be ready (no deps), others pending.
RESEARCH_STATUS=$(psql "$DB" -t -A -c "SELECT status FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Research topic';")
if [ "$RESEARCH_STATUS" = "ready" ]; then
  pass "research transitioned to ready"
else
  fail "research status is '$RESEARCH_STATUS', expected 'ready'"
fi

echo ""
echo "=== Step 4: Simulate agent execution: research → draft-post → gen-images ==="

# Claim and complete research.
RESEARCH_ID=$(psql "$DB" -t -A -c "SELECT id FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Research topic';")
psql "$DB" -q -c "
  UPDATE tasks SET status='claimed', claimed_by='e2e-agent-$$', claimed_at=NOW(), attempt=1 WHERE id='$RESEARCH_ID';
  INSERT INTO task_context (task_id, agent_id, kind, content) VALUES ('$RESEARCH_ID', 'e2e-agent-$$', 'result', 'Research complete');
  UPDATE tasks SET status='done', done_at=NOW(), claimed_by=NULL, claimed_at=NULL WHERE id='$RESEARCH_ID';
"

# draft-post should now be ready.
DRAFT_STATUS=$(psql "$DB" -t -A -c "SELECT status FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Draft blog post';")
if [ "$DRAFT_STATUS" = "ready" ]; then
  pass "draft-post transitioned to ready"
else
  fail "draft-post status is '$DRAFT_STATUS', expected 'ready'"
fi

# Claim and complete draft-post.
DRAFT_POST_ID=$(psql "$DB" -t -A -c "SELECT id FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Draft blog post';")
psql "$DB" -q -c "
  UPDATE tasks SET status='claimed', claimed_by='e2e-agent-$$', claimed_at=NOW(), attempt=1 WHERE id='$DRAFT_POST_ID';
  INSERT INTO task_context (task_id, agent_id, kind, content) VALUES ('$DRAFT_POST_ID', 'e2e-agent-$$', 'result', 'Draft written');
  UPDATE tasks SET status='done', done_at=NOW(), claimed_by=NULL, claimed_at=NULL WHERE id='$DRAFT_POST_ID';
"

# Claim and complete gen-images.
GEN_ID=$(psql "$DB" -t -A -c "SELECT id FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Generate images';")
psql "$DB" -q -c "
  UPDATE tasks SET status='claimed', claimed_by='e2e-agent-$$', claimed_at=NOW(), attempt=1 WHERE id='$GEN_ID';
  INSERT INTO task_context (task_id, agent_id, kind, content) VALUES ('$GEN_ID', 'e2e-agent-$$', 'result', 'Images generated');
  UPDATE tasks SET status='done', done_at=NOW(), claimed_by=NULL, claimed_at=NULL WHERE id='$GEN_ID';
"

# review should transition to pending_approval (requires_approval=true).
REVIEW_STATUS=$(psql "$DB" -t -A -c "SELECT status FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Review blog post';")
if [ "$REVIEW_STATUS" = "pending_approval" ]; then
  pass "review transitioned to pending_approval"
else
  fail "review status is '$REVIEW_STATUS', expected 'pending_approval'"
fi

echo ""
echo "=== Step 5: Check approval would be posted (log-based check) ==="
# In a real test with tramuntana running, the ApprovalHandler would post to APPROVALS_TOPIC_ID.
# Here we verify the task is in the correct state for the handler.
REVIEW_APPROVAL=$(psql "$DB" -t -A -c "SELECT requires_approval FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Review blog post';")
if [ "$REVIEW_APPROVAL" = "t" ]; then
  pass "review has requires_approval=true (approval handler would fire)"
else
  fail "review requires_approval is '$REVIEW_APPROVAL', expected 't'"
fi

echo ""
echo "=== Step 6: Approve review ==="

REVIEW_ID=$(psql "$DB" -t -A -c "SELECT id FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Review blog post';")
$M approve "$REVIEW_ID" --by "e2e-tester" 2>&1

# After approval, review should be ready (claimable), not yet done.
REVIEW_POST_APPROVAL=$(psql "$DB" -t -A -c "SELECT status FROM tasks WHERE id='$REVIEW_ID';")
if [ "$REVIEW_POST_APPROVAL" = "ready" ]; then
  pass "review transitioned to ready after approval"
else
  fail "review status is '$REVIEW_POST_APPROVAL', expected 'ready'"
fi

# publish should still be pending (review not done yet).
PUBLISH_PENDING=$(psql "$DB" -t -A -c "SELECT status FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Publish blog post';")
if [ "$PUBLISH_PENDING" = "pending" ]; then
  pass "publish still pending while review is not done"
else
  fail "publish status is '$PUBLISH_PENDING', expected 'pending'"
fi

echo ""
echo "=== Step 7: Complete review ==="

psql "$DB" -q -c "
  UPDATE tasks SET status='claimed', claimed_by='e2e-agent-$$', claimed_at=NOW(), attempt=1 WHERE id='$REVIEW_ID';
  INSERT INTO task_context (task_id, agent_id, kind, content) VALUES ('$REVIEW_ID', 'e2e-agent-$$', 'result', 'Review approved and completed');
  UPDATE tasks SET status='done', done_at=NOW(), claimed_by=NULL, claimed_at=NULL WHERE id='$REVIEW_ID';
"

# Now publish should cascade to ready.
PUBLISH_STATUS=$(psql "$DB" -t -A -c "SELECT status FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Publish blog post';")
if [ "$PUBLISH_STATUS" = "ready" ]; then
  pass "publish transitioned to ready after review completed"
else
  fail "publish status is '$PUBLISH_STATUS', expected 'ready'"
fi

echo ""
echo "=== Step 8: Complete publish ==="

PUBLISH_ID=$(psql "$DB" -t -A -c "SELECT id FROM tasks WHERE project_id='$TEST_PROJECT' AND title='Publish blog post';")
psql "$DB" -q -c "
  UPDATE tasks SET status='claimed', claimed_by='e2e-agent-$$', claimed_at=NOW(), attempt=1 WHERE id='$PUBLISH_ID';
  INSERT INTO task_context (task_id, agent_id, kind, content) VALUES ('$PUBLISH_ID', 'e2e-agent-$$', 'result', 'Published');
  UPDATE tasks SET status='done', done_at=NOW(), claimed_by=NULL, claimed_at=NULL WHERE id='$PUBLISH_ID';
"

DONE_COUNT=$(psql "$DB" -t -A -c "SELECT COUNT(*) FROM tasks WHERE project_id='$TEST_PROJECT' AND status='done';")
if [ "$DONE_COUNT" -eq 5 ]; then
  pass "all 5 tasks are done"
else
  fail "Expected 5 done tasks, got $DONE_COUNT"
fi

echo ""
echo "========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "========================================="

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
