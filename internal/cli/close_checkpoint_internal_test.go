package cli

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitRunInternal runs a git command in dir and returns trimmed stdout.
func gitRunInternal(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err, "git %v: %s", args, string(out))
	return strings.TrimSpace(string(out))
}

func writeInternalFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func initInternalGitRepo(t *testing.T, dir string) {
	t.Helper()
	gitRunInternal(t, dir, "init")
	gitRunInternal(t, dir, "config", "user.email", "t@t.com")
	gitRunInternal(t, dir, "config", "user.name", "T")
}

// TestCanAmendClosure_HeadsMoved verifies §5.1b guard (i): when a commit lands
// between tp's implementation commit and the amend (HEAD no longer C1), the
// amend is refused so tp falls back to the §5.1d follow-up.
func TestCanAmendClosure_HeadsMoved(t *testing.T) {
	dir := t.TempDir()
	initInternalGitRepo(t, dir)
	writeInternalFile(t, filepath.Join(dir, "spec.tasks.json"), "v1\n")
	gitRunInternal(t, dir, "add", "-A")
	gitRunInternal(t, dir, "commit", "-m", "c1")
	c1 := gitRunInternal(t, dir, "rev-parse", "--short", "HEAD")
	// A later commit moves HEAD past C1.
	gitRunInternal(t, dir, "commit", "--allow-empty", "-m", "sneaky")
	assert.False(t, canAmendClosure(dir, c1, []string{"spec.tasks.json"}),
		"guard (i): HEAD moved past C1")
}

// TestCanAmendClosure_DirtyNonTpPath verifies §5.1b guard (ii): a non-tp-owned
// path differing from HEAD forces the fallback even when HEAD is still C1.
func TestCanAmendClosure_DirtyNonTpPath(t *testing.T) {
	dir := t.TempDir()
	initInternalGitRepo(t, dir)
	writeInternalFile(t, filepath.Join(dir, "tracked.txt"), "v1\n")
	writeInternalFile(t, filepath.Join(dir, "spec.tasks.json"), "v1\n")
	gitRunInternal(t, dir, "add", "-A")
	gitRunInternal(t, dir, "commit", "-m", "c1")
	c1 := gitRunInternal(t, dir, "rev-parse", "--short", "HEAD")
	// Dirty a non-tp-owned tracked file (HEAD still C1).
	writeInternalFile(t, filepath.Join(dir, "tracked.txt"), "v2\n")
	assert.False(t, canAmendClosure(dir, c1, []string{"spec.tasks.json"}),
		"guard (ii): non-tp path differs")
}

// TestCanAmendClosure_OnlyTpWrittenPath verifies both guards hold when HEAD is
// still C1 and the only working-tree difference is a tp-written path.
func TestCanAmendClosure_OnlyTpWrittenPath(t *testing.T) {
	dir := t.TempDir()
	initInternalGitRepo(t, dir)
	writeInternalFile(t, filepath.Join(dir, "spec.tasks.json"), "v1\n")
	gitRunInternal(t, dir, "add", "-A")
	gitRunInternal(t, dir, "commit", "-m", "c1")
	c1 := gitRunInternal(t, dir, "rev-parse", "--short", "HEAD")
	// Only the task file (a tp-written path) differs.
	writeInternalFile(t, filepath.Join(dir, "spec.tasks.json"), "v2\n")
	assert.True(t, canAmendClosure(dir, c1, []string{"spec.tasks.json"}),
		"both guards hold: HEAD is C1 and only the task file differs")
}

// TestFoldClosureCommit_FollowUpOnHeadsMoved verifies the §5.1d follow-up path
// end-to-end at the helper level: when HEAD moved past C1, a follow-up commit
// chore(tp): record <id> closure is created and C1 is not the resulting HEAD.
func TestFoldClosureCommit_FollowUpOnHeadsMoved(t *testing.T) {
	dir := t.TempDir()
	initInternalGitRepo(t, dir)
	writeInternalFile(t, filepath.Join(dir, "spec.tasks.json"), "v1\n")
	gitRunInternal(t, dir, "add", "-A")
	gitRunInternal(t, dir, "commit", "-m", "c1")
	c1 := gitRunInternal(t, dir, "rev-parse", "--short", "HEAD")
	// Move HEAD past C1, then write the closure into the task file.
	gitRunInternal(t, dir, "commit", "--allow-empty", "-m", "sneaky")
	writeInternalFile(t, filepath.Join(dir, "spec.tasks.json"), "v2 closed\n")

	err := foldClosureCommit(dir, "t1", c1, []string{"spec.tasks.json"})
	require.NoError(t, err)

	logOut := gitRunInternal(t, dir, "log", "--format=%s")
	assert.Contains(t, logOut, "chore(tp): record t1 closure")
	head := gitRunInternal(t, dir, "rev-parse", "--short", "HEAD")
	assert.NotEqual(t, c1, head, "C1 is not the follow-up HEAD")
	// The task file is committed (clean for the tp-owned path).
	assert.NotContains(t, gitRunInternal(t, dir, "status", "--porcelain"), "spec.tasks.json")
}
