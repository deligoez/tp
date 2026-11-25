package engine

import (
	"math"
	"slices"
)

// forestBlockGap is the empty columns between two blocks that share a
// layer.
const forestBlockGap = 1.0

// ForestBlocks is the blocks of f, which ForestLayout laid out from in with
// Blocks set: one per group that holds a node, in the order the bands name
// them (a band's Name, then its With), then any other group in the order
// of its first node, each with the layers and columns its nodes take and
// how many it holds.
func ForestBlocks(in *ForestInput, f *Forest) []ForestBlock {
	groups, members := blockGroups(in.IDs, in.GroupOf, f.Bands)
	out := make([]ForestBlock, len(groups))
	for i, g := range groups {
		b := ForestBlock{Group: g, FromLayer: math.MaxInt, ToLayer: -1, FromCol: math.Inf(1), ToCol: math.Inf(-1), Count: len(members[i])}
		for _, v := range members[i] {
			p := f.Places[in.IDs[v]]
			b.FromLayer, b.ToLayer = min(b.FromLayer, p.Layer), max(b.ToLayer, p.Layer)
			b.FromCol, b.ToCol = min(b.FromCol, p.Col), max(b.ToCol, p.Col)
		}
		out[i] = b
	}
	return out
}

// blockColumns is every node's column and the width of a forest laid out
// in blocks: the forest packed group by group, each group then moved to
// start at column 0 and, under PackRows, squeezed into rows on its own,
// aiming at forestColumnsPerLayer of its own layers, and the blocks placed
// by placeBlocks in blockGroups' order.
func blockColumns(in *ForestInput, t *forestTree, bands []LayoutBand) (col []float64, width float64) {
	col = packForest(in.IDs, t.layer, t.parent, t.children, t.spine, in.GroupOf)[:len(in.IDs)]
	_, members := blockGroups(in.IDs, in.GroupOf, bands)
	spans := make([][2]int, len(members))
	widths := make([]float64, len(members))
	for i, vs := range members {
		spans[i], widths[i] = packBlock(in, t, col, vs)
	}
	return col, placeBlocks(spans, widths, members, col)
}

// blockGroups is the groups that hold a node, in the order the bands name
// them (a band's Name, then its With), then any other in the order of its
// first node, and per group its nodes in file order.
func blockGroups(ids []string, groupOf map[string]string, bands []LayoutBand) (groups []string, members [][]int) {
	byGroup := map[string][]int{}
	var seen []string
	for v, id := range ids {
		g := groupOf[id]
		if _, ok := byGroup[g]; !ok {
			seen = append(seen, g)
		}
		byGroup[g] = append(byGroup[g], v)
	}
	add := func(g string) {
		if _, ok := byGroup[g]; ok && !slices.Contains(groups, g) {
			groups = append(groups, g)
		}
	}
	for _, b := range bands {
		add(b.Name)
		for _, w := range b.With {
			add(w)
		}
	}
	for _, g := range seen {
		add(g)
	}
	members = make([][]int, len(groups))
	for i, g := range groups {
		members[i] = byGroup[g]
	}
	return groups, members
}

// packBlock moves the nodes vs of one group to start at column 0 and
// squeezes them into rows under PackRows, returning the first and last
// layer they take and their width.
func packBlock(in *ForestInput, t *forestTree, col []float64, vs []int) (span [2]int, width float64) {
	span = [2]int{math.MaxInt, -1}
	left := math.Inf(1)
	for _, v := range vs {
		l := t.layer[in.IDs[v]]
		span = [2]int{min(span[0], l), max(span[1], l)}
		left = min(left, col[v])
	}
	for _, v := range vs {
		col[v] -= left
		width = max(width, col[v]+1)
	}
	if in.Pack == PackRows {
		width = squeezeRows(vs, in.IDs, t.layer, col, t.up, math.Ceil(forestColumnsPerLayer*float64(span[1]-span[0]+1)))
	}
	return span, width
}

// placeBlocks places the blocks, each spanning the layers spans names and
// widths wide, in order, each at the leftmost column, 0 or just past a
// placed block, where it keeps forestBlockGap columns from every placed
// block whose layers it shares, so blocks sharing layers sit side by side
// and one sharing none starts at the left edge; it moves the members'
// columns there and returns the width.
func placeBlocks(spans [][2]int, widths []float64, members [][]int, col []float64) float64 {
	at := make([]float64, len(spans))
	total := 0.0
	for i := range spans {
		at[i] = blockSlot(spans, widths, at, i)
		for _, v := range members[i] {
			col[v] += at[i]
		}
		total = max(total, at[i]+widths[i])
	}
	return total
}

// blockSlot is the leftmost column block i fits at beside the blocks
// before it (placed at at) whose layers it shares.
func blockSlot(spans [][2]int, widths, at []float64, i int) float64 {
	var near []int
	slots := []float64{0}
	for p := range i {
		if spans[p][0] <= spans[i][1] && spans[i][0] <= spans[p][1] {
			near = append(near, p)
			slots = append(slots, at[p]+widths[p]+forestBlockGap)
		}
	}
	slices.Sort(slots)
	for _, x := range slots {
		if !slices.ContainsFunc(near, func(p int) bool {
			return x < at[p]+widths[p]+forestBlockGap && at[p] < x+widths[i]+forestBlockGap
		}) {
			return x
		}
	}
	return slots[len(slots)-1]
}
