package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// rawTaskJSON is the task's object exactly as spec.tasks.json writes it, so a
// comparison catches a change to any field, not only the ones a test names.
func rawTaskJSON(t *testing.T, dir, id string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	var file struct {
		Tasks []json.RawMessage `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal(data, &file))
	for _, raw := range file.Tasks {
		var task struct {
			ID string `json:"id"`
		}
		require.NoError(t, json.Unmarshal(raw, &task))
		if task.ID == id {
			return string(raw)
		}
	}
	require.Failf(t, "task not in spec.tasks.json", "id %s", id)
	return ""
}

// TestDoneBatch_RefusedRowLeavesOpenTaskUntouched: every refusal a row can
// meet after the gate used to come after the implicit claim, so a refused row
// left its open task wip with started_at set. A refused row now leaves the task
// byte-identical, whatever refused it.
func TestDoneBatch_RefusedRowLeavesOpenTaskUntouched(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		setup []string
		rows  []string
	}{
		{name: "closure reason", rows: []string{
			`{"id":"t1","reason":"deferred","gate_passed":true,"commit":"aaa"}`,
			`{"id":"t2","reason":"done","gate_passed":true,"commit":"bbb"}`,
		}},
		{name: "commit parse", rows: []string{
			`{"id":"t1","reason":"t1 acceptance met","gate_passed":true,"commit":["ddd","ddd"]}`,
			`{"id":"t2","reason":"t2 acceptance met","gate_passed":true,"commit":"eee","commit_shas":["fff"]}`,
		}},
		{name: "covered_by", rows: []string{
			`{"id":"t1","reason":"t1 acceptance met","covered_by":"absent"}`,
			`{"id":"t2","reason":"t2 acceptance met","covered_by":"t1"}`,
		}},
		{name: "hc without commit", setup: []string{"set", "--workflow", "--project", "commit_strategy=hc"}, rows: []string{
			`{"id":"t1","reason":"t1 acceptance met","gate_passed":true}`,
			`{"id":"t2","reason":"t2 acceptance met","gate_passed":true}`,
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := setupBatchCommitProject(t, "t1", "t2")
			if tc.setup != nil {
				_, stderr, code := runTP(t, dir, tc.setup...)
				require.Equal(t, 0, code, "setup: %s", stderr)
			}
			before := rawTasksJSON(t, dir, "t1", "t2")
			res, stderr, code := runBatch(t, dir, tc.rows...)
			require.Equal(t, 4, code, "every row is refused; stderr: %s", stderr)
			assert.EqualValues(t, 0, res["closed"])
			assert.EqualValues(t, 2, res["failed"])
			assertOpenTasksUntouched(t, dir, before)
		})
	}
}

// rawTasksJSON snapshots rawTaskJSON for each id.
func rawTasksJSON(t *testing.T, dir string, ids ...string) map[string]string {
	t.Helper()
	out := make(map[string]string, len(ids))
	for _, id := range ids {
		out[id] = rawTaskJSON(t, dir, id)
	}
	return out
}

// assertOpenTasksUntouched: each open task in the snapshot was refused, so it
// is still open, has no started_at, and its object is byte-identical.
func assertOpenTasksUntouched(t *testing.T, dir string, before map[string]string) {
	t.Helper()
	for id, raw := range before {
		task := taskState(t, dir, id)
		assert.Equal(t, "open", task["status"], "%s: a refusal does not claim its task", id)
		assert.Nil(t, task["started_at"], "%s: a refusal sets no started_at", id)
		assert.Equal(t, raw, rawTaskJSON(t, dir, id), "%s: a refusal leaves every field as it was", id)
	}
}

// TestDoneBatch_MixedBatchClosesGoodRowAndLeavesRefusedRowUntouched: the row
// that passes still closes through the implicit claim; the refused one beside
// it is written back exactly as it was read.
func TestDoneBatch_MixedBatchClosesGoodRowAndLeavesRefusedRowUntouched(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2")
	before := rawTaskJSON(t, dir, "t2")
	res, stderr, code := runBatch(t, dir,
		`{"id":"t1","reason":"t1 acceptance met","gate_passed":true,"commit":"aaa"}`,
		`{"id":"t2","reason":"deferred","gate_passed":true,"commit":"bbb"}`,
	)
	require.Equal(t, 1, code, "a partial failure exits 1; stderr: %s", stderr)
	assert.EqualValues(t, 1, res["closed"])
	assert.EqualValues(t, 1, res["failed"])
	t1 := taskState(t, dir, "t1")
	assert.Equal(t, "done", t1["status"])
	assert.NotNil(t, t1["started_at"], "the closing row is still implicitly claimed")
	assert.Equal(t, []any{"aaa"}, t1["commit_shas"])
	t2 := taskState(t, dir, "t2")
	assert.Equal(t, "open", t2["status"], "the refused row does not claim its task")
	assert.Nil(t, t2["started_at"], "the refused row sets no started_at")
	assert.Equal(t, before, rawTaskJSON(t, dir, "t2"), "the refused row leaves every field as it was")
}

// TestDoneBatch_RefusedRowLeavesWipTaskWip: a task claimed before the batch
// keeps its status and its own started_at when its row is refused — the fix
// must not undo a claim the batch did not make.
func TestDoneBatch_RefusedRowLeavesWipTaskWip(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2")
	_, stderr, code := runTP(t, dir, "claim", "t1")
	require.Equal(t, 0, code, "claim: %s", stderr)
	claimed := taskState(t, dir, "t1")
	require.Equal(t, "wip", claimed["status"])
	require.NotNil(t, claimed["started_at"])
	before := rawTaskJSON(t, dir, "t1")
	res, stderr, code := runBatch(t, dir,
		`{"id":"t1","reason":"deferred","gate_passed":true,"commit":"aaa","started_at":"2020-01-02T03:04:05Z"}`,
	)
	require.Equal(t, 4, code, "the only row is refused; stderr: %s", stderr)
	assert.EqualValues(t, 1, res["failed"])
	t1 := taskState(t, dir, "t1")
	assert.Equal(t, "wip", t1["status"], "a refused row leaves a claimed task claimed")
	assert.Equal(t, claimed["started_at"], t1["started_at"], "a refused row keeps the claim's started_at")
	assert.Equal(t, before, rawTaskJSON(t, dir, "t1"), "a refused row leaves every field as it was")
}
