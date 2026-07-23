package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitRun runs a git command in dir and returns trimmed stdout.
func gitRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err, "git %v: %s", args, string(out))
	return strings.TrimSpace(string(out))
}

// committedTaskField reads a single field from the task file as committed at
// HEAD, so a test can assert what the committed tree carries (not the working
// tree).
func committedTaskField(t *testing.T, dir, taskFile, id, field string) any {
	t.Helper()
	content := gitRun(t, dir, "show", "HEAD:"+taskFile)
	var tf struct {
		Tasks []map[string]any `json:"tasks"`
	}
	require.NoError(t, json.Unmarshal([]byte(content), &tf))
	for _, tk := range tf.Tasks {
		if tk["id"] == id {
			return tk[field]
		}
	}
	return nil
}

// TestCloseCheckpoint_CommitFoldsClosure verifies that under commit_strategy
// builtin, tp commit folds the commit_sha record into the implementation commit
// via amend: the committed task file carries commit_sha, the working tree is
// clean for tp-owned paths, and the recorded sha is the pre-amend C1 (§5.1a-c).
func TestCloseCheckpoint_CommitFoldsClosure(t *testing.T) {
	dir := setupCloseStrategyProject(t, "builtin")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "commit", "t1", "implemented t1")
	require.Equal(t, 0, code, "tp commit: %s", stderr)

	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &res))
	c1 := res["sha"].(string)

	// Working tree clean for tp-owned paths: the task file was folded via amend.
	assert.NotContains(t, gitRun(t, dir, "status", "--porcelain"), "spec.tasks.json",
		"task file folded into the commit, not left dirty")

	// HEAD is the amended commit C2, distinct from the recorded pre-amend sha C1.
	head := gitRun(t, dir, "rev-parse", "--short", "HEAD")
	assert.NotEqual(t, c1, head, "C1 (recorded) differs from C2 (HEAD)")

	// The committed task file carries commit_sha = C1 (§5.1c).
	assert.Equal(t, c1, committedTaskField(t, dir, "spec.tasks.json", "t1", "commit_sha"))
}

// TestCloseCheckpoint_DoneAutoCommitFoldsClosure verifies that tp done
// --auto-commit folds the closure (status:done) into the implementation commit:
// the committed task file shows status:done, the working tree is clean, and the
// recorded sha is the pre-amend C1 (§5.1a-c).
func TestCloseCheckpoint_DoneAutoCommitFoldsClosure(t *testing.T) {
	dir := setupCloseStrategyProject(t, "builtin")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))

	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--auto-commit", "--", "t1 done")
	require.Equal(t, 0, code, "tp done --auto-commit: %s", stderr)

	// C1 recorded (pre-amend); tp show reads the working-tree task file.
	showOut, _, _ := runTP(t, dir, "show", "t1")
	var show map[string]any
	require.NoError(t, json.Unmarshal([]byte(showOut), &show))
	c1 := show["commit_sha"].(string)

	head := gitRun(t, dir, "rev-parse", "--short", "HEAD")
	assert.NotEqual(t, c1, head, "recorded sha is C1 (pre-amend), not C2 (HEAD)")

	// The committed task file shows status:done.
	assert.Equal(t, "done", committedTaskField(t, dir, "spec.tasks.json", "t1", "status"))
	// Working tree clean for tp-owned paths.
	assert.NotContains(t, gitRun(t, dir, "status", "--porcelain"), "spec.tasks.json")
}

// TestCloseCheckpoint_FallbackDirtyNonTpPath verifies the §5.1b(ii) guard: when a
// tracked non-tp-owned path differs from HEAD, tp falls back to a follow-up
// commit chore(tp): record <id> closure, leaving C1 as commit_sha (§5.1d).
func TestCloseCheckpoint_FallbackDirtyNonTpPath(t *testing.T) {
	dir := setupCloseStrategyProject(t, "builtin")
	// A tracked, non-tp-owned file committed, then dirtied before tp runs.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v1\n"), 0o600))
	gitRun(t, dir, "add", "tracked.txt")
	gitRun(t, dir, "commit", "-m", "add tracked")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v2\n"), 0o600))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))

	// --files scopes staging to impl.go, leaving tracked.txt dirty (unstaged) so
	// guard (ii) sees a non-tp-owned differing path after C1.
	stdout, stderr, code := runTP(t, dir, "commit", "t1", "--files", "*.go", "implemented t1")
	require.Equal(t, 0, code, "tp commit with dirty non-tp path: %s", stderr)

	// The follow-up commit landed (§5.1d).
	logOut := gitRun(t, dir, "log", "--format=%s")
	assert.Contains(t, logOut, "chore(tp): record t1 closure")

	// C1 recorded (the implementation commit sha), not the follow-up.
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &res))
	c1 := res["sha"].(string)
	showOut, _, _ := runTP(t, dir, "show", "t1")
	var show map[string]any
	require.NoError(t, json.Unmarshal([]byte(showOut), &show))
	assert.Equal(t, c1, show["commit_sha"], "C1 recorded, not the follow-up commit")

	// The non-tp-owned path stays dirty (never folded); the task file is committed.
	status := gitRun(t, dir, "status", "--porcelain")
	assert.Contains(t, status, "tracked.txt", "non-tp path left untouched")
	assert.NotContains(t, status, "spec.tasks.json", "task file committed via follow-up")
}
