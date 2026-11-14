package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gateReuseRepo is a git repository with one committed source file, a task
// file and a spec, and returns its root and the two paths.
func gateReuseRepo(t *testing.T) (root, taskFile, spec string) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		out, runErr := cmd.CombinedOutput()
		require.NoError(t, runErr, "git %v: %s", args, out)
	}
	run("init", "-q")
	spec = filepath.Join(root, "spec.md")
	taskFile = filepath.Join(root, "spec.tasks.json")
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o600))
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n"), 0o600))
	require.NoError(t, os.WriteFile(taskFile, []byte("{}\n"), 0o600))
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return root, taskFile, spec
}

// TestCleanTree_AnUntrackedFileMeansTheGateDidNotGateTheTree is the
// discriminating pair: the same committed tree is a reusable gate result when
// the checkout is clean, and not one when an untracked file sat beside it,
// since the gate ran on the working directory.
func TestCleanTree_AnUntrackedFileMeansTheGateDidNotGateTheTree(t *testing.T) {
	t.Parallel()
	root, _, _ := gateReuseRepo(t)
	tree := CleanTree(root)
	require.NotEmpty(t, tree, "a clean checkout names its tree")

	require.NoError(t, os.WriteFile(filepath.Join(root, "extra.go"), []byte("package a\n"), 0o600))
	assert.Empty(t, CleanTree(root), "an untracked file means the gate ran on more than the tree")
}

// TestReusableGate_OnlyTheSameTreeCommandAndACleanTreeReuse: the gate result
// is reused only when every condition holds; each one broken alone runs the
// gate.
func TestReusableGate_OnlyTheSameTreeCommandAndACleanTreeReuse(t *testing.T) {
	t.Parallel()
	root, taskFile, spec := gateReuseRepo(t)
	tree := CleanTree(root)
	p := &PendingClose{ID: "t1", GatedTree: tree, GateCmd: "make test"}

	// The task file changing is tp's own bookkeeping.
	require.NoError(t, os.WriteFile(taskFile, []byte("{\"changed\":true}\n"), 0o600))
	assert.True(t, ReusableGate(root, taskFile, spec, p, "make test"), "same tree, same command, only tp's files changed")

	assert.False(t, ReusableGate(root, taskFile, spec, p, "make check"), "another gate command")
	assert.False(t, ReusableGate(root, taskFile, spec, &PendingClose{ID: "t1", GateCmd: "make test"}, "make test"), "no tree recorded")
	assert.False(t, ReusableGate(root, taskFile, spec, &PendingClose{ID: "t1", GatedTree: "0000", GateCmd: "make test"}, "make test"), "another tree")
	assert.False(t, ReusableGate(root, taskFile, spec, nil, "make test"), "no pending close")

	require.NoError(t, os.WriteFile(filepath.Join(root, "kept.go"), []byte("package a\n"), 0o600))
	assert.False(t, ReusableGate(root, taskFile, spec, p, "make test"), "a file the unit never had sits on the main branch")
}
