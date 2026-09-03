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

// TestARowMustCarryTheFloorHashOfTheUnitItNames is §7.3's one value check: a row
// whose `unit_id` names a floor row must carry that row's `text_sha`.
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
