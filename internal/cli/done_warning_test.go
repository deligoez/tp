package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDoneWarning_UnexplainedChangeWarnsButExits0(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "stray.txt"), []byte("leftover"), 0o600))

	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	assert.Equal(t, 0, code, "the close still exits 0")
	assert.Contains(t, stderr, "1 uncommitted change", "the warning names the unexplained-change count")

	content, err := os.ReadFile(filepath.Join(dir, "stray.txt"))
	require.NoError(t, err)
	assert.Equal(t, "leftover", string(content), "tp neither commits nor discards the change")
}

func TestDoneWarning_KeptChangeProducesNoWarning(t *testing.T) {
	dir := setupCommitProject(t, "t1")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "kept.txt"), []byte("k"), 0o600))
	_, _, code := runTP(t, dir, "keep", "kept.txt", "intentional")
	require.Equal(t, 0, code)
	// Commit tp keep's .tp/.gitignore artifact so the only uncommitted file is the kept one.
	git(t, dir, "add", ".tp")
	git(t, dir, "commit", "-m", "keep config")

	_, stderr, code := runTP(t, dir, "done", "t1", "--gate-passed", "--commit", "aaa", "--", "t1 acceptance met")
	assert.Equal(t, 0, code)
	assert.NotContains(t, stderr, "uncommitted change", "a keep-listed change produces no warning")
}
