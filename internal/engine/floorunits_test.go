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
