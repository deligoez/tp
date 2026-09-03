package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundFloorJoinFloor is a three-unit index in which u1 and u2 share a hash.
//
// The shared hash is the point of the fixture, not an incidental property of it,
// so it is asserted rather than read off the literal: without it the
// mis-attribution arm below tests nothing, because a row naming u2 with u1's
// ordinal could only ever be caught by the hash comparison that already ships.
func groundFloorJoinFloor(t *testing.T) []FloorIndexRow {
	t.Helper()
	floor := []FloorIndexRow{
		{ID: "u1", Anchor: "§1", TextSHA: "0123456789ab", Ordinal: 1, Bytes: 24},
		{ID: "u2", Anchor: "§1", TextSHA: "0123456789ab", Ordinal: 2, Bytes: 24},
		{ID: "u3", Anchor: "§2", TextSHA: "abcdef012345", Ordinal: 1, Bytes: 31},
	}
	require.Equal(t, floor[0].TextSHA, floor[1].TextSHA,
		"u1 and u2 must hash alike, or the mis-attribution arm is caught by the hash alone")
	require.NotEqual(t, floor[0].Ordinal, floor[1].Ordinal,
		"and must differ in ordinal, or nothing separates them at all")
	return floor
}

// TestARowMustCarryTheWholeJoinKeyOfTheUnitItNames is §7.3's value check over
// the whole of §8's join key — `(text_sha, ordinal)` — and not over its first
// half alone.
//
// §7.3's own reason for the check is that "§8's carry joins on
// `(text_sha, ordinal)`", so a comparison reading only `text_sha` leaves the
// second half of that key unchecked and reproduces, on `ordinal`, the exact
// failure the check exists to remove. Both arms were built and run against the
// narrow comparison before it was widened, and both recorded at exit 0:
//
//   - drop: a row on u1 carrying u1's hash and `ordinal: 9` recorded, `--status`
//     reported 3 dispositioned of 4, and round 2 on the unchanged spec carried
//     2 — u1's disposition was silently lost.
//   - mis-attribution: on a spec with two identical sentences, a row naming u2
//     with `ordinal: 1` recorded, and round 2 marked **u1** carried — a unit
//     nobody dispositioned — while the prompt told the next unit not to decide
//     it.
//
// The second is the worse of the two and is reachable in the corpus: 9 of 5,396
// floor units across this repository's 54 specs carry `ordinal > 1`.
//
// **Neither input alone is enough**, which is §11 row 18c's own clause and the
// reason there are three refused arms rather than two. A check comparing the
// pair ONLY where the hash repeats in the floor refuses both duplicate-hash
// rows and is not caught by either; the third arm — the right hash and a wrong
// ordinal on `u3`, whose hash nothing shares — is what kills it, and it was run
// as a mutant to confirm that it is the only arm that does.
func TestARowMustCarryTheWholeJoinKeyOfTheUnitItNames(t *testing.T) {
	cases := []struct {
		name  string
		over  map[string]any
		field string
	}{
		{
			name:  "the hash of the unit it names and an ordinal that unit does not have",
			over:  map[string]any{"unit_id": "u1", "anchor": "§1", "text_sha": "0123456789ab", "ordinal": 9},
			field: "ordinal",
		},
		{
			name:  "a sibling's ordinal, on a hash the two units share",
			over:  map[string]any{"unit_id": "u2", "anchor": "§1", "text_sha": "0123456789ab", "ordinal": 1},
			field: "ordinal",
		},
		{
			name:  "the right hash and a wrong ordinal, on a unit whose hash is unique",
			over:  map[string]any{"unit_id": "u3", "anchor": "§2", "text_sha": "abcdef012345", "ordinal": 2},
			field: "ordinal",
		},
		{
			name:  "a hash the unit it names does not have",
			over:  map[string]any{"unit_id": "u3", "anchor": "§2", "text_sha": "deadbeef0000", "ordinal": 1},
			field: "text_sha",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := groundEmittedDir(t, 1)
			payload := groundRecordPayload(groundWireRow(t, groundClaimRow(), tc.over, nil))

			_, _, err := RecordGroundRound(specPath, 1, payload, groundFloorJoinFloor(t))

			require.Error(t, err)
			var lineErr *GroundLineError
			require.ErrorAs(t, err, &lineErr, "the refusal names the line, as every other row refusal does")
			assert.Equal(t, 1, lineErr.Line)
			var rowErr *GroundRowError
			require.ErrorAs(t, err, &rowErr, "and the cell that is wrong, so a caller reads the field")
			assert.Equal(t, tc.field, rowErr.Field)

			_, statErr := os.Stat(filepath.Join(ReviewStateDir(specPath), "ground-round-1.ndjson"))
			assert.True(t, statErr != nil && os.IsNotExist(statErr), "a refused round writes no round file")
		})
	}
}

// TestTheWholeJoinKeyCheckAcceptsTheRowsItMust is the control the test above
// needs to be a check on anything: the same three units, each named by a row
// carrying the index's own `(text_sha, ordinal)`, all record.
//
// The u2 arm is the one that matters. It carries `ordinal: 2` on a hash it
// shares with u1, so an implementation that widened the comparison by demanding
// `ordinal == 1` — or by comparing the ordinal against the first floor row with
// that hash — reddens here while passing every arm above.
func TestTheWholeJoinKeyCheckAcceptsTheRowsItMust(t *testing.T) {
	cases := []struct {
		name string
		over map[string]any
	}{
		{"the first of two units sharing a hash", map[string]any{
			"unit_id": "u1", "anchor": "§1", "text_sha": "0123456789ab", "ordinal": 1}},
		{"the second of two units sharing a hash", map[string]any{
			"unit_id": "u2", "anchor": "§1", "text_sha": "0123456789ab", "ordinal": 2}},
		{"a unit whose hash is its own", map[string]any{
			"unit_id": "u3", "anchor": "§2", "text_sha": "abcdef012345", "ordinal": 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := groundEmittedDir(t, 1)
			payload := groundRecordPayload(groundWireRow(t, groundClaimRow(), tc.over, nil))

			rows, _, err := RecordGroundRound(specPath, 1, payload, groundFloorJoinFloor(t))
			require.NoError(t, err)
			require.Len(t, rows, 1)
			assert.FileExists(t, filepath.Join(ReviewStateDir(specPath), "ground-round-1.ndjson"))
		})
	}
}

// TestTheJoinKeyCheckStillSkipsTheRowsItAlwaysSkipped fences the widening to the
// units the floor carries. §7.3's two exempt readings are unchanged by it: an id
// the floor does not carry is `off_floor`, §8's fact to report at `--status`
// rather than a parse failure, and a unit the arms CUT carries no key at all for
// a row to be compared against.
//
// Both rows below carry an ordinal the floor could not match even if it tried,
// which is what makes them a fence on the widening rather than a restatement of
// the shipped test: an implementation comparing the ordinal before deciding
// whether the id is on the floor reddens here.
func TestTheJoinKeyCheckStillSkipsTheRowsItAlwaysSkipped(t *testing.T) {
	floor := append(groundFloorJoinFloor(t), FloorIndexRow{ID: "u4", Anchor: "§2", TextSHA: "", Ordinal: 0})

	cases := []struct {
		name string
		over map[string]any
	}{
		{"an id no floor row carries", map[string]any{
			"unit_id": "u9", "anchor": "§9", "text_sha": "ffffffffffff", "ordinal": 7}},
		{"a unit the arms cut", map[string]any{
			"unit_id": "u4", "anchor": "§2", "text_sha": "ffffffffffff", "ordinal": 7}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			specPath := groundEmittedDir(t, 1)
			payload := groundRecordPayload(groundWireRow(t, groundClaimRow(), tc.over, nil))

			rows, _, err := RecordGroundRound(specPath, 1, payload, floor)
			require.NoError(t, err)
			require.Len(t, rows, 1)
		})
	}
}
