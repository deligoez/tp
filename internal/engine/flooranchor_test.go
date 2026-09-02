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

