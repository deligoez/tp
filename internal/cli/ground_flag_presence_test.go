package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGroundRefusesAnEmptyRecordPathRatherThanEmitting is §7.1's exit-2 row on
// the reading its shipped test does not reach: `--record ""` is `--record` with
// no path argument.
//
// `tp ground <spec> --record` — no value at all — is refused by cobra before any
// of this command's code runs, so the shipped exit-2 test says nothing about the
// dispatch. An explicitly EMPTY path does reach it, and the dispatch asks
// `recordPath != ""`, which a flag's presence and a flag's value both answer
// through one predicate: the empty path selects neither the record mode nor the
// mode conflict, and falls through to the EMISSION.
//
// The exit code is therefore only half the assertion. The emission rewrites the
// floor §7.3 freezes, so this edits the spec between the emission and the
// invocation and requires the frozen floor to come back byte-identical — an
// implementation that exits 2 after writing passes on the code alone. The
// control at the end is what makes that equality a claim about the refusal
// rather than about the fixture: the same edit, emitted, must move the floor.
func TestGroundRefusesAnEmptyRecordPathRatherThanEmitting(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)
	before, err := os.ReadFile(groundStatePath(dir, "floor-ground-round-1.txt"))
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte(groundFixtureSpec+groundFixtureEdit), 0o600))

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", "")
	require.Equal(t, 2, code, "stdout: %s stderr: %s", stdout, stderr)
	assert.Equal(t, float64(2), groundErrorEnvelope(t, stderr)["code"])

	after, err := os.ReadFile(groundStatePath(dir, "floor-ground-round-1.txt"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after),
		"a refused invocation emits nothing: the round's floor is what its own emission froze")

	groundEmit(t, dir)
	reemitted, err := os.ReadFile(groundStatePath(dir, "floor-ground-round-1.txt"))
	require.NoError(t, err)
	require.NotEqual(t, string(before), string(reemitted),
		"the fixture edit must be one a real emission would write, or the equality above is a tautology")
}

// TestGroundCountsAnEmptyRecordPathAsAModePassed is the same conflation at the
// other site: groundModesPassed counted --record by the same `!= ""` predicate,
// so `--record "" --status` saw ONE mode and ran --status, reporting exit 0 for
// a mode the operator did not ask for on its own.
//
// It is asserted separately from the test above because the two sites fail
// differently — one emits, one silently picks a mode — and a fix to the dispatch
// alone leaves this one standing.
func TestGroundCountsAnEmptyRecordPathAsAModePassed(t *testing.T) {
	dir := writeGroundFixture(t)
	groundEmit(t, dir)

	stdout, stderr, code := runTP(t, dir, "ground", "spec.md", "--status", "--record", "")
	require.Equal(t, 2, code, "stdout: %s stderr: %s", stdout, stderr)
	envelope := groundErrorEnvelope(t, stderr)
	assert.Equal(t, float64(2), envelope["code"])
	assert.Contains(t, envelope["error"], "separate modes",
		"the refusal is the pairing rule, not the missing-path one: two modes were passed")
}
