package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// banded is the forest layout of ids over edges in bands, packed as trees.
func banded(ids []string, edges []LayoutEdge, bands []string, bandOf map[string]string) Forest {
	return ForestLayout(&ForestInput{IDs: ids, Edges: edges, Bands: bands, BandOf: bandOf, Pack: PackTree})
}

// TestBandLayers_GoldenBands pins milestones as bands of consecutive
// layers: a band's layers come from the longest path among its own epics,
// the bands run in dependency order whatever order they are named in, and
// each band reports the layers it spans.
func TestBandLayers_GoldenBands(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"p", "q"}, {"q", "r"}, {"p", "s"}}
	bandOf := map[string]string{"p": "M1", "q": "M1", "r": "M2", "s": "M2"}
	got := banded([]string{"r", "s", "p", "q"}, edges, []string{"M2", "M1"}, bandOf)
	assert.Equal(t, 3, got.Layers)
	assert.Equal(t, []LayoutBand{{Name: "M1", From: 0, To: 1}, {Name: "M2", From: 2, To: 2}}, got.Bands)
	for id, layer := range map[string]int{"p": 0, "q": 1, "r": 2, "s": 2} {
		assert.Equal(t, layer, got.Places[id].Layer, id)
	}
}

// assertEdgesForward is the layout's invariant: every edge's dependency
// sits on a layer above its dependent's.
func assertEdgesForward(t *testing.T, l Forest, edges []LayoutEdge) {
	t.Helper()
	for _, e := range edges {
		assert.Less(t, l.Places[e.From].Layer, l.Places[e.To].Layer, "%s -> %s", e.From, e.To)
	}
}

// TestBandLayers_AChainInOneBandTakesOneLayerPerStep: a node enters the
// longest-path walk only once its last in-run predecessor is done, so a
// chain of single-predecessor steps with a skip edge into its end still
// lays out one layer per step.
func TestBandLayers_AChainInOneBandTakesOneLayerPerStep(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"a", "b"}, {"b", "c"}, {"c", "d"}, {"a", "d"}}
	got := banded([]string{"a", "b", "c", "d"}, edges, nil, nil)
	assertEdgesForward(t, got, edges)
	assert.Equal(t, 4, got.Layers)
	assert.Equal(t, 3, got.Places["d"].Layer)
}

// TestBandLayers_BandCycleMergesIntoOneSpan: two bands that depend on
// each other through different epics (a -> b from M1 into M2, c -> d from
// M2 into M1) cannot run one after the other without an edge pointing
// back, so they share one span, reported as one band naming both, and its
// nodes are laid out by longest path over all their edges.
func TestBandLayers_BandCycleMergesIntoOneSpan(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"a", "b"}, {"c", "d"}}
	bandOf := map[string]string{"a": "M1", "d": "M1", "b": "M2", "c": "M2"}
	got := banded([]string{"a", "b", "c", "d"}, edges, []string{"M1", "M2"}, bandOf)
	assertEdgesForward(t, got, edges)
	assert.Equal(t, []LayoutBand{{Name: "M1", With: []string{"M2"}, From: 0, To: 1}}, got.Bands)
	assert.Equal(t, 2, got.Layers)
}

// TestBandLayers_BandCycleKeepsTheBandsAroundItInOrder: a band before
// the cycle and one after it keep their own spans, every edge forward.
func TestBandLayers_BandCycleKeepsTheBandsAroundItInOrder(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"z", "a"}, {"a", "b"}, {"c", "d"}, {"d", "y"}, {"b", "y"}}
	bandOf := map[string]string{"z": "M0", "a": "M1", "d": "M1", "b": "M2", "c": "M2", "y": "M3"}
	got := banded([]string{"y", "a", "b", "c", "d", "z"}, edges, []string{"M3", "M1", "M2", "M0"}, bandOf)
	assertEdgesForward(t, got, edges)
	assert.Equal(t, []LayoutBand{{Name: "M0", From: 0, To: 0}, {Name: "M1", With: []string{"M2"}, From: 1, To: 2}, {Name: "M3", From: 3, To: 3}}, got.Bands)
}

// TestBandLayers_ABandNamedTwiceOrLeftOutIsOneBandInOrder: a band the
// list names twice is one band, and a band a node names that the list
// leaves out is appended and laid out in dependency order like the rest,
// never folded into layer 0.
func TestBandLayers_ABandNamedTwiceOrLeftOutIsOneBandInOrder(t *testing.T) {
	t.Parallel()
	edges := []LayoutEdge{{"p", "q"}, {"q", "r"}}
	bandOf := map[string]string{"p": "M1", "q": "M2", "r": "M3"}
	got := banded([]string{"p", "q", "r"}, edges, []string{"M1", "M2", "M1", "M2"}, bandOf)
	assertEdgesForward(t, got, edges)
	assert.Equal(t, []LayoutBand{{Name: "M1", From: 0, To: 0}, {Name: "M2", From: 1, To: 1}, {Name: "M3", From: 2, To: 2}}, got.Bands)
	assert.Equal(t, 3, got.Layers)
}
