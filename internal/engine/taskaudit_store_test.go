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

// TestFileChangedSince_ReadsRootRelativePathsFromASpecSubdirectory: the
// path is repository-relative while dir is the spec's directory, which sits
// below the root in most projects.
func TestFileChangedSince_ReadsRootRelativePathsFromASpecSubdirectory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	git := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
		return strings.TrimSpace(string(out))
	}
	git("init", "-q")
	git("config", "user.email", "t@t")
	git("config", "user.name", "t")
	require.NoError(t, os.MkdirAll(filepath.Join(root, "spec"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "spec", "s.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o600))
	git("add", "-A")
	git("commit", "-q", "-m", "one")
	head := git("rev-parse", "HEAD")
	specDir := filepath.Join(root, "spec")
	assert.False(t, FileChangedSince(specDir, head, "a.go"))

	require.NoError(t, os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n\n// changed\n"), 0o600))
	git("commit", "-q", "-am", "two")
	assert.True(t, FileChangedSince(specDir, head, "a.go"), "a root-relative path changed since head")
}
