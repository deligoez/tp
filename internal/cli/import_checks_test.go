package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// convergedWithCheck is a converged review loop over a task-file project whose
// one registered check runs cmd.
func convergedWithCheck(t *testing.T, cmd string) string {
	t.Helper()
	dir := setupEnforceProject(t)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	_, stderr, code = runTP(t, dir, "set", "--workflow", `checks=[{"class":"vague-number","cmd":"`+cmd+`"}]`)
	require.Equal(t, 0, code, "registering the check: %s", stderr)
	var out map[string]any
	for range 2 {
		out, stderr, code = recordRound(t, dir, "")
		require.Equal(t, 0, code, "a clean round records: %s", stderr)
	}
	require.Equal(t, true, out["converged"], "the loop must be converged, or the refusal could be convergence's")
	return dir
}

// TestImport_ARegisteredCheckThatDoesNotPassBlocksTheImport: tp import read
// the loop verdict alone, so it imported a converged spec while `tp review
// --status --check` exited 1 on a registered check. At the moment the loop
// lets import go ahead it runs the checks, and refuses as that gate would —
// exit 1, naming the check and what it did.
func TestImport_ARegisteredCheckThatDoesNotPassBlocksTheImport(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ cmd, exit string }{
		{"exit 2", "exited 2"},
		{"exit 1", "exited 1"},
	} {
		t.Run(tc.cmd, func(t *testing.T) {
			t.Parallel()
			dir := convergedWithCheck(t, tc.cmd)
			stderr, code := importBare(t, dir)
			assert.Equal(t, 1, code, "a registered check that does not pass blocks the import: %s", stderr)
			var envelope struct {
				Error string `json:"error"`
			}
			require.NoError(t, json.Unmarshal([]byte(stderr), &envelope), "the refusal is the JSON error envelope: %s", stderr)
			assert.Contains(t, envelope.Error, `"vague-number"`, "the refusal names the check")
			assert.Contains(t, envelope.Error, tc.exit, "and what it did")
		})
	}
}

// TestImport_APassingCheckImports is the control: the same converged loop with
// the check passing imports as it always has.
func TestImport_APassingCheckImports(t *testing.T) {
	t.Parallel()
	dir := convergedWithCheck(t, "exit 0")
	stderr, code := importBare(t, dir)
	assert.Equal(t, 0, code, "a passing check does not block the import: %s", stderr)
}

// TestImport_ForceSkipsTheCheckGate pins what --force does with it: --force
// skips the convergence checks, and the check gate is one of them, so the
// operator's approved import goes ahead over a check that cannot run.
func TestImport_ForceSkipsTheCheckGate(t *testing.T) {
	t.Parallel()
	dir := convergedWithCheck(t, "exit 2")
	stderr, code := importBare(t, dir, "--force")
	assert.Equal(t, 0, code, "--force imports over a failing check: %s", stderr)
}
