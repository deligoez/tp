package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// anchorsOfEveryUnit renders the anchor of every unit §2.1 produces, in
// document order, so a case states the whole expected anchoring rather than
// spot-checking one unit.
//
// It asks for `i+1` because §7.2's `unit_id` is 1-based over every unit, the
// ones the arms cut included, and that is the index `FloorIndexRows` passes.
func anchorsOfEveryUnit(text string) []string {
	anchorOf := FloorAnchorOf(text)
	out := make([]string, 0)
	for i := range FloorUnits(text) {
		out = append(out, anchorOf(i+1))
	}
	return out
}

// anchorsOfIndexRows renders `<unit_id> <anchor>` for the rows the index
// carries, so a case can assert the anchor a row actually ships with rather
// than only the function that supplies it.
func anchorsOfIndexRows(text string) []string {
	out := make([]string, 0)
	for _, r := range FloorIndexRows(text, FloorAnchorOf(text)) {
		out = append(out, r.ID+" "+r.Anchor)
	}
	return out
}

// TestSection11Row21APreHeadingUnitAnchorsToSectionZero is §7.3's "a unit
// before the first `§` heading anchors to `§0`", on the shape §11 row 21 names:
// a preamble blockquote under a title that is not a numbered heading.
//
// The row's mutant is "leave the anchor empty", so the assertion states the
// whole anchor list — an empty first entry fails, and so does an implementation
// that anchors the preamble to the section that follows it.
func TestSection11Row21APreHeadingUnitAnchorsToSectionZero(t *testing.T) {
	text := "# tp v1.0.0 — `tp ground`\n\n" +
		"> This file was written from 3 pilot runs.\n" +
		"> It measured 7 defects.\n\n" +
		"## 1. Overview\n\n" +
		"No tp command checks 2 things.\n"

	require.Equal(t, []string{
		"This file was written from 3 pilot runs.",
		"It measured 7 defects.",
		"No tp command checks 2 things.",
	}, FloorUnits(text), "the fixture's segmentation, so the anchors below are per unit")

	assert.Equal(t, []string{"§0", "§0", "§1"}, anchorsOfEveryUnit(text))
	// §11 row 21 asks for two things: the anchor, and that the unit counts
	// toward coverage — which it only does if it reaches the index at all.
	assert.Equal(t, []string{"u1 §0", "u2 §0", "u3 §1"}, anchorsOfIndexRows(text))
}

// TestATableRowAnchorsToItsOwnSectionNotToSectionZero is the defect
// `scripts/floor-prototype.py` records against itself: it located a unit by
// searching the file for its first words, a table row's block began with a
// sentinel that matched no line, and every table row in the document anchored
// to §0 — while the test asserting the §0 case passed.
//
// The header row is cut by the arms and the data row is not, so this also fixes
// that a cut unit still occupies its `unit_id`.
func TestATableRowAnchorsToItsOwnSectionNotToSectionZero(t *testing.T) {
	text := "## 7. The record\n\n### 7.2 The row\n\n" +
		"| field | type |\n|---|---|\n| `unit_id` | string |\n\n" +
		"## 8. Convergence\n\nGrounding converges when 100% of units carry a disposition.\n"

	units := FloorUnits(text)
	require.Equal(t, []string{
		"field — type",
		"`unit_id` — string",
		"Grounding converges when 100% of units carry a disposition.",
	}, units)
	require.False(t, inFloor(units[0]), "the header row must be cut, or the row ids below shift")

	assert.Equal(t, []string{"§7.2", "§7.2", "§8"}, anchorsOfEveryUnit(text))
	assert.Equal(t, []string{"u2 §7.2", "u3 §8"}, anchorsOfIndexRows(text))
}

// TestTheAnchorIsTheLastNumberedHeadingAtOrAboveTheUnit is §7.3's rule stated
// as a sequence, so the three readings that are not it each fail: the next
// heading below, the first heading of the document, and the last heading of any
// kind.
//
// `## Motivation` is the third: it is a heading and it is not a `§n(.n)*` one,
// so the unit under it stays in §1.1. The corpus has that shape —
// `spec/0.20.0-review-state.md` opens with it.
func TestTheAnchorIsTheLastNumberedHeadingAtOrAboveTheUnit(t *testing.T) {
	text := "## 1. One\n\nA holds 1.\n\n" +
		"### 1.1 Sub\n\nB holds 2.\n\n" +
		"## Motivation\n\nC holds 3.\n\n" +
		"## 2. Two\n\nD holds 4.\n"

	require.Len(t, FloorUnits(text), 4)

	assert.Equal(t, []string{"§1", "§1.1", "§1.1", "§2"}, anchorsOfEveryUnit(text))
}

