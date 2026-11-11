package engine

import (
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// The kinds of map lane (spec/1.8.0.md §9).
const (
	MapLaneMain      = "main"
	MapLaneParallel  = "parallel"
	MapLaneTriggered = "triggered"
)

// MapLane is one lane of the map: its kind and its milestones, a chain of
// openings in opening order (the triggered lane's in graph-file order).
type MapLane struct {
	Kind       string   `json:"kind"`
	Milestones []string `json:"milestones"`
}

// MapLanes is the map's milestone layout: the lanes, main first, then the
// parallel lanes in graph-file order, then the triggered lane when any
// milestone is in it; and each milestone's column, its opening depth.
type MapLanes struct {
	Lanes   []MapLane
	Columns map[string]int
}

// EpicMapLanes lays the milestones a track lists into lanes from the
// opens_after topology (spec/1.8.0.md §9), each in exactly one, by
// precedence: an explicit [] entry puts a milestone in the triggered lane;
// among the rest the longest chain of openings is the main lane; every other
// milestone goes to a parallel lane, the longest chain of what is left taken
// each time. Equal chains tie to the one whose milestones, compared in order,
// come first in the graph file. A graph whose openings cycle still lays out:
// the edge closing a cycle is ignored.
func EpicMapLanes(g *model.EpicGraph) MapLanes {
	after := EffectiveOpensAfter(g)
	order := trackedMilestones(g)
	pos := make(map[string]int, len(order))
	for i, m := range order {
		pos[m] = i
	}
	out := MapLanes{Columns: openingDepths(order, after)}
	rest := map[string]bool{}
	triggered := []string{}
	for _, m := range order {
		if entry, ok := g.OpensAfter[m]; ok && len(entry) == 0 {
			triggered = append(triggered, m)
		} else {
			rest[m] = true
		}
	}
	chains := takeChains(order, pos, after, rest)
	if len(chains) > 0 {
		out.Lanes = append(out.Lanes, MapLane{Kind: MapLaneMain, Milestones: chains[0]})
		parallel := chains[1:]
		slices.SortStableFunc(parallel, func(a, b []string) int { return pos[a[0]] - pos[b[0]] })
		for _, c := range parallel {
			out.Lanes = append(out.Lanes, MapLane{Kind: MapLaneParallel, Milestones: c})
		}
	}
	if len(triggered) > 0 {
		out.Lanes = append(out.Lanes, MapLane{Kind: MapLaneTriggered, Milestones: triggered})
	}
	return out
}

// openingDepths is each milestone's opening depth: 0 when it opens after
// none, else one more than the deepest milestone it opens after.
func openingDepths(order []string, after map[string][]string) map[string]int {
	depth := map[string]int{}
	visiting := map[string]bool{}
	var walk func(m string) int
	walk = func(m string) int {
		if d, ok := depth[m]; ok {
			return d
		}
		visiting[m] = true
		d := 0
		for _, p := range after[m] {
			if _, tracked := after[p]; tracked && !visiting[p] {
				d = max(d, walk(p)+1)
			}
		}
		visiting[m] = false
		depth[m] = d
		return d
	}
	for _, m := range order {
		walk(m)
	}
	return depth
}

// takeChains removes the longest chain of openings from rest until rest is
// empty, returning the chains in the order taken.
func takeChains(order []string, pos map[string]int, after map[string][]string, rest map[string]bool) [][]string {
	chains := [][]string{}
	for len(rest) > 0 {
		chain := longestChain(order, pos, after, rest)
		for _, m := range chain {
			delete(rest, m)
		}
		chains = append(chains, chain)
	}
	return chains
}

// longestChain is the longest chain of openings over the milestones of rest,
// each one opening after the one before it; between equal chains, the one
// whose milestones come first in the graph file, compared in order.
func longestChain(order []string, pos map[string]int, after map[string][]string, rest map[string]bool) []string {
	next := map[string][]string{}
	for _, m := range order {
		for _, p := range after[m] {
			if rest[m] && rest[p] {
				next[p] = append(next[p], m)
			}
		}
	}
	best := map[string][]string{}
	visiting := map[string]bool{}
	var from func(m string) []string
	from = func(m string) []string {
		if c, ok := best[m]; ok {
			return c
		}
		visiting[m] = true
		var tail []string
		for _, n := range next[m] {
			if !visiting[n] {
				tail = betterChain(tail, from(n), pos)
			}
		}
		visiting[m] = false
		best[m] = append([]string{m}, tail...)
		return best[m]
	}
	var out []string
	for _, m := range order {
		if rest[m] {
			out = betterChain(out, from(m), pos)
		}
	}
	return out
}

// betterChain returns the longer of a and b, or on equal length the one whose
// milestones come first in the graph file at their first difference; a wins
// a full tie.
func betterChain(a, b []string, pos map[string]int) []string {
	if len(a) != len(b) {
		if len(b) > len(a) {
			return b
		}
		return a
	}
	for i := range a {
		if a[i] != b[i] {
			if pos[b[i]] < pos[a[i]] {
				return b
			}
			return a
		}
	}
	return a
}
