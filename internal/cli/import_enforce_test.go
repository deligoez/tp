package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const enforceTask = `{"id":"t1","title":"T","estimate_minutes":5,"acceptance":"setup done","source_sections":["1. Setup"],"depends_on":[]}`

func setupEnforceProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec), 0o600))
	return dir
}

func importBare(t *testing.T, dir string, extra ...string) (string, int) {
	t.Helper()
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(`[`+enforceTask+`]`), 0o600))
	args := append([]string{"import", importPath, "--spec", "spec.md"}, extra...)
	_, stderr, code := runTP(t, dir, args...)
	return stderr, code
}

func TestImportEnforcement_Convergence(t *testing.T) {
	t.Run("no state imports with info only", func(t *testing.T) {
		dir := setupEnforceProject(t)
		stderr, code := importBare(t, dir)
		assert.Equal(t, 0, code, "no recorded rounds must not block: %s", stderr)
	})

	t.Run("unconverged state blocks with exit 1", func(t *testing.T) {
		dir := setupEnforceProject(t)
		_, _, code := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code)

		stderr, code2 := importBare(t, dir)
		assert.Equal(t, 1, code2)
		assert.Contains(t, stderr, "review not converged")
		assert.Contains(t, stderr, "--force")
	})

	t.Run("stale spec blocks with exit 1", func(t *testing.T) {
		dir := setupEnforceProject(t)
		for i := 0; i < 2; i++ {
			_, _, code := recordRound(t, dir, "")
			require.Equal(t, 0, code)
		}
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec+"\nedited\n"), 0o600))

		stderr, code := importBare(t, dir)
		assert.Equal(t, 1, code)
		assert.Contains(t, stderr, "spec changed since round")
	})

	t.Run("force bypasses both checks", func(t *testing.T) {
		dir := setupEnforceProject(t)
		_, _, code := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code)

		stderr, code2 := importBare(t, dir, "--force")
		assert.Equal(t, 0, code2, "--force bypass failed: %s", stderr)
	})

	t.Run("converged state imports cleanly", func(t *testing.T) {
		dir := setupEnforceProject(t)
		for i := 0; i < 2; i++ {
			_, _, code := recordRound(t, dir, "")
			require.Equal(t, 0, code)
		}
		stderr, code := importBare(t, dir)
		assert.Equal(t, 0, code, "converged import blocked: %s", stderr)
	})

	t.Run("exhausted budget swaps the hint to the escalation sequence", func(t *testing.T) {
		dir := setupEnforceProject(t)
		_, _, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code)
		_, _, code = runTP(t, dir, "set", "--workflow", "review_max_rounds=1")
		require.Equal(t, 0, code)
		_, _, code = recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code)

		stderr, code2 := importBare(t, dir)
		assert.Equal(t, 1, code2, "cap never relaxes enforcement")
		assert.Contains(t, stderr, "raise the cap", "hint names the escalation sequence")
	})
}

func TestImport_ShellOverwriteNoForce(t *testing.T) {
	dir := setupEnforceProject(t)

	// tp init creates a zero-task shell; plain import may overwrite it
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	stderr, code2 := importBare(t, dir)
	assert.Equal(t, 0, code2, "zero-task shell overwrite requires no --force: %s", stderr)

	// A file with real tasks still needs --force
	stderr2, code3 := importBare(t, dir)
	assert.NotEqual(t, 0, code3, "file with tasks requires --force")
	assert.Contains(t, stderr2, "--force")

	_, code4 := importBare(t, dir, "--force")
	assert.Equal(t, 0, code4)
}
