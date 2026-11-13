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

// forestColumnsPerLayer is how many columns per layer the row packing aims
// at: a deep, narrow graph squeezed to its fullest layer alone comes out as
// a tall strip, so the packing keeps at least this many columns per layer,
// up to the tree width, and the picture stays closer to a screen's shape.
const forestColumnsPerLayer = 0.4

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

// ForestInput is what ForestLayout lays out: IDs in file order, Edges
// between them, Bands and BandOf as LayeredLayout takes them, GroupOf
// naming each node's group (nil for one group), LaneOf naming each band's
// lane (nil to stack every band after the last) and Pack.
type ForestInput struct {
	IDs     []string
	Edges   []LayoutEdge
	Bands   []string
	BandOf  map[string]string
	GroupOf map[string]string
	LaneOf  map[string]string
	Pack    ForestPacking
}

// ForestLayout lays in.IDs out as a forest packed into columns, after
// lulus's layout. Layers come from the bands as LayeredLayout makes them: a
// band is a run of consecutive layers, bands in a cycle share one run, and
// inside a run a node's layer is its longest path over the run's edges.
// With LaneOf nil every run starts after the one before it, as
// LayeredLayout stacks them; with lanes, a run starts right after the runs
// it depends on and the run before it in its own lane, so runs of different
// lanes share layers. Each node then picks one primary parent among the
// edges into it from an earlier layer and its own group, the one from the
// deepest layer, the earliest in file order on a tie; those edges form a
// forest (RoleTree), every other edge pointing down is RoleCross and one
// that does not is RoleBack. The forest's roots are grouped by GroupOf,
// groups in the order of their first node and roots inside a group by
// layer then file order, and packed left to right as Pack says. An edge
// naming an id not in IDs, or a node to itself, is laid out as no edge and
// its role is RoleBack. One input always lays out alike.
func ForestLayout(in *ForestInput) Forest {
	ids, edges := in.IDs, in.Edges
	file := make(map[string]int, len(ids))
	for i, id := range ids {
		file[id] = i
	}
	layer, spans := forestLayers(in, file)
	out := Forest{Places: make(map[string]ForestPlace, len(ids))}
	for _, s := range spans {
		if s.Name != "" || len(spans) > 1 {
			out.Bands = append(out.Bands, s)
		}
		out.Layers = max(out.Layers, s.To+1)
	}
	parent := primaryParents(edges, file, layer, in.GroupOf)
	var children [][]int
	out.Roles, children = forestRoles(edges, file, layer, parent)
	col := packForest(ids, layer, parent, children, in.GroupOf)
	out.Columns = col[len(ids)]
	col = col[:len(ids)]
	if in.Pack == PackRows {
		out.Columns = squeezeRows(ids, layer, col, math.Ceil(forestColumnsPerLayer*float64(out.Layers)))
	}
	for i, id := range ids {
		out.Places[id] = ForestPlace{Layer: layer[id], Col: col[i]}
	}
	return out
}

// forestLayers is each node's layer and the bands' spans: in.Bands run as
// LayeredLayout runs them over the edges forestEdgeKept keeps, stacked, or
// in lanes when in.LaneOf names them.
func forestLayers(in *ForestInput, file map[string]int) (map[string]int, []LayoutBand) {
	kept := make([]LayoutEdge, 0, len(in.Edges))
	for _, e := range in.Edges {
		if forestEdgeKept(file, e) {
			kept = append(kept, e)
		}
	}
	bands, bandOf := in.Bands, in.BandOf
	if bands == nil {
		bands, bandOf = []string{""}, map[string]string{}
	}
	runs := bandRuns(bandList(bands, in.IDs, bandOf), bandOf, kept)
	if in.LaneOf == nil {
		return bandLayers(in.IDs, kept, runs, bandOf)
	}
	return laneLayers(in.IDs, kept, runs, bandOf, in.LaneOf)
}

// forestRoles is each edge's role, and per node in file order its children
// in the forest, sorted: an edge that is its dependent's primary parent is
// a tree edge, any other pointing down the layers a cross edge, and the
// rest back edges.
func forestRoles(edges []LayoutEdge, file, layer map[string]int, parent []int) (roles []EdgeRole, children [][]int) {
	roles = make([]EdgeRole, len(edges))
	children = make([][]int, len(file))
	for i, e := range edges {
		switch {
		case !forestEdgeKept(file, e) || layer[e.From] >= layer[e.To]:
			roles[i] = RoleBack
		case parent[file[e.To]] == i:
			roles[i] = RoleTree
			children[file[e.From]] = append(children[file[e.From]], file[e.To])
		default:
			roles[i] = RoleCross
		}
	}
	for _, kids := range children {
		slices.Sort(kids)
	}
	return roles, children
}

// laneLayers is bandLayers with runs in lanes: a run starts after every run
// an edge makes it depend on and after the run before it in its lane, the
// lane of its first band, instead of after every run before it. runs are in
// dependency order, so each run's start is known when it is reached.
func laneLayers(ids []string, edges []LayoutEdge, runs [][]string, bandOf, laneOf map[string]string) (map[string]int, []LayoutBand) {
	runOf, held := nodeRuns(ids, runs, bandOf)
	local := longestPaths(ids, edges, runOf)
	width := make([]int, len(runs))
	for _, id := range ids {
		width[runOf[id]] = max(width[runOf[id]], local[id]+1)
	}
	deps := make([][]int, len(runs))
	for _, e := range edges {
		if a, b := runOf[e.From], runOf[e.To]; a != b {
			deps[b] = append(deps[b], a)
		}
	}
	start := make([]int, len(runs))
	lastInLane := map[string]int{}
	var spans []LayoutBand
	for r, members := range runs {
		lane := laneOf[members[0]]
		if p, ok := lastInLane[lane]; ok {
			deps[r] = append(deps[r], p)
		}
		for _, d := range deps[r] {
			start[r] = max(start[r], start[d]+width[d])
		}
		if width[r] == 0 {
			continue
		}
		lastInLane[lane] = r
		spans = append(spans, runSpan(members, held, start[r], width[r]))
	}
	layer := make(map[string]int, len(ids))
	for _, id := range ids {
		layer[id] = start[runOf[id]] + local[id]
	}
	return layer, spans
}

// nodeRuns is the run each node's band is in, and the bands that hold a
// node.
func nodeRuns(ids []string, runs [][]string, bandOf map[string]string) (runOf map[string]int, held map[string]bool) {
	runOfBand := map[string]int{}
	for r, members := range runs {
		for _, b := range members {
			runOfBand[b] = r
		}
	}
	runOf = make(map[string]int, len(ids))
	held = map[string]bool{}
	for _, id := range ids {
		runOf[id], held[bandOf[id]] = runOfBand[bandOf[id]], true
	}
	return runOf, held
}

// runSpan is the span of a run of members from start, width layers wide,
// named for the first member that holds a node and with the others that do.
func runSpan(members []string, held map[string]bool, start, width int) LayoutBand {
	named := slices.DeleteFunc(slices.Clone(members), func(b string) bool { return !held[b] })
	span := LayoutBand{Name: named[0], From: start, To: start + width - 1}
	if len(named) > 1 {
		span.With = named[1:]
	}
	return span
}

// forestEdgeKept reports whether e joins two different ids of file.
func forestEdgeKept(file map[string]int, e LayoutEdge) bool {
	_, from := file[e.From]
	_, to := file[e.To]
	return from && to && e.From != e.To
}

// primaryParents is, per node in file order, the index of its primary
// parent's edge, or -1 for a root: of the edges into it from an earlier
// layer and a node of its own group, the one from the deepest layer, then
// the earliest dependency in file order, then the earliest edge.
func primaryParents(edges []LayoutEdge, file, layer map[string]int, groupOf map[string]string) []int {
	parent := slices.Repeat([]int{-1}, len(file))
	for i, e := range edges {
		if !forestEdgeKept(file, e) || layer[e.From] >= layer[e.To] || groupOf[e.From] != groupOf[e.To] {
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

// squeezeRows squeezes every layer into the width of the fullest one, or
// aim when that is wider and the tree is wider still, and returns that
// width. Inside a layer nodes keep their tree order; each aims at its tree
// column scaled into the new width, a left-to-right sweep pushes
// overlapping nodes one column apart and a right-to-left sweep pulls back
// what ran past the right edge.
func squeezeRows(ids []string, layer map[string]int, col []float64, aim float64) float64 {
	rows := map[int][]int{}
	treeWidth := 0.0
	fullest := 0
	for v, id := range ids {
		rows[layer[id]] = append(rows[layer[id]], v)
		treeWidth = max(treeWidth, col[v]+1)
		fullest = max(fullest, len(rows[layer[id]]))
	}
	width := max(float64(fullest), min(aim, math.Ceil(treeWidth)))
	scale := 0.0
	if treeWidth > 1 {
		scale = max(0, width-1) / (treeWidth - 1)
	}
	for _, r := range rows {
		squeezeRow(r, col, scale, width)
	}
	return width
}

// squeezeRow places one layer's nodes r inside width: in tree order, each
// at its column times scale, pushed one column past the one before it, then
// pulled back from the right edge.
func squeezeRow(r []int, col []float64, scale, width float64) {
	slices.SortFunc(r, func(a, b int) int { return cmp.Or(cmp.Compare(col[a], col[b]), cmp.Compare(a, b)) })
	placed := make([]float64, len(r))
	for i, v := range r {
		placed[i] = math.Round(col[v] * scale)
		if i > 0 {
			placed[i] = max(placed[i], placed[i-1]+1)
		}
	}
	ceiling := width - 1
	for i := len(r) - 1; i >= 0; i-- {
		placed[i] = min(placed[i], ceiling)
		ceiling = placed[i] - 1
	}
	for i, v := range r {
		col[v] = placed[i]
	}
}
