package engine

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderBlocks flattens floorBlocks' output into one comparable string per
// block, so a case states the whole expected blocking rather than a count. A
// literal "\n" separates a block's lines (blocks never contain one) and a
// "TABLE:" prefix marks a table data row, so a case that gets the right lines
// in the wrong kind of block still fails.
func renderBlocks(blocks []floorBlock) []string {
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		s := strings.Join(b.Lines, "\\n")
		if b.IsTableRow {
			s = "TABLE:" + s
		}
		out = append(out, s)
	}
	return out
}

// TestFloorUnitsTakesTheSpecTextNotAPath is §2.1's "an implementation ships the
// derivation as a function taking the spec's text". The fixture writes a real
// file and passes its path, so the assertion discriminates: an implementation
// that opened the argument would return the file's two blocks, and one that
// treats the argument as text returns the path itself as a one-line document.
// Nothing in this repository is reachable from here — the file is in TempDir.
func TestFloorUnitsTakesTheSpecTextNotAPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spec.md")
	require.NoError(t, os.WriteFile(path,
		[]byte("# Heading\n\nThe run measured 3 things.\n\nIt `ran` twice.\n"), 0o600))
	// The fixture is only a test of the signature if the path resolves: a
	// FloorUnits that opened it must succeed and return something different.
	require.FileExists(t, path)

	units := FloorUnits(path)

	require.Len(t, units, 1, "the argument is a document of one line, not a file to open")
	assert.Equal(t, path, units[0])
}

// TestFloorBlocksDropsWhatStep1Drops is §2.1 step 1's drop list: fenced blocks,
// ATX headings, horizontal rules and table separator rows. Each case names one
// class and keeps a prose line on either side, so a rule that dropped too much
// fails alongside one that dropped too little.
func TestFloorBlocksDropsWhatStep1Drops(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "backtick fence and everything inside it",
			text: "Before.\n\n```go\n# not a heading\n| not | a table |\nfmt.Println(1)\n```\n\nAfter.\n",
			want: []string{"Before.", "After."},
		},
		{
			name: "tilde fence",
			text: "Before.\n\n~~~\ndropped\n~~~\n\nAfter.\n",
			want: []string{"Before.", "After."},
		},
		{
			name: "an unterminated fence swallows the rest of the document",
			text: "Before.\n\n```\nstill code\n\nalso code\n",
			want: []string{"Before."},
		},
		{
			name: "ATX headings at every level, indented or not",
			text: "# One\n\nBefore.\n\n## Two\n\n   ### Three\n\nAfter.\n",
			want: []string{"Before.", "After."},
		},
		{
			name: "horizontal rules of each marker",
			text: "Before.\n\n---\n\n***\n\n___\n\n  -----  \n\nAfter.\n",
			want: []string{"Before.", "After."},
		},
		{
			name: "two markers are not a rule, and a mixed run is not one either",
			text: "--\n\n-*-\n",
			want: []string{"--", "-*-"},
		},
		{
			name: "table separator rows, aligned or not",
			text: "| a | b |\n|---|---|\n| 1 | 2 |\n\n| c | d |\n| :--- | ---: |\n| 3 | 4 |\n",
			want: []string{
				"TABLE:| a | b |", "TABLE:| 1 | 2 |",
				"TABLE:| c | d |", "TABLE:| 3 | 4 |",
			},
		},
		{
			name: "a dropped line is dropped, not turned into a boundary",
			text: "Before.\n## Heading\nAfter.\n",
			want: []string{"Before.\\nAfter."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, renderBlocks(floorBlocks(tt.text)))
		})
	}
}

// TestFloorBlocksSplitsOnBlankLines is §2.1 step 2. A whitespace-only line is a
// blank line, runs of them produce no empty block, and neither do leading or
// trailing ones.
func TestFloorBlocksSplitsOnBlankLines(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "two paragraphs, each keeping its own lines",
			text: "one\ntwo\n\nthree\nfour\n",
			want: []string{"one\\ntwo", "three\\nfour"},
		},
		{
			name: "a run of blank lines yields no empty block",
			text: "one\n\n\n\ntwo\n",
			want: []string{"one", "two"},
		},
		{
			name: "a whitespace-only line is a boundary",
			text: "one\n   \ntwo\n",
			want: []string{"one", "two"},
		},
		{
			name: "leading and trailing blanks add nothing",
			text: "\n\none\n\n\n",
			want: []string{"one"},
		},
		{
			name: "a document with no blank line is one block",
			text: "one\ntwo\nthree",
			want: []string{"one\\ntwo\\nthree"},
		},
		{
			name: "an empty document has no blocks",
			text: "",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, renderBlocks(floorBlocks(tt.text)))
		})
	}
}

// TestEachTableDataRowIsItsOwnBlock is the second half of §2.1 step 1: a data
// row is one unit, so it must not share a block with the row above it, with the
// prose above it, or with the prose below it — none of which carries a blank
// line in a real table. The rows here are deliberately adjacent, because that is
// the input a blank-line-only blocking rule gets wrong.
func TestEachTableDataRowIsItsOwnBlock(t *testing.T) {
	text := "Prose above.\n| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\nProse below.\n"

	blocks := floorBlocks(text)

	assert.Equal(t, []string{
		"Prose above.",
		"TABLE:| a | b |",
		"TABLE:| 1 | 2 |",
		"TABLE:| 3 | 4 |",
		"Prose below.",
	}, renderBlocks(blocks))

	require.Len(t, blocks, 5)
	for i, b := range blocks {
		if i == 0 || i == 4 {
			assert.False(t, b.IsTableRow, "block %d is prose", i)
			continue
		}
		assert.True(t, b.IsTableRow, "block %d is a table row", i)
		assert.Len(t, b.Lines, 1, "a table row block holds exactly its own line")
	}
}

// TestTableRowsInsideAFenceAreNotTableRows pins the order step 1 applies its
// rules in: the fence is read first, so a pipe line inside a code block is
// dropped with the block rather than promoted to a unit of its own.
func TestTableRowsInsideAFenceAreNotTableRows(t *testing.T) {
	text := "```\n| a | b |\n|---|---|\n| 1 | 2 |\n```\n\nAfter.\n"

	assert.Equal(t, []string{"After."}, renderBlocks(floorBlocks(text)))
}

// TestATableDataRowBecomesOneUnit is §2.1 step 1's second paragraph: "strip the
// outer pipes, join the cells with an em dash, collapse whitespace. It is one
// unit however many full stops its cells hold."
//
// Each case names the rule it is the only witness for, so a port that gets the
// join right and the escape wrong fails on exactly one row.
func TestATableDataRowBecomesOneUnit(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "three cells joined by an em dash, outer pipes gone",
			text: "| a | b | c |\n",
			want: []string{"a — b — c"},
		},
		{
			name: "one unit however many full stops its cells hold",
			text: "| One. | Two. | Three. |\n",
			want: []string{"One. — Two. — Three."},
		},
		{
			name: "an escaped pipe is content, not a separator",
			text: "| `string \\| null` | yes |\n",
			want: []string{"`string | null` — yes"},
		},
		{
			name: "an escaped pipe at end of line is not the closing pipe",
			text: "| a | b \\|\n",
			want: []string{"a — b |"},
		},
		{
			name: "an empty cell contributes nothing",
			text: "| a || b |\n",
			want: []string{"a — b"},
		},
		{
			name: "whitespace inside a cell collapses to one space",
			text: "|  a   b  |  c |\n",
			want: []string{"a b — c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FloorUnits(tt.text))
		})
	}
}

// TestSection11Row18dTable is §11 row 18d on a constructed three-column table:
// one unit per data row with cells joined by an em dash, no unit for the
// separator row, no special case for a header row of bare labels, and a data row
// whose cells hold three full stops is still one unit.
//
// The row's remaining clause — that a header row of bare labels produces no
// FLOOR unit — is "through the arms, with no special case", and the arms are not
// this seam. What is asserted here is the antecedent that makes that clause
// true: the header row goes through the same derivation as a data row and comes
// out as an ordinary unit, which is what leaves the arms something to cut.
// Whether they cut it is `floor-arms`, which depends on this task.
func TestSection11Row18dTable(t *testing.T) {
	text := "| kind | tier | note |\n" +
		"|---|---|---|\n" +
		"| behaviour | run | One. Two. Three. |\n" +
		"| document | read | plain |\n"

	units := FloorUnits(text)

	// Four table lines in, three units out: the separator contributed none.
	assert.Equal(t, []string{
		"kind — tier — note",
		"behaviour — run — One. Two. Three.",
		"document — read — plain",
	}, units)
	assert.Equal(t, 4, strings.Count(text, "\n"), "the fixture is four table lines")
}

// TestABlockquotePrefixIsStrippedFromEveryLine is §2.1 step 3's first clause:
// "strip a leading `> ` from every line". Every case keeps the whole expected
// unit rather than a count, and the last one is the over-stripping guard — a
// `>` that is not at the start of a line is content, not a prefix.
func TestABlockquotePrefixIsStrippedFromEveryLine(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			// The discriminating case: an implementation stripping only the
			// block's first line leaves "> beta" inside the joined unit.
			name: "every line carries the prefix",
			text: "> alpha holds 1\n> beta holds 2.",
			want: "alpha holds 1 beta holds 2.",
		},
		{
			name: "a continuation line without the prefix is untouched",
			text: "> alpha holds 1\nbeta holds 2.",
			want: "alpha holds 1 beta holds 2.",
		},
		{
			name: "the prefix may be indented",
			text: "  > alpha holds 1.",
			want: "alpha holds 1.",
		},
		{
			name: "the space after the marker is optional",
			text: ">alpha holds 1.",
			want: "alpha holds 1.",
		},
		{
			name: "only one marker is stripped from a nested quote",
			text: "> > alpha holds 1.",
			want: "> alpha holds 1.",
		},
		{
			name: "a marker that is not leading is content",
			text: "alpha > beta holds 1.",
			want: "alpha > beta holds 1.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units := FloorUnits(tt.text)
			require.Len(t, units, 1, "the fixture is one block of one sentence")
			assert.Equal(t, tt.want, units[0])
		})
	}
}

// TestBlockLinesJoinWithOneSpaceAndWhitespaceCollapses is §2.1 step 3's second
// clause: "join the block's lines with a single space and collapse whitespace
// runs to one". Collapsing is not cosmetic — `text_sha` (§7.2) is the sha256 of
// exactly this string, so a run of two spaces surviving anywhere is a hash tp
// and a reader compute differently.
//
// The empty-quote-line case is the one real markdown produces: a `>` alone
// canonicalises to nothing, and without the collapse the join leaves a double
// space where that line was.
func TestBlockLinesJoinWithOneSpaceAndWhitespaceCollapses(t *testing.T) {
	tests := []struct {
		name string
		text string
		want string
	}{
		{
			name: "lines join with a single space",
			text: "alpha holds 1\nbeta holds 2.",
			want: "alpha holds 1 beta holds 2.",
		},
		{
			name: "a run inside a line collapses",
			text: "alpha  holds\t\t1.",
			want: "alpha holds 1.",
		},
		{
			name: "a line's own leading and trailing whitespace goes",
			text: "   alpha holds 1   \n\tbeta holds 2.  ",
			want: "alpha holds 1 beta holds 2.",
		},
		{
			name: "an empty quote line leaves no gap",
			text: "> alpha holds 1\n>\n> beta holds 2.",
			want: "alpha holds 1 beta holds 2.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			units := FloorUnits(tt.text)
			require.Len(t, units, 1, "the fixture is one block of one sentence")
			assert.Equal(t, tt.want, units[0])
		})
	}
}

// TestProseSplitsAtATerminatorFollowedByWhitespace is §2.1 step 4: "split the
// joined block at each `.`, `!` or `?` followed by whitespace; the terminator
// stays with the unit on its left".
//
// The second clause is asserted by keeping the whole expected unit, not by
// counting: the two readings of step 4 that the spec's own repair chose between
// give the SAME segmentation and different strings, so a count cannot tell them
// apart and a `text_sha` computed from the wrong one never matches.
//
// The abbreviation case is deliberate. "e.g. 1 thing" splits, because the rule
// is a terminator followed by whitespace and nothing else; that coarseness is
// the rule as written, and pinning it here means a later reader changes it on
// purpose rather than by accident.
func TestProseSplitsAtATerminatorFollowedByWhitespace(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "the terminator stays with the unit on its left",
			text: "Alpha holds 1. Beta holds 2.",
			want: []string{"Alpha holds 1.", "Beta holds 2."},
		},
		{
			name: "the exclamation and question marks split too",
			text: "Alpha holds 1! Beta holds 2? Gamma holds 3.",
			want: []string{"Alpha holds 1!", "Beta holds 2?", "Gamma holds 3."},
		},
		{
			name: "a terminator not followed by whitespace does not split",
			text: "Version 1.2 holds.",
			want: []string{"Version 1.2 holds."},
		},
		{
			name: "the rule is literal, so an abbreviation splits",
			text: "It holds e.g. 1 thing.",
			want: []string{"It holds e.g.", "1 thing."},
		},
		{
			name: "a trailing terminator yields no empty unit",
			text: "Alpha holds 1.",
			want: []string{"Alpha holds 1."},
		},
		{
			name: "a run of terminators splits after the last one",
			text: "Alpha holds 1... Beta holds 2.",
			want: []string{"Alpha holds 1...", "Beta holds 2."},
		},
		{
			name: "the split runs across a line join",
			text: "Alpha holds 1.\nBeta holds 2.",
			want: []string{"Alpha holds 1.", "Beta holds 2."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FloorUnits(tt.text))
		})
	}
}

// TestStep5TrimsAndDropsEmpties asserts §2.1 step 5 at the seam rather than
// through FloorUnits, and says why: step 3 has already collapsed and trimmed
// everything FloorUnits can reach, so no spec text produces a segment needing
// either. Asserting it here is the difference between a rule that holds and a
// rule that is merely unreachable — floorSplitUnits is also what a later step
// would reuse, and it must not hand back a padded or empty unit.
func TestStep5TrimsAndDropsEmpties(t *testing.T) {
	assert.Equal(t, []string{"Alpha holds 1."},
		floorSplitUnits("  Alpha holds 1.  "), "the segment is trimmed")
	assert.Equal(t, []string{"."},
		floorSplitUnits(". "), "the empty tail after the terminator is dropped")
	assert.Empty(t, floorSplitUnits("   "), "a whitespace-only input yields no unit")
}

// TestAListMarkerIsStrippedOnlyWhenTheBlockOpensAList is §2.1 step 3's third
// clause: strip a leading `- `, `* ` or `N. ` from every line, but only when the
// block's first line opens a list.
//
// Both halves matter and they fail in opposite directions. Stripping only the
// first line leaves each later item's marker embedded, and step 4 then emits the
// bare strings `2.` through `9.` as units — the defect §11 row 18b names.
// Stripping every line unconditionally deletes text from hard-wrapped prose,
// which is §11 row 20's. The negative case here is the guard for the second;
// row 20 asserts the consequence that makes it matter.
func TestAListMarkerIsStrippedOnlyWhenTheBlockOpensAList(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{
			name: "a numbered list loses every marker",
			text: "1. Alpha holds 1.\n2. Beta holds 2.\n3. Gamma holds 3.",
			want: []string{"Alpha holds 1.", "Beta holds 2.", "Gamma holds 3."},
		},
		{
			name: "a bulleted list loses every marker",
			text: "- Alpha holds 1.\n- Beta holds 2.",
			want: []string{"Alpha holds 1.", "Beta holds 2."},
		},
		{
			name: "the star and plus bullets are markers too",
			text: "* Alpha holds 1.\n+ Beta holds 2.",
			want: []string{"Alpha holds 1.", "Beta holds 2."},
		},
		{
			name: "an indented marker is still a marker",
			text: "  - Alpha holds 1.\n  - Beta holds 2.",
			want: []string{"Alpha holds 1.", "Beta holds 2."},
		},
		{
			// The gate. The block's first line is prose, so the `2. ` on the
			// second line is content and survives — under an unconditional
			// strip the unit silently loses it, and the two units here become
			// one that reads "…by input of the six built here.".
			//
			// The `2.` then splits the sentence, because step 4 is a terminator
			// followed by whitespace and knows nothing about ordinals. That is
			// the rule as written and it is what the prototype does on §7.1's
			// own paragraph; §11 row 20 calls the result "the same single unit",
			// which no input of this shape can produce.
			name: "a block that does not open a list keeps a later line's marker",
			text: "The rule was refuted by input\n2. of the six built here.",
			want: []string{"The rule was refuted by input 2.", "of the six built here."},
		},
		{
			name: "a marker needs whitespace after it",
			text: "1.Alpha holds 1.",
			want: []string{"1.Alpha holds 1."},
		},
		{
			name: "a marker that is not leading is content",
			text: "Alpha - beta holds 1.",
			want: []string{"Alpha - beta holds 1."},
		},
		{
			name: "a quoted list loses both prefixes",
			text: "> - Alpha holds 1.\n> - Beta holds 2.",
			want: []string{"Alpha holds 1.", "Beta holds 2."},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FloorUnits(tt.text))
		})
	}
}

// bareFloorMarkerRe matches a unit that is nothing but a list marker — the
// fragment §11 row 18b exists to keep out of the floor. `2.` and `-` are what
// step 4 emits from a marker the canonicaliser left embedded.
var bareFloorMarkerRe = regexp.MustCompile(`^(?:\d+\.|[-*+])$`)

// TestSection11Row18bListMarkers is §11 row 18b on a constructed block holding a
// nine-item numbered list, a bulleted list and a list whose items carry bold
// markers: every unit is a sentence and none is a bare marker fragment.
//
// The three lists share ONE block, which is the arrangement that exercises the
// gate: the block's first line opens a numbered list, so the strip runs over the
// bullets too, and over the `**1.` lines, which no marker pattern matches and
// which must therefore come through untouched rather than mangled.
//
// The expected units are stated in full rather than counted. A count cannot tell
// the row's mutant from the rule — stripping the first line only yields MORE
// units, not fewer, and every extra one is the defect.
func TestSection11Row18bListMarkers(t *testing.T) {
	text := "1. Alpha holds 1.\n" +
		"2. Beta holds 2.\n" +
		"3. Gamma holds 3.\n" +
		"4. Delta holds 4.\n" +
		"5. Epsilon holds 5.\n" +
		"6. Zeta holds 6.\n" +
		"7. Eta holds 7.\n" +
		"8. Theta holds 8.\n" +
		"9. Iota holds 9.\n" +
		"- Kappa holds 10.\n" +
		"- Lambda holds 11.\n" +
		"**1.** Mu holds 12.\n" +
		"**2.** Nu holds 13."

	units := FloorUnits(text)

	assert.Equal(t, []string{
		"Alpha holds 1.", "Beta holds 2.", "Gamma holds 3.", "Delta holds 4.",
		"Epsilon holds 5.", "Zeta holds 6.", "Eta holds 7.", "Theta holds 8.",
		"Iota holds 9.", "Kappa holds 10.", "Lambda holds 11.",
		"**1.** Mu holds 12.", "**2.** Nu holds 13.",
	}, units)

	// The row's own words, asserted as a property over whatever came back, so
	// this half still fails if the expectation above is ever loosened.
	for _, u := range units {
		assert.NotRegexp(t, bareFloorMarkerRe, u, "no unit is a bare marker fragment")
		assert.Contains(t, u, " ", "every unit is a sentence, not a fragment")
	}
}

// floorTextSHA is §7.2's `text_sha` — the first twelve lowercase hex characters
// of the sha256 of exactly the unit string, UTF-8 encoded. It is computed here
// rather than called, because the shipped function is a later task; §11 row 20
// asserts over hashes, and a test that compared unit strings alone would be
// asserting something weaker than the row.
func floorTextSHA(unit string) string {
	sum := sha256.Sum256([]byte(unit))
	return hex.EncodeToString(sum[:])[:12]
}

// TestSection11Row20CanonicalForm is §11 row 20: reflow stability asserted on
// the case that breaks it rather than on a benign one.
//
// The fixture is §7.1's own paragraph, whose continuation line begins `14. `.
// A paragraph with no ordinal at a line start passes under all three readings of
// step 3 and so proves nothing; this one separates them. Under an unconditional
// marker strip the wrapped copy loses its `14. ` and every hash after that point
// differs from the one-line copy's, which is the row's first named mutant.
//
// The row says the two "yield the same single unit". They do not, and no input
// of this shape can: `14. ` is a terminator followed by whitespace, so step 4
// splits there, and `scripts/floor-prototype.py` segments §7.1's paragraph into
// two units in both copies. What the row is actually about — that the
// segmentation and every `text_sha` are decided by the text and not by where the
// author's line breaks fell — is what is asserted here.
func TestSection11Row20CanonicalForm(t *testing.T) {
	const wrapped = "Exit codes follow tp's existing table with no additions, " +
		"and each is pinned to a named input by test\n" +
		"14. The mapping is read from `exitStateError` rather than invented."
	oneLine := strings.ReplaceAll(wrapped, "\n", " ")

	// The fixture's discriminating property, asserted rather than assumed: the
	// continuation line must actually begin with an ordinal.
	lines := strings.Split(wrapped, "\n")
	require.Len(t, lines, 2)
	require.True(t, strings.HasPrefix(lines[1], "14. "),
		"the fixture is the breaking case only if its continuation line opens with an ordinal")
	require.NotContains(t, oneLine, "\n")

	want := []string{
		"Exit codes follow tp's existing table with no additions, " +
			"and each is pinned to a named input by test 14.",
		"The mapping is read from `exitStateError` rather than invented.",
	}
	assert.Equal(t, want, FloorUnits(wrapped), "the ordinal survives the reflow")
	assert.Equal(t, want, FloorUnits(oneLine))

	wrappedSHAs := make([]string, 0, len(want))
	for _, u := range FloorUnits(wrapped) {
		wrappedSHAs = append(wrappedSHAs, floorTextSHA(u))
	}
	oneLineSHAs := make([]string, 0, len(want))
	for _, u := range FloorUnits(oneLine) {
		oneLineSHAs = append(oneLineSHAs, floorTextSHA(u))
	}
	assert.Equal(t, oneLineSHAs, wrappedSHAs, "a reflow does not move a text_sha")

	// The row's second clause, which is what kills its other named mutant: a
	// genuine numbered list still loses every marker.
	list := "1. Alpha holds 1.\n2. Beta holds 2.\n3. Gamma holds 3.\n" +
		"4. Delta holds 4.\n5. Epsilon holds 5.\n6. Zeta holds 6.\n" +
		"7. Eta holds 7.\n8. Theta holds 8.\n9. Iota holds 9."
	assert.Equal(t, []string{
		"Alpha holds 1.", "Beta holds 2.", "Gamma holds 3.", "Delta holds 4.",
		"Epsilon holds 5.", "Zeta holds 6.", "Eta holds 7.", "Theta holds 8.",
		"Iota holds 9.",
	}, FloorUnits(list), "a genuine nine-item list loses every marker")
}

// TestSection11Row1TheThreeArmsDecideFloorMembership is §11 row 1: a unit
// holding only a digit, only a backtick span, and only a listed verb each land
// in the floor, and a unit holding none does not.
//
// Every case states all three arms rather than only the one it is about, so a
// unit that reaches the floor through the WRONG arm fails here instead of
// passing as a membership test — which is what "separately assertable" has to
// mean if the row's mutant is to die. The verb-only case is the row's own
// example, and it is the unit that mutant loses.
func TestSection11Row1TheThreeArmsDecideFloorMembership(t *testing.T) {
	tests := []struct {
		name              string
		unit              string
		digit, span, verb bool
		want              bool
	}{
		{"digit only", "The gate runs 4 steps and stops at the first red one.",
			true, false, false, true},
		{"backtick span only", "The flag is spelled `--record` in every prompt.",
			false, true, false, true},
		{"listed verb only", "the round is recorded before the snapshot is written",
			false, false, true, true},
		{"no arm at all", "the round precedes the snapshot in every prompt the command emits",
			false, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// The table may not contradict itself: membership is the
			// disjunction, so a row claiming otherwise is a broken fixture
			// rather than a finding about the code.
			require.Equal(t, tc.digit || tc.span || tc.verb, tc.want)

			assert.Equal(t, tc.digit, floorHasDigit(tc.unit), "digit arm")
			assert.Equal(t, tc.span, floorHasCodeSpan(tc.unit), "identifier arm")
			assert.Equal(t, tc.verb, floorHasMeasurementVerb(tc.unit), "verb arm")
			assert.Equal(t, tc.want, inFloor(tc.unit), "floor membership")
		})
	}

	// The two single-arm cases carry their arm ALONE, checked without the
	// predicates under test: a fixture whose isolation is asserted only by the
	// code it is meant to discriminate proves nothing about that code.
	require.False(t, strings.ContainsAny(tests[2].unit, "0123456789`"),
		"the verb-only unit must hold neither a digit nor a backtick")
	require.False(t, strings.ContainsAny(tests[0].unit, "`"),
		"the digit-only unit must hold no backtick")
	require.False(t, strings.ContainsAny(tests[3].unit, "0123456789`"),
		"the no-arm unit must hold neither a digit nor a backtick")
}

// TestSection11Row2ABareCodeSpanIsInTheFloor is §11 row 2: a unit whose only
// signal is a bare `internal/cli/audit.go` — no line number, no other digit, no
// listed verb — is in the floor, via the identifier arm.
//
// The path is the row's own and it is chosen for what it LACKS. §2.1 argues
// there is no fourth path-citation arm because a path in this family is written
// inside a code span; a path carrying `:N` would reach the floor through the
// digit arm whatever the identifier arm did, so it cannot discriminate. This one
// carries no digit at all, which makes it the input under which a fourth arm
// would have been load-bearing — and the input the row's mutant loses.
func TestSection11Row2ABareCodeSpanIsInTheFloor(t *testing.T) {
	const path = "internal/cli/audit.go"
	// The fixture's discriminating property, asserted without the predicates
	// under test: this path must carry no line number and no other digit.
	require.False(t, strings.ContainsAny(path, "0123456789"),
		"row 2's citation is bare — a digit anywhere in it would let the digit arm carry the unit")

	tests := []struct{ name, unit string }{
		{"the span alone", "`" + path + "`"},
		{"the span inside a sentence", "The fence lives in `" + path + "` and nowhere else."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.False(t, strings.ContainsAny(tc.unit, "0123456789"),
				"the unit as a whole must carry no digit either")

			assert.False(t, floorHasDigit(tc.unit), "digit arm")
			assert.False(t, floorHasMeasurementVerb(tc.unit), "verb arm")
			assert.True(t, floorHasCodeSpan(tc.unit), "identifier arm")
			assert.True(t, inFloor(tc.unit),
				"the identifier arm alone must put this unit in the floor")
		})
	}
}
