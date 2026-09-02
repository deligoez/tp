package engine

import (
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
