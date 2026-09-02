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
