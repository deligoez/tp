package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAUnitsLineIsIdHashAndTheWholeCanonicalUnit is the first half of §11
// row 4b: `--units` prints one line per floor unit whose text is the WHOLE
// canonical unit.
//
// The fixture's unit is 83 bytes, and that number is required rather than
// eyeballed, because row 4b's named mutant is "print a truncation" and §2.2's
// truncation is a 60-byte head: a fixture at or under 60 bytes is identical
// under the mutant and the rule, so the row would assert something no input of
// its shape can refute.
//
// The digest is computed OUTSIDE Go — python3's hashlib and `shasum -a 256`
// agreeing on the same twelve characters — so the line is stated whole rather
// than re-derived by the implementation under test.
func TestAUnitsLineIsIdHashAndTheWholeCanonicalUnit(t *testing.T) {
	const unit = "The index says what a disposition is owed for and `--units` says what each unit is."
	const digest = "5c6b8ae4e8c6"

	require.Greater(t, len(unit), 60,
		"the fixture must exceed §2.2's 60-byte head, or a truncation cannot be seen")
	require.Equal(t, 83, len(unit), "83 UTF-8 bytes")

	rows := FloorUnitRows(unit + "\n")
	require.Len(t, rows, 1)
	assert.Equal(t, FloorUnitRow{ID: "u1", Text: unit}, rows[0])
	assert.Equal(t, "u1\t"+digest+"\t"+unit, rows[0].String())
}

// TestTheHashOnAUnitsLineIsTheSHAOfTheTextThatFollowsIt is the second half of
// §11 row 4b, and it is a different assertion from the first rather than a
// restatement of it. The two mutants row 4b's "print a truncation" admits are
// separated exactly here, and both were built and run:
//
//   - truncate only the printed text, leaving the hash over the whole unit —
//     this test goes red, and so does the whole-unit test above;
//   - truncate the text AND hash the same prefix — the line is then internally
//     consistent and THIS test stays green; only the whole-unit assertion above
//     catches it.
//
// So neither test subsumes the other. The digest here is computed from the
// line's own third field, so what is asserted is the relation between two
// printed fields and not the value of either.
func TestTheHashOnAUnitsLineIsTheSHAOfTheTextThatFollowsIt(t *testing.T) {
	const text = "The gate ran 3 times.\n\n" +
		"A cut sentence about nothing in particular.\n\n" +
		"| the field | 2 bytes |\n\n" +
		"The panel is `emitted` once per round and the count was verified.\n"

	rows := FloorUnitRows(text)
	require.Len(t, rows, 3, "three floor units: prose, a table data row, prose")
	longest := 0
	for _, r := range rows {
		longest = max(longest, len(r.Text))
	}
	require.Greater(t, longest, 60,
		"one unit must exceed §2.2's 60-byte head, or the self-consistent "+
			"truncation this test is stated to survive would not be reached")

	for _, r := range rows {
		parts := strings.SplitN(r.String(), "\t", 3)
		require.Len(t, parts, 3, "a line is id, hash and text")

		sum := sha256.Sum256([]byte(parts[2]))
		assert.Equal(t, hex.EncodeToString(sum[:])[:12], parts[1],
			"the hash on the line is the sha256 of the text that follows it")
	}
}

// TestUnitsRowsNumberOverEveryUnitAndCarryNoCutOne pins the id `--units` prints
// to the id the index prints. The two artifacts are read side by side and join
// on `unit_id`, so a numbering over the floor alone here would name a different
// unit there.
//
// The cut unit sits BETWEEN two floor units, which is the only arrangement the
// two numberings disagree on: with the cut unit first or last, both readings
// return the same ids. That the middle unit is cut is a require, not an
// assumption of the fixture's author.
func TestUnitsRowsNumberOverEveryUnitAndCarryNoCutOne(t *testing.T) {
	const kept1 = "The first claim carries `a span`."
	const dropped = "This sentence tells you nothing about the world."
	const kept2 = "The third claim carries 3 items."
	text := kept1 + "\n\n" + dropped + "\n\n" + kept2 + "\n"

	require.Equal(t, []string{kept1, dropped, kept2}, FloorUnits(text),
		"the fixture is three units in this order")
	require.False(t, inFloor(dropped), "the MIDDLE unit must be one the arms cut")

	rows := FloorUnitRows(text)
	require.Len(t, rows, 2, "a cut unit owes no disposition, so it gets no line")
	assert.Equal(t, "u1", rows[0].ID)
	assert.Equal(t, "u3", rows[1].ID, "the cut unit consumed u2")
	assert.Equal(t, kept1, rows[0].Text)
	assert.Equal(t, kept2, rows[1].Text)

	indexIDs := make([]string, 0, 2)
	for _, r := range FloorIndexRows(text, func(int) string { return "§1" }) {
		indexIDs = append(indexIDs, r.ID)
	}
	assert.Equal(t, []string{"u1", "u3"}, indexIDs,
		"and they are the ids the index prints for the same text")
}

// TestFormatFloorUnitsIsOneTerminatedLinePerFloorUnit is §2.2's "one line per
// floor unit" as a property of the whole rendering rather than of one row.
//
// The fixture's first unit is hard-wrapped across two source lines, which is
// what makes the line count discriminating: a unit that carried its source
// newlines would render as two lines while the row count stayed at two. Both
// halves are required — that the source wraps, and that the unit does not.
func TestFormatFloorUnitsIsOneTerminatedLinePerFloorUnit(t *testing.T) {
	const text = "The round is recorded 3 times\nbefore the snapshot is written.\n\n" +
		"A second claim carries `a span`.\n"
	require.Contains(t, text, "3 times\nbefore",
		"the fixture's first unit must wrap across source lines")

	rows := FloorUnitRows(text)
	require.Len(t, rows, 2)
	require.Equal(t, "The round is recorded 3 times before the snapshot is written.", rows[0].Text,
		"the wrap is joined with a single space, so the unit holds no newline")

	out := FormatFloorUnits(rows)
	require.True(t, strings.HasSuffix(out, "\n"), "every line is terminated")
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	assert.Len(t, lines, len(rows), "one line per floor unit")
	assert.Equal(t, rows[0].String(), lines[0])
	assert.Equal(t, rows[1].String(), lines[1])
}
