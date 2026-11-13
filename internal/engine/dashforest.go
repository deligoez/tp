package engine

import (
	"cmp"
	"math"
	"slices"
)

// EdgeRole is what an edge is to a forest layout's picture.
type EdgeRole string

// The three edge roles: the primary-parent edge the columns are packed
// from, any other edge pointing down the layers, and an edge that points
// up or along a layer because a cycle left no other way to lay it out.
const (
	RoleTree  EdgeRole = "tree"
	RoleCross EdgeRole = "cross"
	RoleBack  EdgeRole = "back"
)

// ForestPacking is how a forest layout assigns columns once the layers are
// known.
type ForestPacking string

// PackTree gives every leaf of the primary-parent forest its own column and
// centres each parent over its children, so subtrees never overlap; a
// shallow graph with many leaves comes out as a wide strip. PackRows keeps
// that left-to-right order but squeezes every layer into the width of the
// fullest one, so the picture is as wide as its widest layer.
const (
	PackTree ForestPacking = "tree"
	PackRows ForestPacking = "rows"
)

// The empty columns between two trees of one group, and between two groups.
const (
	forestTreeGap  = 0.5
	forestGroupGap = 3.0
)

// ForestPlace is a node's place in a forest layout: its layer, top to
// bottom, and its column, left to right, possibly half-way between two.
type ForestPlace struct {
	Layer int     `json:"layer"`
	Col   float64 `json:"col"`
}

// Forest is a forest layout: each node's place, each given edge's role in
// the order given, the bands as LayeredLayout makes them, the number of
// layers and the width in columns.
type Forest struct {
	Places  map[string]ForestPlace
	Roles   []EdgeRole
	Bands   []LayoutBand
	Layers  int
	Columns float64
}

// ForestLayout lays ids, given in file order, out as a forest packed into
// columns, after lulus's layout: layers are LayeredLayout's, bands and
// bandOf taken as it takes them, so a milestone stays a run of consecutive
// layers and bands in a cycle share one run. Each node then picks one
// primary parent among the edges into it from an earlier layer, the one
// from the deepest layer, the earliest in file order on a tie; those edges
// form a forest (RoleTree), every other edge pointing down is RoleCross and
// one that does not is RoleBack. The forest's roots are grouped by groupOf
// (nil for one group), groups in the order of their first node in ids and
// roots inside a group by layer then file order, and packed left to right
// as pack says. An edge naming an id not in ids, or a node to itself, is
// laid out as no edge and its role is RoleBack. One input always lays out
// alike.
func ForestLayout(ids []string, edges []LayoutEdge, bands []string, bandOf, groupOf map[string]string, pack ForestPacking) Forest {
	file := make(map[string]int, len(ids))
	for i, id := range ids {
		file[id] = i
	}
	kept := make([]LayoutEdge, 0, len(edges))
	for _, e := range edges {
		if forestEdgeKept(file, e) {
			kept = append(kept, e)
		}
	}
	if bands == nil {
		bands, bandOf = []string{""}, map[string]string{}
	}
	layer, spans := bandLayers(ids, kept, bandRuns(bandList(bands, ids, bandOf), bandOf, kept), bandOf)
	out := Forest{Places: make(map[string]ForestPlace, len(ids)), Roles: make([]EdgeRole, len(edges))}
	for _, s := range spans {
		if s.Name != "" || len(spans) > 1 {
			out.Bands = append(out.Bands, s)
		}
		out.Layers = max(out.Layers, s.To+1)
	}
	parent := primaryParents(edges, file, layer)
	children := make([][]int, len(ids))
	for i, e := range edges {
		switch {
		case !forestEdgeKept(file, e) || layer[e.From] >= layer[e.To]:
			out.Roles[i] = RoleBack
		case parent[file[e.To]] == i:
			out.Roles[i] = RoleTree
			children[file[e.From]] = append(children[file[e.From]], file[e.To])
		default:
			out.Roles[i] = RoleCross
		}
	}
	for _, kids := range children {
		slices.Sort(kids)
	}
	col := packForest(ids, layer, parent, children, groupOf)
	out.Columns = col[len(ids)]
	col = col[:len(ids)]
	if pack == PackRows {
		out.Columns = squeezeRows(ids, layer, col)
	}
	for i, id := range ids {
		out.Places[id] = ForestPlace{Layer: layer[id], Col: col[i]}
	}
	return out
}

// forestEdgeKept reports whether e joins two different ids of file.
func forestEdgeKept(file map[string]int, e LayoutEdge) bool {
	_, from := file[e.From]
	_, to := file[e.To]
	return from && to && e.From != e.To
}

// primaryParents is, per node in file order, the index of its primary
// parent's edge, or -1 for a root: of the edges into it from an earlier
// layer, the one from the deepest layer, then the earliest dependency in
// file order, then the earliest edge.
func primaryParents(edges []LayoutEdge, file map[string]int, layer map[string]int) []int {
	parent := slices.Repeat([]int{-1}, len(file))
	for i, e := range edges {
		if !forestEdgeKept(file, e) || layer[e.From] >= layer[e.To] {
			continue
		}
		v := file[e.To]
		if p := parent[v]; p < 0 || cmp.Or(cmp.Compare(layer[e.From], layer[edges[p].From]), cmp.Compare(file[edges[p].From], file[e.From])) > 0 {
			parent[v] = i
		}
	}
	return parent
}

// packForest gives every node its tree column, the roots grouped by groupOf
// and packed group by group, and returns the columns with the width after
// them, one past the last node's index.
func packForest(ids []string, layer map[string]int, parent []int, children [][]int, groupOf map[string]string) []float64 {
	var groups []string
	roots := map[string][]int{}
	for v, id := range ids {
		if parent[v] >= 0 {
			continue
		}
		g := groupOf[id]
		if _, seen := roots[g]; !seen {
			groups = append(groups, g)
		}
		roots[g] = append(roots[g], v)
	}
	col := make([]float64, len(ids)+1)
	cursor := 0.0
	for gi, g := range groups {
		if gi > 0 {
			cursor += forestGroupGap
		}
		rs := roots[g]
		slices.SortStableFunc(rs, func(a, b int) int { return cmp.Compare(layer[ids[a]], layer[ids[b]]) })
		for t, r := range rs {
			if t > 0 {
				cursor += forestTreeGap
			}
			packTree(r, children, col, &cursor)
		}
	}
	col[len(ids)] = cursor
	return col
}

// packTree places the tree under root: a leaf takes the next column, a
// parent sits centred over its first and last child.
func packTree(root int, children [][]int, col []float64, cursor *float64) {
	type frame struct {
		v        int
		expanded bool
	}
	stack := []frame{{root, false}}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		kids := children[f.v]
		switch {
		case len(kids) == 0:
			col[f.v] = *cursor
			*cursor++
		case f.expanded:
			col[f.v] = (col[kids[0]] + col[kids[len(kids)-1]]) / 2
		default:
			stack = append(stack, frame{f.v, true})
			for i := len(kids) - 1; i >= 0; i-- {
				stack = append(stack, frame{kids[i], false})
			}
		}
	}
}

// squeezeRows squeezes every layer into the width of the fullest one and
// returns that width. Inside a layer nodes keep their tree order; each aims
// at its tree column scaled into the new width, a left-to-right sweep
// pushes overlapping nodes one column apart and a right-to-left sweep pulls
// back what ran past the right edge.
func squeezeRows(ids []string, layer map[string]int, col []float64) float64 {
	rows := map[int][]int{}
	treeWidth := 0.0
	for v, id := range ids {
		rows[layer[id]] = append(rows[layer[id]], v)
		treeWidth = max(treeWidth, col[v]+1)
	}
	fullest := 0
	for _, r := range rows {
		fullest = max(fullest, len(r))
	}
	width := float64(fullest)
	scale := 0.0
	if treeWidth > 1 {
		scale = max(0, width-1) / (treeWidth - 1)
	}
	for _, r := range rows {
		slices.SortFunc(r, func(a, b int) int { return cmp.Or(cmp.Compare(col[a], col[b]), cmp.Compare(a, b)) })
		placed := make([]float64, len(r))
		for i, v := range r {
			placed[i] = math.Round(col[v] * scale)
			if i > 0 {
				placed[i] = max(placed[i], placed[i-1]+1)
			}
		}
		for i := len(r) - 1; i >= 0; i-- {
			ceiling := width - 1
			if i+1 < len(r) {
				ceiling = placed[i+1] - 1
			}
			placed[i] = min(placed[i], ceiling)
		}
		for i, v := range r {
			col[v] = placed[i]
		}
	}
	return width
}
