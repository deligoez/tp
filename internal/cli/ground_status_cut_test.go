package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundStatusCheckQuiet runs the gated invocation with --quiet, which is the
// condition that decides this file's question: `output.Notice` is suppressed
// there (`internal/output/output.go`), so anything the refusal says only on
// stderr is said to nobody.
func groundStatusCheckQuiet(t *testing.T, dir string) (payload map[string]any, stderr string, exitCode int) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--status", "--check", "--quiet")
	require.NoError(t, json.Unmarshal([]byte(stdout), &payload),
		"--status --check carries --status's payload whatever it exits: stdout %q stderr %q", stdout, stderr)
	return payload, stderr, code
}

// TestStatusReportsTheCutCountCheckGatesOn pins the repair to §7.1's own
// description of what `--check` is: the exit code of the coverage `--status`
// reports.
//
// `--check` exits 1 when the emitted floor is empty and the arms cut units to
// empty it, and 0 when §2.1 produced nothing to cut. Only `cut` separates those
// two states, and until this test the payload did not carry it — so the gate
// read a quantity the report did not print, and the two arms below were
// byte-identical on stdout while exiting differently.
//
// The expected count is read back off the emitted floor index rather than
// written here as a literal, so the assertion cannot drift from the artifact
// the notice sends the operator to.
func TestStatusReportsTheCutCountCheckGatesOn(t *testing.T) {
	t.Run("every unit cut", func(t *testing.T) {
		dir := writeGroundSpec(t, groundAllCutSpec)
		groundEmit(t, dir)

		inFloor, cut := groundFloorSummary(t, dir)
		require.Equal(t, 0, inFloor, "the fixture must leave no unit owing a disposition")
		require.Positive(t, cut, "and must have had units for the arms to cut, or it is the other fixture")

		payload, code := groundStatusCheck(t, dir)
		require.Contains(t, payload, "cut", "the quantity --check gates on must appear in the payload --check prints")
		assert.Equal(t, float64(cut), payload["cut"], "cut is the floor index's own count, not a re-derivation")
		assert.Equal(t, float64(0), payload["emitted"])
		assert.Equal(t, 1, code)
	})

	t.Run("no unit to cut", func(t *testing.T) {
		dir := writeGroundSpec(t, groundNoUnitsSpec)
		groundEmit(t, dir)

		inFloor, cut := groundFloorSummary(t, dir)
		require.Equal(t, 0, inFloor)
		require.Equal(t, 0, cut, "the fixture must produce no unit at all, or it is the other fixture")

		payload, code := groundStatusCheck(t, dir)
		require.Contains(t, payload, "cut",
			"present and zero, not absent: a key a reader must first decide whether an absence stands for is not a key they can read")
		assert.Equal(t, float64(0), payload["cut"])
		assert.Equal(t, float64(0), payload["emitted"])
		assert.Equal(t, 0, code)
	})
}

