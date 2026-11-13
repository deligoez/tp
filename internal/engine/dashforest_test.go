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
	got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, GroupOf: groupOf, Pack: PackTree})
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
	got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Bands: []string{"M1", "M2", "M3"}, BandOf: bandOf, Pack: PackTree})
	assert.Equal(t, []EdgeRole{RoleTree, RoleTree, RoleCross, RoleBack, RoleBack, RoleBack}, got.Roles)
	assert.Equal(t, []LayoutBand{{Name: "M1", From: 0, To: 1}, {Name: "M2", From: 2, To: 2}, {Name: "M3", From: 3, To: 3}}, got.Bands)
	assert.Equal(t, ForestPlace{2, 0}, got.Places["r"])
	assert.Equal(t, ForestPlace{3, 1.5}, got.Places["x"])
	assert.Equal(t, ForestPlace{3, 3}, got.Places["y"])
}

// TestForestLayout_AnEdgeOnThePathIsThePrimaryParent: the primary parent
// prefers an edge on the marked path over a deeper one, so the path draws
// as one run of tree edges; without a mark the deepest edge wins.
func TestForestLayout_AnEdgeOnThePathIsThePrimaryParent(t *testing.T) {
	t.Parallel()
	ids := []string{"p", "q", "r"}
	edges := []LayoutEdge{{"p", "q"}, {"q", "r"}, {"p", "r"}}
	plain := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackTree})
	assert.Equal(t, []EdgeRole{RoleTree, RoleTree, RoleCross}, plain.Roles)
	marked := ForestLayout(&ForestInput{IDs: ids, Edges: edges, OnPath: []bool{false, false, true}, Pack: PackTree})
	assert.Equal(t, []EdgeRole{RoleTree, RoleCross, RoleTree}, marked.Roles)
}

// TestForestLayout_ThePathRunsDownOneColumn: a parent sits over its child
// on the marked path rather than centred over all its children, and under
// row packing a node on the path aims at its parent's column, so the path
// draws as a vertical spine in both packings.
func TestForestLayout_ThePathRunsDownOneColumn(t *testing.T) {
	t.Parallel()
	ids := []string{"a", "b", "c", "d", "e"}
	edges := []LayoutEdge{{"a", "b"}, {"a", "c"}, {"a", "d"}, {"d", "e"}}
	onPath := []bool{false, false, true, true}
	for _, pack := range []ForestPacking{PackTree, PackRows} {
		got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, OnPath: onPath, Pack: pack})
		assert.InDelta(t, got.Places["d"].Col, got.Places["a"].Col, 0, pack)
		assert.InDelta(t, got.Places["d"].Col, got.Places["e"].Col, 0, pack)
	}
}

// TestForestLayout_GoldenLanes pins bands in lanes: Y1 depends only on X1,
// so with lanes it starts right after X1, beside X2, where stacked bands
// would put it after X2. A tree edge never joins two groups: y1b's parent
// is y1a in its own group, and its edge from x1a is a cross edge.
func TestForestLayout_GoldenLanes(t *testing.T) {
	t.Parallel()
	ids := []string{"x1a", "x1b", "x2a", "y1a", "y1b"}
	edges := []LayoutEdge{{"x1a", "x1b"}, {"x1b", "x2a"}, {"y1a", "y1b"}, {"x1a", "y1b"}}
	bandOf := map[string]string{"x1a": "X1", "x1b": "X1", "x2a": "X2", "y1a": "Y1", "y1b": "Y1"}
	groupOf := map[string]string{"x1a": "X", "x1b": "X", "x2a": "X", "y1a": "Y", "y1b": "Y"}
	in := ForestInput{IDs: ids, Edges: edges, Bands: []string{"X1", "X2", "Y1"}, BandOf: bandOf, GroupOf: groupOf, Pack: PackTree}
	stacked := ForestLayout(&in)
	assert.Equal(t, LayoutBand{Name: "Y1", From: 3, To: 4}, stacked.Bands[2])
	in.LaneOf = map[string]string{"X1": "X", "X2": "X", "Y1": "Y"}
	assert.Equal(t, Forest{
		Layers:  4,
		Columns: 5,
		Bands:   []LayoutBand{{Name: "X1", From: 0, To: 1}, {Name: "X2", From: 2, To: 2}, {Name: "Y1", From: 2, To: 3}},
		Roles:   []EdgeRole{RoleTree, RoleTree, RoleTree, RoleCross},
		Places: map[string]ForestPlace{
			"x1a": {0, 0}, "x1b": {1, 0}, "x2a": {2, 0}, "y1a": {2, 4}, "y1b": {3, 4},
		},
	}, ForestLayout(&in))
}

// TestForestLayout_GoldenRoots pins the forest's roots (decision 1 of
// spec/backlog/dashboard-graph.md): a0 has no predecessor and b0's only
// predecessors sit in another track, so both are roots, b0 packed in its
// own group's columns, and both edges into b0 are cross edges.
func TestForestLayout_GoldenRoots(t *testing.T) {
	t.Parallel()
	ids := []string{"a0", "a1", "b0"}
	edges := []LayoutEdge{{"a0", "a1"}, {"a0", "b0"}, {"a1", "b0"}}
	groupOf := map[string]string{"a0": "A", "a1": "A", "b0": "B"}
	assert.Equal(t, Forest{
		Layers:  3,
		Columns: 5,
		Roles:   []EdgeRole{RoleTree, RoleCross, RoleCross},
		Places:  map[string]ForestPlace{"a0": {0, 0}, "a1": {1, 0}, "b0": {2, 4}},
	}, ForestLayout(&ForestInput{IDs: ids, Edges: edges, GroupOf: groupOf, Pack: PackTree}))
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
	tree := ForestLayout(&ForestInput{IDs: ids, Edges: edges, GroupOf: groupOf, Pack: PackTree})
	assert.InDelta(t, 7.0, tree.Columns, 0)
	got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, GroupOf: groupOf, Pack: PackRows})
	assert.InDelta(t, 4.0, got.Columns, 0)
	assert.Equal(t, map[string]ForestPlace{
		"a0": {0, 0}, "b0": {0, 3},
		"a1": {1, 0}, "a2": {1, 1}, "b1": {1, 2}, "b2": {1, 3},
	}, got.Places)
}

// TestForestLayout_InvariantsOnARandomGraph checks, on a seeded random
// graph banded in three milestones and grouped in three tracks, under both
// packings, stacked and in lanes: no tree edge points backwards or joins
// two groups, every node with an edge from an earlier layer of its own
// group has exactly one tree edge in and any other none, no two nodes of a
// layer share a column, every column is inside the width, and no edge but
// a back edge points up.
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
	lanes := map[string]string{"M0": "a", "M1": "b", "M2": "a"}
	for _, laneOf := range []map[string]string{nil, lanes} {
		for _, pack := range []ForestPacking{PackTree, PackRows} {
			got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Bands: []string{"M0", "M1", "M2"}, BandOf: bandOf, GroupOf: groupOf, LaneOf: laneOf, Pack: pack})
			require.Len(t, got.Roles, len(edges))
			treeIn := map[string]int{}
			earlierIn := map[string]bool{}
			for i, e := range edges {
				from, to := got.Places[e.From], got.Places[e.To]
				if from.Layer < to.Layer && groupOf[e.From] == groupOf[e.To] {
					earlierIn[e.To] = true
				}
				switch got.Roles[i] {
				case RoleTree:
					assert.Less(t, from.Layer, to.Layer, "%s: tree edge %s->%s points backwards", pack, e.From, e.To)
					assert.Equal(t, groupOf[e.From], groupOf[e.To], "%s: tree edge %s->%s joins two groups", pack, e.From, e.To)
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
}

// TestForestLayout_IsStable: the same input lays out the same.
func TestForestLayout_IsStable(t *testing.T) {
	t.Parallel()
	ids := []string{"a1", "a2", "a3", "a4", "a5", "a6"}
	edges := []LayoutEdge{{"a1", "a2"}, {"a1", "a3"}, {"a2", "a4"}, {"a3", "a4"}, {"a4", "a5"}, {"a1", "a6"}}
	in := ForestInput{IDs: ids, Edges: edges, Pack: PackRows}
	first := ForestLayout(&in)
	for range 20 {
		assert.Equal(t, first, ForestLayout(&in))
	}
}

// TestForestLayout_GoldenDeepRowPacking pins the row packing of a deep,
// narrow graph: a ten-layer chain with four side leaves is five tree
// columns wide and at most two nodes per layer, so it packs into
// forestColumnsPerLayer's four columns rather than its fullest layer's two.
func TestForestLayout_GoldenDeepRowPacking(t *testing.T) {
	t.Parallel()
	ids := []string{"c0", "c1", "c2", "c3", "c4", "c5", "c6", "c7", "c8", "c9", "s1", "s3", "s5", "s7"}
	var edges []LayoutEdge
	for i := 1; i < 10; i++ {
		edges = append(edges, LayoutEdge{ids[i-1], ids[i]})
	}
	edges = append(edges, LayoutEdge{"c0", "s1"}, LayoutEdge{"c2", "s3"}, LayoutEdge{"c4", "s5"}, LayoutEdge{"c6", "s7"})
	got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackRows})
	assert.InDelta(t, 4.0, got.Columns, 0)
	assert.Equal(t, map[string]ForestPlace{
		"c0": {0, 2}, "c1": {1, 2}, "s1": {1, 3}, "c2": {2, 2}, "c3": {3, 1}, "s3": {3, 2}, "c4": {4, 1},
		"c5": {5, 0}, "s5": {5, 2}, "c6": {6, 0}, "c7": {7, 0}, "s7": {7, 1}, "c8": {8, 0}, "c9": {9, 0},
	}, got.Places)
}

// TestForestLayout_RowPackingALoneColumn: a chain one tree column wide
// packs into one column, every node at column 0.
func TestForestLayout_RowPackingALoneColumn(t *testing.T) {
	t.Parallel()
	got := ForestLayout(&ForestInput{IDs: []string{"a", "b"}, Edges: []LayoutEdge{{"a", "b"}}, Pack: PackRows})
	assert.Equal(t, Forest{
		Layers:  2,
		Columns: 1,
		Roles:   []EdgeRole{RoleTree},
		Places:  map[string]ForestPlace{"a": {0, 0}, "b": {1, 0}},
	}, got)
}

// TestForestLayout_RowPackingRoundsAHalfColumnUp: a parent centred half-way
// over two children two tree columns wide lands on the right child's
// column, math.Round taking a half away from zero.
func TestForestLayout_RowPackingRoundsAHalfColumnUp(t *testing.T) {
	t.Parallel()
	got := ForestLayout(&ForestInput{IDs: []string{"r", "a", "b"}, Edges: []LayoutEdge{{"r", "a"}, {"r", "b"}}, Pack: PackRows})
	assert.InDelta(t, 2.0, got.Columns, 0)
	assert.Equal(t, map[string]ForestPlace{"r": {0, 1}, "a": {1, 0}, "b": {1, 1}}, got.Places)
}

// TestForestLayout_RowPackingNothing: no nodes pack into no columns.
func TestForestLayout_RowPackingNothing(t *testing.T) {
	t.Parallel()
	got := ForestLayout(&ForestInput{Pack: PackRows})
	assert.InDelta(t, 0.0, got.Columns, 0)
	assert.Empty(t, got.Places)
}

// TestForestLayout_TheEarliestOfTwoEqualEdgesIsTheTreeEdge: an edge given
// twice ties on every rank, and the earlier one is the tree edge; with no
// path marked, ranking the first edge never reads past the missing marks.
func TestForestLayout_TheEarliestOfTwoEqualEdgesIsTheTreeEdge(t *testing.T) {
	t.Parallel()
	got := ForestLayout(&ForestInput{IDs: []string{"p", "q"}, Edges: []LayoutEdge{{"p", "q"}, {"p", "q"}}, Pack: PackTree})
	assert.Equal(t, []EdgeRole{RoleTree, RoleCross}, got.Roles)
}

// TestForestLayout_APathFromTheFirstNodeTakesItsParentsColumn: under row
// packing a squeezes into the column after s, and d, on the path from a
// by the first edge, takes a's squeezed column rather than its own scaled
// one, even though a is the first node in file order.
func TestForestLayout_APathFromTheFirstNodeTakesItsParentsColumn(t *testing.T) {
	t.Parallel()
	ids := []string{"a", "d", "r", "u", "s", "z1", "z2"}
	edges := []LayoutEdge{{"a", "d"}, {"u", "a"}, {"r", "s"}}
	groupOf := map[string]string{"a": "g", "d": "g", "r": "g", "u": "g", "s": "g", "z1": "h", "z2": "k"}
	got := ForestLayout(&ForestInput{IDs: ids, Edges: edges, GroupOf: groupOf, OnPath: []bool{true}, Pack: PackRows})
	assert.Equal(t, ForestPlace{1, 1}, got.Places["a"])
	assert.Equal(t, ForestPlace{2, 1}, got.Places["d"])
}
