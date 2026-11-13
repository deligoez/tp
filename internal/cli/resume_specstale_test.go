package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResume_SpecStaleWithARoundInFlightNamesItsRecord: two converged review
// rounds, round 3 emitted over that text, then the spec edited. `tp review
// <spec>` now exits 3 over the in-flight round, so the blocker's data names
// that round and the commands that clear it start with its record, with its
// discard under operator as the operator's decision.
func TestResume_SpecStaleWithARoundInFlightNamesItsRecord(t *testing.T) {
	t.Parallel()
	dir := newPayloadRepo(t, `[{"id":"t1","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]`)
	writeConvergedRounds(t, dir, 2, 0)
	old, err := os.ReadFile(filepath.Join(dir, "spec.md"))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".tp-review", "spec", "snapshot-round-3.md"), old, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\nedited after round 3 was emitted\n"), 0o600))

	_, _, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 3, code, "the fixture must make a new emission refuse")

	res := resumeResult(t, dir)
	b := blockerByCode(res, "spec-stale")
	require.NotNil(t, b)
	data := b["data"].(map[string]any)
	spec := res["spec"].(string)
	assert.Equal(t, float64(3), data["in_flight_round"])
	commands := jsonStrings(data["commands"])
	require.NotEmpty(t, commands)
	assert.Equal(t, "tp review "+spec+" --record <file>", commands[0], "the first command is the record of the round in flight")
	operator, ok := data["operator"].(map[string]any)
	require.True(t, ok, "the discard is named as the operator's decision: %v", data)
	assert.Equal(t, "discard-emission", operator["decision"])
}
