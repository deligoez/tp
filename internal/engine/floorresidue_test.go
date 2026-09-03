package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSection11Row18eInterleavedResidue is §11 row 18e: the shape row 18b's
// fixture cannot reach.
//
// 18b's block is one contiguous list, so its first line opens a list and step 3
// strips every item's marker. Here the items are separated by INDENTED
// CONTINUATION PARAGRAPHS, so each later item's ordinal falls inside a block
// that opens with prose, no marker is stripped, and step 4 emits the ordinal on
// its own. §2.1 names that residue and declines to repair it, because the only
// repair reaching it — stripping every line — is the one that deletes text.
//
// So this test asserts the four bare markers ARE emitted. Asserting their
// absence is 18b's claim and is false of this shape: §2.1's own list is written
// this way, and a row claiming the fix is total would be refuted by the document
// it ships in.
//
// The control is the same five items written contiguously, where no bare marker
// survives. Without it the first half alone cannot tell §2.1's rule from an
// implementation that strips nothing at all.
func TestSection11Row18eInterleavedResidue(t *testing.T) {
	const interleaved = "1. Alpha holds 1.\n" +
		"\n" +
		"   Indented continuation prose for item 1.\n" +
		"2. Beta holds 2.\n" +
		"\n" +
		"   Indented continuation prose for item 2.\n" +
		"3. Gamma holds 3.\n" +
		"\n" +
		"   Indented continuation prose for item 3.\n" +
		"4. Delta holds 4.\n" +
		"\n" +
		"   Indented continuation prose for item 4.\n" +
		"5. Epsilon holds 5."

	rows := FloorUnitRows(interleaved)
	texts := make([]string, 0, len(rows))
	for _, r := range rows {
		texts = append(texts, r.Text)
	}

	// Stated in full rather than counted: a count cannot separate this set from
	// a different set of fragments the same size.
	require.Equal(t, []string{
		"Alpha holds 1.",
		"Indented continuation prose for item 1.", "2.", "Beta holds 2.",
		"Indented continuation prose for item 2.", "3.", "Gamma holds 3.",
		"Indented continuation prose for item 3.", "4.", "Delta holds 4.",
		"Indented continuation prose for item 4.", "5.", "Epsilon holds 5.",
	}, texts)

	// The row's own words as a property over whatever came back. These come from
	// FloorUnitRows, which drops the units the arms cut, so each bare marker
	// here is a FLOOR unit and owes a disposition (§8).
	bare := make([]string, 0, 4)
	for _, u := range texts {
		if bareFloorMarkerRe.MatchString(u) {
			bare = append(bare, u)
		}
	}
	assert.Equal(t, []string{"2.", "3.", "4.", "5."}, bare,
		"every item after the first is emitted as a bare marker of its own")

	const contiguous = "1. Alpha holds 1.\n" +
		"2. Beta holds 2.\n" +
		"3. Gamma holds 3.\n" +
		"4. Delta holds 4.\n" +
		"5. Epsilon holds 5."
	for _, u := range FloorUnits(contiguous) {
		assert.NotRegexp(t, bareFloorMarkerRe, u,
			"the residue is the interleaving, not the items")
	}
}
