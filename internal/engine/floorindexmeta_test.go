package engine

import (
	"fmt"
	"strconv"
	"strings"
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

	assert.NotContains(t, FormatFloorIndex("7ced1edb", rows), "zqxjvwkfgb",
		"the cut unit is announced, never carried")
}

// TestTheIndexNamesTheCommitOnItsFirstLine is §2.2: "The index carries the
// commit it was derived from, on its first line."
//
// The reason is a measurement rather than a preference. A grounding round takes
// hours — the second end-to-end run took two and a half — and the spec moved
// three commits underneath the first one, so five dispositioned units no longer
// existed by the time it finished, and the run found that by accident. §7.3
// pins the round to a snapshot; this line says which snapshot the index IS, so
// a stale one is recognisable without a diff.
//
// The three assertions do not subsume each other, and each names the mutant it
// alone kills:
//
//   - the line is FIRST — an implementation appending it after the rows passes
//     the other two;
//   - the sha is the caller's, asserted on two different commits — a hard-coded
//     or truncated line sits in first position and passes that assertion;
//   - a caller with no commit still gets the line, and it says so — omitting
//     the line when there is nothing to name passes both of the others. It has
//     to be there unconditionally, or a reader cannot tell "derived at an
//     unknown commit" from "derived at the commit on line 1" without first
//     parsing the rest.
func TestTheIndexNamesTheCommitOnItsFirstLine(t *testing.T) {
	const text = "The claim carries 3 items.\n"
	rows := FloorIndexRows(text, func(int) string { return "§1" })
	require.Len(t, rows, 1, "one unit, so a first line that is not the commit line is a row")

	firstLine := func(index string) string { return strings.SplitN(index, "\n", 2)[0] }

	assert.Equal(t, "# commit 7ced1edb", firstLine(FormatFloorIndex("7ced1edb", rows)))
	assert.Equal(t, "# commit c2b9b555", firstLine(FormatFloorIndex("c2b9b555", rows)),
		"the commit is carried, not decorated: a different one is a different line")
	assert.Equal(t, "# commit unknown", firstLine(FormatFloorIndex("", rows)),
		"a caller with no commit to name still gets the line, and it says so")

	assert.Contains(t, FormatFloorIndex("7ced1edb", rows), "\n"+rows[0].String()+"\n",
		"the line is added ahead of the rows, not in place of one")
}
