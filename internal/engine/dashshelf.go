package engine

import (
	"cmp"
	"math"
	"slices"
)

// shelfTree is one tree of a laid-out forest: its nodes' indexes in file
// order and the columns and layers it spans.
type shelfTree struct {
	nodes          []int
	minCol, maxCol float64
	minLay, maxLay int
}

// ShelveForest lays out taller than wide a forest ForestLayout packed with
// PackTree. Its trees, the nodes its tree edges join, are each laid out
// narrow, no wider than width where a chain allows (compactBlock), and
// stacked into width columns (or the widest tree's, when that is wider),
// each where it
// sits highest without passing an edge from a tree placed before it: a
// tree an edge leaves from is placed before the tree it enters where the
// edges allow, else in the forest's order, and the tree the edge enters
// sits low enough that the edge points down. Trees stay a tree gap apart across and one empty layer
// apart down. The nodes no edge touches close the picture as a grid width
// wide, in file order, one empty layer below the trees. A tree edge keeps
// its role; any other is RoleCross when it points down and RoleBack when it
// does not. The result has no bands.
func ShelveForest(ids []string, edges []LayoutEdge, f Forest, width float64) Forest {
	index := make(map[string]int, len(ids))
	for i, id := range ids {
		index[id] = i
	}
	trees, lone, treeOf := shelfTrees(ids, edges, f, index)
	width = math.Max(1, math.Floor(width))
	rel := compactTrees(ids, edges, f, trees, width)
	for _, t := range trees {
		width = math.Max(width, math.Ceil(t.maxCol-t.minCol+1))
	}
	out := Forest{Places: make(map[string]ForestPlace, len(ids))}
	sk := skyline{ids: ids, from: rel, to: out.Places, top: make([]int, len(trees)), sky: make([]int, int(width))}
	for k := range sk.top {
		sk.top[k] = -1
	}
	for _, k := range shelfOrder(len(trees), edges, treeOf, index) {
		sk.place(trees, k, sk.lowest(edges, treeOf, index, trees, k))
	}
	end := slices.Max(append(sk.sky, 0))
	out.Layers, out.Columns = max(0, end-1), sk.wide
	if len(lone) > 0 {
		per := int(width)
		for i, v := range lone {
			out.Places[ids[v]] = ForestPlace{Layer: end + i/per, Col: float64(i % per)}
		}
		out.Layers = end + (len(lone)-1)/per + 1
		out.Columns = math.Max(sk.wide, math.Min(width, float64(len(lone))))
	}
	out.Roles = shelfRoles(edges, f.Roles, out.Places)
	return out
}

// shelfTrees splits the forest into its trees, ordered by their leftmost
// column as the forest packed them, the nodes no edge touches, and each
// node's tree, -1 for a lone node.
func shelfTrees(ids []string, edges []LayoutEdge, f Forest, index map[string]int) (trees []shelfTree, lone, treeOf []int) {
	root := make([]int, len(ids))
	for i := range root {
		root[i] = i
	}
	find := func(v int) int {
		for root[v] != v {
			root[v] = root[root[v]]
			v = root[v]
		}
		return v
	}
	touched := make([]bool, len(ids))
	for i, e := range edges {
		a, okA := index[e.From]
		b, okB := index[e.To]
		if !okA || !okB || a == b {
			continue
		}
		touched[a], touched[b] = true, true
		if f.Roles[i] == RoleTree {
			root[find(a)] = find(b)
		}
	}
	at := map[int]int{}
	for v, id := range ids {
		if !touched[v] {
			lone = append(lone, v)
			continue
		}
		p := f.Places[id]
		k, seen := at[find(v)]
		if !seen {
			k = len(trees)
			at[find(v)] = k
			trees = append(trees, shelfTree{minCol: p.Col, maxCol: p.Col, minLay: p.Layer, maxLay: p.Layer})
		}
		t := &trees[k]
		t.nodes = append(t.nodes, v)
		t.minCol, t.maxCol = math.Min(t.minCol, p.Col), math.Max(t.maxCol, p.Col)
		t.minLay, t.maxLay = min(t.minLay, p.Layer), max(t.maxLay, p.Layer)
	}
	slices.SortStableFunc(trees, func(a, b shelfTree) int { return cmp.Compare(a.minCol, b.minCol) })
	treeOf = make([]int, len(ids))
	for v := range treeOf {
		treeOf[v] = -1
	}
	for k, t := range trees {
		for _, v := range t.nodes {
			treeOf[v] = k
		}
	}
	return trees, lone, treeOf
}

// compactTrees lays each tree out narrow, each node's place from its tree's
// top-left corner (compactBlock), and sets each tree's span to it.
func compactTrees(ids []string, edges []LayoutEdge, f Forest, trees []shelfTree, limit float64) map[string]ForestPlace {
	kids := map[string][]string{}
	hasParent := map[string]bool{}
	for i, e := range edges {
		if f.Roles[i] == RoleTree {
			kids[e.From] = append(kids[e.From], e.To)
			hasParent[e.To] = true
		}
	}
	for p := range kids {
		slices.SortStableFunc(kids[p], func(a, b string) int { return cmp.Compare(f.Places[a].Col, f.Places[b].Col) })
	}
	rel := map[string]ForestPlace{}
	for k := range trees {
		t := &trees[k]
		t.minCol, t.maxCol, t.minLay, t.maxLay = 0, 0, 0, 0
		for _, v := range t.nodes {
			if !hasParent[ids[v]] {
				b := compactBlock(ids[v], kids, limit)
				for id, p := range b.places {
					rel[id] = p
				}
				t.maxCol, t.maxLay = b.width-1, b.height-1
			}
		}
	}
	return rel
}

// block is a node and what hangs under it, each place from the block's
// top-left corner, and the block's size.
type block struct {
	places map[string]ForestPlace
	width  float64
	height int
	anchor float64
}

// compactBlock lays id and what hangs under it out narrow. Under id come
// its childless children stacked one under the other in one column, so a
// node many tasks wait on reads down rather than across, then the blocks
// of its children with children of their own in the forest's column order,
// each laid out alike. The blocks go side by side a tree gap
// apart while they fit limit, else start a row under the tallest of the row
// before, one empty layer between. id sits over the middle of the first
// row's first and last child.
func compactBlock(id string, kids map[string][]string, limit float64) block {
	stack := block{places: map[string]ForestPlace{}, width: 1}
	var parts []block
	for _, c := range kids[id] {
		if len(kids[c]) > 0 {
			parts = append(parts, compactBlock(c, kids, limit))
		} else {
			stack.places[c] = ForestPlace{Layer: stack.height}
			stack.height++
		}
	}
	if stack.height > 0 {
		parts = append([]block{stack}, parts...)
	}
	out := block{places: map[string]ForestPlace{}, width: 1, height: 1}
	cursor, top, rowH := 0.0, 1, 0
	first, last := -1.0, -1.0
	for _, b := range parts {
		if cursor > 0 && cursor+forestTreeGap+b.width > limit {
			top, cursor, rowH = top+rowH+1, 0, 0
		} else if cursor > 0 {
			cursor += forestTreeGap
		}
		for n, p := range b.places {
			out.places[n] = ForestPlace{Layer: top + p.Layer, Col: cursor + p.Col}
		}
		if top == 1 {
			if first < 0 {
				first = cursor + b.anchor
			}
			last = cursor + b.anchor
		}
		cursor += b.width
		rowH = max(rowH, b.height)
		out.width = math.Max(out.width, cursor)
		out.height = max(out.height, top+rowH)
	}
	if first >= 0 {
		out.anchor = (first + last) / 2
	}
	out.places[id] = ForestPlace{Col: out.anchor}
	return out
}

// crossing calls fn with the trees and nodes of every edge between two
// different trees.
func crossing(edges []LayoutEdge, treeOf []int, index map[string]int, fn func(ta, tb, a, b int)) {
	for _, e := range edges {
		a, okA := index[e.From]
		b, okB := index[e.To]
		if okA && okB && treeOf[a] >= 0 && treeOf[b] >= 0 && treeOf[a] != treeOf[b] {
			fn(treeOf[a], treeOf[b], a, b)
		}
	}
}

// shelfOrder is the n trees' indexes with each one an edge leaves from
// before the one it enters: next comes the first tree in the forest's order
// that waits on no tree still to come, or, when every remaining tree waits
// on one, the first remaining.
func shelfOrder(n int, edges []LayoutEdge, treeOf []int, index map[string]int) []int {
	waits := make([][]int, n)
	crossing(edges, treeOf, index, func(ta, tb, _, _ int) { waits[tb] = append(waits[tb], ta) })
	done := make([]bool, n)
	free := func(k int) bool {
		return !slices.ContainsFunc(waits[k], func(w int) bool { return !done[w] })
	}
	out := make([]int, 0, n)
	for len(out) < n {
		next := -1
		for k := range n {
			if done[k] {
				continue
			}
			if next < 0 {
				next = k
			}
			if free(k) {
				next = k
				break
			}
		}
		done[next] = true
		out = append(out, next)
	}
	return out
}

// skyline stacks trees into its columns: sky is, per column, the first
// layer free below what is placed there, top each placed tree's first
// layer (-1 while it is not placed), wide the rightmost column used.
type skyline struct {
	ids      []string
	from, to map[string]ForestPlace
	top      []int
	sky      []int
	wide     float64
}

// lowest is the first layer tree k may start at: below every node an edge
// into it leaves from in a tree already placed.
func (s *skyline) lowest(edges []LayoutEdge, treeOf []int, index map[string]int, trees []shelfTree, k int) int {
	floor := 0
	crossing(edges, treeOf, index, func(ta, tb, a, b int) {
		if tb != k || s.top[ta] < 0 || ta == k {
			return
		}
		layerA := s.to[s.ids[a]].Layer
		relB := s.from[s.ids[b]].Layer - trees[k].minLay
		floor = max(floor, layerA-relB+1)
	})
	return floor
}

// place puts tree k at the columns where it starts highest, no higher than
// floor, the leftmost of equals.
func (s *skyline) place(trees []shelfTree, k, floor int) {
	t := trees[k]
	w := t.maxCol - t.minCol + 1
	span := int(math.Ceil(w))
	bestX, bestTop := 0, math.MaxInt
	for x := 0; x+span <= len(s.sky); x++ {
		top := max(floor, slices.Max(s.sky[x:x+span]))
		if top < bestTop {
			bestX, bestTop = x, top
		}
	}
	s.top[k] = bestTop
	for _, v := range t.nodes {
		p := s.from[s.ids[v]]
		s.to[s.ids[v]] = ForestPlace{Layer: bestTop + p.Layer - t.minLay, Col: float64(bestX) + p.Col - t.minCol}
	}
	reach := min(len(s.sky), bestX+int(math.Ceil(w+forestTreeGap)))
	for x := bestX; x < reach; x++ {
		s.sky[x] = bestTop + t.maxLay - t.minLay + 2
	}
	s.wide = math.Max(s.wide, float64(bestX)+w)
}

// shelfRoles keeps each tree edge's role and names any other RoleCross when
// it points down on the shelves and RoleBack when it does not.
func shelfRoles(edges []LayoutEdge, roles []EdgeRole, places map[string]ForestPlace) []EdgeRole {
	out := make([]EdgeRole, len(edges))
	for i, e := range edges {
		from, okA := places[e.From]
		to, okB := places[e.To]
		switch {
		case roles[i] == RoleTree:
			out[i] = RoleTree
		case okA && okB && to.Layer > from.Layer && e.From != e.To:
			out[i] = RoleCross
		default:
			out[i] = RoleBack
		}
	}
	return out
}
