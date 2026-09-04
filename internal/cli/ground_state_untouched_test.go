package cli_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundFutureStateKey is a top-level key in state.json that engine.ReviewState's
// typed struct does not know.
//
// It is the fixture's whole point, and it is named after the mutant §11 row 21b
// asks about: "index the ground round in state.json, whose typed struct an older
// binary then drops on its next write". Seeding it makes the byte comparison
// below refuse a WIDER class of implementations than a plain state.json would.
// A ground round that merely loaded the state and saved it back unchanged writes
// bytes that differ, because SaveReviewState marshals the struct and the key is
// not in it — so the assertion catches a round-trip that adds nothing as well as
// one that adds an index entry.
//
// The last step of the test is the control that this is true rather than
// assumed: a `tp review --record` over the same file drops the key.
const groundFutureStateKey = "ground_rounds"

// seedReviewStateWithAFutureKey gives dir a state.json a ground round must not
// disturb, and returns its exact bytes.
//
// The file is tp's own — written by a real `tp review --record` rather than
// hand-assembled — so the fixture is a state.json this binary produces and not
// one only this test believes in. The unknown key is then added to it, and the
// require.NotContains before that is what keeps the key unknown: if a later
// release adds this field to ReviewState, the seed stops being a future key and
// the control at the end of the test would silently become the assertion.
func seedReviewStateWithAFutureKey(t *testing.T, dir string) []byte {
	t.Helper()
	_, stderr, code := recordRound(t, dir,
		`{"severity":"low","category":"consistency","location":"L1","finding":"f1","suggestion":"s"}`+"\n")
	require.Equal(t, 0, code, "the fixture needs a real state.json: %s", stderr)

	path := groundStatePath(dir, "state.json")
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var state map[string]any
	require.NoError(t, json.Unmarshal(data, &state))
	require.NotContains(t, state, groundFutureStateKey,
		"the seeded key must be one ReviewState does not know, or the control below asserts nothing")
	state[groundFutureStateKey] = []any{map[string]any{"round": 1}}

	seeded, err := json.MarshalIndent(state, "", "  ")
	require.NoError(t, err)
	seeded = append(seeded, '\n')
	require.NoError(t, os.WriteFile(path, seeded, 0o600))
	return seeded
}

// TestAGroundRoundLeavesAnExistingStateJSONByteIdentical is §11 row 21b: a
// ground round on a spec whose spec/.tp-review/<base>/state.json ALREADY exists
// leaves that file byte-identical. It is the case §11 row 12 does not reach —
// row 12's spec has no state directory at all, and every spec this release's
// ordering does not run on first has one.
//
// Both halves of the round are asserted, because either could write. Emit is
// where the state directory is created when absent (§7.3), and record is where
// state is loaded, so record is the half with a state value in hand to save.
//
// Each half carries its own control — the artifact that half writes — so a
// ground command that failed, or did nothing at all, cannot be read as
// leaving state.json alone.
func TestAGroundRoundLeavesAnExistingStateJSONByteIdentical(t *testing.T) {
	t.Parallel()
	dir := writeGroundFixture(t)
	before := seedReviewStateWithAFutureKey(t, dir)
	statePath := groundStatePath(dir, "state.json")

	groundEmit(t, dir)
	require.FileExists(t, groundStatePath(dir, "floor-ground-round-1.txt"),
		"control: the emission must have run, or the comparison below is about a command that did nothing")
	afterEmit, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(afterEmit),
		"an emission writes its snapshot and its floor and leaves state.json byte-identical")

	rows := writeGroundRows(t, dir, groundRecordRow(1))
	_, stderr, code := runTP(t, dir, "ground", "spec.md", "--record", rows)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	require.FileExists(t, groundStatePath(dir, "ground-round-1.ndjson"),
		"control: the round must have been recorded, or the comparison below is about a command that did nothing")
	afterRecord, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.Equal(t, string(before), string(afterRecord),
		"--record adds the round file and leaves state.json byte-identical (§7.3, §11 row 21b)")

	// The control, and it runs LAST because it destroys the seed. A review
	// record is a writer of this same file, so it establishes that the seeded
	// key is droppable — which is what makes the two assertions above say
	// something. Without it a state.json no writer could ever have changed
	// would pass them.
	_, stderr, code = recordRound(t, dir, "\n")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	written, err := os.ReadFile(statePath)
	require.NoError(t, err)
	assert.NotEqual(t, string(before), string(written),
		"control: a writer of state.json changes its bytes")
	var reloaded map[string]any
	require.NoError(t, json.Unmarshal(written, &reloaded))
	assert.NotContains(t, reloaded, groundFutureStateKey,
		"control: SaveReviewState marshals a typed struct, so a key it does not know is gone after one write (§7.3)")
}
