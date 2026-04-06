package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper to init a project in a temp dir and return the dir path.
func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte("# Test Spec\n\nSome content.\n"), 0o600)
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)
	return dir
}

// helper to add a task via JSON
func addTask(t *testing.T, dir, taskJSON string) {
	t.Helper()
	_, stderr, code := runTP(t, dir, "add", taskJSON)
	require.Equal(t, 0, code, "add failed: %s", stderr)
}

func Test2CallWorkflow(t *testing.T) {
	dir := setupProject(t)

	// Add 3 tasks: a, b depends on a, c depends on b
	addTask(t, dir, `{"id":"a","title":"Task A","depends_on":[],"estimate_minutes":10,"acceptance":"A is done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"Task B","depends_on":["a"],"estimate_minutes":15,"acceptance":"B is done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"c","title":"Task C","depends_on":["b"],"estimate_minutes":20,"acceptance":"C is done","source_sections":["s1"]}`)

	// tp plan --json
	stdout, stderr, code := runTP(t, dir, "plan")
	require.Equal(t, 0, code, "plan failed: %s", stderr)

	var planOut map[string]any
	err := json.Unmarshal([]byte(stdout), &planOut)
	require.NoError(t, err, "plan output should be valid JSON")

	execOrder, ok := planOut["execution_order"].([]any)
	require.True(t, ok, "execution_order should be an array")
	require.Len(t, execOrder, 3)

	// Verify topological order: a before b before c
	ids := make([]string, 3)
	for i, item := range execOrder {
		task := item.(map[string]any)
		ids[i] = task["id"].(string)
	}
	assert.Equal(t, "a", ids[0])
	assert.Equal(t, "b", ids[1])
	assert.Equal(t, "c", ids[2])

	// Write results.ndjson with closure reasons for all 3
	ndjsonPath := filepath.Join(dir, "results.ndjson")
	lines := []string{
		`{"id":"a","reason":"A is done and fully verified with complete implementation"}`,
		`{"id":"b","reason":"B is done and fully verified with complete implementation"}`,
		`{"id":"c","reason":"C is done and fully verified with complete implementation"}`,
	}
	err = os.WriteFile(ndjsonPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	require.NoError(t, err)

	// tp done --batch results.ndjson
	stdout, stderr, code = runTP(t, dir, "done", "--batch", ndjsonPath)
	require.Equal(t, 0, code, "done --batch failed: %s\nstdout: %s", stderr, stdout)

	var batchOut map[string]any
	err = json.Unmarshal([]byte(stdout), &batchOut)
	require.NoError(t, err)
	assert.Equal(t, float64(3), batchOut["closed"])
	assert.Equal(t, float64(0), batchOut["failed"])

	// tp status → verify done=3
	stdout, _, code = runTP(t, dir, "status")
	require.Equal(t, 0, code)

	var statusOut map[string]any
	err = json.Unmarshal([]byte(stdout), &statusOut)
	require.NoError(t, err)
	assert.Equal(t, float64(3), statusOut["done"])
	assert.Equal(t, float64(3), statusOut["total"])
}

func TestDoneSingleImplicitClaim(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"t1","title":"Solo task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	// tp done t1 "reason" -- should work with implicit claim (open -> wip -> done)
	stdout, stderr, code := runTP(t, dir, "done", "t1", "Task complete and verified with full implementation details")
	require.Equal(t, 0, code, "done with implicit claim failed: stderr=%s stdout=%s", stderr, stdout)

	var doneOut map[string]any
	err := json.Unmarshal([]byte(stdout), &doneOut)
	require.NoError(t, err)
	assert.Equal(t, "t1", doneOut["closed"])

	// Verify task is done via status
	stdout, _, code = runTP(t, dir, "status")
	require.Equal(t, 0, code)

	var statusOut map[string]any
	err = json.Unmarshal([]byte(stdout), &statusOut)
	require.NoError(t, err)
	assert.Equal(t, float64(1), statusOut["done"])
}

func TestNextWIPResume(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"a","title":"Task A","depends_on":[],"estimate_minutes":10,"acceptance":"A done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"Task B","depends_on":["a"],"estimate_minutes":15,"acceptance":"B done","source_sections":["s1"]}`)

	// tp next → claims a, returns a
	stdout, stderr, code := runTP(t, dir, "next")
	require.Equal(t, 0, code, "next failed: %s", stderr)

	var nextOut map[string]any
	err := json.Unmarshal([]byte(stdout), &nextOut)
	require.NoError(t, err)
	taskMap := nextOut["task"].(map[string]any)
	assert.Equal(t, "a", taskMap["id"])
	assert.Equal(t, "wip", taskMap["status"])

	// tp next → returns a again (WIP resume, idempotent)
	stdout, stderr, code = runTP(t, dir, "next")
	require.Equal(t, 0, code, "next (resume) failed: %s", stderr)

	err = json.Unmarshal([]byte(stdout), &nextOut)
	require.NoError(t, err)
	taskMap = nextOut["task"].(map[string]any)
	assert.Equal(t, "a", taskMap["id"], "should resume WIP task a")

	// tp done a "reason"
	_, stderr, code = runTP(t, dir, "done", "a", "A done and fully implemented and verified completely")
	require.Equal(t, 0, code, "done a failed: %s", stderr)

	// tp next → claims b, returns b
	stdout, stderr, code = runTP(t, dir, "next")
	require.Equal(t, 0, code, "next (after done a) failed: %s", stderr)

	err = json.Unmarshal([]byte(stdout), &nextOut)
	require.NoError(t, err)
	taskMap = nextOut["task"].(map[string]any)
	assert.Equal(t, "b", taskMap["id"])
	assert.Equal(t, "wip", taskMap["status"])
}

func TestListFilters(t *testing.T) {
	dir := setupProject(t)

	// Add tasks with different statuses and tags
	addTask(t, dir, `{"id":"t1","title":"Auth login","status":"open","tags":["auth"],"depends_on":[],"estimate_minutes":10,"acceptance":"Login works","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Auth logout","status":"open","tags":["auth","api"],"depends_on":[],"estimate_minutes":5,"acceptance":"Logout works","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t3","title":"Dashboard","status":"open","tags":["ui"],"depends_on":[],"estimate_minutes":20,"acceptance":"Dashboard renders","source_sections":["s1"]}`)

	// Claim t1 to make it wip
	_, stderr, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim failed: %s", stderr)

	// tp list --status open → only open tasks (t2, t3)
	stdout, stderr, code := runTP(t, dir, "list", "--status", "open")
	require.Equal(t, 0, code, "list --status failed: %s", stderr)

	var listOut []map[string]any
	err := json.Unmarshal([]byte(stdout), &listOut)
	require.NoError(t, err)
	require.Len(t, listOut, 2)
	listIDs := []string{listOut[0]["id"].(string), listOut[1]["id"].(string)}
	assert.Contains(t, listIDs, "t2")
	assert.Contains(t, listIDs, "t3")
	assert.NotContains(t, listIDs, "t1")

	// tp list --tag auth → only auth-tagged tasks (t1, t2)
	stdout, _, code = runTP(t, dir, "list", "--tag", "auth")
	require.Equal(t, 0, code)

	err = json.Unmarshal([]byte(stdout), &listOut)
	require.NoError(t, err)
	require.Len(t, listOut, 2)
	listIDs = []string{listOut[0]["id"].(string), listOut[1]["id"].(string)}
	assert.Contains(t, listIDs, "t1")
	assert.Contains(t, listIDs, "t2")

	// tp list --ids → just IDs (newline-separated, not JSON)
	stdout, _, code = runTP(t, dir, "list", "--ids")
	require.Equal(t, 0, code)
	idLines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.Len(t, idLines, 3)
	assert.Contains(t, idLines, "t1")
	assert.Contains(t, idLines, "t2")
	assert.Contains(t, idLines, "t3")
}

func TestDoneBatchPartialFailure(t *testing.T) {
	dir := setupProject(t)

	// Add 2 tasks
	addTask(t, dir, `{"id":"t1","title":"Good task","depends_on":[],"estimate_minutes":5,"acceptance":"Good task acceptance criteria fulfilled","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Bad task","depends_on":[],"estimate_minutes":5,"acceptance":"Bad task needs very detailed and specific acceptance criteria that must be addressed in the reason","source_sections":["s1"]}`)

	// Write NDJSON: t1 with good reason, t2 with bad reason (too short)
	ndjsonPath := filepath.Join(dir, "batch.ndjson")
	lines := []string{
		`{"id":"t1","reason":"Good task acceptance criteria fulfilled completely and verified"}`,
		`{"id":"t2","reason":"short"}`,
	}
	err := os.WriteFile(ndjsonPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	require.NoError(t, err)

	// tp done --batch → should have partial failure (exit code != 0 for partial)
	stdout, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	// Partial failure exits with ExitValidation (1)
	assert.Equal(t, 1, code, "partial batch failure should exit with code 1")

	var batchOut map[string]any
	err = json.Unmarshal([]byte(stdout), &batchOut)
	require.NoError(t, err)
	assert.Equal(t, float64(1), batchOut["closed"])
	assert.Equal(t, float64(1), batchOut["failed"])

	// Verify t1 is done, t2 is NOT done
	stdout, _, code = runTP(t, dir, "list", "--status", "done", "--ids")
	require.Equal(t, 0, code)
	assert.Contains(t, strings.TrimSpace(stdout), "t1")
	assert.NotContains(t, strings.TrimSpace(stdout), "t2")

	// t2 may be open or wip (batch implicit claim may have transitioned it)
	// Just verify it's not done
	stdout, _, code = runTP(t, dir, "list", "--status", "open,wip", "--ids")
	require.Equal(t, 0, code)
	assert.Contains(t, strings.TrimSpace(stdout), "t2")
}

func TestNilSlicesInJSON(t *testing.T) {
	dir := setupProject(t)

	// Add 1 task with no dependents → blocks should be []
	addTask(t, dir, `{"id":"solo","title":"Solo task","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)

	// tp show → blocks should be [] not null
	stdout, stderr, code := runTP(t, dir, "show", "solo")
	require.Equal(t, 0, code, "show failed: %s", stderr)

	var showOut map[string]any
	err := json.Unmarshal([]byte(stdout), &showOut)
	require.NoError(t, err)
	blocks, ok := showOut["blocks"].([]any)
	require.True(t, ok, "blocks should be an array, not null")
	assert.Empty(t, blocks, "blocks should be empty")

	// Close the solo task so no tasks are ready
	_, stderr, code = runTP(t, dir, "done", "solo", "Solo task is done and fully verified completely")
	require.Equal(t, 0, code, "done failed: %s", stderr)

	// tp ready → should be [] not null
	stdout, _, code = runTP(t, dir, "ready")
	require.Equal(t, 0, code)

	var readyOut []any
	err = json.Unmarshal([]byte(stdout), &readyOut)
	require.NoError(t, err, "ready output should be valid JSON array")
	assert.Empty(t, readyOut, "ready should be empty array")

	// tp blocked → should be [] not null (no blocked tasks since all done)
	stdout, _, code = runTP(t, dir, "blocked")
	require.Equal(t, 0, code)

	var blockedOut []any
	err = json.Unmarshal([]byte(stdout), &blockedOut)
	require.NoError(t, err, "blocked output should be valid JSON array")
	assert.Empty(t, blockedOut, "blocked should be empty array")
}

func TestImportWorkflow(t *testing.T) {
	dir := t.TempDir()

	// Create a spec file
	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte("# Test Spec\n\nSome content.\n"), 0o600)
	require.NoError(t, err)

	// Create a complete task file JSON
	taskFileJSON := `{
		"version": 1,
		"spec": "spec.md",
		"created_at": "2025-01-01T00:00:00Z",
		"updated_at": "2025-01-01T00:00:00Z",
		"workflow": {"quality_gate": "tests_pass"},
		"coverage": {"total_sections": 1, "mapped_sections": 1, "context_only": [], "unmapped": []},
		"tasks": [
			{"id":"imp-1","title":"Import task 1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"Task 1 done","source_sections":["# Test Spec"],"closed_at":null,"closed_reason":null,"gate_passed_at":null,"commit_sha":null},
			{"id":"imp-2","title":"Import task 2","status":"open","depends_on":["imp-1"],"estimate_minutes":10,"acceptance":"Task 2 done","source_sections":["# Test Spec"],"closed_at":null,"closed_reason":null,"gate_passed_at":null,"commit_sha":null}
		]
	}`
	importPath := filepath.Join(dir, "import.json")
	err = os.WriteFile(importPath, []byte(taskFileJSON), 0o600)
	require.NoError(t, err)

	// tp import file.json
	_, stderr, code := runTP(t, dir, "import", importPath)
	require.Equal(t, 0, code, "import failed: %s", stderr)

	// tp status → verify task count matches
	stdout, stderr, code := runTP(t, dir, "status")
	require.Equal(t, 0, code, "status failed: %s", stderr)

	var statusOut map[string]any
	err = json.Unmarshal([]byte(stdout), &statusOut)
	require.NoError(t, err)
	assert.Equal(t, float64(2), statusOut["total"])
}

func TestDoneWithCommitAndGatePassed(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"gated","title":"Gated task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	// tp done with --gate-passed --commit abc123
	_, stderr, code := runTP(t, dir, "done", "gated", "Task complete and fully verified with all criteria met", "--gate-passed", "--commit", "abc123")
	require.Equal(t, 0, code, "done failed: %s", stderr)

	// tp show → verify gate_passed_at is not null, commit_sha is "abc123"
	stdout, stderr, code := runTP(t, dir, "show", "gated")
	require.Equal(t, 0, code, "show failed: %s", stderr)

	var showOut map[string]any
	err := json.Unmarshal([]byte(stdout), &showOut)
	require.NoError(t, err)
	assert.NotNil(t, showOut["gate_passed_at"], "gate_passed_at should not be null")
	assert.Equal(t, "abc123", showOut["commit_sha"])
}

func TestClaimMultipleIDs(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"m1","title":"Multi 1","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"m2","title":"Multi 2","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"m3","title":"Multi 3","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)

	// tp claim m1 m2 → should claim both
	stdout, stderr, code := runTP(t, dir, "claim", "m1", "m2")
	require.Equal(t, 0, code, "claim failed: %s", stderr)

	var claimOut map[string]any
	err := json.Unmarshal([]byte(stdout), &claimOut)
	require.NoError(t, err)

	claimed, ok := claimOut["claimed"].([]any)
	require.True(t, ok, "claimed should be an array")
	assert.Len(t, claimed, 2)
	assert.Contains(t, claimed, "m1")
	assert.Contains(t, claimed, "m2")
}

func TestPlanSpecExcerpt(t *testing.T) {
	dir := t.TempDir()

	// Create a spec with known content at specific lines
	specContent := "Line one\nLine two\nLine three\nLine four\nLine five\n"
	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte(specContent), 0o600)
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)

	// Add a task with source_lines: "2-4"
	addTask(t, dir, `{"id":"excerpt","title":"Excerpt task","depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"],"source_lines":"2-4"}`)

	// tp plan → verify spec_excerpt contains lines 2-4
	stdout, stderr, code := runTP(t, dir, "plan")
	require.Equal(t, 0, code, "plan failed: %s", stderr)

	var planOut map[string]any
	err = json.Unmarshal([]byte(stdout), &planOut)
	require.NoError(t, err)

	execOrder, ok := planOut["execution_order"].([]any)
	require.True(t, ok)
	require.Len(t, execOrder, 1)

	task := execOrder[0].(map[string]any)
	excerpt, ok := task["spec_excerpt"].(string)
	require.True(t, ok, "spec_excerpt should be a string")
	assert.Contains(t, excerpt, "Line two")
	assert.Contains(t, excerpt, "Line three")
	assert.Contains(t, excerpt, "Line four")
}

func TestValidateStrict(t *testing.T) {
	dir := t.TempDir()

	// Create a spec with NO headings so coverage validation passes cleanly
	specPath := filepath.Join(dir, "spec.md")
	err := os.WriteFile(specPath, []byte("Just some plain text content.\n"), 0o600)
	require.NoError(t, err)

	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init failed: %s", stderr)

	// Add a task with estimate_minutes: 20 (exceeds 15) - no source_sections to avoid coverage issues
	addTask(t, dir, `{"id":"big","title":"Big task","depends_on":[],"estimate_minutes":20,"acceptance":"Done","source_sections":[]}`)

	// tp validate → should be valid (warnings only)
	stdout, stderr, code := runTP(t, dir, "validate")
	require.Equal(t, 0, code, "validate should pass (warnings only): stderr=%s stdout=%s", stderr, stdout)

	var valOut map[string]any
	err = json.Unmarshal([]byte(stdout), &valOut)
	require.NoError(t, err)
	assert.Equal(t, true, valOut["valid"])

	// tp validate --strict → should be invalid (exit 1)
	stdout, _, code = runTP(t, dir, "validate", "--strict")
	assert.Equal(t, 1, code, "validate --strict should fail with exit 1")

	err = json.Unmarshal([]byte(stdout), &valOut)
	require.NoError(t, err)
	assert.Equal(t, false, valOut["valid"])
}

func TestDoneBatchAllFail(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"fail1","title":"Fail task 1","depends_on":[],"estimate_minutes":5,"acceptance":"Very specific acceptance criteria that must be fully addressed","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"fail2","title":"Fail task 2","depends_on":[],"estimate_minutes":5,"acceptance":"Another very specific acceptance criteria that must be fully addressed","source_sections":["s1"]}`)

	// Write batch with bad reasons for both
	ndjsonPath := filepath.Join(dir, "allfail.ndjson")
	lines := []string{
		`{"id":"fail1","reason":"bad"}`,
		`{"id":"fail2","reason":"bad"}`,
	}
	err := os.WriteFile(ndjsonPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	require.NoError(t, err)

	// tp done --batch → closed=0, failed=2, exit 4 (ExitState)
	stdout, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	assert.Equal(t, 4, code, "all-fail batch should exit with code 4 (ExitState)")

	var batchOut map[string]any
	err = json.Unmarshal([]byte(stdout), &batchOut)
	require.NoError(t, err)
	assert.Equal(t, float64(0), batchOut["closed"])
	assert.Equal(t, float64(2), batchOut["failed"])
}

func TestDoneBatchIdempotent(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"idem","title":"Idempotent task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	// Close the task normally
	_, stderr, code := runTP(t, dir, "done", "idem", "Task complete and fully verified with all acceptance criteria")
	require.Equal(t, 0, code, "done failed: %s", stderr)

	// Batch close same task again → should be idempotent
	ndjsonPath := filepath.Join(dir, "idem.ndjson")
	err := os.WriteFile(ndjsonPath, []byte(`{"id":"idem","reason":"Task complete and fully verified with all acceptance criteria"}`+"\n"), 0o600)
	require.NoError(t, err)

	stdout, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	require.Equal(t, 0, code, "idempotent batch should succeed")

	var batchOut map[string]any
	err = json.Unmarshal([]byte(stdout), &batchOut)
	require.NoError(t, err)
	assert.Equal(t, float64(0), batchOut["closed"], "closed count should not include idempotent skips")
	assert.Equal(t, float64(1), batchOut["skipped"], "skipped count should include idempotent re-close")
	assert.Equal(t, float64(0), batchOut["failed"])
}

func TestListTagFilter(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"auth1","title":"Auth login","tags":["auth"],"depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"auth2","title":"Auth logout","tags":["auth","api"],"depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"ui1","title":"Dashboard","tags":["ui"],"depends_on":[],"estimate_minutes":5,"acceptance":"Done","source_sections":["s1"]}`)

	// tp list --tag auth → only auth-tagged tasks
	stdout, stderr, code := runTP(t, dir, "list", "--tag", "auth")
	require.Equal(t, 0, code, "list --tag failed: %s", stderr)

	var listOut []map[string]any
	err := json.Unmarshal([]byte(stdout), &listOut)
	require.NoError(t, err)
	require.Len(t, listOut, 2)

	ids := []string{listOut[0]["id"].(string), listOut[1]["id"].(string)}
	assert.Contains(t, ids, "auth1")
	assert.Contains(t, ids, "auth2")
	assert.NotContains(t, ids, "ui1")
}

func TestRemoveForceCleansDepsToEmptyArray(t *testing.T) {
	dir := setupProject(t)

	addTask(t, dir, `{"id":"base","title":"Base task","estimate_minutes":3,"acceptance":"Base done.","source_sections":["# Test Spec"],"depends_on":[]}`)
	addTask(t, dir, `{"id":"child","title":"Child task","estimate_minutes":3,"acceptance":"Child done.","source_sections":["# Test Spec"],"depends_on":["base"]}`)

	// remove --force should clean up child's depends_on to [] not null
	_, _, code := runTP(t, dir, "remove", "base", "--force")
	require.Equal(t, 0, code)

	// Verify child's depends_on is empty array, not null
	stdout, _, code := runTP(t, dir, "show", "child")
	require.Equal(t, 0, code)

	var result map[string]any
	err := json.Unmarshal([]byte(stdout), &result)
	require.NoError(t, err)

	deps := result["depends_on"]
	require.NotNil(t, deps, "depends_on should not be null")

	depsArr, ok := deps.([]any)
	require.True(t, ok, "depends_on should be an array")
	assert.Empty(t, depsArr, "depends_on should be empty after force remove")
}
