package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReviewStatusCheck_ARegisteredCheckThatNeverRanIsNotAPass closes a gate
// that reported success having run nothing.
//
// runMechanicalChecks returned allPass=true when no task file resolved, without
// running a single registered check. Measured on two copies of one directory,
// identical in every other way — same `.tp/config.json`, same registered check,
// same three recorded clean rounds, `converged: true` in both: with a task file
// present `--status --check` ran the check; with none it exited 0 having run
// nothing, reporting `mechanical_checks: []`. A fresh clone therefore passed the
// gate for free, while tp went on telling every reviewer that the check's class
// is mechanized and should not be reported.
//
// The pair is the point. A check that PASSES is used deliberately, so the two
// runs differ in exit code (0 with a task file, non-zero without) rather than
// agreeing for unrelated reasons; and `converged` is read back from both, so
// neither verdict can be resting on convergence instead of on the checks.
func TestReviewStatusCheck_ARegisteredCheckThatNeverRanIsNotAPass(t *testing.T) {
	t.Parallel()

	// setup returns a converged review state with one registered check that
	// passes when it is run at all.
	setup := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
		require.NoError(t, os.Mkdir(filepath.Join(dir, ".tp"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp", "config.json"),
			[]byte(`{"workflow":{"checks":[{"class":"always-passes","cmd":"exit 0"}]}}`), 0o600))
		for range 3 {
			_, stderr, code := recordRound(t, dir, "")
			require.Equal(t, 0, code, "a clean round records: %s", stderr)
		}
		return dir
	}

	statusCheck := func(t *testing.T, dir string) (checks []any, converged bool, code int) {
		t.Helper()
		stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--status", "--check")
		var res map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &res), "status must be JSON: %s / %s", stdout, stderr)
		checks, _ = res["mechanical_checks"].([]any)
		converged, _ = res["converged"].(bool)
		return checks, converged, code
	}

	t.Run("a task file resolves: the check runs and the gate passes", func(t *testing.T) {
		t.Parallel()
		dir := setup(t)
		_, stderr, code := runTP(t, dir, "init", "spec.md")
		require.Equal(t, 0, code, "init: %s", stderr)

		checks, converged, code := statusCheck(t, dir)
		require.True(t, converged, "the control must be converged, or --check's exit says nothing about checks")
		require.Len(t, checks, 1, "the registered check ran")
		assert.Equal(t, 0, code, "a passing check over a converged state is the ship signal")
	})

	t.Run("no task file: the check cannot run, so the gate does not pass", func(t *testing.T) {
		t.Parallel()
		dir := setup(t)

		checks, converged, code := statusCheck(t, dir)
		require.True(t, converged, "the two runs must agree on convergence, or the pair proves nothing")
		require.Empty(t, checks, "no check ran — this is the condition under test, not an assumption")
		assert.NotEqual(t, 0, code,
			"a registered check that never ran is not a check that passed")
	})
}
