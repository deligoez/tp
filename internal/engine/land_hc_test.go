package engine

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Under hc a land's task-file commit goes through hc itself
// (spec/backlog/land-lifecycle.md §2.1): one plan, one commit of the task file
// alone, anything else in the tree left out of it.
func TestLandCommitTaskFileHC_CommitsTheTaskFileThroughHC(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, string(out))
		return string(out)
	}
	git("init", "-q")
	git("config", "user.email", "t@example.com")
	git("config", "user.name", "t")
	taskFile := filepath.Join(root, "spec.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte("{}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.txt"), []byte("a\n"), 0o644))
	git("add", ".")
	git("commit", "-q", "-m", "base")
	require.NoError(t, os.WriteFile(taskFile, []byte("{\"x\":1}\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "other.txt"), []byte("b\n"), 0o644))

	// A stand-in hc: it keeps the plan it was handed and makes the commit
	// the plan names, as hc run does.
	bin := t.TempDir()
	planOut := filepath.Join(bin, "plan.json")
	script := "#!/bin/sh\n[ \"$1\" = run ] && [ \"$2\" = - ] || exit 9\ncat > " + planOut +
		"\ngit commit -q -m land -- spec.tasks.json\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "hc"), []byte(script), 0o755))
	prev := hcBinary
	hcBinary = filepath.Join(bin, "hc")
	t.Cleanup(func() { hcBinary = prev })

	sha, err := LandCommitTaskFileHC(root, taskFile, "chore(tp): land t1")
	require.NoError(t, err)
	assert.NotEmpty(t, sha)

	plan, err := os.ReadFile(planOut)
	require.NoError(t, err)
	assert.JSONEq(t, `{"allow_unplanned":["**"],"commits":[{"message":"chore(tp): land t1","files":[{"path":"spec.tasks.json"}]}]}`, string(plan))
	assert.Empty(t, git("status", "--porcelain", "--", "spec.tasks.json"))
	assert.NotEmpty(t, git("status", "--porcelain", "--", "other.txt"), "the rest of the tree stays out of the commit")
}
