package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// git runs a git command in dir, failing the test on error.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// newResumeRepo creates a committed tp project with one open task in a git repo.
func newResumeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\n## 1. X\nx\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "add", `{"id":"t1","title":"X","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":["### 1. X"]}`)
	require.Equal(t, 0, code)
	initGitRepo(t, dir)
	return dir
}

func resumeResult(t *testing.T, dir string) map[string]any {
	t.Helper()
	out, stderr, code := runTP(t, dir, "resume")
	require.Equal(t, 0, code, "resume failed: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	return res
}

func jsonStrings(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		out = append(out, e.(string))
	}
	return out
}

func keptByPath(res map[string]any) map[string]string {
	arr, _ := res["kept"].([]any)
	out := make(map[string]string, len(arr))
	for _, e := range arr {
		m := e.(map[string]any)
		out[m["path"].(string)] = m["reason"].(string)
	}
	return out
}

func blockerByCode(res map[string]any, code string) map[string]any {
	arr, _ := res["blockers"].([]any)
	for _, e := range arr {
		m := e.(map[string]any)
		if m["code"] == code {
			return m
		}
	}
	return nil
}

func TestResume_ChangesModifiedStagedUntracked(t *testing.T) {
	dir := newResumeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("orig"), 0o600))
	git(t, dir, "add", "tracked.txt")
	git(t, dir, "commit", "-m", "add tracked")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("changed"), 0o600)) // modified
	require.NoError(t, os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("s"), 0o600))
	git(t, dir, "add", "staged.txt") // staged
	require.NoError(t, os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("u"), 0o600))

	res := resumeResult(t, dir)
	changes := jsonStrings(res["changes"])
	assert.ElementsMatch(t, []string{"tracked.txt", "staged.txt", "untracked.txt"}, changes)

	b := blockerByCode(res, "unexplained-changes")
	require.NotNil(t, b)
	assert.Equal(t, float64(len(changes)), b["data"].(map[string]any)["count"])
}

func TestResume_StagedRenameReportsDestination(t *testing.T) {
	dir := newResumeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "old.txt"), []byte("content"), 0o600))
	git(t, dir, "add", "old.txt")
	git(t, dir, "commit", "-m", "add old")
	git(t, dir, "mv", "old.txt", "new.txt") // staged rename

	changes := jsonStrings(resumeResult(t, dir)["changes"])
	assert.Contains(t, changes, "new.txt", "a staged rename reports its destination")
	assert.NotContains(t, changes, "old.txt")
}

func TestResume_KeepMovesChangeToKept(t *testing.T) {
	dir := newResumeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "note.txt"), []byte("n"), 0o600))
	assert.Contains(t, jsonStrings(resumeResult(t, dir)["changes"]), "note.txt")

	_, _, code := runTP(t, dir, "keep", "note.txt", "intentional")
	require.Equal(t, 0, code)

	res := resumeResult(t, dir)
	assert.NotContains(t, jsonStrings(res["changes"]), "note.txt")
	assert.Equal(t, "intentional", keptByPath(res)["note.txt"])
}

func TestResume_GlobMatchesTwoFiles(t *testing.T) {
	dir := newResumeRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.log"), []byte(""), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.log"), []byte(""), 0o600))

	_, _, code := runTP(t, dir, "keep", "*.log", "logs")
	require.Equal(t, 0, code)

	kept := keptByPath(resumeResult(t, dir))
	assert.Equal(t, "logs", kept["a.log"], "a glob keep entry yields one kept entry per matched file")
	assert.Equal(t, "logs", kept["b.log"])
}

func TestResume_NonGitDirectoryEmptyArrays(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\n## 1. X\nx\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "add", `{"id":"t1","title":"X","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":["### 1. X"]}`)
	require.Equal(t, 0, code)
	// No git init: git state cannot be read.
	res := resumeResult(t, dir)
	assert.Empty(t, jsonStrings(res["changes"]))
	assert.Empty(t, res["kept"])
}
