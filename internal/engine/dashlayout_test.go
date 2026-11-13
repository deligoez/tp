package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLayeredLayout_GoldenOneBand pins the layout of a small graph with no
// bands (spec/backlog/dashboard-ux.md §2): a node's layer is its longest
// dependency path, and each layer's order comes from four barycenter sweeps,
// down, up, down, up, ties broken by file order.
func TestLayeredLayout_GoldenOneBand(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"a", "c"}, {"b", "c"}, {"c", "e"}, {"a", "d"}, {"d", "e"}, {"b", "f"}}
	got := LayeredLayout([]string{"a", "b", "c", "d", "e", "f"}, edges, nil, nil)
	assert.Equal(t, Layout{
		Layers: 3,
		Places: map[string]LayoutPlace{
			"a": {0, 0}, "b": {0, 1},
			"c": {1, 0}, "d": {1, 1}, "f": {1, 2},
			"e": {2, 0},
		},
	}, got)
}

// TestLayeredLayout_GoldenBands pins milestones as bands of consecutive
// layers: a band's layers come from the longest path among its own epics,
// the bands run in dependency order whatever order they are named in, and
// each band reports the layers it spans.
func TestLayeredLayout_GoldenBands(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"p", "q"}, {"q", "r"}, {"p", "s"}}
	bandOf := map[string]string{"p": "M1", "q": "M1", "r": "M2", "s": "M2"}
	got := LayeredLayout([]string{"r", "s", "p", "q"}, edges, []string{"M2", "M1"}, bandOf)
	assert.Equal(t, Layout{
		Layers: 3,
		Bands:  []LayoutBand{{Name: "M1", From: 0, To: 1}, {Name: "M2", From: 2, To: 2}},
		Places: map[string]LayoutPlace{"p": {0, 0}, "q": {1, 0}, "r": {2, 0}, "s": {2, 1}},
	}, got)
}

// TestLayeredLayout_IsStable: the same input lays out the same, whatever map
// order Go iterates in.
func TestLayeredLayout_IsStable(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"a", "c"}, {"b", "c"}, {"c", "e"}, {"a", "d"}, {"d", "e"}, {"b", "f"}}
	first := LayeredLayout([]string{"a", "b", "c", "d", "e", "f"}, edges, nil, nil)
	for range 20 {
		assert.Equal(t, first, LayeredLayout([]string{"a", "b", "c", "d", "e", "f"}, edges, nil, nil))
	}
}
