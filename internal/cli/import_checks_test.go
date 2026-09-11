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

// TestImport_ACheckThatWritesTheTaskFileCompletes: import ran the registered
// checks while it held the task-file write lock, so a check that itself runs a
// tp write against that task file waited on the lock import held and timed
// out, failing the check and refusing the import. The checks run before the
// lock is taken. The lock timeout is cut to one second so the old behaviour
// fails fast rather than slowly.
func TestImport_ACheckThatWritesTheTaskFileCompletes(t *testing.T) {
	t.Parallel()
	dir := convergedWithCheck(t, binaryPath+" set --workflow gate_timeout_seconds=600")
	_, stderr, code := runTP(t, dir, "set", "--workflow", "lock_timeout_seconds=1")
	require.Equal(t, 0, code, "cutting the lock timeout: %s", stderr)

	stderr, code = importBare(t, dir)
	assert.Equal(t, 0, code, "a check writing the task file completes during tp import: %s", stderr)
}

// TestImport_ChecksThatMovedUnderTheImportAreRetried: the checks' verdict is
// taken before the lock, so the locked enforcement confirms it was taken over
// the registration still in force. Here the one registered check replaces
// itself with a passing one when it runs: the verdict answers for a check no
// longer registered, and the import is refused with a retry, not decided on
// it. The retry then runs the check now registered and imports.
func TestImport_ChecksThatMovedUnderTheImportAreRetried(t *testing.T) {
	t.Parallel()
	passing, err := json.Marshal([]map[string]string{{"class": "vague-number", "cmd": "exit 0"}})
	require.NoError(t, err)
	rewrite := binaryPath + " set --workflow 'checks=" + string(passing) + "'"
	checks, err := json.Marshal([]map[string]string{{"class": "vague-number", "cmd": rewrite}})
	require.NoError(t, err)

	dir := setupEnforceProject(t)
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, "init: %s", stderr)
	for range 2 {
		_, stderr, code = recordRound(t, dir, "")
		require.Equal(t, 0, code, "a clean round records: %s", stderr)
	}
	// Registered after the rounds, so no --record runs the rewrite first.
	_, stderr, code = runTP(t, dir, "set", "--workflow", "checks="+string(checks))
	require.Equal(t, 0, code, "registering the check: %s", stderr)

	stderr, code = importBare(t, dir)
	assert.Equal(t, 4, code, "the registered checks moved while they ran: %s", stderr)
	assert.Contains(t, stderr, "run the same tp import again")

	stderr, code = importBare(t, dir)
	assert.Equal(t, 0, code, "the retry runs the check now registered and imports: %s", stderr)
}
