package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// shelfGoldenPath holds ShelveForest's output for shelfGoldenSample at each
// width the golden test lays it out at.
const shelfGoldenPath = "testdata/shelve_forest_golden.json"

// shelfGoldenSample is a forest of 48 tasks shaped like a spec's: chains of
// different depths, a parent with more childless children than a narrow
// width holds, trees joined by cross edges in both orders, a diamond, and
// tasks no edge touches. It is built by rule, not by chance, so the golden
// file never moves under a fixed implementation.
func shelfGoldenSample() ([]string, []LayoutEdge) {
	var ids []string
	var edges []LayoutEdge
	id := func(i int) string { return fmt.Sprintf("t%02d", i) }
	for i := range 48 {
		ids = append(ids, id(i))
	}
	link := func(a, b int) { edges = append(edges, LayoutEdge{From: id(a), To: id(b)}) }
	for i := range 7 { // a chain of eight
		link(i, i+1)
	}
	for i := 9; i <= 15; i++ { // a parent with seven childless children
		link(8, i)
	}
	link(16, 17) // a diamond
	link(16, 18)
	link(17, 19)
	link(18, 19)
	link(19, 20)
	for i := 21; i < 26; i++ { // a short chain fanning at its foot
		link(i, i+1)
	}
	link(26, 27)
	link(26, 28)
	link(3, 22)  // cross edges in both directions of the file order
	link(25, 10) //
	link(20, 29)
	for i := 30; i < 36; i++ { // a second chain
		link(i, i+1)
	}
	link(36, 37)
	link(37, 38)
	link(37, 39)
	// 40-47 are touched by no edge.
	return ids, edges
}

// TestShelveForest_MatchesItsGoldenLayout pins ShelveForest's whole output,
// places, roles, layers and columns, for a spec-shaped forest at narrow,
// middle and wide widths. The shelf's arithmetic is all off-by-one choices a
// picture tolerates and a mutant flips, so the golden file is what holds
// them. TP_UPDATE_GOLDEN=1 rewrites it from the current implementation.
func TestShelveForest_MatchesItsGoldenLayout(t *testing.T) {
	t.Parallel()
	ids, edges := shelfGoldenSample()
	got := map[string]Forest{}
	for _, pack := range []ForestPacking{PackTree, PackRows} {
		f := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: pack})
		for _, width := range []float64{1, 3, 5, 7, 12} {
			got[fmt.Sprintf("%s/%g", pack, width)] = ShelveForest(ids, edges, f, width)
		}
	}
	data, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	if os.Getenv("TP_UPDATE_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(shelfGoldenPath), 0o750))
		require.NoError(t, os.WriteFile(shelfGoldenPath, append(data, '\n'), 0o600))
	}
	want, err := os.ReadFile(shelfGoldenPath)
	require.NoError(t, err)
	require.JSONEq(t, string(want), string(data))
}
