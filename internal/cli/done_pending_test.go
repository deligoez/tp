package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// pendingProject is a committed project whose task t1 is claimed, with a
// linked worktree on branch unit and a gate that leaves a marker when it runs.
func pendingProject(t *testing.T) (root, worktree, taskFile string) {
	t.Helper()
	root = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "spec.md"), []byte("# Spec\n## 1. A\ncontent\n"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", "config.json"), []byte(`{"workflow":{"quality_gate":"touch gate-ran"}}`), 0o600))
	_, stderr, code := runTP(t, root, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, root, "add",
		`{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"Done.","source_sections":["# Spec"]}`)
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, root, "claim", "t1")
	require.Equal(t, 0, code, stderr)
	initGitRepo(t, root)
	worktree = filepath.Join(t.TempDir(), "wt")
	gitRun(t, root, "worktree", "add", "-q", "-b", "unit", worktree)
	return root, worktree, filepath.Join(root, "spec.tasks.json")
}

// TestDone_InALinkedWorktreeWritesAPendingClose: in a unit's worktree tp done
// runs no gate and closes nothing; it records the request for the driver
// (spec/backlog/parallel-implement.md §2.4).
func TestDone_InALinkedWorktreeWritesAPendingClose(t *testing.T) {
	t.Parallel()
	root, worktree, taskFile := pendingProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "a.go"), []byte("package a\n"), 0o600))
	gitRun(t, worktree, "add", "a.go")
	gitRun(t, worktree, "commit", "-q", "-m", "add a")
	sha := gitRun(t, worktree, "rev-parse", "HEAD")

	stdout, stderr, code := runTPEnv(t, worktree, []string{"TP_FILE=" + taskFile},
		"done", "t1", "implemented and verified", "--commit", sha)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout, `"pending": true`)

	pending, err := engine.ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, []string{sha}, pending[0].Commits)

	status, _, _ := runTP(t, root, "show", "t1")
	assert.Contains(t, status, `"status": "wip"`, "the driver closes the task, not the unit")
	assert.NoFileExists(t, filepath.Join(worktree, "gate-ran"))
	assert.NoFileExists(t, filepath.Join(root, "gate-ran"))
}

// TestDone_InALinkedWorktreeRefusesATpOwnedPath: a unit's commit that changes
// the task file is refused and no request is written.
func TestDone_InALinkedWorktreeRefusesATpOwnedPath(t *testing.T) {
	t.Parallel()
	root, worktree, taskFile := pendingProject(t)
	require.NoError(t, os.WriteFile(filepath.Join(worktree, "spec.tasks.json"), []byte("{}\n"), 0o600))
	gitRun(t, worktree, "commit", "-q", "-am", "edit the task file")
	sha := gitRun(t, worktree, "rev-parse", "HEAD")

	_, stderr, code := runTPEnv(t, worktree, []string{"TP_FILE=" + taskFile},
		"done", "t1", "implemented and verified", "--commit", sha)
	assert.Equal(t, 1, code, stderr)
	assert.Contains(t, stderr, "only tp writes")
	pending, err := engine.ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// TestDone_InALinkedWorktreeNeedsACommit: the request names the commits to
// integrate, so a close without --commit is a usage error.
func TestDone_InALinkedWorktreeNeedsACommit(t *testing.T) {
	t.Parallel()
	_, worktree, taskFile := pendingProject(t)
	_, stderr, code := runTPEnv(t, worktree, []string{"TP_FILE=" + taskFile},
		"done", "t1", "implemented and verified")
	assert.Equal(t, 2, code, stderr)
	assert.Contains(t, stderr, "--commit")
}
