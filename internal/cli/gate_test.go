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

// setupProjectWithGate inits a temp project whose workflow has a quality gate.
func setupProjectWithGate(t *testing.T, gate string) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Test Spec\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md", "--quality-gate", gate)
	require.Equal(t, 0, code, "init failed: %s", stderr)
	return dir
}

func showTask(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "show", id)
	require.Equal(t, 0, code, "show failed: %s", stderr)
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &task))
	return task
}

func TestGate_DoneRunsGateAndStampsGatePassedAt(t *testing.T) {
	dir := setupProjectWithGate(t, "echo ok")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	stdout, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully")
	require.Equal(t, 0, code, "done failed: stderr=%s stdout=%s", stderr, stdout)

	task := showTask(t, dir, "t1")
	assert.Equal(t, "done", task["status"])
	assert.NotNil(t, task["gate_passed_at"], "gate success must stamp gate_passed_at")
}

func TestGate_FailureClosesNothingWithExit4(t *testing.T) {
	dir := setupProjectWithGate(t, "echo boom; exit 7")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully")
	assert.Equal(t, 4, code, "gate failure must exit 4")

	var errOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &errOut), "stderr: %s", stderr)
	assert.Equal(t, "echo boom; exit 7", errOut["gate_cmd"])
	assert.Equal(t, float64(7), errOut["exit_code"])
	assert.Contains(t, errOut["output_tail"], "boom")
	assert.Contains(t, errOut["hint"], "--skip-gate")

	task := showTask(t, dir, "t1")
	assert.Equal(t, "open", task["status"], "no task closes on gate failure")
	assert.Nil(t, task["gate_passed_at"])
}

func TestGate_MultiIDRunsGateOnce(t *testing.T) {
	dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	stdout, stderr, code := runTP(t, dir, "done", "a", "b", "both tasks complete and verified")
	require.Equal(t, 0, code, "multi done failed: stderr=%s stdout=%s", stderr, stdout)

	data, err := os.ReadFile(filepath.Join(dir, "gate_runs.txt"))
	require.NoError(t, err, "gate must have run in the task file's directory")
	assert.Equal(t, 1, strings.Count(string(data), "run"), "gate runs exactly once per invocation")

	for _, id := range []string{"a", "b"} {
		task := showTask(t, dir, id)
		assert.Equal(t, "done", task["status"])
		assert.NotNil(t, task["gate_passed_at"])
	}
}

func TestGate_BatchRunsGateOnceBeforeEntries(t *testing.T) {
	dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":["a"],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	lines := []string{
		`{"id":"a","reason":"A complete and verified"}`,
		`{"id":"b","reason":"B complete and verified"}`,
	}
	require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "done", "--batch", ndjson)
	require.Equal(t, 0, code, "batch failed: stderr=%s stdout=%s", stderr, stdout)

	data, err := os.ReadFile(filepath.Join(dir, "gate_runs.txt"))
	require.NoError(t, err)
	assert.Equal(t, 1, strings.Count(string(data), "run"), "batch runs the gate once before entries")

	var batchOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &batchOut))
	assert.Equal(t, float64(2), batchOut["closed"])
}

func TestGate_BatchFailureFailsEveryEntry(t *testing.T) {
	dir := setupProjectWithGate(t, "exit 3")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	lines := []string{
		`{"id":"a","reason":"A complete and verified"}`,
		`{"id":"b","reason":"B complete and verified"}`,
	}
	require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	stdout, _, code := runTP(t, dir, "done", "--batch", ndjson)
	assert.Equal(t, 4, code)

	var batchOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &batchOut))
	assert.Equal(t, float64(0), batchOut["closed"])
	assert.Equal(t, float64(2), batchOut["failed"])

	for _, id := range []string{"a", "b"} {
		task := showTask(t, dir, id)
		assert.Equal(t, "open", task["status"])
	}
}

func TestGate_CloseRunsGateAndStamps(t *testing.T) {
	dir := setupProjectWithGate(t, "echo ok")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)
	_, _, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code)

	_, stderr, code := runTP(t, dir, "close", "t1", "task complete and fully verified")
	require.Equal(t, 0, code, "close failed: %s", stderr)

	task := showTask(t, dir, "t1")
	assert.Equal(t, "done", task["status"])
	assert.NotNil(t, task["gate_passed_at"])
}

func TestSkipGate_RecordsReasonAndSkipsExecution(t *testing.T) {
	dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	stdout, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully", "--skip-gate", "CI environment unavailable")
	require.Equal(t, 0, code, "done --skip-gate failed: stderr=%s stdout=%s", stderr, stdout)

	_, err := os.Stat(filepath.Join(dir, "gate_runs.txt"))
	assert.True(t, os.IsNotExist(err), "gate must not execute with --skip-gate")

	task := showTask(t, dir, "t1")
	assert.Equal(t, "done", task["status"])
	assert.Equal(t, "CI environment unavailable", task["gate_skipped_reason"])
	assert.Nil(t, task["gate_passed_at"], "gate_passed_at stays null on skip")
}

func TestSkipGate_UsageErrors(t *testing.T) {
	dir := setupProjectWithGate(t, "echo ok")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, _, code := runTP(t, dir, "done", "t1", "reason text here", "--skip-gate", "  ")
	assert.Equal(t, 2, code, "empty --skip-gate reason is a usage error")

	_, _, code = runTP(t, dir, "done", "t1", "reason text here", "--skip-gate", "why", "--gate-passed")
	assert.Equal(t, 2, code, "--skip-gate with --gate-passed is a usage error")

	_, _, code = runTP(t, dir, "done", "--batch", "whatever.ndjson", "--skip-gate", "why")
	assert.Equal(t, 2, code, "--skip-gate with --batch is a usage error")
}

func TestSkipGate_RecordedEvenWithoutGate(t *testing.T) {
	dir := setupProject(t) // no quality gate configured
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully", "--skip-gate", "no gate but honest record")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	task := showTask(t, dir, "t1")
	assert.Equal(t, "no gate but honest record", task["gate_skipped_reason"])
}

func TestSkipGate_BatchEntriesCloseDespiteGateFailure(t *testing.T) {
	dir := setupProjectWithGate(t, "exit 3")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	lines := []string{
		`{"id":"a","reason":"A complete and verified","skip_gate":"flaky environment"}`,
		`{"id":"b","reason":"B complete and verified"}`,
	}
	require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	stdout, _, code := runTP(t, dir, "done", "--batch", ndjson)
	assert.Equal(t, 1, code, "partial failure exit code")

	var batchOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &batchOut))
	assert.Equal(t, float64(1), batchOut["closed"])
	assert.Equal(t, float64(1), batchOut["failed"])

	taskA := showTask(t, dir, "a")
	assert.Equal(t, "done", taskA["status"])
	assert.Equal(t, "flaky environment", taskA["gate_skipped_reason"])
	taskB := showTask(t, dir, "b")
	assert.Equal(t, "open", taskB["status"])
}

func TestSkipGate_BatchAllSkipNeverRunsGate(t *testing.T) {
	dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	lines := []string{
		`{"id":"a","reason":"A complete and verified","skip_gate":"reason a"}`,
		`{"id":"b","reason":"B complete and verified","skip_gate":"reason b"}`,
	}
	require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "done", "--batch", ndjson)
	require.Equal(t, 0, code, "stderr=%s stdout=%s", stderr, stdout)

	_, err := os.Stat(filepath.Join(dir, "gate_runs.txt"))
	assert.True(t, os.IsNotExist(err), "all-skip batch never executes the gate")
}

func TestSkipGate_BatchEmptySkipGateFailsEntry(t *testing.T) {
	dir := setupProjectWithGate(t, "echo ok")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(ndjson, []byte(`{"id":"a","reason":"A complete and verified","skip_gate":"  "}`+"\n"), 0o600))

	stdout, _, code := runTP(t, dir, "done", "--batch", ndjson)
	assert.Equal(t, 4, code, "all entries failed")

	var batchOut map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &batchOut))
	assert.Equal(t, float64(0), batchOut["closed"])
	assert.Equal(t, float64(1), batchOut["failed"])

	task := showTask(t, dir, "a")
	assert.Equal(t, "open", task["status"])
}

func TestSkipGate_CloseRecordsReason(t *testing.T) {
	dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)
	_, _, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code)

	_, stderr, code := runTP(t, dir, "close", "t1", "task complete and fully verified", "--skip-gate", "gate broken today")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	_, err := os.Stat(filepath.Join(dir, "gate_runs.txt"))
	assert.True(t, os.IsNotExist(err))

	task := showTask(t, dir, "t1")
	assert.Equal(t, "gate broken today", task["gate_skipped_reason"])
	assert.Nil(t, task["gate_passed_at"])
}
