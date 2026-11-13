package engine

import (
	"cmp"
	"slices"
)

// LayoutEdge is one edge to lay out: From, the dependency, is drawn left of
// To, its dependent.
type LayoutEdge struct {
	From string
	To   string
}

// LayoutPlace is a node's place in a layered layout: its layer, left to
// right, and its order inside the layer, top to bottom, both from 0.
type LayoutPlace struct {
	Layer int `json:"layer"`
	Order int `json:"order"`
}

// LayoutBand is one band of a banded layout: the layers From to To,
// inclusive, that hold the nodes of the band Name and of the bands With.
// With is empty unless bands depend on each other in a cycle, through
// different nodes: those bands share one span, Name the first of them in
// the given order and With the rest.
type LayoutBand struct {
	Name string   `json:"milestone"`
	With []string `json:"with,omitempty"`
	From int      `json:"from_layer"`
	To   int      `json:"to_layer"`
}

// Layout is a layered layout (spec/backlog/dashboard-ux.md §2): each node's
// place, the bands in their order (none for an unbanded layout) and the
// number of layers.
type Layout struct {
	Places map[string]LayoutPlace
	Bands  []LayoutBand
	Layers int
}

// layoutSweeps is how many barycenter sweeps order the layers, alternating
// down and up.
const layoutSweeps = 4

// LayeredLayout lays out ids, given in file order, over edges between them;
// an edge naming an id not in ids is ignored. With bands nil every node
// shares one band; otherwise bandOf names each node's band, and the bands
// become runs of consecutive layers in dependency order, bands breaking a
// tie in the order given; a band named twice counts once, and a band a
// node names that bands leaves out is added after them. Bands that depend
// on each other in a cycle, through different nodes, share one run, so no
// edge points back. Inside a run a node's layer is its longest path from
// the run's sources over the run's own edges. Each layer is then ordered
// by layoutSweeps barycenter sweeps, ties broken by file order, so one
// input always lays out alike.
func LayeredLayout(ids []string, edges []LayoutEdge, bands []string, bandOf map[string]string) Layout {
	file := make(map[string]int, len(ids))
	for i, id := range ids {
		file[id] = i
	}
	kept := make([]LayoutEdge, 0, len(edges))
	for _, e := range edges {
		if _, ok := file[e.From]; ok {
			if _, ok := file[e.To]; ok && e.From != e.To {
				kept = append(kept, e)
			}
		}
	}
	if bands == nil {
		bands, bandOf = []string{""}, map[string]string{}
	}
	out := Layout{Places: make(map[string]LayoutPlace, len(ids))}
	layer, spans := bandLayers(ids, kept, bandRuns(bandList(bands, ids, bandOf), bandOf, kept), bandOf)
	for _, s := range spans {
		if s.Name != "" || len(spans) > 1 {
			out.Bands = append(out.Bands, s)
		}
		out.Layers = max(out.Layers, s.To+1)
	}
	for l, row := range orderLayers(ids, kept, layer, file, out.Layers) {
		for i, id := range row {
			out.Places[id] = LayoutPlace{Layer: l, Order: i}
		}
	}
	return out
}

// bandList is bands with each named once, where it is first named, then
// each band a node of ids names that bands leaves out, in ids order.
func bandList(bands, ids []string, bandOf map[string]string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(bands))
	add := func(b string) {
		if !seen[b] {
			seen[b] = true
			out = append(out, b)
		}
	}
	for _, b := range bands {
		add(b)
	}
	for _, id := range ids {
		add(bandOf[id])
	}
	return out
}

// bandRuns groups bands, each named once, into runs of layers and orders
// them: bands that reach each other through edges, a cycle of the band
// graph, make one run, listed in the given order; a run follows every run
// it depends on, the run holding the earliest given band first among the
// ready ones.
func bandRuns(bands []string, bandOf map[string]string, edges []LayoutEdge) [][]string {
	next := bandEdges(bands, bandOf, edges)
	run, count := bandComponents(next)
	after := make([][]bool, count)
	for r := range after {
		after[r] = make([]bool, count)
	}
	for a := range next {
		for b, on := range next[a] {
			if on && run[a] != run[b] {
				after[run[b]][run[a]] = true
			}
		}
	}
	out := make([][]string, 0, count)
	for _, r := range runOrder(after) {
		var members []string
		for i, b := range bands {
			if run[i] == r {
				members = append(members, b)
			}
		}
		out = append(out, members)
	}
	return out
}

// bandEdges is the band graph: next[i][j] when an edge runs from a node of
// bands[i] to one of bands[j], another band.
func bandEdges(bands []string, bandOf map[string]string, edges []LayoutEdge) [][]bool {
	index := make(map[string]int, len(bands))
	for i, b := range bands {
		index[b] = i
	}
	next := make([][]bool, len(bands))
	for i := range next {
		next[i] = make([]bool, len(bands))
	}
	for _, e := range edges {
		from, okFrom := index[bandOf[e.From]]
		to, okTo := index[bandOf[e.To]]
		if okFrom && okTo && from != to {
			next[from][to] = true
		}
	}
	return next
}

// bandComponents numbers the strongly connected components of the band
// graph next in the order of their first band, returning each band's
// component and their count.
func bandComponents(next [][]bool) ([]int, int) {
	reach := reachability(next)
	comp := slices.Repeat([]int{-1}, len(next))
	count := 0
	for i := range next {
		if comp[i] >= 0 {
			continue
		}
		for j := i; j < len(next); j++ {
			if reach[i][j] && reach[j][i] {
				comp[j] = count
			}
		}
		count++
	}
	return comp, count
}

// reachability is next's reflexive transitive closure.
func reachability(next [][]bool) [][]bool {
	reach := make([][]bool, len(next))
	for i := range next {
		reach[i] = slices.Clone(next[i])
		reach[i][i] = true
	}
	for k := range reach {
		for i := range reach {
			if !reach[i][k] {
				continue
			}
			for j := range reach {
				reach[i][j] = reach[i][j] || reach[k][j]
			}
		}
	}
	return reach
}

// runOrder is Kahn's order over runs, after[r][s] saying r follows s, the
// lowest-numbered ready run first; after is acyclic, so one is always ready.
func runOrder(after [][]bool) []int {
	placed := make([]bool, len(after))
	out := make([]int, 0, len(after))
	for len(out) < len(after) {
		for r := range after {
			if !placed[r] && readyRun(after[r], placed) {
				placed[r] = true
				out = append(out, r)
				break
			}
		}
	}
	return out
}

// readyRun reports whether every run deps marks is placed.
func readyRun(deps, placed []bool) bool {
	for s, d := range deps {
		if d && !placed[s] {
			return false
		}
	}
	return true
}

// bandLayers gives each node its layer: its run's first layer plus its
// longest path inside the run, the runs laid end to end in order. A run's
// span names the bands of it that hold a node.
func bandLayers(ids []string, edges []LayoutEdge, runs [][]string, bandOf map[string]string) (map[string]int, []LayoutBand) {
	runOfBand := map[string]int{}
	for r, members := range runs {
		for _, b := range members {
			runOfBand[b] = r
		}
	}
	runOf := make(map[string]int, len(ids))
	held := map[string]bool{}
	for _, id := range ids {
		runOf[id], held[bandOf[id]] = runOfBand[bandOf[id]], true
	}
	local := longestPaths(ids, edges, runOf)
	width := make([]int, len(runs))
	for _, id := range ids {
		width[runOf[id]] = max(width[runOf[id]], local[id]+1)
	}
	start := make([]int, len(runs))
	spans := make([]LayoutBand, 0, len(runs))
	next := 0
	for r, members := range runs {
		if width[r] == 0 {
			continue
		}
		named := slices.DeleteFunc(slices.Clone(members), func(b string) bool { return !held[b] })
		span := LayoutBand{Name: named[0], From: next, To: next + width[r] - 1}
		if len(named) > 1 {
			span.With = named[1:]
		}
		start[r] = next
		spans = append(spans, span)
		next += width[r]
	}
	layer := make(map[string]int, len(ids))
	for _, id := range ids {
		layer[id] = start[runOf[id]] + local[id]
	}
	return layer, spans
}

// longestPaths is each node's longest path from a source over the edges
// whose two ends share a run, in Kahn's order; a node on a cycle, which a
// linted graph has none of, keeps the depth it reached.
func longestPaths(ids []string, edges []LayoutEdge, runOf map[string]int) map[string]int {
	preds := map[string]int{}
	next := map[string][]string{}
	for _, e := range edges {
		if runOf[e.From] == runOf[e.To] {
			preds[e.To]++
			next[e.From] = append(next[e.From], e.To)
		}
	}
	depth := make(map[string]int, len(ids))
	queue := []string{}
	for _, id := range ids {
		if preds[id] == 0 {
			queue = append(queue, id)
		}
	}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, to := range next[id] {
			depth[to] = max(depth[to], depth[id]+1)
			preds[to]--
			if preds[to] == 0 {
				queue = append(queue, to)
			}
		}
	}
	return depth
}

// orderLayers buckets ids by layer in file order and runs the barycenter
// sweeps: a down sweep keys each node by the mean order of its
// predecessors, an up sweep by that of its successors, a node with none
// keeping its own order as its key; equal keys go by file order.
func orderLayers(ids []string, edges []LayoutEdge, layer, file map[string]int, layers int) [][]string {
	rows := make([][]string, layers)
	for _, id := range ids {
		rows[layer[id]] = append(rows[layer[id]], id)
	}
	preds, succs := map[string][]string{}, map[string][]string{}
	for _, e := range edges {
		preds[e.To] = append(preds[e.To], e.From)
		succs[e.From] = append(succs[e.From], e.To)
	}
	pos := map[string]int{}
	index := func(row []string) {
		for i, id := range row {
			pos[id] = i
		}
	}
	for _, row := range rows {
		index(row)
	}
	for sweep := range layoutSweeps {
		down := sweep%2 == 0
		for i := range layers {
			l := i
			if !down {
				l = layers - 1 - i
			}
			by := succs
			if down {
				by = preds
			}
			sortByBarycenter(rows[l], by, pos, file)
			index(rows[l])
		}
	}
	return rows
}

// sortByBarycenter sorts row by each node's mean neighbour order, a node
// with no neighbour keyed by its own, ties by file order.
func sortByBarycenter(row []string, neighbours map[string][]string, pos, file map[string]int) {
	key := make(map[string]float64, len(row))
	for _, id := range row {
		key[id] = float64(pos[id])
		if ns := neighbours[id]; len(ns) > 0 {
			sum := 0
			for _, n := range ns {
				sum += pos[n]
			}
			key[id] = float64(sum) / float64(len(ns))
		}
	}
	slices.SortFunc(row, func(a, b string) int {
		return cmp.Or(cmp.Compare(key[a], key[b]), cmp.Compare(file[a], file[b]))
	})
}
