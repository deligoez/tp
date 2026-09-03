package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundFloorOf builds an emitted index over ids in which every row is a floor
// unit. A row is a floor unit exactly when it carries a hash (§2.2 — the
// ABSENCE of the hash is the cut), so each id gets the sha256 of its own name:
// non-empty for every id, and distinct across them.
func groundFloorOf(ids ...string) []FloorIndexRow {
	rows := make([]FloorIndexRow, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, FloorIndexRow{
			ID:      id,
			Anchor:  "§1",
			TextSHA: FloorTextSHA(id),
			Ordinal: 1,
			Bytes:   len(id),
		})
	}
	return rows
}

// groundDisposition is a recorded row naming a unit: §7.2's `unit_id` carries
// an id, so the row is the disposition of the unit the index emitted under it.
func groundDisposition(id string, v GroundVerdict) GroundRow {
	unit := id
	return GroundRow{UnitID: &unit, Anchor: "§1", TextSHA: FloorTextSHA(id), Verdict: v}
}

// groundReaderAdded is §8's reader-added row: a claim the floor missed, whose
// `unit_id` is null because tp did not emit it (Non-Goal 5).
func groundReaderAdded(v GroundVerdict) GroundRow {
	return GroundRow{Anchor: "§1", TextSHA: FloorTextSHA("added"), Verdict: v}
}

// groundFloorWithACutUnit derives a real index whose MIDDLE unit the arms cut.
//
// It is derived from text rather than stated as literals so that the
// denominator is measured against what §2.1 and §2.2 actually emit, and the cut
// unit sits in the middle because that is the only arrangement in which the row
// can be seen to occupy an id: with it first or last, the two readings of the
// denominator agree on which ids are floor units.
//
// That the middle unit is the cut one is a require, not an assumption of the
// fixture's author.
func groundFloorWithACutUnit(t *testing.T) []FloorIndexRow {
	t.Helper()

	const kept1 = "The first claim carries `a span`."
	const dropped = "This sentence tells you nothing about the world."
	const kept2 = "The third claim carries 3 items."
	text := kept1 + "\n\n" + dropped + "\n\n" + kept2 + "\n"

	require.Equal(t, []string{kept1, dropped, kept2}, FloorUnits(text),
		"the fixture is three units in this order")
	require.False(t, inFloor(dropped), "the MIDDLE unit must be one the arms cut")

	rows := FloorIndexRows(text, func(int) string { return "§1" })
	require.Len(t, rows, 3, "a cut unit is announced, so every unit has an index row")
	require.Equal(t, "u2", rows[1].ID, "the cut unit occupies u2")
	require.Empty(t, rows[1].TextSHA, "and it carries no hash, which is what makes it cut")
	require.NotEmpty(t, rows[0].TextSHA, "u1 must be a floor unit")
	require.NotEmpty(t, rows[2].TextSHA, "u3 must be a floor unit")
	return rows
}

// TestGroundCoverageIsDispositionsOverTheEmittedFloor is §8's ratio: the
// denominator is what tp emitted, and the numerator is how many of those units
// a recorded row decided.
func TestGroundCoverageIsDispositionsOverTheEmittedFloor(t *testing.T) {
	floor := groundFloorOf("u1", "u2", "u3", "u4")
	rows := []GroundRow{
		groundDisposition("u2", VerdictPass),
		groundDisposition("u4", VerdictFail),
	}

	assert.Equal(t, GroundCoverage{Emitted: 4, Dispositioned: 2}, GroundCoverageOf(floor, rows))
}

// TestAReaderAddedRowMovesNeitherSideAndIsCountedApart is §11 row 15.
//
// The fixture is sized so that row 15's named mutant — counting the
// reader-added rows in the numerator — reads exactly 100%: four emitted units,
// two of them dispositioned, and two added rows. That is the defect stated as
// an input rather than as a sentence, and it is asserted below rather than
// eyeballed, because it is the property that makes this fixture discriminating.
//
// The added rows are interleaved with the dispositions so that a reader keyed
// on a prefix of the record fails too.
func TestAReaderAddedRowMovesNeitherSideAndIsCountedApart(t *testing.T) {
	floor := groundFloorOf("u1", "u2", "u3", "u4")
	rows := []GroundRow{
		groundDisposition("u2", VerdictPass),
		groundReaderAdded(VerdictFail),
		groundDisposition("u4", VerdictFail),
		groundReaderAdded(VerdictQuestion),
	}

	added := 0
	for _, r := range rows {
		if r.UnitID == nil {
			added++
		}
	}
	require.Equal(t, 2, added, "the fixture's added rows are the ones carrying a null unit_id")
	require.Equal(t, len(floor)-2, added,
		"and there are exactly as many of them as there are undispositioned floor units, "+
			"so counting them in the numerator reads exactly 100%")

	assert.Equal(t, GroundCoverage{Emitted: 4, Dispositioned: 2, ReaderAdded: 2},
		GroundCoverageOf(floor, rows))
}

