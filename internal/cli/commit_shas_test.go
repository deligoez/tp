package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func taskState(t *testing.T, dir, id string) map[string]any {
	t.Helper()
	out, _, code := runTP(t, dir, "show", id)
	require.Equal(t, 0, code)
	var task map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &task))
	return task
}

func TestCommitShas_RecordTwoWithMirror(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--commit", "bbb", "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{"aaa", "bbb"}, task["commit_shas"])
	assert.Equal(t, "aaa", task["commit_sha"], "commit_sha mirrors commit_shas[0]")
}

func TestCommitShas_RecordSingle(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, _, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	require.Equal(t, 0, code)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{"aaa"}, task["commit_shas"])
	assert.Equal(t, "aaa", task["commit_sha"])
}

func TestCommitShas_DuplicateExit1(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "ccc", "--commit", "ccc", "--", "t1 acceptance met")
	assert.Equal(t, 1, code, "a duplicate sha exits 1")
	assert.Contains(t, stderr, "duplicate commit sha")
	assert.Equal(t, "open", taskState(t, dir, "t1")["status"], "the task is not closed on a duplicate")
}

func TestCommitShas_CoveredByRecordsNone(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, _, code := runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 done","source_sections":["s1"]}`)
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	require.Equal(t, 0, code)
	_, stderr, code := runTP(t, dir, "done", "t2", "--covered-by", "t1", "--", "covered by t1")
	require.Equal(t, 0, code, "covered-by: %s", stderr)
	task := taskState(t, dir, "t2")
	assert.Nil(t, task["commit_shas"])
	assert.Nil(t, task["commit_sha"])
}

func TestCommitShas_SetRejectsManagedField(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "set", "t1", "commit_shas=x")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "managed")
}

func TestCommitShas_ReopenClears(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, _, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code)
	task := taskState(t, dir, "t1")
	assert.Equal(t, "open", task["status"])
	assert.Nil(t, task["commit_shas"], "reopen clears commit_shas")
	assert.Nil(t, task["commit_sha"])
	assert.Nil(t, task["gate_passed_at"])
}

func TestCommitShas_BuiltinCommitRecordsSingleElement(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))
	out, stderr, code := runTP(t, dir, "commit", "t1", "implemented t1")
	require.Equal(t, 0, code, "commit: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	sha := res["sha"].(string)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{sha}, task["commit_shas"], "builtin tp commit records a single-element commit_shas")
	assert.Equal(t, sha, task["commit_sha"])
}
