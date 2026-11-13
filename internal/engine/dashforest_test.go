package engine

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestForestLayout_GoldenTreeColumns pins the tree packing on lulus's
// sample: a diamond with a tail and a side branch in one group, two lone
// nodes in another. Leaves take the next column, a parent sits centred
// over its children, the diamond's second edge in is a cross edge, and the
// second group starts forestGroupGap columns after the first.
func TestForestLayout_GoldenTreeColumns(t *testing.T) {
	t.Parallel()
	ids := []string{"a1", "a2", "a3", "a4", "a5", "a6", "b1", "b2"}
	edges := []LayoutEdge{{"a1", "a2"}, {"a1", "a3"}, {"a2", "a4"}, {"a3", "a4"}, {"a4", "a5"}, {"a1", "a6"}}
	groupOf := map[string]string{"a1": "a", "a2": "a", "a3": "a", "a4": "a", "a5": "a", "a6": "a", "b1": "b", "b2": "b"}
	got := ForestLayout(ids, edges, nil, nil, groupOf, PackTree)
	assert.Equal(t, Forest{
		Layers:  4,
		Columns: 8.5,
		Roles:   []EdgeRole{RoleTree, RoleTree, RoleTree, RoleCross, RoleTree, RoleTree},
		Places: map[string]ForestPlace{
			"a1": {0, 1}, "a2": {1, 0}, "a3": {1, 1}, "a6": {1, 2}, "a4": {2, 0}, "a5": {3, 0},
			"b1": {0, 6}, "b2": {0, 7.5},
		},
	}, got)
}

// TestForestLayout_GoldenBandsAndBackEdges pins bands and edge classes: the
// primary parent is the dependency on the deepest layer (q over p for r),
// the other edge into r is a cross edge, a two-node cycle inside one band
// lays out with both its edges back, and an edge naming an unknown id is
// back too.
func TestForestLayout_GoldenBandsAndBackEdges(t *testing.T) {
	t.Parallel()
	ids := []string{"p", "q", "r", "x", "y"}
	edges := []LayoutEdge{{"p", "q"}, {"q", "r"}, {"p", "r"}, {"x", "y"}, {"y", "x"}, {"p", "zz"}}
	bandOf := map[string]string{"p": "M1", "q": "M1", "r": "M2", "x": "M3", "y": "M3"}
	got := ForestLayout(ids, edges, []string{"M1", "M2", "M3"}, bandOf, nil, PackTree)
	assert.Equal(t, []EdgeRole{RoleTree, RoleTree, RoleCross, RoleBack, RoleBack, RoleBack}, got.Roles)
	assert.Equal(t, []LayoutBand{{Name: "M1", From: 0, To: 1}, {Name: "M2", From: 2, To: 2}, {Name: "M3", From: 3, To: 3}}, got.Bands)
	assert.Equal(t, ForestPlace{2, 0}, got.Places["r"])
	assert.Equal(t, ForestPlace{3, 1.5}, got.Places["x"])
	assert.Equal(t, ForestPlace{3, 3}, got.Places["y"])
}

// TestForestLayout_GoldenRowPacking pins the row packing: two groups of
// one root and two children, seven tree columns wide with the group gap,
// squeezed into the four columns of the fullest layer, each layer keeping
// its left-to-right order.
func TestForestLayout_GoldenRowPacking(t *testing.T) {
	t.Parallel()
	ids := []string{"a0", "a1", "a2", "b0", "b1", "b2"}
	edges := []LayoutEdge{{"a0", "a1"}, {"a0", "a2"}, {"b0", "b1"}, {"b0", "b2"}}
	groupOf := map[string]string{"a0": "a", "a1": "a", "a2": "a", "b0": "b", "b1": "b", "b2": "b"}
	tree := ForestLayout(ids, edges, nil, nil, groupOf, PackTree)
	assert.InDelta(t, 7.0, tree.Columns, 0)
	got := ForestLayout(ids, edges, nil, nil, groupOf, PackRows)
	assert.InDelta(t, 4.0, got.Columns, 0)
	assert.Equal(t, map[string]ForestPlace{
		"a0": {0, 0}, "b0": {0, 3},
		"a1": {1, 0}, "a2": {1, 1}, "b1": {1, 2}, "b2": {1, 3},
	}, got.Places)
}

// TestForestLayout_InvariantsOnARandomGraph checks, on a seeded random
// graph banded in three milestones, under both packings: no tree edge
// points backwards, every node with an edge from an earlier layer has
// exactly one tree edge in and a root none, no two nodes of a layer share
// a column, every column is inside the width, and no edge but a back edge
// points up.
func TestForestLayout_InvariantsOnARandomGraph(t *testing.T) {
	t.Parallel()
	r := rand.New(rand.NewPCG(7, 11))
	var ids []string
	bandOf := map[string]string{}
	groupOf := map[string]string{}
	for i := range 120 {
		id := fmt.Sprintf("e%03d", i)
		ids = append(ids, id)
		bandOf[id] = fmt.Sprintf("M%d", i/40)
		groupOf[id] = fmt.Sprintf("T%d", i%3)
	}
	var edges []LayoutEdge
	for i := 1; i < len(ids); i++ {
		for range r.IntN(4) {
			edges = append(edges, LayoutEdge{From: ids[r.IntN(i)], To: ids[i]})
		}
	}
	for _, pack := range []ForestPacking{PackTree, PackRows} {
		got := ForestLayout(ids, edges, []string{"M0", "M1", "M2"}, bandOf, groupOf, pack)
		require.Len(t, got.Roles, len(edges))
		treeIn := map[string]int{}
		earlierIn := map[string]bool{}
		for i, e := range edges {
			from, to := got.Places[e.From], got.Places[e.To]
			if from.Layer < to.Layer {
				earlierIn[e.To] = true
			}
			switch got.Roles[i] {
			case RoleTree:
				assert.Less(t, from.Layer, to.Layer, "%s: tree edge %s->%s points backwards", pack, e.From, e.To)
				treeIn[e.To]++
			case RoleCross:
				assert.Less(t, from.Layer, to.Layer, "%s: cross edge %s->%s points up", pack, e.From, e.To)
			}
		}
		taken := map[[2]float64]string{}
		for _, id := range ids {
			want := 0
			if earlierIn[id] {
				want = 1
			}
			assert.Equal(t, want, treeIn[id], "%s: tree edges into %s", pack, id)
			p := got.Places[id]
			key := [2]float64{float64(p.Layer), p.Col}
			assert.Empty(t, taken[key], "%s: %s and %s share a place", pack, id, taken[key])
			taken[key] = id
			assert.True(t, p.Col >= 0 && p.Col < got.Columns, "%s: %s at column %v of %v", pack, id, p.Col, got.Columns)
		}
	}
}

// TestForestLayout_IsStable: the same input lays out the same.
func TestForestLayout_IsStable(t *testing.T) {
	t.Parallel()
	ids := []string{"a1", "a2", "a3", "a4", "a5", "a6"}
	edges := []LayoutEdge{{"a1", "a2"}, {"a1", "a3"}, {"a2", "a4"}, {"a3", "a4"}, {"a4", "a5"}, {"a1", "a6"}}
	first := ForestLayout(ids, edges, nil, nil, nil, PackRows)
	for range 20 {
		assert.Equal(t, first, ForestLayout(ids, edges, nil, nil, nil, PackRows))
	}
}
