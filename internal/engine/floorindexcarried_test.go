package engine

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// carriedFixtureRows is the arrangement both tests below need: a floor unit, a
// unit all three arms cut, and a second floor unit — the cut one BETWEEN them,
// which is the only order in which a marker leaking onto a cut row can be seen
// to occupy a position rather than to be appended at the end.
//
// That the middle unit is cut is required rather than assumed, and so is the
// numbering: the ids are what the carry set is keyed on, so a fixture whose ids
// were not u1/u2/u3 would be asserting against a map nobody could have written.
func carriedFixtureRows(t *testing.T) []FloorIndexRow {
	t.Helper()

	const kept1 = "The first claim carries `a span`."
	const dropped = "This sentence tells you nothing."
	const kept2 = "The third claim carries 3 items."
	text := kept1 + "\n\n" + dropped + "\n\n" + kept2 + "\n"

	require.Equal(t, []string{kept1, dropped, kept2}, FloorUnits(text),
		"the fixture is three units in this order")
	require.True(t, inFloor(kept1), "the first unit must be in the floor")
	require.False(t, inFloor(dropped), "the MIDDLE unit must be one the arms cut")
	require.True(t, inFloor(kept2), "the third unit must be in the floor")

	rows := FloorIndexRows(text, func(n int) string { return fmt.Sprintf("§%d", n) })
	require.Len(t, rows, 3, "every unit gets a row, the cut one included")
	require.Equal(t, []string{"u1", "u2", "u3"},
		[]string{rows[0].ID, rows[1].ID, rows[2].ID}, "the ids the carry set is keyed on")
	return rows
}

// TestACarriedUnitIsMarkedInPlaceAndTheIndexStillListsEveryUnit is §8's "a round
// emits an index that marks each unit tp can already carry ... the index still
// lists every unit".
//
// Four assertions, each naming the mutant it alone kills:
//
//   - the marker is a SUFFIX to the five fields — a renderer putting it in place
//     of the hash, or of the length, leaves a row that still says `(carried)`;
//   - a cut unit named in the carry set is still rendered `(cut)` and nothing
//     more. It is named here deliberately: a cut unit owes no disposition, so it
//     can inherit none, and a renderer that marks whatever id it is handed
//     produces the one row §2.2 forbids twice over;
//   - an uncarried floor unit is unmarked — a renderer marking every floor row
//     passes the first assertion;
//   - the summary is unchanged, because a carried unit is still a floor unit.
//     A renderer counting it as cut, or dropping it from both counts, reports a
//     denominator §8's coverage would then disagree with.
func TestACarriedUnitIsMarkedInPlaceAndTheIndexStillListsEveryUnit(t *testing.T) {
	rows := carriedFixtureRows(t)

	index := FormatFloorIndexCarried("7ced1edb", rows, map[string]bool{"u1": true, "u2": true})
	lines := strings.Split(strings.TrimSuffix(index, "\n"), "\n")
	require.Len(t, lines, 5, "the commit line, one line per unit, and the summary")

	assert.Equal(t, "# commit 7ced1edb", lines[0])
	assert.Equal(t, rows[0].String()+" "+floorIndexCarriedMarker, lines[1],
		"a carried unit keeps its five fields and takes the marker after them")
	assert.Equal(t, "u2 §2 (cut)", lines[2],
		"a cut unit inherits nothing, however a caller names it: it owes nothing to inherit")
	assert.Equal(t, rows[2].String(), lines[3],
		"a floor unit the carry set does not name is unmarked")
	assert.Equal(t, "# 2 in floor, 1 cut", lines[4],
		"a carried unit is still a floor unit, and still in §8's denominator")
}

// TestAnIndexWithNothingCarriedIsTheOneEveryOtherReaderGets pins the other
// direction: the marked rendering and §2.2's are the same bytes when there is
// nothing to mark.
//
// It matters because the two renderings are emitted to two places — the prompt
// takes the marked one and `floor-ground-round-N.txt` takes §2.2's — and round 1
// has no preceding round to carry from. A renderer that changed the row shape
// unconditionally would leave round 1's prompt disagreeing with the floor
// `--record` grades it against, which is exactly what nothing downstream checks.
//
// The second case is the one an empty map does not reach: a carry set naming an
// id the index does not hold marks nothing rather than marking the row at that
// position.
func TestAnIndexWithNothingCarriedIsTheOneEveryOtherReaderGets(t *testing.T) {
	rows := carriedFixtureRows(t)
	plain := FormatFloorIndex("7ced1edb", rows)

	assert.Equal(t, plain, FormatFloorIndexCarried("7ced1edb", rows, nil),
		"no carry set is §2.2's index unchanged")
	assert.Equal(t, plain, FormatFloorIndexCarried("7ced1edb", rows, map[string]bool{}),
		"and so is an empty one")
	assert.Equal(t, plain, FormatFloorIndexCarried("7ced1edb", rows, map[string]bool{"u9": true}),
		"an id no row in this index carries marks no row in it")
}
