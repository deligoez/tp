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

// setupBatchCommitProject is setupCommitProject with extra open tasks, so a
// batch can carry one row that must close beside the row under test.
func setupBatchCommitProject(t *testing.T, ids ...string) string {
	t.Helper()
	dir := setupCommitProject(t, ids[0])
	for _, id := range ids[1:] {
		_, _, code := runTP(t, dir, "add", `{"id":"`+id+`","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"`+id+` acceptance met","source_sections":["s1"]}`)
		require.Equal(t, 0, code)
	}
	return dir
}

func runBatch(t *testing.T, dir string, rows ...string) (res map[string]any, stderr string, code int) {
	t.Helper()
	path := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(rows, "\n")+"\n"), 0o600))
	out, stderr, code := runTP(t, dir, "done", "--batch", "batch.ndjson")
	if out != "" {
		require.NoError(t, json.Unmarshal([]byte(out), &res), "stdout: %s", out)
	}
	return res, stderr, code
}

func batchFailureFor(t *testing.T, res map[string]any, id string) map[string]any {
	t.Helper()
	failures, _ := res["failures"].([]any)
	for _, f := range failures {
		if m := f.(map[string]any); m["id"] == id {
			return m
		}
	}
	require.Failf(t, "no failure row", "id %s in %v", id, res)
	return nil
}

// TestDoneBatch_CommitAcceptsStringOrArray: a batch row's commit takes the same
// shapes --commit does — one sha, or several — and each becomes a commit_shas
// entry. An array used to fail the whole file as unparseable NDJSON.
func TestDoneBatch_CommitAcceptsStringOrArray(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2")
	res, stderr, code := runBatch(t, dir,
		`{"id":"t1","reason":"t1 acceptance met","gate_passed":true,"commit":["aaa","bbb"]}`,
		`{"id":"t2","reason":"t2 acceptance met","gate_passed":true,"commit":"ccc"}`,
	)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.EqualValues(t, 2, res["closed"])
	t1 := taskState(t, dir, "t1")
	assert.Equal(t, []any{"aaa", "bbb"}, t1["commit_shas"], "each array element is a commit_shas entry, in order")
	assert.Equal(t, "aaa", t1["commit_sha"], "commit_sha mirrors commit_shas[0]")
	assert.Equal(t, []any{"ccc"}, taskState(t, dir, "t2")["commit_shas"])
}

// TestDoneBatch_CommitArrayIsValidatedLikeRepeatedCommit: the array goes
// through the --commit validation, row by row, under partial-failure semantics.
func TestDoneBatch_CommitArrayIsValidatedLikeRepeatedCommit(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2", "t3")
	res, stderr, code := runBatch(t, dir,
		`{"id":"t1","reason":"t1 acceptance met","gate_passed":true,"commit":["ddd","ddd"]}`,
		`{"id":"t2","reason":"t2 acceptance met","gate_passed":true,"commit":["eee","--output=x"]}`,
		`{"id":"t3","reason":"t3 acceptance met","gate_passed":true,"commit":["fff"]}`,
	)
	require.Equal(t, 1, code, "a partial failure exits 1; stderr: %s", stderr)
	assert.EqualValues(t, 1, res["closed"])
	assert.EqualValues(t, 2, res["failed"])
	assert.Contains(t, batchFailureFor(t, res, "t1")["error"], "duplicate commit sha")
	assert.Contains(t, batchFailureFor(t, res, "t2")["error"], "must not start with")
	assert.NotEqual(t, "done", taskState(t, dir, "t1")["status"])
	assert.NotEqual(t, "done", taskState(t, dir, "t2")["status"])
	assert.Equal(t, "done", taskState(t, dir, "t3")["status"])
}

// TestDoneBatch_CommitAndCommitShasTogetherIsAFailedRow: a row carrying both
// keys used to close with commit silently dropped. It is now refused, naming
// both keys, and the other rows still close.
func TestDoneBatch_CommitAndCommitShasTogetherIsAFailedRow(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2")
	res, stderr, code := runBatch(t, dir,
		`{"id":"t1","reason":"t1 acceptance met","gate_passed":true,"commit":"aaa","commit_shas":["bbb"]}`,
		`{"id":"t2","reason":"t2 acceptance met","gate_passed":true,"commit_shas":["ccc"]}`,
	)
	require.Equal(t, 1, code, "a partial failure exits 1; stderr: %s", stderr)
	assert.EqualValues(t, 1, res["closed"])
	msg, _ := batchFailureFor(t, res, "t1")["error"].(string)
	assert.Contains(t, msg, `"commit"`)
	assert.Contains(t, msg, `"commit_shas"`)
	assert.NotEqual(t, "done", taskState(t, dir, "t1")["status"])
	assert.Nil(t, taskState(t, dir, "t1")["commit_shas"], "nothing from the refused row is recorded")
	assert.Equal(t, "done", taskState(t, dir, "t2")["status"])
}

// TestDoneBatch_UnparseableRowNamesItsLineAndTheRowSchema: a row that does not
// parse is reported by its physical line number with a hint naming the row
// schema — never the task-file hint (`tp use`) the file-error default carries.
func TestDoneBatch_UnparseableRowNamesItsLineAndTheRowSchema(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1", "t2")
	before, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	_, stderr, code := runBatch(t, dir,
		`{"id":"t1","reason":"t1 acceptance met","gate_passed":true,"commit":"aaa"}`,
		``,
		`{"id":"t2","reason":"t2 acceptance met","commit":[1]}`,
	)
	require.NotEqual(t, 0, code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &envelope), "stderr: %s", stderr)
	assert.Contains(t, envelope["error"], "line 3", "the blank line 2 still counts")
	hint, _ := envelope["hint"].(string)
	assert.NotContains(t, hint, "tp use")
	for _, key := range []string{"id", "reason", "commit", "commit_shas", "covered_by"} {
		assert.Contains(t, hint, `"`+key+`"`, "the hint names the batch row schema")
	}
	after, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "an unparseable file closes nothing")
}

// TestDoneBatch_MissingFileHintNamesTheBatchPath: the batch file is not the
// task file, so its open error does not point the agent at `tp use` either.
func TestDoneBatch_MissingFileHintNamesTheBatchPath(t *testing.T) {
	t.Parallel()
	dir := setupBatchCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "done", "--batch", "absent.ndjson")
	require.Equal(t, 3, code)
	var envelope map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(stderr)), &envelope), "stderr: %s", stderr)
	assert.Contains(t, envelope["hint"], "--batch")
	assert.NotContains(t, envelope["hint"], "tp use")
}
