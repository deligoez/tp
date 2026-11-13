package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitIn runs git in dir and returns its trimmed stdout, failing the test on error.
func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// unitRepo makes a repository with one commit holding a task file and a source
// file, plus a linked worktree on branch unit, and returns (root, worktree).
func unitRepo(t *testing.T) (root, worktree string) {
	t.Helper()
	root = t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	gitIn(t, root, "config", "user.email", "t@example.com")
	gitIn(t, root, "config", "user.name", "t")
	require.NoError(t, os.WriteFile(filepath.Join(root, "spec.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "spec.tasks.json"), []byte(`{"spec":"spec.md","tasks":[]}`), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o600))
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "base")
	worktree = filepath.Join(t.TempDir(), "wt")
	gitIn(t, root, "worktree", "add", "-q", "-b", "unit", worktree)
	return root, worktree
}

func commitIn(t *testing.T, dir, file, body string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, file)), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte(body), 0o600))
	gitIn(t, dir, "add", file)
	gitIn(t, dir, "commit", "-q", "-m", "change "+file)
	return gitIn(t, dir, "rev-parse", "HEAD")
}

// TestPendingClose_RoundTrip: a pending close is written under the cycle's
// key, read back in id order, and removed (spec/backlog/parallel-implement.md §2.4).
func TestPendingClose_RoundTrip(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	taskFile := filepath.Join(root, "spec.tasks.json")
	require.NoError(t, WritePendingClose(root, taskFile, &PendingClose{ID: "b", Reason: "r", Commits: []string{"2"}}))
	require.NoError(t, WritePendingClose(root, taskFile, &PendingClose{ID: "a", Reason: "r", Commits: []string{"1"}}))

	got, err := ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.NotEmpty(t, got[0].At)

	require.NoError(t, RemovePendingClose(root, taskFile, "a"))
	got, err = ReadPendingCloses(root, taskFile)
	require.NoError(t, err)
	assert.Equal(t, []string{"b"}, []string{got[0].ID})
}

// TestLinkedWorktreeOf: a linked worktree of the same repository is told apart
// from the main tree and from an unrelated repository.
func TestLinkedWorktreeOf(t *testing.T) {
	t.Parallel()
	root, worktree := unitRepo(t)
	assert.True(t, LinkedWorktreeOf(worktree, root))
	assert.False(t, LinkedWorktreeOf(root, root), "the main tree is not a linked worktree")
	other, _ := unitRepo(t)
	assert.False(t, LinkedWorktreeOf(worktree, other), "a worktree of another repository")
}

// TestCheckUnitCommits: a unit's commits must be on its branch, not on the
// main branch, and touch no tp-owned path.
func TestCheckUnitCommits(t *testing.T) {
	t.Parallel()
	root, worktree := unitRepo(t)
	taskFile := filepath.Join(root, "spec.tasks.json")
	spec := filepath.Join(root, "spec.md")
	good := commitIn(t, worktree, "a.go", "package a\n\nvar X = 1\n")
	got, err := CheckUnitCommits(root, worktree, taskFile, spec, []string{good})
	require.NoError(t, err)
	assert.Equal(t, []string{good}, got)

	base := gitIn(t, root, "rev-parse", "HEAD")
	_, err = CheckUnitCommits(root, worktree, taskFile, spec, []string{base})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already on the main branch")

	owned := commitIn(t, worktree, "spec.tasks.json", `{"spec":"spec.md","tasks":[{"id":"x"}]}`)
	_, err = CheckUnitCommits(root, worktree, taskFile, spec, []string{owned})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.tasks.json")

	_, err = CheckUnitCommits(root, worktree, taskFile, spec, []string{"deadbeef"})
	require.Error(t, err)
}

// TestCheckUnitCommits_NamesEveryCommitOfTheBranch: the request must name the
// branch's every commit since it left the main branch, and they come back in
// branch order whatever order they were named in.
func TestCheckUnitCommits_NamesEveryCommitOfTheBranch(t *testing.T) {
	t.Parallel()
	root, worktree := unitRepo(t)
	taskFile := filepath.Join(root, "spec.tasks.json")
	spec := filepath.Join(root, "spec.md")
	first := commitIn(t, worktree, "a.go", "package a\n\nvar X = 1\n")
	second := commitIn(t, worktree, "b.go", "package a\n")

	_, err := CheckUnitCommits(root, worktree, taskFile, spec, []string{second})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "names 1 of the branch's 2 commits")

	got, err := CheckUnitCommits(root, worktree, taskFile, spec, []string{second, first[:12]})
	require.NoError(t, err)
	assert.Equal(t, []string{first, second}, got)
}

// TestCheckUnitCommits_AProjectBelowTheGitTopLevel: in a project whose .tp/ sits
// in a subdirectory of the repository, a commit changing its task file is
// still refused.
func TestCheckUnitCommits_AProjectBelowTheGitTopLevel(t *testing.T) {
	t.Parallel()
	root, worktree := unitRepo(t)
	sub := filepath.Join(root, "sub")
	require.NoError(t, os.MkdirAll(filepath.Join(sub, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(sub, ".tp", "config.json"), []byte("{}\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "spec.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "spec.tasks.json"), []byte("{}\n"), 0o600))
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "sub project")
	gitIn(t, worktree, "merge", "-q", "--ff-only", "main")
	owned := commitIn(t, worktree, "sub/spec.tasks.json", `{"tasks":[]}`)

	_, err := CheckUnitCommits(sub, worktree, filepath.Join(sub, "spec.tasks.json"), filepath.Join(sub, "spec.md"), []string{owned})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only tp writes")
}
