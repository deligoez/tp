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
// carries — every unit §2.1 produces, the ones the arms cut included (§2.2) —
// so a case can assert the anchor a row actually ships with rather than only
// the function that supplies it.
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
	assert.Equal(t, []string{"u1 §7.2", "u2 §7.2", "u3 §8"}, anchorsOfIndexRows(text),
		"the cut header row is announced under §7.2 too, and still occupies u1")
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

// TestAHeadingInsideAFencedBlockIsNotAnAnchor pins that the anchor scan reads
// fences the way step 1 does. Over `spec/*.md` four numbered headings sit inside
// fenced blocks — `spec/0.1.0.md:921` and `spec/0.16.0-review-orchestration.md`
// carry them as sample output — and a fence-blind scan re-anchors every unit
// after one of them to a section the document does not have.
func TestAHeadingInsideAFencedBlockIsNotAnAnchor(t *testing.T) {
	text := "## 1. One\n\n```\n## 9. Fenced\n```\n\nA holds 1.\n\n" +
		"## 2. Two\n\n~~~\n### 9.9 Also fenced\n~~~\n\nB holds 2.\n"

	require.Equal(t, []string{"A holds 1.", "B holds 2."}, FloorUnits(text),
		"the fenced lines must be gone, or this asserts nothing about anchoring")

	assert.Equal(t, []string{"§1", "§2"}, anchorsOfEveryUnit(text))
}

// TestAnH1IsNotASectionAndAnH4Is pins the heading levels the anchor rule reads,
// which §7.3 does not state and which two plausible bounds get wrong in
// opposite directions. Both are in this repository:
//
//   - Five specs title themselves with a version — `spec/0.19.0-agent-friction.md`
//     opens `# 0.19.0 — Agent Friction Reduction` — so a rule that reads level 1
//     anchors that document's whole preamble to `§0.19.0` instead of `§0`.
//   - `spec/0.36.0.md` carries `#### 4.2.1 A recognised name the round does not
//     emit`, so the prototype's level-2-to-3 rule reports its units as `§4.2`.
//
// One fixture, both mutants: each bound gets exactly one of the two entries
// wrong.
func TestAnH1IsNotASectionAndAnH4Is(t *testing.T) {
	text := "# 0.19.0 — Agent Friction Reduction\n\n" +
		"The preamble measured 3 things.\n\n" +
		"#### 4.2.1 A recognised name\n\n" +
		"The rule fired 2 times.\n"

	require.Len(t, FloorUnits(text), 2)

	assert.Equal(t, []string{"§0", "§4.2.1"}, anchorsOfEveryUnit(text))
}

// TestTheAnchorIndexIsOverEveryUnitIncludingTheOnesTheArmsCut is the numbering
// half of §7.2's `unit_id`, asserted at the one arrangement that separates the
// two readings: a cut unit sits BETWEEN two floor units and across a heading
// boundary, so numbering the anchors over the floor alone shifts the last unit's
// anchor into the previous section — or off the end of the list.
func TestTheAnchorIndexIsOverEveryUnitIncludingTheOnesTheArmsCut(t *testing.T) {
	text := "## 1. One\n\nThe run measured 3 things.\n\n" +
		"Plain prose sits alone.\n\n" +
		"## 2. Two\n\nIt `ran` twice.\n"

	units := FloorUnits(text)
	require.Len(t, units, 3)
	require.True(t, inFloor(units[0]), "u1 must be in the floor")
	require.False(t, inFloor(units[1]), "u2 must be CUT, or the arrangement is not the one under test")
	require.True(t, inFloor(units[2]), "u3 must be in the floor")

	assert.Equal(t, []string{"§1", "§1", "§2"}, anchorsOfEveryUnit(text))
	assert.Equal(t, []string{"u1 §1", "u2 §1", "u3 §2"}, anchorsOfIndexRows(text),
		"the cut u2 is announced under the section it sits in, not dropped")
}

// TestABlockThatStraddlesADroppedHeadingKeepsTheSectionItOpensIn states the one
// case §7.3's rule does not settle, so it is a decision rather than an accident.
//
// §2.1 step 2 splits blocks on blank lines and a dropped heading is not a
// boundary, so a heading with prose flush against it on both sides leaves one
// block whose lines span two sections — and a unit of that block can straddle
// the heading, which has no per-unit answer at all. The anchor is resolved at
// the line the block opens on, so the whole block stays in the section it
// started in.
//
// Measured over this repository's 54 specs: six headings are not preceded by a
// blank line and NONE is followed by prose, so the corpus has zero instances;
// the fixture is constructed, and `require` on the block count is what makes it
// the case under test rather than two ordinary blocks.
func TestABlockThatStraddlesADroppedHeadingKeepsTheSectionItOpensIn(t *testing.T) {
	text := "## 1. One\n1 before the heading.\n## 2. Two\n2 after the heading.\n"

	blocks := floorBlocks(text)
	require.Len(t, blocks, 1, "the heading must not have split the block")
	require.Len(t, blocks[0].Lines, 2)

	assert.Equal(t, []string{"§1", "§1"}, anchorsOfEveryUnit(text))
}

// TestAnIndexOutsideTheTextsUnitsHasNoAnchor pins the failure direction of a
// mismatched pairing — an anchorer built from one text asked for another's unit
// — as empty rather than `§0`. `§0` is a legal anchor and a structurally valid
// row; empty is neither, and §7.2 requires the field, so the row is rejected at
// record instead of carrying a plausible wrong section.
func TestAnIndexOutsideTheTextsUnitsHasNoAnchor(t *testing.T) {
	text := "## 1. One\n\nA holds 1.\n"
	anchorOf := FloorAnchorOf(text)

	require.Len(t, FloorUnits(text), 1)
	assert.Equal(t, "§1", anchorOf(1))

	assert.Empty(t, anchorOf(0), "unit ids are 1-based")
	assert.Empty(t, anchorOf(2), "this text has no second unit")
	assert.Empty(t, anchorOf(-1))
}

// TestSection11Row21OnThisReleasesOwnSpec is row 21 itself: the row names this
// document's own preamble blockquote, so the fixture is the file.
//
// It reads one named file rather than a glob, and every assertion is structural
// — that the first block is the preamble quote, that its units anchor to `§0`,
// that no table row in the file anchors there, and that §7.2's table reports
// §7.2 — so editing the preamble's wording cannot make it red while editing the
// anchor rule can.
func TestSection11Row21OnThisReleasesOwnSpec(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "spec", "1.0.0.md"))
	require.NoError(t, err)
	text := string(raw)

	blocks := floorBlocks(text)
	require.NotEmpty(t, blocks)
	require.True(t, strings.HasPrefix(strings.TrimSpace(blocks[0].Lines[0]), ">"),
		"the document's first block must be the preamble blockquote row 21 names")

	anchorOf := FloorAnchorOf(text)
	rows := FloorIndexRows(text, anchorOf)
	require.NotEmpty(t, rows)
	assert.Equal(t, "u1", rows[0].ID, "the preamble's first unit is in the floor, so it counts toward coverage")
	assert.Equal(t, "§0", rows[0].Anchor)

	unit, preamble, tablesAtZero, tablesIn72 := 0, 0, 0, 0
	for i, b := range blocks {
		for range floorUnitsFromBlock(b) {
			unit++
			anchor := anchorOf(unit)
			if i == 0 {
				preamble++
				assert.Equal(t, "§0", anchor, "every unit of the preamble blockquote")
			}
			if !b.IsTableRow {
				continue
			}
			switch anchor {
			case "§0":
				tablesAtZero++
			case "§7.2":
				tablesIn72++
			}
		}
	}

	require.Equal(t, len(FloorUnits(text)), unit, "the walk must cover every unit")
	assert.Positive(t, preamble)
	assert.Zero(t, tablesAtZero, "the prototype anchored every table row to §0 while its §0 test passed")
	assert.Positive(t, tablesIn72, "§7.2's field table must report §7.2")
}
