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

