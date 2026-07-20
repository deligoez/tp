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

func countGateRuns(t *testing.T, dir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "gate_runs.txt"))
	if os.IsNotExist(err) {
		return 0
	}
	require.NoError(t, err)
	return strings.Count(string(data), "run")
}

func TestDoneGate_RunsOncePerInvocation(t *testing.T) {
	t.Run("batch of 3 runs gate once", func(t *testing.T) {
		dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
		for _, id := range []string{"a", "b", "c"} {
			addTask(t, dir, `{"id":"`+id+`","title":"T","depends_on":[],"estimate_minutes":5,"acceptance":"`+id+` complete","source_sections":["s1"]}`)
		}
		ndjson := filepath.Join(dir, "results.ndjson")
		lines := []string{
			`{"id":"a","reason":"a complete and verified"}`,
			`{"id":"b","reason":"b complete and verified"}`,
			`{"id":"c","reason":"c complete and verified"}`,
		}
		require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

		stdout, stderr, code := runTP(t, dir, "done", "--batch", ndjson)
		require.Equal(t, 0, code, "stderr=%s stdout=%s", stderr, stdout)
		assert.Equal(t, 1, countGateRuns(t, dir), "closing 3 tasks costs one gate run")

		var out map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
		assert.Equal(t, float64(3), out["closed"])
	})

	t.Run("non-batch failure closes nothing", func(t *testing.T) {
		dir := setupProjectWithGate(t, "exit 9")
		addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
		addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

		_, _, code := runTP(t, dir, "done", "a", "b", "both complete and verified")
		assert.Equal(t, 4, code)
		for _, id := range []string{"a", "b"} {
			assert.Equal(t, "open", showTask(t, dir, id)["status"])
		}
	})

	t.Run("batch failure closes only skip entries", func(t *testing.T) {
		dir := setupProjectWithGate(t, "exit 9")
		addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
		addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

		ndjson := filepath.Join(dir, "results.ndjson")
		lines := []string{
			`{"id":"a","reason":"a complete and verified","skip_gate":"env broken"}`,
			`{"id":"b","reason":"b complete and verified"}`,
		}
		require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

		_, _, code := runTP(t, dir, "done", "--batch", ndjson)
		assert.Equal(t, 1, code)
		assert.Equal(t, "done", showTask(t, dir, "a")["status"])
		assert.Equal(t, "open", showTask(t, dir, "b")["status"])
	})
}

func TestDoneGate_AllSkipBatchNoRun(t *testing.T) {
	dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
	addTask(t, dir, `{"id":"a","title":"A","depends_on":[],"estimate_minutes":5,"acceptance":"A complete","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"b","title":"B","depends_on":[],"estimate_minutes":5,"acceptance":"B complete","source_sections":["s1"]}`)

	ndjson := filepath.Join(dir, "results.ndjson")
	lines := []string{
		`{"id":"a","reason":"a complete and verified","skip_gate":"reason a"}`,
		`{"id":"b","reason":"b complete and verified","skip_gate":"reason b"}`,
	}
	require.NoError(t, os.WriteFile(ndjson, []byte(strings.Join(lines, "\n")+"\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "done", "--batch", ndjson)
	require.Equal(t, 0, code, "stderr=%s stdout=%s", stderr, stdout)
	assert.Equal(t, 0, countGateRuns(t, dir), "all-skip batch never executes the gate")

	for _, id := range []string{"a", "b"} {
		task := showTask(t, dir, id)
		assert.Equal(t, "done", task["status"])
		assert.NotNil(t, task["gate_skipped_reason"])
		assert.Nil(t, task["gate_passed_at"])
	}
}

func TestDoneGate_SkipGateRecorded(t *testing.T) {
	dir := setupProjectWithGate(t, "echo ok")
	addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

	_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully", "--skip-gate", "gate host down")
	require.Equal(t, 0, code, "stderr: %s", stderr)

	task := showTask(t, dir, "t1")
	assert.Equal(t, "gate host down", task["gate_skipped_reason"])
	assert.Nil(t, task["gate_passed_at"])

	// tp set rejects the managed field
	_, stderr, code = runTP(t, dir, "set", "t1", "gate_skipped_reason=whatever")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "gate_skipped_reason")

	// reopen clears it
	_, _, code = runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code)
	task = showTask(t, dir, "t1")
	assert.Equal(t, "open", task["status"])
	assert.Nil(t, task["gate_skipped_reason"], "reopen clears gate_skipped_reason")
}

func TestDoneGate_GatePassedCompat(t *testing.T) {
	t.Run("flag ignored when gate configured", func(t *testing.T) {
		dir := setupProjectWithGate(t, "echo run >> gate_runs.txt")
		addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

		_, stderr, code := runTP(t, dir, "done", "t1", "task complete and verified fully", "--gate-passed")
		require.Equal(t, 0, code, "stderr: %s", stderr)
		assert.Equal(t, 1, countGateRuns(t, dir), "gate executes despite --gate-passed")
		assert.NotNil(t, showTask(t, dir, "t1")["gate_passed_at"])
	})

	t.Run("attestation preserved without gate", func(t *testing.T) {
		dir := setupProject(t)
		addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

		_, _, code := runTP(t, dir, "done", "t1", "task complete and verified fully", "--gate-passed")
		require.Equal(t, 0, code)
		assert.NotNil(t, showTask(t, dir, "t1")["gate_passed_at"])
	})

	t.Run("combined with --skip-gate is usage error", func(t *testing.T) {
		dir := setupProjectWithGate(t, "echo ok")
		addTask(t, dir, `{"id":"t1","title":"Task","depends_on":[],"estimate_minutes":5,"acceptance":"Task complete","source_sections":["s1"]}`)

		_, _, code := runTP(t, dir, "done", "t1", "task complete and verified fully", "--gate-passed", "--skip-gate", "why")
		assert.Equal(t, 2, code)
	})
}
