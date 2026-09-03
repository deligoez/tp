package engine

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

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
		// §2.1 step 1's outer-pipe half: a separator row must begin AND end
		// with a pipe. The three rows below are the three inputs §2.1 names as
		// kept, each kept differently — and none of them was asserted anywhere
		// until this was written, so the shipped `floorTableSepRe` requiring
		// both pipes was correct and unguarded.
		//
		// **Two mutants, and only one of them reaches all three — measured.**
		// Relaxing `floorTableSepRe` to `^\s*[\s:|-]+$` kills the first case
		// alone: the separator check runs INSIDE the `floorTableRowRe` branch,
		// so a line that does not begin with a pipe never reaches it, and the
		// other two are decided by `floorTableRowRe`'s leading pipe and by
		// `isFloorHorizontalRule`'s three-marker minimum instead. §2.1's own
		// looser reading — drop such a line wherever it sits — is the mutant
		// that kills all three, and it is the one §11 row 18g names. So the
		// three cases are not three views of one rule; naming them as such is
		// what made the paragraph read as if the outer pipes decided all three.
		{
			name: "a separator missing its trailing pipe stays a table data row",
			text: "| a | b |\n|--- |---\n| 1 | 2 |\n",
			want: []string{"TABLE:| a | b |", "TABLE:|--- |---", "TABLE:| 1 | 2 |"},
		},
		{
			name: "a separator with no outer pipes at all leaves its whole table one prose block",
			text: "c | d\n--- | ---\n3 | 4\n",
			want: []string{"c | d\\n--- | ---\\n3 | 4"},
		},
		{
			name: "a lone dash is neither a horizontal rule nor a separator",
			text: "Before.\n\n-\n\nAfter.\n",
			want: []string{"Before.", "-", "After."},
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
		wrappedSHAs = append(wrappedSHAs, FloorTextSHA(u))
	}
	oneLineSHAs := make([]string, 0, len(want))
	for _, u := range FloorUnits(oneLine) {
		oneLineSHAs = append(oneLineSHAs, FloorTextSHA(u))
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
		// Zero is a digit. §2.1's arm is the character range [0-9], and every
		// other fixture here reaches it through 1-9, so narrowing the class to
		// [1-9] passed the whole table — a mutant left in the tree by an
		// interrupted run, and green until this row existed.
		{"the only digit is zero", "A round that dispositioned 0 units is still a round.",
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

	// Each single-arm case carries its arm ALONE, checked without the
	// predicates under test: a fixture whose isolation is asserted only by the
	// code it is meant to discriminate proves nothing about that code.
	//
	// Looked up by NAME, not by index. These were index-based and a row
	// inserted in the middle silently repointed them at the wrong fixtures —
	// the assertions still ran, still passed their own check, and no longer
	// guarded anything.
	byName := make(map[string]string, len(tests))
	for _, tc := range tests {
		byName[tc.name] = tc.unit
	}
	require.Len(t, byName, len(tests), "fixture names must be unique to look up by")
	require.False(t, strings.ContainsAny(byName["listed verb only"], "0123456789`"),
		"the verb-only unit must hold neither a digit nor a backtick")
	require.False(t, strings.ContainsAny(byName["digit only"], "`"),
		"the digit-only unit must hold no backtick")
	require.False(t, strings.ContainsAny(byName["the only digit is zero"], "123456789`"),
		"the zero unit's only digit must be 0, and it must hold no backtick")
	require.False(t, strings.ContainsAny(byName["no arm at all"], "0123456789`"),
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

// TestTheVerbArmIsExactlyTheTwelveListedVerbs pins §2.1's third arm to the
// twelve verbs its table names — no fewer, and no more.
//
// The expectation is restated from the spec rather than read from `floorVerbs`:
// a test that derived its list from the code would agree with that list however
// wrong it was. Both halves are needed and they catch different mutants. The
// equality pins WHICH verbs are listed, including a thirteenth quietly added;
// the loop pins that the arm is actually built from that list, which a pattern
// compiled from some other literal would pass the equality and fail here.
func TestTheVerbArmIsExactlyTheTwelveListedVerbs(t *testing.T) {
	listed := []string{
		"measured", "ran", "counted", "derived", "reproduced", "observed",
		"verified", "asserted", "recorded", "fired", "held", "refuted",
	}
	require.Len(t, listed, 12, "§2.1's table names twelve verbs")
	assert.Equal(t, listed, floorVerbs, "the shipped list is §2.1's, in its order")

	// And the literal above is bound to the spec rather than restated from it.
	// Without this, `listed` and `floorVerbs` and §2.1's table were three
	// copies of one set with only the first two compared: adding a thirteenth
	// verb to the table left the whole suite green, so the doc comment's
	// "verbatim from the spec's table" was a correspondence nothing enforced.
	//
	// This reads ONE section of ONE file, which is why it is allowed where a
	// test quantified over the corpus would not be — §7.2's own key set is
	// pinned the same way by TestTheAllowedKeySetIsExactlySection72sTable, and
	// this borrows its shape, including the NotEmpty that makes a parse failure
	// loud instead of vacuously true.
	inSpec := floorSection21Verbs(t)
	require.NotEmpty(t, inSpec, "§2.1's verb row must be readable for this to be checkable")
	assert.Equal(t, inSpec, floorVerbs,
		"the shipped verbs are exactly §2.1's, in the table's own order")

	for _, verb := range listed {
		t.Run(verb, func(t *testing.T) {
			unit := "The round " + verb + " what the prompt asked for."
			require.False(t, strings.ContainsAny(unit, "0123456789`"),
				"the verb must be the unit's only arm")

			assert.True(t, floorHasMeasurementVerb(unit), "verb arm")
			assert.True(t, inFloor(unit), "the verb arm alone puts the unit in the floor")
		})
	}

	// Verbs of the same family that §2.1 does NOT list, plus two inflections of
	// listed ones. The arm is a closed list, so a unit reporting a result in any
	// of these words is invisible to it by construction (§2.2), and a reading
	// that generalised the list would fail here.
	for _, verb := range []string{"showed", "found", "checked", "confirmed", "measures", "running"} {
		t.Run("not listed: "+verb, func(t *testing.T) {
			unit := "The round " + verb + " what the prompt asked for."
			require.False(t, strings.ContainsAny(unit, "0123456789`"))

			assert.False(t, floorHasMeasurementVerb(unit), "verb arm")
			assert.False(t, inFloor(unit), "no arm holds, so the unit is cut")
		})
	}
}

// TestTheVerbArmMatchesWholeWordsOnly settles what §2.1 leaves open. Its table
// says the unit "contains one of" the twelve, and read as plain substring
// containment that is untenable: "Transition" contains `ran`, "withheld"
// contains `held`, "discounted" contains `counted`. Whole-word matching is
// strictly narrower than substring matching, so every unit the two readings
// disagree on is a false positive for the substring reading, and the arm matches
// whole words.
//
// Each case carries the substring INSIDE a longer word and nothing else, so the
// unit is in the floor under the substring reading and out under this one — and
// the positives are asserted beside them, because a rule that stopped matching
// altogether would pass a negatives-only test.
func TestTheVerbArmMatchesWholeWordsOnly(t *testing.T) {
	notAVerb := map[string]string{
		"ran inside Transition":     "Transition open to wip.",
		"ran inside branches":       "The branches diverge and nobody merges them.",
		"held inside withheld":      "Nothing about the round was withheld.",
		"counted inside discounted": "The cost of a second pass is discounted here.",
		"fired inside misfired":     "A misfired hook leaves the gate green.",
	}
	for name, unit := range notAVerb {
		t.Run(name, func(t *testing.T) {
			require.False(t, strings.ContainsAny(unit, "0123456789`"),
				"the verb arm must be the only arm that could hold")

			assert.False(t, floorHasMeasurementVerb(unit), "verb arm")
			assert.False(t, inFloor(unit), "a verb inside a longer word is not the verb")
		})
	}

	for name, unit := range map[string]string{
		"ran as a word":  "The suite ran clean on the third attempt of the morning.",
		"held as a word": "The invariant held for every input the round put to it.",
	} {
		t.Run(name, func(t *testing.T) {
			require.False(t, strings.ContainsAny(unit, "0123456789`"))

			assert.True(t, floorHasMeasurementVerb(unit), "verb arm")
			assert.True(t, inFloor(unit), "the whole word still matches")
		})
	}
}

// TestTheVerbArmIgnoresCase settles the other thing §2.1 leaves open: its table
// lists the twelve in lower case and says nothing about how a unit spells them.
//
// The fold is not a convenience. A claim's measurement verb is routinely its
// sentence's FIRST word, so a case-sensitive arm cuts exactly the sentences the
// arm exists to catch.
//
// How many units that is over `spec/*.md` is deliberately not stated. It is a
// glob figure, it moves with every spec written, and the two comments either
// side of this one had theirs deleted for that reason while this one survived
// the sweep. What is checkable is here instead: each fixture below was taken
// from a spec in this repository rather than written for the test, and the
// `require`s in the loop pin the property that makes it discriminating — the
// verb is not spelled the table's way, so a case-sensitive arm would cut it.
func TestTheVerbArmIgnoresCase(t *testing.T) {
	for _, unit := range []string{
		"Measured while implementing: it can.",
		"Recorded so neither is re-proposed:",
		"Verified against the installed copy across six paths.",
		"Measured on this repository, that predicate is wrong in both directions.",
	} {
		t.Run(unit, func(t *testing.T) {
			require.False(t, strings.ContainsAny(unit, "0123456789`"),
				"the verb arm must be the only arm that could hold")
			// The fixture discriminates only if the VERB itself is not lower
			// case; one spelled the table's way passes either reading. The
			// property is asserted on the verb rather than on the unit, so a
			// capital elsewhere in the sentence cannot stand in for it.
			first := strings.Fields(unit)[0]
			require.NotEqual(t, first, strings.ToLower(first),
				"the fixture's verb must differ from the table's spelling")
			require.Contains(t, floorVerbs, strings.ToLower(first),
				"the fixture must open with one of §2.1's twelve")

			assert.True(t, floorHasMeasurementVerb(unit), "verb arm")
			assert.True(t, inFloor(unit), "a capitalised verb is still the verb")
		})
	}
}

// TestTheIdentifierArmNeedsADelimitedSpan is the one place this port departs
// from `scripts/floor-prototype.py`, and §2.1 decides it: the arm's test is "the
// unit contains a backtick-delimited SPAN", while the prototype's `in_floor`
// asks only whether a backtick is present.
//
// A unit the two readings disagree on holds exactly one backtick — with two or
// more, `[^`]*` finds a pair and both readings say yes — so every disagreement
// is the same shape: a code span whose delimiters landed in different units
// because step 4 split between them. A lone delimiter is the wreckage of a span,
// not a span, so this asserts the spec's wording. The count over `spec/*.md`
// that stood here is deleted rather than refreshed: §2.1 rules a figure over a
// glob worse than an unpinned one.
//
// The pair is asserted both ways on the SAME sentence, differing only in the
// closing backtick, so nothing but the delimiter can carry the verdict.
func TestTheIdentifierArmNeedsADelimitedSpan(t *testing.T) {
	tests := []struct{ name, unit string }{
		{"constructed", "The sentence trails off with a stray ` and never closes it."},
		// spec/0.12.0-review-rounds.md, verbatim: the corpus's own instance of
		// a span the sentence split cut in half.
		{"from the corpus", "If more, append: `..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, 1, strings.Count(tc.unit, "`"),
				"the fixture is the discriminating case only with exactly one backtick")
			require.False(t, strings.ContainsAny(tc.unit, "0123456789"),
				"no digit, so the identifier arm is the only one that could hold")
			require.False(t, floorHasMeasurementVerb(tc.unit), "and no listed verb")

			assert.False(t, floorHasCodeSpan(tc.unit), "identifier arm")
			assert.False(t, inFloor(tc.unit), "an unclosed delimiter is not a span")

			closed := tc.unit + "`"
			require.Equal(t, 2, strings.Count(closed, "`"))
			assert.True(t, floorHasCodeSpan(closed), "identifier arm")
			assert.True(t, inFloor(closed), "closing the span is the whole difference")
		})
	}
}

// TestFloorTextSHAIsTheFirstTwelveLowercaseHexOfTheSha256 pins §2.1 step 5's
// three separable decisions — the digest, the truncation and the case — against
// vectors computed outside Go (`python3 -c` and `shasum -a 256` agreeing), so
// none of them is asserted by re-running the implementation.
//
// "" and "abc" are sha256's published test vectors. The third is U+2014 alone,
// the em dash §2.1 step 1 joins table cells with: its UTF-8 encoding is asserted
// beside its hash, because "UTF-8 encoded" is a clause of the rule and every
// unit derived from a multi-cell table row carries that character.
//
// The vectors are the assertion and the pattern is the generalisation: a case
// the table does not hold still has to be twelve lowercase hex characters.
func TestFloorTextSHAIsTheFirstTwelveLowercaseHexOfTheSha256(t *testing.T) {
	tests := []struct{ name, unit, utf8, want string }{
		{"the empty string", "", "", "e3b0c44298fc"},
		{"abc", "abc", "616263", "ba7816bf8f01"},
		{"an em dash", "—", "e28094", "bda050585a00"},
		{"a table row's unit", "ölçüm — measured", "c3b66cc3a7c3bc6d20e28094206d65617375726564", "12a00876e2e1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.utf8, hex.EncodeToString([]byte(tc.unit)),
				"the vector was computed over exactly these bytes")

			got := FloorTextSHA(tc.unit)
			assert.Equal(t, tc.want, got)
			assert.Regexp(t, `^[0-9a-f]{12}$`, got, "twelve characters, lowercase, hex")
		})
	}
}

// TestFloorTextSHACoversTheWholeUnitNotAPrefix is §2.2's reason for emitting the
// hash rather than letting a reader derive one from the index: most units of
// this document exceed the 60-character display prefix, so a hash over that
// prefix carries a disposition forward across a sentence rewritten from
// character 61 on — exactly the defect §8 introduces `text_sha` to prevent.
//
// The two fixtures are identical for exactly 60 bytes, asserted rather than
// eyeballed, so the prefix reading and this one differ on nothing else.
func TestFloorTextSHACoversTheWholeUnitNotAPrefix(t *testing.T) {
	const shared = "The floor is a coverage obligation and the index carries no "
	require.Len(t, shared, 60, "the fixtures discriminate only if the shared head is exactly the prefix")

	a, b := shared+"unit text at all.", shared+"unit text whatsoever."
	require.Equal(t, a[:60], b[:60])
	require.NotEqual(t, a, b)

	assert.NotEqual(t, FloorTextSHA(a), FloorTextSHA(b),
		"a rewrite from character 61 on must move the hash")
}

// TestFloorOrdinalsCountWithinAHashInEmissionOrder is §7.2's `ordinal`: the
// 1-based index of a unit among those sharing its `text_sha`, in emission order,
// and 1 when the hash is unique.
//
// The interleaved case is the one that separates the three readings a plain
// "index" admits — per-hash (this rule), position in the whole round, and
// per-hash counted from the end — because on two adjacent duplicates all three
// agree on at least half the slice.
func TestFloorOrdinalsCountWithinAHashInEmissionOrder(t *testing.T) {
	tests := []struct {
		name  string
		units []string
		want  []int
	}{
		{"every hash unique", []string{"alpha 1.", "beta 2.", "gamma 3."}, []int{1, 1, 1}},
		{"one repeat, not adjacent", []string{"alpha 1.", "beta 2.", "alpha 1."}, []int{1, 1, 2}},
		{"two hashes interleaved", []string{"x 1.", "y 2.", "x 1.", "y 2.", "x 1."}, []int{1, 1, 2, 2, 3}},
		{"nothing to number", []string{}, []int{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hashes := make([]string, 0, len(tc.units))
			for _, u := range tc.units {
				hashes = append(hashes, FloorTextSHA(u))
			}
			assert.Equal(t, tc.want, FloorOrdinals(hashes))
		})
	}
}

// TestSection11Row18cTheJoinKeyIsTheHashAndTheOrdinal is the half of §11 row 18c
// this derivation owns: two units with identical canonical text in one round are
// told apart by `(text_sha, ordinal)`, and are not told apart by `text_sha`
// alone. The carrying forward is §8's and is asserted where the join is
// implemented; what is checkable here is that the key the join needs exists.
//
// The repeated sentence is the corpus's own — `**Exit codes:** 0 = success.`
// occurs five times in `spec/0.1.0.md`, re-derived through
// `scripts/floor-prototype.py` at the commit this test was written — which is
// why §8 says `text_sha` alone "is not unique and cannot be made so". The
// fixture is constructed rather than read from that file, because §2.1 rules out
// a test quantified over `spec/*.md`.
//
// The row's named mutant is run rather than described: the last assertion is
// what joining on the hash alone would see.
func TestSection11Row18cTheJoinKeyIsTheHashAndTheOrdinal(t *testing.T) {
	const repeated = "**Exit codes:** 0 = success."
	const other = "A sentence carrying `a span` and nothing else."
	units := FloorUnits(repeated + "\n\n" + other + "\n\n" + repeated)
	require.Equal(t, []string{repeated, other, repeated}, units,
		"the fixture is two identical units with a third between them")

	hashes := make([]string, 0, len(units))
	for _, u := range units {
		hashes = append(hashes, FloorTextSHA(u))
	}
	ordinals := FloorOrdinals(hashes)

	require.Equal(t, hashes[0], hashes[2], "identical canonical text, identical hash")
	require.NotEqual(t, hashes[0], hashes[1])
	assert.Equal(t, []int{1, 1, 2}, ordinals)

	type joinKey struct {
		sha string
		ord int
	}
	pairs := make(map[joinKey]int, len(units))
	byHashAlone := make(map[string]int, len(units))
	for i := range units {
		pairs[joinKey{hashes[i], ordinals[i]}]++
		byHashAlone[hashes[i]]++
	}
	assert.Len(t, pairs, 3, "(text_sha, ordinal) is a key over the round's units")
	assert.Len(t, byHashAlone, 2, "text_sha alone is not: it matches the first for both")
}

// TestFloorIndexRowsNumberOverEveryUnitIncludingTheOnesTheArmsCut is §7.2's
// `unit_id`: `u<N>` where N is the 1-based index of the unit over every unit
// §2.1 produces, in document order, counting the ones the arms cut. §2.2 gives
// the reason — numbering over the floor alone renumbers every later unit when an
// edit changes one unit's arms, and §8 joins dispositions across rounds.
//
// The cut unit sits BETWEEN two floor units, because that is the only
// arrangement the two readings disagree on: they agree on every unit up to the
// first cut one, so a fixture whose cut unit is last passes under both.
//
// The index announces the cut unit rather than dropping it (§2.2), so the two
// readings are no longer separated by the row COUNT — every unit has a row
// under both. They are separated by the ids, and the rows are looked up by the
// hash of the unit they stand for rather than by position, so a numbering over
// the floor alone gives the third unit `u2` and fails here whatever order the
// rows arrive in.
//
// For the same reason the `asked` assertion below no longer distinguishes "the
// unit's id" from "the row's index": with a row per unit the two are the same
// number, and no input of this shape can separate them. What it still pins is
// that anchorOf is called exactly once per unit, in document order. The
// distinction survives where the floor IS filtered — `FloorUnitRows`, in
// TestUnitsRowsNumberOverEveryUnitAndCarryNoCutOne.
//
// That the middle unit is cut is asserted rather than assumed. It carries no
// digit, no backtick span and no listed verb — "asserts" is not "asserted" to a
// whole-word arm — and if a later change to the arms made it a floor unit this
// test would be measuring nothing.
func TestFloorIndexRowsNumberOverEveryUnitIncludingTheOnesTheArmsCut(t *testing.T) {
	const kept1 = "The first claim carries `a span`."
	const dropped = "This sentence asserts nothing about the world."
	const kept2 = "The third claim carries 3 items."
	text := kept1 + "\n\n" + dropped + "\n\n" + kept2

	units := FloorUnits(text)
	require.Equal(t, []string{kept1, dropped, kept2}, units, "the fixture is three units in this order")
	require.True(t, inFloor(kept1), "the first unit must be in the floor")
	require.False(t, inFloor(dropped), "the middle unit must be one the arms cut")
	require.True(t, inFloor(kept2), "the third unit must be in the floor")

	asked := make([]int, 0, len(units))
	rows := FloorIndexRows(text, func(n int) string {
		asked = append(asked, n)
		return fmt.Sprintf("§%d", n)
	})

	require.Len(t, rows, 3, "a cut unit is announced, so every unit has a row")
	assert.Equal(t, []int{1, 2, 3}, asked, "anchorOf is called once per unit, in document order")

	byHash := make(map[string]FloorIndexRow, len(rows))
	for _, r := range rows {
		if r.TextSHA != "" {
			byHash[r.TextSHA] = r
		}
	}
	require.Len(t, byHash, 2, "two floor units, two distinct hashes")

	first, third := byHash[FloorTextSHA(kept1)], byHash[FloorTextSHA(kept2)]
	assert.Equal(t, "u1", first.ID)
	assert.Equal(t, "u3", third.ID, "the cut unit consumed u2")
	assert.Equal(t, "§1", first.Anchor)
	assert.Equal(t, "§3", third.Anchor, "and the anchor asked for under that id")
}

// TestAFloorIndexRowCarriesTheFiveFields is §2.2's row: every floor unit emits
// `(unit_id, anchor, text_sha, ordinal, byte length)`.
//
// The fixture is a table data row so the unit carries an em dash, which is what
// makes the length assertion discriminating: `Bytes` is the unit's length in
// UTF-8 BYTES and a rune count reads two short on exactly this input. Both
// numbers are stated, so neither can be read off the other.
func TestAFloorIndexRowCarriesTheFiveFields(t *testing.T) {
	const text = "| the field | 2 bytes |\n"
	units := FloorUnits(text)
	require.Equal(t, []string{"the field — 2 bytes"}, units)
	require.Equal(t, 19, utf8.RuneCountInString(units[0]), "19 runes")
	require.Equal(t, 21, len(units[0]), "21 bytes: the em dash is three of them")

	rows := FloorIndexRows(text, func(int) string { return "§7.2" })
	require.Len(t, rows, 1)

	assert.Equal(t, FloorIndexRow{
		ID:      "u1",
		Anchor:  "§7.2",
		TextSHA: FloorTextSHA(units[0]),
		Ordinal: 1,
		Bytes:   21,
	}, rows[0])
	assert.Equal(t, "u1 §7.2 "+FloorTextSHA(units[0])+" #1 21B", rows[0].String())
}

// TestSection11Row4TheIndexIsBoundedAndCarriesNoUnitText is §11 row 4.
//
// The row names one mutant — carry the first 60 bytes of each unit — and both
// halves of the assertion are built so that it fails them: a 60-byte head more
// than doubles a row, and the head of a short unit IS the unit.
//
// The bound is asserted on a floor derived from real text rather than on
// hand-built structs, so the same mutant that leaks text also inflates the
// measured size. Getting the worst case §2.2 admits out of a derivation means
// 1,200 identical units (so the ordinal reaches four digits) each over 1,000
// bytes long (so the length field does too), under a seven-segment anchor. Each
// of those three properties is asserted, because every one of them is what makes
// the size a worst case rather than a comfortable case.
func TestSection11Row4TheIndexIsBoundedAndCarriesNoUnitText(t *testing.T) {
	t.Run("1,200 units at the worst case stay under units*48+256 bytes", func(t *testing.T) {
		const count = 1200
		const anchor = "§1.2.3.4.5.6.7"
		require.Equal(t, 6, strings.Count(anchor, "."), "seven segments is the worst case §2.2 admits")

		unit := strings.Repeat("payload ", 130) + "1234."
		require.Len(t, unit, 1045, "a four-digit byte length is the worst case for the length field")

		var text strings.Builder
		for i := 0; i < count; i++ {
			text.WriteString(unit)
			text.WriteString("\n\n")
		}
		rows := FloorIndexRows(text.String(), func(int) string { return anchor })

		require.Len(t, rows, count)
		require.Equal(t, "u1200", rows[count-1].ID, "four-digit ids")
		require.Equal(t, count, rows[count-1].Ordinal, "identical units, so the ordinal reaches four digits")
		require.Equal(t, len(unit), rows[count-1].Bytes)

		index := FormatFloorIndex("7ced1edb", rows)
		assert.Less(t, len(index), count*48+256,
			"the index is bounded at units*48+256; carrying 60 bytes of each unit is over it")
	})

	t.Run("no substring of a unit's text reaches the index", func(t *testing.T) {
		// Row 4's own input: a unit whose text is a rare token, searched for in
		// the index. The token is not hex, so it cannot turn up inside a hash.
		const rare = "zqxjvwkfgb"
		const unit = "The claim names zqxjvwkfgb and 1 other thing."
		require.Contains(t, unit, rare)

		units := FloorUnits(unit)
		require.Equal(t, []string{unit}, units, "the rare token is inside a floor unit")

		index := FormatFloorIndex("7ced1edb", FloorIndexRows(unit, func(int) string { return "§0" }))
		assert.NotContains(t, index, rare, "the index carries no unit text")

		// The token search is row 4's named input and it is not sufficient on its
		// own: read literally, "no substring of any unit's text" is false of any
		// single character, since the index is made of characters. So the
		// property is also pinned exhaustively — every byte of the index is
		// accounted for by this grammar, which leaves nowhere for text to sit.
		// The grammar covers the whole artifact and not just the rows: the
		// commit line and the summary are the only other things in it, and both
		// are pinned to a shape with no free text in it — a hex revision or the
		// literal `unknown`, and two counts.
		shape := regexp.MustCompile(
			`^# commit (?:[0-9a-f]+|unknown)\n` +
				`(?:u\d+ §[0-9.]+ (?:[0-9a-f]{12} #\d+ \d+B|\(cut\))\n)+` +
				`# \d+ in floor, \d+ cut\n$`)
		assert.Regexp(t, shape, index,
			"every byte of the index is an id, an anchor, a hash, an ordinal, a length, the commit or a count")
	})
}

// floorSection21Verbs reads the verbs out of §2.1's arms table, in the table's
// own order, so the shipped list can be compared against the document rather
// than against a second copy of itself.
//
// It finds the verb row by the arm's name in the first cell rather than by a
// line number, because a line number is the thing this repository has watched
// rot three times in one cycle. Every `require` here is what keeps a parse
// failure from passing as an empty set that trivially matches nothing.
//
// The search is anchored to §2.1's heading and stops at the next one, so the
// row it reads is the row this function claims to read. An earlier version
// took the first such line in the whole file: a decoy `| **verb** |` row
// carrying the twelve, placed anywhere above §2.1, left both packages green
// while a thirteenth verb sat in the real table. The cell pattern tolerates
// padding for the same reason the sibling's does — a cosmetically realigned
// table is a true failure with a misleading cause.
func floorSection21Verbs(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "spec", "1.0.0.md"))
	require.NoError(t, err, "this list is §2.1's table; the spec must be readable for that to be checkable")

	lines := strings.Split(string(data), "\n")
	i := slices.Index(lines, "### 2.1 The floor")
	require.GreaterOrEqual(t, i, 0, "§2.1 must be findable by its heading")

	row := regexp.MustCompile(`^\|\s*\*\*verb\*\*\s*\|`)
	for i++; i < len(lines) && !strings.HasPrefix(lines[i], "### "); i++ {
		if row.MatchString(lines[i]) {
			break
		}
	}
	require.Less(t, i, len(lines), "§2.1's arms table must carry a row whose first cell is **verb**")
	require.True(t, row.MatchString(lines[i]),
		"the verb row must sit inside §2.1, not in a later section")

	quoted := regexp.MustCompile("`([a-z]+)`")
	verbs := make([]string, 0, 12)
	for _, m := range quoted.FindAllStringSubmatch(lines[i], -1) {
		verbs = append(verbs, m[1])
	}
	return verbs
}
