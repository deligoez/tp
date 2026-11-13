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
// inclusive, that hold the nodes of the band Name.
type LayoutBand struct {
	Name string `json:"milestone"`
	From int    `json:"from_layer"`
	To   int    `json:"to_layer"`
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
// tie or a cycle in the order given. Inside a band a node's layer is its
// longest path from the band's sources over the band's own edges. Each
// layer is then ordered by layoutSweeps barycenter sweeps, ties broken by
// file order, so one input always lays out alike.
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
	layer, spans := bandLayers(ids, kept, bandOrder(bands, bandOf, kept), bandOf)
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

// bandOrder sorts bands so a band follows every band it depends on through
// edges: Kahn's order, the earliest given band first among the ready ones,
// and the earliest given band left when a cycle leaves none ready.
func bandOrder(bands []string, bandOf map[string]string, edges []LayoutEdge) []string {
	after := map[string]map[string]bool{}
	for _, e := range edges {
		from, to := bandOf[e.From], bandOf[e.To]
		if from != to {
			if after[to] == nil {
				after[to] = map[string]bool{}
			}
			after[to][from] = true
		}
	}
	placed := map[string]bool{}
	out := make([]string, 0, len(bands))
	for len(out) < len(bands) {
		next := ""
		for _, b := range bands {
			if !placed[b] && readyBand(after[b], placed) {
				next = b
				break
			}
		}
		if next == "" {
			next = bands[slices.IndexFunc(bands, func(b string) bool { return !placed[b] })]
		}
		placed[next] = true
		out = append(out, next)
	}
	return out
}

// readyBand reports whether every band of deps is placed.
func readyBand(deps, placed map[string]bool) bool {
	for d := range deps {
		if !placed[d] {
			return false
		}
	}
	return true
}

// bandLayers gives each node its layer: its band's first layer plus its
// longest path inside the band, the bands laid end to end in order.
func bandLayers(ids []string, edges []LayoutEdge, order []string, bandOf map[string]string) (map[string]int, []LayoutBand) {
	local := longestPaths(ids, edges, bandOf)
	width := map[string]int{}
	for _, id := range ids {
		width[bandOf[id]] = max(width[bandOf[id]], local[id]+1)
	}
	start := map[string]int{}
	spans := make([]LayoutBand, 0, len(order))
	next := 0
	for _, b := range order {
		if width[b] == 0 {
			continue
		}
		start[b] = next
		spans = append(spans, LayoutBand{Name: b, From: next, To: next + width[b] - 1})
		next += width[b]
	}
	layer := make(map[string]int, len(ids))
	for _, id := range ids {
		layer[id] = start[bandOf[id]] + local[id]
	}
	return layer, spans
}

// longestPaths is each node's longest path from a source over the edges
// whose two ends share a band, in Kahn's order; a node on a cycle, which a
// linted graph has none of, keeps the depth it reached.
func longestPaths(ids []string, edges []LayoutEdge, bandOf map[string]string) map[string]int {
	preds := map[string]int{}
	next := map[string][]string{}
	for _, e := range edges {
		if bandOf[e.From] == bandOf[e.To] {
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
