package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// shelfSample is two trees and three lone nodes: d → e, d → f listed first,
// a → b → c second, a cross edge a → e that puts b's tree after a's, and
// g, h, i with no edge at all.
func shelfSample() ([]string, []LayoutEdge) {
	ids := []string{"d", "e", "f", "a", "b", "c", "g", "h", "i"}
	edges := []LayoutEdge{{"d", "e"}, {"d", "f"}, {"a", "b"}, {"b", "c"}, {"a", "e"}}
	return ids, edges
}

// TestShelveForest_StacksTreesOnShelvesNoWiderThanAsked: a tree a cross
// edge leaves from comes before the tree it enters, a tree too wide to sit
// beside the one placed goes under it, one empty layer between, each tree
// keeping its own columns and depth, and the lone nodes close the picture
// as a grid that width wide.
func TestShelveForest_StacksTreesOnShelvesNoWiderThanAsked(t *testing.T) {
	t.Parallel()
	ids, edges := shelfSample()
	f := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackTree})
	got := ShelveForest(ids, edges, f, 2)
	assert.Equal(t, Forest{
		Layers:  10,
		Columns: 2,
		Roles:   []EdgeRole{RoleTree, RoleTree, RoleTree, RoleTree, RoleCross},
		Places: map[string]ForestPlace{
			"a": {0, 0}, "b": {1, 0}, "c": {2, 0},
			"d": {4, 0}, "e": {5, 0}, "f": {6, 0},
			"g": {8, 0}, "h": {8, 1}, "i": {9, 0},
		},
	}, got)
}

// TestShelveForest_KeepsTreesOnOneShelfWhenTheyFit: wide enough, the trees
// sit side by side at the top, the second past the first's column and its
// tree gap, and the lone nodes' grid starts under the deepest of them.
func TestShelveForest_KeepsTreesOnOneShelfWhenTheyFit(t *testing.T) {
	t.Parallel()
	ids, edges := shelfSample()
	f := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackTree})
	got := ShelveForest(ids, edges, f, 4)
	assert.Equal(t, map[string]ForestPlace{
		"a": {0, 0}, "b": {1, 0}, "c": {2, 0},
		"d": {0, 2}, "e": {1, 2}, "f": {2, 2},
		"g": {4, 0}, "h": {4, 1}, "i": {4, 2},
	}, got.Places)
	assert.Equal(t, 5, got.Layers)
	assert.InDelta(t, 3.0, got.Columns, 0)
}

// fanSample is one parent r with three childless children l1-l3 and two
// children a and b with one child each, laid out by the forest.
func fanSample() ([]string, []LayoutEdge, Forest) {
	ids := []string{"r", "a", "a1", "b", "b1", "l1", "l2", "l3"}
	edges := []LayoutEdge{{"r", "a"}, {"a", "a1"}, {"r", "b"}, {"b", "b1"}, {"r", "l1"}, {"r", "l2"}, {"r", "l3"}}
	return ids, edges, ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackTree})
}

// TestShelveForest_ATreeStacksTheChildlessUnderAParent: a parent's
// childless children stack in one column, its children with children of
// their own sit side by side to its right, and the parent sits over the
// middle of them.
func TestShelveForest_ATreeStacksTheChildlessUnderAParent(t *testing.T) {
	t.Parallel()
	ids, edges, f := fanSample()
	got := ShelveForest(ids, edges, f, 4)
	want := map[string]ForestPlace{"r": {0, 1.5}, "l1": {1, 0}, "l2": {2, 0}, "l3": {3, 0}, "a": {1, 1.5}, "a1": {2, 1.5}, "b": {1, 3}, "b1": {2, 3}}
	assert.Equal(t, want, got.Places)
	assert.Equal(t, [2]float64{4, 4}, [2]float64{float64(got.Layers), got.Columns})
}

// TestShelveForest_ChildrenTooWideForTheWidthWrapUnderEachOther: a
// parent's children's blocks that would pass the width start a row under the
// row before, one empty layer between, and the parent sits over the first
// row, so a wide fan reads down the page.
func TestShelveForest_ChildrenTooWideForTheWidthWrapUnderEachOther(t *testing.T) {
	t.Parallel()
	ids, edges, f := fanSample()
	got := ShelveForest(ids, edges, f, 2)
	want := map[string]ForestPlace{"r": {0, 0}, "l1": {1, 0}, "l2": {2, 0}, "l3": {3, 0}, "a": {5, 0}, "a1": {6, 0}, "b": {8, 0}, "b1": {9, 0}}
	assert.Equal(t, want, got.Places)
	assert.Equal(t, [2]float64{10, 1}, [2]float64{float64(got.Layers), got.Columns})
}

// TestShelveForest_ShortTreesFillTheRoomBesideATallOne: a short tree sits
// under another short one beside a tall chain rather than under the chain,
// so one deep chain does not leave the rest of its rows empty.
func TestShelveForest_ShortTreesFillTheRoomBesideATallOne(t *testing.T) {
	t.Parallel()
	ids := []string{"p", "q", "r", "s", "u", "v", "x", "y"}
	edges := []LayoutEdge{{"p", "q"}, {"q", "r"}, {"r", "s"}, {"u", "v"}, {"x", "y"}}
	f := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackTree})
	got := ShelveForest(ids, edges, f, 3)
	assert.Equal(t, map[string]ForestPlace{
		"p": {0, 0}, "q": {1, 0}, "r": {2, 0}, "s": {3, 0},
		"u": {0, 2}, "v": {1, 2},
		"x": {3, 2}, "y": {4, 2},
	}, got.Places)
	assert.Equal(t, 5, got.Layers)
}

// TestShelveForest_AnEdgeThatEndsUpPointingUpIsABackEdge: two trees that
// each wait on the other cannot both come first; the edge the order leaves
// pointing up is drawn as a back edge.
func TestShelveForest_AnEdgeThatEndsUpPointingUpIsABackEdge(t *testing.T) {
	t.Parallel()
	ids := []string{"a", "b", "c", "d"}
	edges := []LayoutEdge{{"a", "b"}, {"c", "d"}, {"b", "d"}, {"d", "b"}}
	f := ForestLayout(&ForestInput{IDs: ids, Edges: edges, Pack: PackTree})
	got := ShelveForest(ids, edges, f, 1)
	ups := 0
	for i, e := range edges {
		if got.Roles[i] == RoleTree {
			continue
		}
		up := got.Places[e.To].Layer <= got.Places[e.From].Layer
		assert.Equal(t, up, got.Roles[i] == RoleBack, "%v", e)
		if up {
			ups++
		}
	}
	assert.Equal(t, 1, ups, "one of the two edges between the trees points up")
}
