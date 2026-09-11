package cli_test

import (
	"encoding/json"
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

func importBare(t *testing.T, dir string, extra ...string) (stderr string, code int) {
	t.Helper()
	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(`[`+enforceTask+`]`), 0o600))
	args := append([]string{"import", importPath, "--spec", "spec.md"}, extra...)
	_, stderr, code = runTP(t, dir, args...)
	return stderr, code
}

func TestImport_ConvergenceEnforced(t *testing.T) {
	t.Parallel()
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
		for range 2 {
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
		for range 2 {
			_, _, code := recordRound(t, dir, "")
			require.Equal(t, 0, code)
		}
		stderr, code := importBare(t, dir)
		assert.Equal(t, 0, code, "converged import blocked: %s", stderr)
	})

	t.Run("at the cap an open finding blocks and the hint names disposition", func(t *testing.T) {
		dir := setupEnforceProject(t)
		_, _, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code)
		_, _, code = runTP(t, dir, "set", "--workflow", "review_max_rounds=1")
		require.Equal(t, 0, code)
		_, _, code = recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code)

		stderr, code2 := importBare(t, dir)
		assert.Equal(t, 1, code2, "an undispositioned finding at the cap still blocks")
		assert.Contains(t, stderr, "--resolve", "the hint names disposition, the way out at the cap")
	})
}

func TestImport_ShellOverwriteNoForce(t *testing.T) {
	t.Parallel()
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

// TestImport_AtTheCap: the default cap ends review once every finding of the
// latest round carries a disposition. Before that, import names the way out;
// after it, import passes and says what the cap waived — a spec changed after
// the last round, which no round has read.
func TestImport_AtTheCap(t *testing.T) {
	t.Parallel()
	dir := setupEnforceProject(t)
	for range 3 {
		_, stderr, code := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code, stderr)
	}

	stderr, code := importBare(t, dir)
	assert.Equal(t, 1, code, "an open finding at the cap still blocks: %s", stderr)
	assert.Contains(t, stderr, "--resolve", "the hint names disposition, the only way out at the cap")

	round3 := filepath.Join(".tp-review", "spec", "review-round-3.ndjson")
	_, stderr, code = runTP(t, dir, "review", round3, "--resolve", "0", "wontfix", "accepted by the operator: out of scope")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(normSpec+"\nedited after round 3\n"), 0o600))

	stderr, code = importBare(t, dir)
	assert.Equal(t, 0, code, "the cap ended review: %s", stderr)
	assert.Contains(t, stderr, "round cap")
	assert.Contains(t, stderr, "spec changed since round 3", "the waived staleness is said, not hidden")
}

// TestImport_ABlockingFixedAtTheCapIsNoEscape is tp-smart's reproduction:
// under TP_UNATTENDED a unit marks the blocking finding of round 3 fixed with
// the spec unchanged, and import must not pass on it — fixed is the unit's own
// write, but at the cap no round will re-read it. The operator's acceptance
// with evidence is what ends the loop, and the import payload then says so.
func TestImport_ABlockingFixedAtTheCapIsNoEscape(t *testing.T) {
	t.Parallel()
	dir := setupEnforceProject(t)
	for range 3 {
		_, stderr, code := recordRound(t, dir, dirtyRow)
		require.Equal(t, 0, code, stderr)
	}
	round3 := filepath.Join(".tp-review", "spec", "review-round-3.ndjson")
	_, stderr, code := runTPFence(t, dir, true, "review", round3, "--resolve", "0", "fixed", "claimed")
	require.Equal(t, 0, code, "fixed stays a unit's to write: %s", stderr)

	importPath := filepath.Join(dir, "import.json")
	require.NoError(t, os.WriteFile(importPath, []byte(`[`+enforceTask+`]`), 0o600))
	_, stderr, code = runTPFence(t, dir, true, "import", importPath, "--spec", "spec.md")
	assert.Equal(t, 1, code, "a blocking finding marked fixed at the cap keeps review open: %s", stderr)
	assert.Contains(t, stderr, "marked fixed")

	_, stderr, code = runTPFence(t, dir, false, "review", round3, "--resolve", "0", "wontfix", "accepted by the operator", "--force")
	require.Equal(t, 0, code, stderr)
	stdout, stderr, code := runTPFence(t, dir, false, "import", importPath, "--spec", "spec.md")
	require.Equal(t, 0, code, "the operator's acceptance ends the loop: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), stdout)
	assert.Equal(t, "cap", out["done_by"], "drivers read the waiver from the payload, not only stderr")
	assert.Contains(t, out, "fixed_at_cap")
	assert.Contains(t, out, "stale_waived")
}
