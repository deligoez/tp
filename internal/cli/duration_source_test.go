package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- explicit claim paths report "claimed" (§11.2) ---

func TestDurationSource_ClaimSingle(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	stdout, _, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code)

	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	assert.Equal(t, "claimed", task["duration_source"])
}

func TestDurationSource_ClaimBatch(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	addTaskWithEstimate(t, dir, "t2", 5)

	stdout, _, code := runTP(t, dir, "claim", "t1", "t2")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Contains(t, result["claimed"], "t1")
	assert.Contains(t, result["claimed"], "t2")

	for _, id := range []string{"t1", "t2"} {
		assert.Equal(t, "claimed", taskState(t, dir, id)["duration_source"])
	}
}

func TestDurationSource_Next(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	stdout, _, code := runTP(t, dir, "next")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	task := result["task"].(map[string]any)
	assert.Equal(t, "claimed", task["duration_source"])
}

// --- implicit claim paths report "implicit" (§11.2) ---

func TestDurationSource_DoneImplicit(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	_, _, code := runTP(t, dir, "done", "t1", "t1 acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)

	assert.Equal(t, "implicit", taskState(t, dir, "t1")["duration_source"])
}

func TestDurationSource_CommitImplicit(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package main\n"), 0o600))

	_, _, code := runTP(t, dir, "commit", "t1", "implemented t1")
	require.Equal(t, 0, code)

	assert.Equal(t, "implicit", taskState(t, dir, "t1")["duration_source"],
		"tp commit on an open task implicitly claims it → implicit")
}

// --- covered-by is exempt: no duration_source classified as implicit (§11.2) ---

func TestDurationSource_CoveredByExempt(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)
	addTaskWithEstimate(t, dir, "t2", 5)

	_, _, code := runTP(t, dir, "done", "t1", "t1 acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)

	_, _, code = runTP(t, dir, "done", "t2", "--covered-by", "t1", "--", "covered by t1")
	require.Equal(t, 0, code)

	task := taskState(t, dir, "t2")
	_, hasField := task["duration_source"]
	assert.False(t, hasField, "covered-by close is exempt — duration_source is not set")
}

// --- managed field: tp set rejects, tp reopen clears (§11.2) ---

func TestDurationSource_SetManaged(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	_, stderr, code := runTP(t, dir, "set", "t1", "duration_source=claimed")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "managed")
}

func TestDurationSource_ReopenClears(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	_, _, code := runTP(t, dir, "done", "t1", "t1 acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code)
	require.Equal(t, "implicit", taskState(t, dir, "t1")["duration_source"])

	_, _, code = runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code)

	task := taskState(t, dir, "t1")
	_, hasField := task["duration_source"]
	assert.False(t, hasField, "reopen clears duration_source")
}

// --- tp report: implicit_duration count, disjoint from excluded_from_accuracy (§11.2) ---

func TestDurationSource_ReportImplicitDisjointFromExcluded(t *testing.T) {
	dir := setupProject(t)

	now := time.Now().UTC()
	flashStart := now.Add(-1 * time.Millisecond)   // near-zero duration
	normalStart := now.Add(-10 * time.Minute)      // real duration
	coveredStart := now.Add(-1 * time.Millisecond) // near-zero, no duration_source
	reason := "acceptance criteria met"

	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	taskFileContent := `{
  "version": 1,
  "spec": "spec.md",
  "tasks": [
    {
      "id": "implicit-task",
      "title": "Implicit",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "done",
      "source_sections": ["s1"],
      "started_at": "` + flashStart.Format(time.RFC3339Nano) + `",
      "duration_source": "implicit",
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
      "closed_reason": "` + reason + `",
      "gate_passed_at": null,
      "commit_sha": null
    },
    {
      "id": "claimed-task",
      "title": "Claimed",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "done",
      "source_sections": ["s1"],
      "started_at": "` + normalStart.Format(time.RFC3339Nano) + `",
      "duration_source": "claimed",
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
      "closed_reason": "` + reason + `",
      "gate_passed_at": null,
      "commit_sha": null
    },
    {
      "id": "covered-task",
      "title": "Covered",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "done",
      "source_sections": ["s1"],
      "started_at": "` + coveredStart.Format(time.RFC3339Nano) + `",
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
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

	tasks := result["tasks"].([]any)
	require.Len(t, tasks, 3)

	var implicitTask, claimedTask, coveredTask map[string]any
	for _, tk := range tasks {
		tk := tk.(map[string]any)
		switch tk["id"] {
		case "implicit-task":
			implicitTask = tk
		case "claimed-task":
			claimedTask = tk
		case "covered-task":
			coveredTask = tk
		}
	}
	require.NotNil(t, implicitTask)
	require.NotNil(t, claimedTask)
	require.NotNil(t, coveredTask)

	// implicit task: counted in implicit_duration, NOT in excluded_from_accuracy.
	assert.Equal(t, "implicit", implicitTask["duration_source"])
	assert.Nil(t, implicitTask["accuracy"], "implicit task excluded from accuracy")
	assert.Equal(t, "implicit claim (duration not measured)", implicitTask["note"])

	// claimed task: has a real accuracy.
	assert.Equal(t, "claimed", claimedTask["duration_source"])
	assert.NotNil(t, claimedTask["accuracy"])

	// covered task: no duration_source, near-zero → excluded_from_accuracy.
	_, hasDS := coveredTask["duration_source"]
	assert.False(t, hasDS, "covered task has no duration_source")
	assert.Nil(t, coveredTask["accuracy"])
	assert.Equal(t, "duration below resolution", coveredTask["note"])

	// Both counts reported, disjoint.
	summary := result["summary"].(map[string]any)
	assert.Equal(t, float64(1), summary["implicit_duration"],
		"implicit task counted once in implicit_duration")
	assert.Equal(t, float64(1), summary["excluded_from_accuracy"],
		"covered task counted once in excluded_from_accuracy")
}

// TestDurationSource_ReportImplicitPrecedenceOverExcluded covers §11.2: an
// implicit-duration task whose near-zero duration WOULD round to 0.0 is counted
// only in implicit_duration — never double-counted in excluded_from_accuracy.
func TestDurationSource_ReportImplicitPrecedenceOverExcluded(t *testing.T) {
	dir := setupProject(t)

	now := time.Now().UTC()
	flashStart := now.Add(-1 * time.Millisecond) // would round to 0.0
	reason := "done"

	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	taskFileContent := `{
  "version": 1,
  "spec": "spec.md",
  "tasks": [
    {
      "id": "implicit-flash",
      "title": "Implicit flash",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "done",
      "source_sections": ["s1"],
      "started_at": "` + flashStart.Format(time.RFC3339Nano) + `",
      "duration_source": "implicit",
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
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
	assert.Equal(t, float64(1), summary["implicit_duration"],
		"implicit task counted in implicit_duration")
	assert.Equal(t, float64(0), summary["excluded_from_accuracy"],
		"implicit task never also in excluded_from_accuracy (precedence)")
	assert.Nil(t, summary["estimation_accuracy"],
		"no usable accuracy when the only task is implicit")
}

// TestDurationSource_ReportCarriedPerTask verifies duration_source appears per
// task in report output (§11.2: "tp report carries it per task").
func TestDurationSource_ReportCarriedPerTask(t *testing.T) {
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "t1", 5)

	// Claim explicitly, then close → duration_source = "claimed"
	runTP(t, dir, "claim", "t1")
	time.Sleep(10 * time.Millisecond)
	runTP(t, dir, "close", "t1", "t1 acceptance criteria met completely")

	stdout, _, code := runTP(t, dir, "report")
	require.Equal(t, 0, code)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	tasks := result["tasks"].([]any)
	require.Len(t, tasks, 1)
	task := tasks[0].(map[string]any)
	assert.Equal(t, "claimed", task["duration_source"],
		"report carries duration_source per task")
}
