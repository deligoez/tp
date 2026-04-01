package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func addTaskWithEstimate(t *testing.T, dir, id string, estimate int) {
	t.Helper()
	taskJSON := `{"id":"` + id + `","title":"Task ` + id + `","status":"open","depends_on":[],"estimate_minutes":` +
		fmt.Sprintf("%d", estimate) + `,"acceptance":"` + id + ` acceptance criteria met","source_sections":["s1"]}`
	_, _, code := runTP(t, dir, "add", taskJSON)
	require.Equal(t, 0, code, "add task %s should succeed", id)
}

// --- started_at tests ---

func TestClaimSetsStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	before := time.Now().UTC()
	stdout, _, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code)

	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	assert.Equal(t, "wip", task["status"])
	assert.NotNil(t, task["started_at"], "claim should set started_at")

	// Parse and verify it's recent
	startedAt, err := time.Parse(time.RFC3339Nano, task["started_at"].(string))
	require.NoError(t, err)
	assert.True(t, startedAt.After(before.Add(-time.Second)), "started_at should be recent")
}

func TestClaimBatchSetsStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	addTaskWithEstimate(t, dir, "t2", 5)

	stdout, _, code := runTP(t, dir, "claim", "--all-ready")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	claimed := result["claimed"].([]any)
	assert.Len(t, claimed, 2)

	// Verify both tasks have started_at set via list
	stdout, _, _ = runTP(t, dir, "list", "--status", "wip")
	var tasks []map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &tasks))
	require.Len(t, tasks, 2)
	for _, task := range tasks {
		assert.NotNil(t, task["started_at"], "batch claim should set started_at on %s", task["id"])
	}
}

func TestNextSetsStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	stdout, _, code := runTP(t, dir, "next")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	task := result["task"].(map[string]any)
	assert.Equal(t, "wip", task["status"])
	assert.NotNil(t, task["started_at"], "next should set started_at")
}

func TestNextPeekDoesNotSetStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	stdout, _, code := runTP(t, dir, "next", "--peek")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	task := result["task"].(map[string]any)
	assert.Equal(t, "open", task["status"])
	assert.Nil(t, task["started_at"], "peek should not set started_at")
}

func TestDoneImplicitClaimSetsStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// tp done on open task = implicit claim + close
	_, _, code := runTP(t, dir, "done", "t1", "t1 acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)

	// Verify started_at was set via show (show embeds Task at root)
	stdout, _, _ := runTP(t, dir, "show", "t1")
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	assert.NotNil(t, task["started_at"], "implicit claim in done should set started_at")
	assert.NotNil(t, task["closed_at"])
}

func TestDoneOnAlreadyClaimedPreservesStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Claim first
	stdout, _, _ := runTP(t, dir, "claim", "t1")
	var claimed map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &claimed))
	originalStartedAt := claimed["started_at"].(string)

	// Wait a tiny bit so timestamps differ
	time.Sleep(10 * time.Millisecond)

	// Done should NOT overwrite started_at
	_, _, code := runTP(t, dir, "done", "t1", "t1 acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)

	stdout, _, _ = runTP(t, dir, "show", "t1")
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	assert.Equal(t, originalStartedAt, task["started_at"], "done should preserve original started_at from claim")
}

func TestReopenClearsStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Claim, close, then reopen
	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "t1 acceptance criteria met completely")
	_, _, code := runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code)

	stdout, _, _ := runTP(t, dir, "show", "t1")
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	assert.Nil(t, task["started_at"], "reopen should clear started_at")
	assert.Nil(t, task["closed_at"], "reopen should clear closed_at")
}

func TestReopenAndReclaimGetsNewStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Claim → close → reopen → claim again
	stdout, _, _ := runTP(t, dir, "claim", "t1")
	var first map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &first))
	firstStarted := first["started_at"].(string)

	runTP(t, dir, "close", "t1", "t1 acceptance criteria met completely")
	runTP(t, dir, "reopen", "t1")

	time.Sleep(10 * time.Millisecond)

	stdout, _, _ = runTP(t, dir, "claim", "t1")
	var second map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &second))
	secondStarted := second["started_at"].(string)

	assert.NotEqual(t, firstStarted, secondStarted, "reclaim after reopen should get new started_at")
}

// --- batch NDJSON started_at tests ---

func TestBatchDoneAcceptsStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Write NDJSON with explicit started_at
	startTime := time.Now().UTC().Add(-10 * time.Minute)
	ndjson := `{"id":"t1","reason":"t1 acceptance criteria met completely","gate_passed":true,"started_at":"` + startTime.Format(time.RFC3339Nano) + `"}`
	ndjsonPath := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjsonPath, []byte(ndjson+"\n"), 0o600))

	_, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	require.Equal(t, 0, code)

	stdout, _, _ := runTP(t, dir, "show", "t1")
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))

	parsedStart, _ := time.Parse(time.RFC3339Nano, task["started_at"].(string))
	assert.WithinDuration(t, startTime, parsedStart, time.Second, "batch should use provided started_at")
}

func TestBatchDoneWithoutStartedAtUsesNow(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	before := time.Now().UTC()

	ndjson := `{"id":"t1","reason":"t1 acceptance criteria met completely","gate_passed":true}`
	ndjsonPath := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjsonPath, []byte(ndjson+"\n"), 0o600))

	_, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	require.Equal(t, 0, code)

	stdout, _, _ := runTP(t, dir, "show", "t1")
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	assert.NotNil(t, task["started_at"], "batch without started_at should still set it")

	parsedStart, _ := time.Parse(time.RFC3339Nano, task["started_at"].(string))
	assert.True(t, parsedStart.After(before.Add(-time.Second)), "fallback started_at should be recent")
}

func TestBatchDonePreservesClaimedStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Claim first to set started_at
	stdout, _, _ := runTP(t, dir, "claim", "t1")
	var claimed map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &claimed))
	originalStarted := claimed["started_at"].(string)

	// Batch done on wip task — should NOT overwrite started_at
	ndjson := `{"id":"t1","reason":"t1 acceptance criteria met completely","gate_passed":true}`
	ndjsonPath := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjsonPath, []byte(ndjson+"\n"), 0o600))

	_, _, code := runTP(t, dir, "done", "--batch", ndjsonPath)
	require.Equal(t, 0, code)

	stdout, _, _ = runTP(t, dir, "show", "t1")
	var showResult map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &showResult))
	assert.Equal(t, originalStarted, showResult["started_at"], "batch done on wip should preserve original started_at")
}

// --- tp report tests ---

func TestReportBasic(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Claim, then close
	runTP(t, dir, "claim", "t1")
	time.Sleep(10 * time.Millisecond)
	runTP(t, dir, "close", "t1", "t1 acceptance criteria met completely")

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	tasks := result["tasks"].([]any)
	assert.Len(t, tasks, 1)

	task := tasks[0].(map[string]any)
	assert.Equal(t, "t1", task["id"])
	assert.GreaterOrEqual(t, task["actual_minutes"].(float64), 0.0)
	assert.Equal(t, float64(5), task["estimate_minutes"])

	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(1), summary["total_tasks"])
	assert.Equal(t, float64(1), summary["completed"])
	assert.Equal(t, float64(1), summary["tracked"])
	assert.Equal(t, float64(0), summary["untracked"])
}

func TestReportNoCompletedTasks(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	tasks := result["tasks"].([]any)
	assert.Len(t, tasks, 0, "no completed tasks = empty tasks array")

	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(0), summary["completed"])
	assert.Equal(t, float64(0), summary["tracked"])
}

func TestReportUntrackedTasks(t *testing.T) {
	dir := setupProject(t)

	// Directly write a task file with a done task that has no started_at
	// (simulating pre-v0.6 data)
	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	now := time.Now().UTC()
	nowStr := now.Format(time.RFC3339Nano)
	taskFileContent := `{
  "version": 1,
  "spec": "spec.md",
  "tasks": [
    {
      "id": "old-task",
      "title": "Old task",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "Done",
      "source_sections": ["s1"],
      "started_at": null,
      "closed_at": "` + nowStr + `",
      "closed_reason": "completed",
      "gate_passed_at": null,
      "commit_sha": null
    }
  ],
  "workflow": {},
  "coverage": {},
  "updated_at": "` + nowStr + `"
}`
	require.NoError(t, os.WriteFile(taskFilePath, []byte(taskFileContent), 0o600))

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(1), summary["completed"])
	assert.Equal(t, float64(0), summary["tracked"])
	assert.Equal(t, float64(1), summary["untracked"])
}

func TestReportWithBatchStartedAt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Use batch done with explicit started_at 10 minutes ago
	startTime := time.Now().UTC().Add(-10 * time.Minute)
	ndjson := `{"id":"t1","reason":"t1 acceptance criteria met completely","gate_passed":true,"started_at":"` + startTime.Format(time.RFC3339Nano) + `"}`
	ndjsonPath := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjsonPath, []byte(ndjson+"\n"), 0o600))

	runTP(t, dir, "done", "--batch", ndjsonPath)

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	tasks := result["tasks"].([]any)
	require.Len(t, tasks, 1)
	task := tasks[0].(map[string]any)
	// Should be ~10 minutes
	assert.Greater(t, task["actual_minutes"].(float64), 9.0, "should reflect the 10-minute gap")
	assert.Less(t, task["actual_minutes"].(float64), 11.0)
}

func TestReportFastestSlowest(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	addTaskWithEstimate(t, dir, "t2", 8)

	now := time.Now().UTC()

	// Write task file directly with controlled timestamps
	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	t1Start := now.Add(-20 * time.Minute)
	t1Close := now.Add(-15 * time.Minute) // 5 min duration
	t2Start := now.Add(-15 * time.Minute)
	t2Close := now.Add(-3 * time.Minute) // 12 min duration
	reason := "acceptance criteria met"

	taskFileContent := `{
  "version": 1,
  "spec": "spec.md",
  "tasks": [
    {
      "id": "t1",
      "title": "Task t1",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "t1 acceptance criteria met",
      "source_sections": ["s1"],
      "started_at": "` + t1Start.Format(time.RFC3339Nano) + `",
      "closed_at": "` + t1Close.Format(time.RFC3339Nano) + `",
      "closed_reason": "` + reason + `",
      "gate_passed_at": null,
      "commit_sha": null
    },
    {
      "id": "t2",
      "title": "Task t2",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 8,
      "acceptance": "t2 acceptance criteria met",
      "source_sections": ["s1"],
      "started_at": "` + t2Start.Format(time.RFC3339Nano) + `",
      "closed_at": "` + t2Close.Format(time.RFC3339Nano) + `",
      "closed_reason": "` + reason + `",
      "gate_passed_at": null,
      "commit_sha": null
    }
  ],
  "workflow": {},
  "coverage": {},
  "updated_at": "` + now.Format(time.RFC3339Nano) + `"
}`
	require.NoError(t, os.WriteFile(taskFilePath, []byte(taskFileContent), 0o600))

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(2), summary["tracked"])

	fastest := summary["fastest_task"].(map[string]any)
	assert.Equal(t, "t1", fastest["id"])
	assert.InDelta(t, 5.0, fastest["minutes"], 0.2)

	slowest := summary["slowest_task"].(map[string]any)
	assert.Equal(t, "t2", slowest["id"])
	assert.InDelta(t, 12.0, slowest["minutes"], 0.2)

	// Estimation accuracy: total estimated (13) / total actual (~17) ≈ 0.76
	assert.Greater(t, summary["estimation_accuracy"].(float64), 0.5)
	assert.Less(t, summary["estimation_accuracy"].(float64), 1.0)
}

func TestReportMixedTrackedAndUntracked(t *testing.T) {
	dir := setupProject(t)

	now := time.Now().UTC()
	t1Start := now.Add(-10 * time.Minute)
	reason := "acceptance criteria met"

	taskFileContent := `{
  "version": 1,
  "spec": "spec.md",
  "tasks": [
    {
      "id": "tracked",
      "title": "Tracked task",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "tracked acceptance criteria met",
      "source_sections": ["s1"],
      "started_at": "` + t1Start.Format(time.RFC3339Nano) + `",
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
      "closed_reason": "` + reason + `",
      "gate_passed_at": null,
      "commit_sha": null
    },
    {
      "id": "untracked",
      "title": "Untracked task",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 8,
      "acceptance": "untracked acceptance criteria met",
      "source_sections": ["s1"],
      "started_at": null,
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
      "closed_reason": "` + reason + `",
      "gate_passed_at": null,
      "commit_sha": null
    },
    {
      "id": "open-task",
      "title": "Open task",
      "status": "open",
      "depends_on": [],
      "estimate_minutes": 3,
      "acceptance": "open task acceptance",
      "source_sections": ["s1"],
      "started_at": null,
      "closed_at": null,
      "closed_reason": null,
      "gate_passed_at": null,
      "commit_sha": null
    }
  ],
  "workflow": {},
  "coverage": {},
  "updated_at": "` + now.Format(time.RFC3339Nano) + `"
}`
	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFilePath, []byte(taskFileContent), 0o600))

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(3), summary["total_tasks"])
	assert.Equal(t, float64(2), summary["completed"])
	assert.Equal(t, float64(1), summary["tracked"])
	assert.Equal(t, float64(1), summary["untracked"])

	tasks := result["tasks"].([]any)
	assert.Len(t, tasks, 1, "only tracked tasks in output")
	assert.Equal(t, "tracked", tasks[0].(map[string]any)["id"])
}

func TestReportZeroDurationTask(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// tp done with implicit claim: started_at ≈ closed_at (zero duration)
	_, _, code := runTP(t, dir, "done", "t1", "t1 acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	tasks := result["tasks"].([]any)
	require.Len(t, tasks, 1)
	task := tasks[0].(map[string]any)
	// Duration should be ~0 (started_at ≈ closed_at)
	assert.Less(t, task["actual_minutes"].(float64), 1.0)
}

// --- started_at as managed field ---

func TestSetStartedAtIsManaged(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	_, stderr, code := runTP(t, dir, "set", "t1", "started_at=2024-01-01T00:00:00Z")
	assert.NotEqual(t, 0, code, "setting managed field should fail")
	assert.Contains(t, stderr, "managed")
}
