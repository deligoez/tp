package engine

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestACutUnitEmitsItsIdAndAnchorWithNoHashAndNoText is §2.2's "The index also
// names what the arms cut": a unit the three arms rejected appears as
// `unit_id anchor (cut)` — no hash, no obligation, no text.
//
// The cut unit sits BETWEEN two floor units, because that is the only
// arrangement in which its row can be seen to occupy an id: with the cut unit
// last, an implementation that drops it and one that announces it agree on
// every id before it. That the middle unit is cut is a require, not an
// assumption of the fixture's author.
//
// The cut unit carries a rare token — not hex, and no word of the two floor
// units — so any of its text reaching the index would turn up in the search at
// the end. That assertion is a regression guard and not an independent
// discrimination, and the difference is worth stating rather than leaving to a
// reader: no mutant of the renderer can fail it while `FloorIndexRow` has no
// field to put text in, which is the point of the type. It fails the day
// someone adds one.
//
// The absence of the hash IS the cut, and that reading is only sound because
// no floor unit can have an empty `text_sha`. That is required here rather
// than believed: `FloorTextSHA` returns twelve hex characters for every input,
// the empty string included.
func TestACutUnitEmitsItsIdAndAnchorWithNoHashAndNoText(t *testing.T) {
	const kept1 = "The first claim carries `a span`."
	const dropped = "This sentence tells you nothing zqxjvwkfgb."
	const kept2 = "The third claim carries 3 items."
	text := kept1 + "\n\n" + dropped + "\n\n" + kept2 + "\n"

	require.Equal(t, []string{kept1, dropped, kept2}, FloorUnits(text),
		"the fixture is three units in this order")
	require.True(t, inFloor(kept1), "the first unit must be in the floor")
	require.False(t, inFloor(dropped), "the MIDDLE unit must be one the arms cut")
	require.True(t, inFloor(kept2), "the third unit must be in the floor")
	require.NotEmpty(t, FloorTextSHA(""),
		"an empty text_sha marks a cut row, so no unit may hash to one")

	rows := FloorIndexRows(text, func(n int) string { return fmt.Sprintf("§%d", n) })
	require.Len(t, rows, 3, "every unit gets a row: the cut one is announced, not dropped")

	assert.Equal(t, FloorIndexRow{ID: "u2", Anchor: "§2"}, rows[1],
		"a cut row carries an id and an anchor and nothing else")
	assert.Equal(t, "u2 §2 (cut)", rows[1].String())

	// The floor rows are unchanged by the cut row beside them: the five fields
	// are still five fields, and they still belong to the units they name.
	assert.Equal(t, "u1 §1 "+FloorTextSHA(kept1)+" #1 "+strconv.Itoa(len(kept1))+"B",
		rows[0].String())
	assert.Equal(t, "u3 §3 "+FloorTextSHA(kept2)+" #1 "+strconv.Itoa(len(kept2))+"B",
		rows[2].String())

	assert.NotContains(t, FormatFloorIndex(rows), "zqxjvwkfgb",
		"the cut unit is announced, never carried")
}
