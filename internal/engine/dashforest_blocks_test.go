package engine

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// twoLanes is a synthetic graph in blocks: band A (a chain) and band B (a
// small tree) in different lanes, so they share layers 0-2, and band C in
// A's lane after A, on layer 3 alone; each band is its own group.
func twoLanes(pack ForestPacking) ForestInput {
	ids := []string{"a0", "a1", "a2", "b0", "b1", "b2", "b3", "c0"}
	edges := []LayoutEdge{{"a0", "a1"}, {"a1", "a2"}, {"b0", "b1"}, {"b0", "b2"}, {"b1", "b3"}, {"a2", "c0"}}
	bandOf := map[string]string{"a0": "A", "a1": "A", "a2": "A", "b0": "B", "b1": "B", "b2": "B", "b3": "B", "c0": "C"}
	return ForestInput{
		IDs: ids, Edges: edges, Bands: []string{"A", "B", "C"}, BandOf: bandOf, GroupOf: bandOf,
		LaneOf: map[string]string{"A": "x", "B": "y", "C": "x"}, Pack: pack, Blocks: true,
	}
}

// cycleRun is a synthetic graph whose three bands P, Q and R depend on
// each other in a cycle, so they share one run of layers.
func cycleRun(pack ForestPacking) ForestInput {
	ids := []string{"p0", "p1", "q0", "q1", "r0", "r1", "r2"}
	edges := []LayoutEdge{{"p0", "q0"}, {"q0", "r0"}, {"r0", "p1"}, {"q0", "q1"}, {"r0", "r1"}, {"r0", "r2"}}
	bandOf := map[string]string{"p0": "P", "p1": "P", "q0": "Q", "q1": "Q", "r0": "R", "r1": "R", "r2": "R"}
	return ForestInput{
		IDs: ids, Edges: edges, Bands: []string{"P", "Q", "R"}, BandOf: bandOf, GroupOf: bandOf,
		LaneOf: map[string]string{"P": "x", "Q": "x", "R": "x"}, Pack: pack, Blocks: true,
	}
}

func blockNamed(t *testing.T, blocks []ForestBlock, name string) ForestBlock {
	t.Helper()
	for _, b := range blocks {
		if b.Group == name {
			return b
		}
	}
	require.Failf(t, "no block", "%s", name)
	return ForestBlock{}
}

// assertBlocksApart checks that every two blocks sharing a layer take
// column ranges with at least one empty column between them, and that each
// node lies inside its own block's layers and columns.
func assertBlocksApart(t *testing.T, in *ForestInput, f *Forest) {
	t.Helper()
	blocks := ForestBlocks(in, f)
	for i, a := range blocks {
		for _, b := range blocks[i+1:] {
			if a.FromLayer <= b.ToLayer && b.FromLayer <= a.ToLayer {
				apart := a.ToCol+1 < b.FromCol || b.ToCol+1 < a.FromCol
				assert.True(t, apart, "%s: blocks %+v and %+v share layers and touch", in.Pack, a, b)
			}
		}
	}
	for _, id := range in.IDs {
		b, p := blockNamed(t, blocks, in.GroupOf[id]), f.Places[id]
		assert.True(t, b.FromCol <= p.Col && p.Col <= b.ToCol && b.FromLayer <= p.Layer && p.Layer <= b.ToLayer, "%s: %s at %+v outside %+v", in.Pack, id, p, b)
	}
}

// TestForestLayout_BlocksSharingLayersTakeDisjointColumns: A and B share
// layers 0-2 and come out as two blocks of their own columns, a gap
// between them, each holding only its own nodes.
func TestForestLayout_BlocksSharingLayersTakeDisjointColumns(t *testing.T) {
	t.Parallel()
	for _, pack := range []ForestPacking{PackRows, PackTree} {
		in := twoLanes(pack)
		got := ForestLayout(&in)
		blocks := ForestBlocks(&in, &got)
		require.Len(t, blocks, 3, "%s", pack)
		a, b := blockNamed(t, blocks, "A"), blockNamed(t, blocks, "B")
		assert.Equal(t, [2]int{0, 2}, [2]int{a.FromLayer, a.ToLayer}, "%s", pack)
		assert.Equal(t, [2]int{0, 2}, [2]int{b.FromLayer, b.ToLayer}, "%s", pack)
		assertBlocksApart(t, &in, &got)
	}
}

