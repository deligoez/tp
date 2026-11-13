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

// assertEdgesForward is the layout's invariant: every edge's dependency
// sits on a layer left of its dependent's.
func assertEdgesForward(t *testing.T, l Layout, edges []LayoutEdge) {
	t.Helper()
	for _, e := range edges {
		assert.Less(t, l.Places[e.From].Layer, l.Places[e.To].Layer, "%s -> %s", e.From, e.To)
	}
}

// TestLayeredLayout_BandCycleMergesIntoOneSpan: two bands that depend on
// each other through different epics (a -> b from M1 into M2, c -> d from
// M2 into M1) cannot run one after the other without an edge pointing
// back, so they share one span, reported as one band naming both, and its
// nodes are laid out by longest path over all their edges.
func TestLayeredLayout_BandCycleMergesIntoOneSpan(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"a", "b"}, {"c", "d"}}
	bandOf := map[string]string{"a": "M1", "d": "M1", "b": "M2", "c": "M2"}
	got := LayeredLayout([]string{"a", "b", "c", "d"}, edges, []string{"M1", "M2"}, bandOf)
	assertEdgesForward(t, got, edges)
	assert.Equal(t, []LayoutBand{{Name: "M1", With: []string{"M2"}, From: 0, To: 1}}, got.Bands)
	assert.Equal(t, 2, got.Layers)
}

// TestLayeredLayout_BandCycleKeepsTheBandsAroundItInOrder: a band before
// the cycle and one after it keep their own spans, every edge forward.
func TestLayeredLayout_BandCycleKeepsTheBandsAroundItInOrder(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"z", "a"}, {"a", "b"}, {"c", "d"}, {"d", "y"}, {"b", "y"}}
	bandOf := map[string]string{"z": "M0", "a": "M1", "d": "M1", "b": "M2", "c": "M2", "y": "M3"}
	got := LayeredLayout([]string{"y", "a", "b", "c", "d", "z"}, edges, []string{"M3", "M1", "M2", "M0"}, bandOf)
	assertEdgesForward(t, got, edges)
	assert.Equal(t, []LayoutBand{{Name: "M0", From: 0, To: 0}, {Name: "M1", With: []string{"M2"}, From: 1, To: 2}, {Name: "M3", From: 3, To: 3}}, got.Bands)
}
