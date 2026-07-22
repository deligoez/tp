package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupCloseStrategyProject creates a committed tp project with task t1 and the
// given commit_strategy (empty for the default), in a git repo.
func setupCloseStrategyProject(t *testing.T, strategy string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\nContent.\n"), 0o600))
	args := []string{"init", "spec.md"}
	if strategy != "" {
		args = append(args, "--commit-strategy", strategy)
	}
	_, _, code := runTP(t, dir, args...)
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "add", `{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t1 done","source_sections":["s1"]}`)
	require.Equal(t, 0, code)
	initGitRepo(t, dir)
	return dir
}

func TestCloseStrategy_BuiltinCommitWorks(t *testing.T) {
	dir := setupCloseStrategyProject(t, "builtin")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))
	_, stderr, code := runTP(t, dir, "commit", "t1", "implemented t1")
	assert.Equal(t, 0, code, "builtin tp commit works: %s", stderr)
}

func TestCloseStrategy_BuiltinAutoCommitWorks(t *testing.T) {
	dir := setupCloseStrategyProject(t, "builtin")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--auto-commit", "--", "t1 done")
	assert.Equal(t, 0, code, "builtin tp done --auto-commit works: %s", stderr)
}

func TestCloseStrategy_HCRejectsCommit(t *testing.T) {
	dir := setupCloseStrategyProject(t, "hc")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))
	_, stderr, code := runTP(t, dir, "commit", "t1", "did t1")
	assert.Equal(t, 2, code, "hc rejects tp commit with exit 2")
	assert.Contains(t, stderr, "commit_strategy is hc")
}

func TestCloseStrategy_HCRejectsAutoCommit(t *testing.T) {
	dir := setupCloseStrategyProject(t, "hc")
	_, _, code := runTP(t, dir, "done", "t1", "--gate-passed", "--auto-commit", "--", "t1 done")
	assert.Equal(t, 2, code, "hc rejects --auto-commit with exit 2")
}

func TestCloseStrategy_HCRejectsBareDone(t *testing.T) {
	dir := setupCloseStrategyProject(t, "hc")
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--", "t1 done")
	assert.Equal(t, 2, code, "hc rejects a bare tp done with exit 2")
	assert.NotEqual(t, 4, code, "no commit-strategy path returns exit 4")
	assert.Contains(t, stderr, "commit_strategy is hc")
}

func TestCloseStrategy_HCCommitAndCoveredByClose(t *testing.T) {
	dir := setupCloseStrategyProject(t, "hc")
	_, _, code := runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 done","source_sections":["s1"]}`)
	require.Equal(t, 0, code)
	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 done")
	assert.Equal(t, 0, code, "hc closes with --commit: %s", stderr)
	_, stderr, code = runTP(t, dir, "done", "t2", "--covered-by", "t1", "--", "covered by t1")
	assert.Equal(t, 0, code, "hc closes with --covered-by: %s", stderr)
}

func TestCloseStrategy_BatchBadRowIsFailedRow(t *testing.T) {
	dir := setupCloseStrategyProject(t, "hc")
	_, _, code := runTP(t, dir, "add", `{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t2 done","source_sections":["s1"]}`)
	require.Equal(t, 0, code)
	ndjson := `{"id":"t1","reason":"t1 done","gate_passed":true,"commit_shas":["aaa"]}` + "\n" +
		`{"id":"t2","reason":"t2 done","gate_passed":true}` + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "batch.ndjson"), []byte(ndjson), 0o600))

	out, _, code := runTP(t, dir, "done", "--batch", "batch.ndjson")
	assert.Equal(t, 1, code, "a partial failure exits 1, never a commit-strategy exit 4")
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.EqualValues(t, 1, res["closed"], "the commit_shas row closes")
	assert.EqualValues(t, 1, res["failed"], "the row lacking commit_shas and covered_by is a failed row")
	failures := res["failures"].([]any)
	assert.Contains(t, failures[0].(map[string]any)["error"], "commit_strategy is hc")
}
