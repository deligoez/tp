package engine

import (
	"os"
	"path/filepath"
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
