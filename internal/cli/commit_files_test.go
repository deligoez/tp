package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCommitFiles_DoneCommitResolvesSortedDedup(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	shaZ := commitFile(t, dir, "z.go", "add z")
	shaA := commitFile(t, dir, "a.go", "add a")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed",
		"--commit", shaZ, "--commit", shaA, "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{"a.go", "z.go"}, task["commit_files"], "deduped + byte-sorted")
	assert.Nil(t, task["commit_files_total"], "under cap: total omitted")
}

func TestCommitFiles_RenameExcludesOld(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	commitFile(t, dir, "old.go", "add old")
	git(t, dir, "mv", "old.go", "new.go")
	git(t, dir, "commit", "-m", "rename old to new")
	sha := gitOut(t, dir, "rev-parse", "HEAD")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed",
		"--commit", sha, "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{"new.go"}, task["commit_files"], "renamed-new kept, renamed-old excluded")
}

func TestCommitFiles_CapFiftyWithTotal(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	const n = 60
	for i := 0; i < n; i++ {
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, fmt.Sprintf("f%02d.go", i)), []byte("package main\n"), 0o600))
	}
	git(t, dir, "add", "-A")
	git(t, dir, "commit", "-m", "many files")
	sha := gitOut(t, dir, "rev-parse", "HEAD")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed",
		"--commit", sha, "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	task := taskState(t, dir, "t1")
	files := toStringSlice(task["commit_files"])
	assert.Len(t, files, 50, "capped at 50")
	total, _ := task["commit_files_total"].(float64)
	assert.Equal(t, float64(n), total, "total records the full count")
	expected := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		expected = append(expected, fmt.Sprintf("f%02d.go", i))
	}
	assert.Equal(t, expected, files, "first 50 in sorted order")
}

func TestCommitFiles_CoveredByRecordsNone(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, _, code := runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 done","source_sections":["s1"]}`)
	require.Equal(t, 0, code)
	sha := commitFile(t, dir, "a.go", "add a")
	_, _, code = runTP(t, dir, "done", "t1", "--gate-passed", "--commit", sha, "--", "t1 acceptance met")
	require.Equal(t, 0, code)
	_, stderr, code := runTP(t, dir, "done", "t2", "--covered-by", "t1", "--", "covered by t1")
	require.Equal(t, 0, code, "covered-by: %s", stderr)
	task := taskState(t, dir, "t2")
	assert.Nil(t, task["commit_files"], "covered-by records none")
	assert.Nil(t, task["commit_files_total"])
}

func TestCommitFiles_SetRejectsManagedField(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "set", "t1", "commit_files=x")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "managed")
	_, stderr, code = runTP(t, dir, "set", "t1", "commit_files_total=3")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "managed")
}

func TestCommitFiles_ReopenClears(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	sha := commitFile(t, dir, "a.go", "add a")
	_, _, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", sha, "--", "t1 acceptance met")
	require.Equal(t, 0, code)
	require.NotNil(t, taskState(t, dir, "t1")["commit_files"], "precondition: field set at close")
	_, _, code = runTP(t, dir, "reopen", "t1")
	require.Equal(t, 0, code)
	task := taskState(t, dir, "t1")
	assert.Nil(t, task["commit_files"], "reopen clears commit_files")
	assert.Nil(t, task["commit_files_total"], "reopen clears commit_files_total")
}

func TestCommitFiles_UnresolvableShaOmitted(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed",
		"--commit", "deadbeef", "--", "t1 acceptance met")
	require.Equal(t, 0, code, "done: %s", stderr)
	task := taskState(t, dir, "t1")
	assert.NotNil(t, task["commit_shas"], "commit_shas still recorded")
	assert.Nil(t, task["commit_files"], "unresolvable sha: field omitted, not guessed")
	assert.Nil(t, task["commit_files_total"])
}

func TestCommitFiles_BuiltinCommitResolves(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package main\n"), 0o600))
	out, stderr, code := runTP(t, dir, "commit", "t1", "implemented t1")
	require.Equal(t, 0, code, "commit: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	sha := res["sha"].(string)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{sha}, task["commit_shas"])
	assert.Equal(t, []any{"impl.go", "spec.tasks.json"}, task["commit_files"],
		"C1 touches impl.go (add) and the task file (modified to wip)")
}

func TestCommitFiles_BatchResolvesPerRow(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	sha := commitFile(t, dir, "a.go", "add a")
	batch := filepath.Join(dir, "batch.ndjson")
	require.NoError(t, os.WriteFile(batch,
		[]byte(fmt.Sprintf(`{"id":"t1","reason":"t1 acceptance met","commit_shas":[%q]}`, sha)), 0o600))
	_, stderr, code := runTP(t, dir, "done", "--batch", batch)
	require.Equal(t, 0, code, "batch: %s", stderr)
	task := taskState(t, dir, "t1")
	assert.Equal(t, []any{"a.go"}, task["commit_files"], "batch resolves per-row commit_shas")
}
