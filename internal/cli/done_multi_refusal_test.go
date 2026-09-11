package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func runDoneMultiJSON(t *testing.T, dir string, args ...string) (res map[string]any, stderr string, code int) {
	t.Helper()
	out, stderr, code := runTP(t, dir, append([]string{"done"}, args...)...)
	require.NoError(t, json.Unmarshal([]byte(out), &res), "stdout: %s; stderr: %s", out, stderr)
	return res, stderr, code
}

// TestDoneMulti_RefusedTargetLeavesOpenTaskUntouched: `tp done t1 t2 <reason>`
// made the implicit claim before the --covered-by check and closure
// verification, and wrote the file after skipping a refused target, so a
// refused open task was left wip with started_at set. A refused target now
// leaves the task byte-identical. (Commit parsing and the hc rule refuse the
// whole invocation before the lock, so they never reach this loop.)
func TestDoneMulti_RefusedTargetLeavesOpenTaskUntouched(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		args []string
	}{
		{name: "closure reason", args: []string{"--gate-passed", "--commit", "aaa", "--", "deferred"}},
		{name: "covered-by absent", args: []string{"--gate-passed", "--covered-by", "absent", "--", "t1 acceptance met"}},
		{name: "covered-by not done", args: []string{"--gate-passed", "--covered-by", "t3", "--", "t1 acceptance met"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := setupBatchCommitProject(t, "t1", "t2", "t3")
			before := rawTasksJSON(t, dir, "t1", "t2")
			res, stderr, code := runDoneMultiJSON(t, dir, append([]string{"t1", "t2"}, tc.args...)...)
			require.Equal(t, 1, code, "every target is refused; stderr: %s", stderr)
			assert.Empty(t, res["closed"])
			assert.Len(t, res["failed"], 2)
			assertOpenTasksUntouched(t, dir, before)
		})
	}
}

// TestDoneMulti_MixedClosesPassingTargetAndLeavesRefusedTargetUntouched: one
// reason, two targets — t1's single criterion passes, t2's two criteria need
// two evidence lines. t1 still closes through the implicit claim; t2 is written
// back exactly as it was read.
func TestDoneMulti_MixedClosesPassingTargetAndLeavesRefusedTargetUntouched(t *testing.T) {
	t.Parallel()
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "add", `{"id":"t2","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 part one works; t2 part two works","source_sections":["s1"]}`)
	require.Equal(t, 0, code, "add: %s", stderr)
	before := rawTasksJSON(t, dir, "t2")
	res, stderr, code := runDoneMultiJSON(t, dir, "t1", "t2", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	require.Equal(t, 0, code, "a target closed; stderr: %s", stderr)
	assert.Equal(t, []any{"t1"}, res["closed"])
	assert.Len(t, res["failed"], 1)
	t1 := taskState(t, dir, "t1")
	assert.Equal(t, "done", t1["status"])
	assert.NotNil(t, t1["started_at"], "the closing target is still implicitly claimed")
	assert.Equal(t, []any{"aaa"}, t1["commit_shas"])
	assertOpenTasksUntouched(t, dir, before)
}

// TestDoneMulti_RefusedTargetLeavesWipTaskWip: a task claimed before the
// invocation keeps its status and its own started_at when it is refused — the
// fix must not undo a claim this invocation did not make.
func TestDoneMulti_RefusedTargetLeavesWipTaskWip(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2")
	_, stderr, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim: %s", stderr)
	claimed := taskState(t, dir, "t1")
	require.Equal(t, "wip", claimed["status"])
	require.NotNil(t, claimed["started_at"])
	before := rawTaskJSON(t, dir, "t1")
	_, stderr, code = runDoneMultiJSON(t, dir, "t1", "t2", "--gate-passed", "--commit", "aaa", "--", "deferred")
	require.Equal(t, 1, code, "every target is refused; stderr: %s", stderr)
	t1 := taskState(t, dir, "t1")
	assert.Equal(t, "wip", t1["status"], "a refusal leaves a claimed task claimed")
	assert.Equal(t, claimed["started_at"], t1["started_at"], "a refusal keeps the claim's started_at")
	assert.Equal(t, before, rawTaskJSON(t, dir, "t1"), "a refusal leaves every field as it was")
}
