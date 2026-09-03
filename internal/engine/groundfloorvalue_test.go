package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundFloorValueFloor is the two-unit index these tests grade rows against.
// Only u1 is compared: u2 is a unit the arms cut, and a cut row carries no hash
// for a payload row to have to match.
func groundFloorValueFloor() []FloorIndexRow {
	return []FloorIndexRow{
		{ID: "u1", Anchor: "§1", TextSHA: "0123456789ab", Ordinal: 1, Bytes: 24},
		{ID: "u2", Anchor: "§2", TextSHA: "", Ordinal: 0},
	}
}

// TestARowMustCarryTheFloorHashOfTheUnitItNames is the `text_sha` half of §7.3's
// value check: a row whose `unit_id` names a floor row must carry that row's
// hash. The `ordinal` half is groundfloorjoin_test.go's, which also carries the
// input that separates the two.
//
// The rule exists because the alternative is a SILENT loss rather than a wrong
// number. §8's carry joins on `(text_sha, ordinal)` while coverage joins on
// `unit_id`, so a row with a valid id and a fabricated hash is counted as
// dispositioned in this round and fails to carry into the next, with nothing in
// either record saying why. Measured on a built fixture before this check
// existed: the row recorded at exit 0, `--status` reported 2 of 2 with
// `off_floor: 0`, `--check` exited 0, and round 2 on the unchanged spec carried
// 1 of 2.
//
// The refusal is atomic, which is asserted rather than assumed: the round file
// must not exist afterwards, because a round holding a row nobody can join is
// worse than no round.
func TestARowMustCarryTheFloorHashOfTheUnitItNames(t *testing.T) {
	specPath := groundEmittedDir(t, 1)
	payload := groundRecordPayload(groundWireRow(t, groundClaimRow(), map[string]any{
		"unit_id":  "u1",
		"anchor":   "§1",
		"text_sha": "deadbeef0000",
	}, nil))

	_, _, err := RecordGroundRound(specPath, 1, payload, groundFloorValueFloor())

	require.Error(t, err, "a row naming u1 while carrying a hash u1 does not have is refused")
	var lineErr *GroundLineError
	require.ErrorAs(t, err, &lineErr, "the refusal names the line, as every other row refusal does")
	assert.Equal(t, 1, lineErr.Line)
	var rowErr *GroundRowError
	require.ErrorAs(t, err, &rowErr, "and the cell, so a caller reads the field rather than the sentence")
	assert.Equal(t, "text_sha", rowErr.Field)

	_, statErr := os.Stat(filepath.Join(ReviewStateDir(specPath), "ground-round-1.ndjson"))
	assert.True(t, os.IsNotExist(statErr), "a refused round writes no round file")
}

// TestTheFloorJoinKeyIsTheOnlyThingCompared fences §7.3's exception to exactly
// what it says, in all three directions the rule has.
//
// **The name used to say "the floor hash check is the only value compared",
// and §7.3 has since been widened**: the check reads the whole of §8's join
// key — a row whose `unit_id` names a floor row must carry that row's
// `text_sha` AND that row's `ordinal`. "The only VALUE" was true of a check
// that compared one cell and is false of one that compares two, and a test name
// is a claim like any other. What survived the widening is the other half of the
// sentence — that nothing BEYOND the join key is compared — which is what these
// three arms fence and what the name now says.
//
// The first arm is the control that stops the test above being a check on
// nothing: the same row with the floor's own key records. The second and third
// are the halves §7.3 keeps shape-checked — a row whose `unit_id` matches no
// floor row stays valid, because that is `off_floor` and §8's fact to report,
// and a row on a unit the arms CUT is not compared either, since a cut floor row
// carries no key for it to match. Without them a stricter implementation —
// refusing every unmatched id, or demanding a cut unit's empty hash — passes the
// test above while breaking the two readings §7.3 names.
func TestTheFloorJoinKeyIsTheOnlyThingCompared(t *testing.T) {
	cases := []struct {
		name string
		over map[string]any
	}{
		{"the floor's own hash on the unit it names", map[string]any{
			"unit_id": "u1", "anchor": "§1", "text_sha": "0123456789ab"}},
		{"an id no floor row carries", map[string]any{
			"unit_id": "u9", "anchor": "§9", "text_sha": "ffffffffffff"}},
		{"a unit the arms cut", map[string]any{
			"unit_id": "u2", "anchor": "§2", "text_sha": "ffffffffffff"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := groundEmittedDir(t, 1)
			payload := groundRecordPayload(groundWireRow(t, groundClaimRow(), tc.over, nil))

			rows, _, err := RecordGroundRound(specPath, 1, payload, groundFloorValueFloor())
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.FileExists(t, filepath.Join(ReviewStateDir(specPath), "ground-round-1.ndjson"))
		})
	}
}

// TestAReaderAddedRowIsNotComparedToTheFloor is the fourth reading, kept apart
// because it fails for a different reason: a `unit_id` of null names no floor
// row at all, and §7.2 has such a row supply its own `text_sha` over text tp
// never emitted. An implementation joining on the hash instead of on the id
// would refuse it.
func TestAReaderAddedRowIsNotComparedToTheFloor(t *testing.T) {
	specPath := groundEmittedDir(t, 1)
	payload := groundRecordPayload(groundWireRow(t, groundClaimRow(), map[string]any{
		"unit_id":  nil,
		"text_sha": "ffffffffffff",
	}, nil))

	rows, _, err := RecordGroundRound(specPath, 1, payload, groundFloorValueFloor())
	require.NoError(t, err, "a claim the floor did not carry is recorded, not refused")
	require.Len(t, rows, 1)
	assert.Nil(t, rows[0].UnitID)
}
