package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundAdvisorySpec writes a spec in a fresh directory and returns its path.
// The text is never read by the advisory — §7.3 has it grade the EMITTED floor
// and never the spec as it now stands — so the file exists only to give
// ReviewStateDir a base to derive the state directory from.
func groundAdvisorySpec(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))
	return specPath
}

// emitGroundFloor writes what an emission writes for a round: the rendered
// index. It goes through FormatFloorIndex rather than a literal, so the fixture
// is the file tp itself writes — commit line, rows and summary line — and not a
// test author's idea of one.
func emitGroundFloor(t *testing.T, specPath string, round int, rows []FloorIndexRow) {
	t.Helper()
	require.NoError(t, os.MkdirAll(ReviewStateDir(specPath), 0o755))
	require.NoError(t, os.WriteFile(GroundFloorPath(specPath, round),
		[]byte(FormatFloorIndex("c0ffee123456", rows)), 0o600))
}

// recordGroundDispositions writes a round file holding one row per index row
// handed in, copying that row's own anchor, hash and ordinal so the record is
// joined to the floor the way a real one is.
//
// Every row is NOT-A-CLAIM, on which §7.2 makes kind and tier optional. What a
// row SAYS is deliberately irrelevant here: §8 counts units DECIDED, and every
// one of §3's six verdicts is a disposition.
func recordGroundDispositions(t *testing.T, specPath string, round int, rows ...FloorIndexRow) {
	t.Helper()
	var b strings.Builder
	for _, r := range rows {
		fmt.Fprintf(&b, `{"unit_id":%q,"anchor":%q,"text_sha":%q,"ordinal":%d,"verdict":"NOT-A-CLAIM"}`+"\n",
			r.ID, r.Anchor, r.TextSHA, r.Ordinal)
	}
	require.NoError(t, os.MkdirAll(ReviewStateDir(specPath), 0o755))
	require.NoError(t, os.WriteFile(GroundRoundPath(specPath, round), []byte(b.String()), 0o600))

	// The fixture must be a record tp would accept, or a test built on it says
	// nothing about a real round.
	data, err := os.ReadFile(GroundRoundPath(specPath, round))
	require.NoError(t, err)
	_, err = ParseGroundRows(data)
	require.NoError(t, err, "the fixture must be a round §7.2 accepts")
}

// TestASpecWithNoGroundRoundHasNoAdvisory: §9 makes the key absent when no
// ground round exists, so there is no round number to name and no floor to
// count against. This is every spec before its first `tp ground`.
func TestASpecWithNoGroundRoundHasNoAdvisory(t *testing.T) {
	assert.Nil(t, LatestGroundAdvisory(groundAdvisorySpec(t)),
		"no emission, so nothing to say")
}

// TestAFullyDispositionedRoundHasNoAdvisory: §9's key is absent when every
// floor unit is dispositioned, on divergence's precedent that a permanent
// zero-valued key is a key every reader learns to skip.
func TestAFullyDispositionedRoundHasNoAdvisory(t *testing.T) {
	specPath := groundAdvisorySpec(t)
	floor := groundFloorWithACutUnit(t)
	emitGroundFloor(t, specPath, 1, floor)
	recordGroundDispositions(t, specPath, 1, floor[0], floor[2])

	assert.Nil(t, LatestGroundAdvisory(specPath),
		"every EMITTED unit is dispositioned — the cut one owes nothing")
}

// TestAnUndispositionedUnitIsReportedAgainstTheEmittedFloor is §9's advisory:
// the round, the count of floor units without a disposition, and the floor's
// size.
//
// The floor's size is 2 while the index carries 3 rows, and the middle row is
// the cut one — required, not assumed, by groundFloorWithACutUnit. So a
// denominator counting every index row rather than every EMITTED one is visible
// here rather than only in a comment.
func TestAnUndispositionedUnitIsReportedAgainstTheEmittedFloor(t *testing.T) {
	specPath := groundAdvisorySpec(t)
	floor := groundFloorWithACutUnit(t)
	require.Len(t, floor, 3, "the index carries a row for every unit, cut ones included")
	emitGroundFloor(t, specPath, 1, floor)
	recordGroundDispositions(t, specPath, 1, floor[0])

	assert.Equal(t, &GroundAdvisory{Round: 1, Undispositioned: 1, FloorSize: 2},
		LatestGroundAdvisory(specPath))
}

// TestAnEmittedRoundNobodyRecordedIsWhollyUndispositioned: a round exists from
// its emission, and between the emission and the record NOTHING in it has been
// decided. That is the state §9's advisory most needs to name, so it is the
// reading of "the latest ground round" this takes: a round the operator emitted
// and has not come back to is not silence, it is 100% ungrounded.
//
// Reading the latest ROUND FILE instead would report the previous round's
// complete coverage here and drop the key altogether, which tells review that a
// spec whose current round nobody has touched is fully grounded.
func TestAnEmittedRoundNobodyRecordedIsWhollyUndispositioned(t *testing.T) {
	specPath := groundAdvisorySpec(t)
	floor := groundFloorWithACutUnit(t)
	emitGroundFloor(t, specPath, 1, floor)

	assert.Equal(t, &GroundAdvisory{Round: 1, Undispositioned: 2, FloorSize: 2},
		LatestGroundAdvisory(specPath))
}

// TestTheAdvisoryIsTheLatestRoundAndNotTheFirst: round 1 is complete and round
// 2 has been emitted and not recorded, which is the only arrangement in which
// the two readings give different answers — the first round says nothing, the
// latest says everything is open.
func TestTheAdvisoryIsTheLatestRoundAndNotTheFirst(t *testing.T) {
	specPath := groundAdvisorySpec(t)
	floor := groundFloorWithACutUnit(t)
	emitGroundFloor(t, specPath, 1, floor)
	recordGroundDispositions(t, specPath, 1, floor[0], floor[2])
	require.Nil(t, LatestGroundAdvisory(specPath), "round 1 alone is complete")

	emitGroundFloor(t, specPath, 2, floor)

	assert.Equal(t, &GroundAdvisory{Round: 2, Undispositioned: 2, FloorSize: 2},
		LatestGroundAdvisory(specPath))
}

// TestTwoRowsForOneUnitLeaveTheOtherUndispositioned: the count of open units is
// derived from the floor — of each emitted unit, did a row decide it — never
// from the number of rows recorded. Subtracting the row count instead reports
// zero here and drops the advisory, so a round that graded one unit twice would
// read as a round that graded both.
func TestTwoRowsForOneUnitLeaveTheOtherUndispositioned(t *testing.T) {
	specPath := groundAdvisorySpec(t)
	floor := groundFloorWithACutUnit(t)
	emitGroundFloor(t, specPath, 1, floor)
	recordGroundDispositions(t, specPath, 1, floor[0], floor[0])

	assert.Equal(t, &GroundAdvisory{Round: 1, Undispositioned: 1, FloorSize: 2},
		LatestGroundAdvisory(specPath))
}

// TestAFloorNoLongerReadableSaysNothingRatherThanSomethingWrong: §9's advisory
// never changes `tp review`'s exit code, so every failure it can meet is a
// reason to stay silent rather than to report a count derived from a file it
// could not read.
func TestAFloorNoLongerReadableSaysNothingRatherThanSomethingWrong(t *testing.T) {
	specPath := groundAdvisorySpec(t)
	emitGroundFloor(t, specPath, 1, groundFloorWithACutUnit(t))
	require.NotNil(t, LatestGroundAdvisory(specPath), "the fixture must have something to say first")

	require.NoError(t, os.WriteFile(GroundFloorPath(specPath, 1), []byte("# commit c0ffee\nu1 §1 half\n"), 0o600))
	assert.Nil(t, LatestGroundAdvisory(specPath), "an index that does not parse is not a count")
}
