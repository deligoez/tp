package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestWriteEpicGraph_KeepsClearsAndNormalisesDescriptions: the graph's write
// sink keeps a description with text byte for byte, clears a whitespace-only
// one so its key is not written, and turns CRLF line ends into LF.
func TestWriteEpicGraph_KeepsClearsAndNormalisesDescriptions(t *testing.T) {
	dir := t.TempDir()
	kept := "  ## Purpose\n\nfirst\tline  \n  indented\n"
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"A"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "A", Root: true, Description: kept},
		{ID: "b", Title: "B", Milestone: "A", Root: true, Description: " \n\t \r\n"},
		{ID: "c", Title: "C", Milestone: "A", Root: true, Description: "one\r\ntwo\r\n\r\nthree"},
	}}
	require.NoError(t, WriteEpicGraph(dir, g))

	loaded, _, err := LoadEpicGraph(dir)
	require.NoError(t, err)
	assert.Equal(t, kept, loaded.Epics[0].Description, "a description with text is kept verbatim")
	assert.Empty(t, loaded.Epics[1].Description, "a whitespace-only description is cleared")
	assert.Equal(t, "one\ntwo\n\nthree", loaded.Epics[2].Description, "CRLF becomes LF")
	raw, err := os.ReadFile(filepath.Join(dir, EpicGraphFile))
	require.NoError(t, err)
	assert.Equal(t, 2, strings.Count(string(raw), `"description"`), "the cleared description writes no key: %s", raw)
}

// TestLineDiff_OpsAndHunks pins the line diff a description-only change is
// shown as: the exact op sequence lineDiffOps returns, removals before
// additions where the two differ, and the hunks lineHunks renders from it,
// one unchanged line around each change and one … per elided run.
func TestLineDiff_OpsAndHunks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		a, b  []string
		ops   []diffOp
		hunks string
	}{
		{"insert at the start", []string{"x", "y"}, []string{"n", "x", "y"},
			[]diffOp{{'+', "n"}, {' ', "x"}, {' ', "y"}}, "+ n\n  x\n…\n"},
		{"two inserts before the one kept line", []string{"x"}, []string{"y", "z", "x"},
			[]diffOp{{'+', "y"}, {'+', "z"}, {' ', "x"}}, "+ y\n+ z\n  x\n"},
		{"delete at the end", []string{"x", "y", "z"}, []string{"x", "y"},
			[]diffOp{{' ', "x"}, {' ', "y"}, {'-', "z"}}, "…\n  y\n- z\n"},
		{"change in the middle", []string{"x", "y", "z"}, []string{"x", "Y", "z"},
			[]diffOp{{' ', "x"}, {'-', "y"}, {'+', "Y"}, {' ', "z"}}, "  x\n- y\n+ Y\n  z\n"},
		{"two separate changes", []string{"a", "b", "c", "d", "e", "f", "g"}, []string{"a", "B", "c", "d", "e", "F", "g"},
			[]diffOp{{' ', "a"}, {'-', "b"}, {'+', "B"}, {' ', "c"}, {' ', "d"}, {' ', "e"}, {'-', "f"}, {'+', "F"}, {' ', "g"}},
			"  a\n- b\n+ B\n  c\n…\n  e\n- f\n+ F\n  g\n"},
		{"all lines replaced", []string{"p", "q"}, []string{"r", "s"},
			[]diffOp{{'-', "p"}, {'-', "q"}, {'+', "r"}, {'+', "s"}}, "- p\n- q\n+ r\n+ s\n"},
		{"identical inputs", []string{"x", "y"}, []string{"x", "y"},
			[]diffOp{{' ', "x"}, {' ', "y"}}, "…\n"},
		{"empty before", nil, []string{"x", "y"},
			[]diffOp{{'+', "x"}, {'+', "y"}}, "+ x\n+ y\n"},
		{"empty after", []string{"x", "y"}, nil,
			[]diffOp{{'-', "x"}, {'-', "y"}}, "- x\n- y\n"},
		{"both empty", nil, nil, nil, ""},
	} {
		assert.Equal(t, tc.ops, lineDiffOps(tc.a, tc.b), tc.name)
		assert.Equal(t, tc.hunks, lineHunks(tc.a, tc.b), tc.name)
	}
}
