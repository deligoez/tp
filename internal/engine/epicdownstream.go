package engine

import (
	"math/bits"

	"github.com/deligoez/tp/internal/model"
)

// EpicDownstream is what waits on one not-done epic (spec/1.6.0.md F1). Only
// not-done epics reached over not-done epics count: a done epic cuts every
// path through it. A condition is passed through but never counted, since it
// is the operator's precondition, not work.
type EpicDownstream struct {
	// LongestChain is the most work epics on one path of epics depending on
	// the epic, transitively, the epic itself excluded.
	LongestChain int
	// Dependents is the count of work epics depending on the epic,
	// transitively.
	Dependents int
	// DependentsOpen is the part of Dependents in an open milestone.
	DependentsOpen int
	// DependentsByMilestone is Dependents per milestone, listing only
	// milestones with at least one.
	DependentsByMilestone map[string]int
}

// EpicDownstreams returns the downstream of each of ids that names a not-done
// epic of g; the rest are absent. states is DeriveEpicStates' result and
// gates MilestoneGates'. Every id shares one memoized walk over the reverse
// edges, so a large graph is walked once, not once per id.
func EpicDownstreams(g *model.EpicGraph, states map[string]EpicState, gates map[string]MilestoneGate, ids []string) map[string]EpicDownstream {
	w := newDownstreamWalk(g, states)
	out := make(map[string]EpicDownstream, len(ids))
	for _, id := range ids {
		i, ok := w.index[id]
		if !ok || !w.undone(i) {
			continue
		}
		d := EpicDownstream{LongestChain: w.longest(i), DependentsByMilestone: map[string]int{}}
		for _, j := range w.reach(i).members() {
			e := &g.Epics[j]
			d.Dependents++
			d.DependentsByMilestone[e.Milestone]++
			if gates[e.Milestone].Open {
				d.DependentsOpen++
			}
		}
		out[id] = d
	}
	return out
}

// GraphCriticalPath returns the longest chain of not-done epics in the whole
// graph by count of work epics, in work order: it starts at a not-done epic
// with no not-done dependency and may cross milestones and tracks. Ties go to
// the earlier epic in graph order; empty when no work epic is left.
func GraphCriticalPath(g *model.EpicGraph, states map[string]EpicState) []string {
	w := newDownstreamWalk(g, states)
	head, best := -1, 0
	for i := range g.Epics {
		if !w.undone(i) || w.waits[i] {
			continue
		}
		if c := w.weight(i) + w.longest(i); c > best {
			head, best = i, c
		}
	}
	path := []string{}
	for i := head; i >= 0; i = w.next[i] {
		path = append(path, g.Epics[i].ID)
	}
	return path
}

// downstreamWalk walks depends_on edges forwards, from an epic to the
// not-done epics depending on it, memoizing each epic's chain and reach.
// Epics are addressed by graph position; a repeated id keeps its first.
type downstreamWalk struct {
	g          *model.EpicGraph
	states     map[string]EpicState
	index      map[string]int
	dependents [][]int
	waits      []bool
	chain      []int
	next       []int
	reached    []epicBits
	visiting   []bool
}

// newDownstreamWalk indexes g's reverse edges between not-done epics.
func newDownstreamWalk(g *model.EpicGraph, states map[string]EpicState) *downstreamWalk {
	n := len(g.Epics)
	w := &downstreamWalk{g: g, states: states, index: make(map[string]int, n),
		dependents: make([][]int, n), waits: make([]bool, n), chain: make([]int, n),
		next: make([]int, n), reached: make([]epicBits, n), visiting: make([]bool, n)}
	for i := range g.Epics {
		if _, ok := w.index[g.Epics[i].ID]; !ok {
			w.index[g.Epics[i].ID] = i
		}
		w.chain[i], w.next[i] = -1, -1
	}
	for j := range g.Epics {
		if w.index[g.Epics[j].ID] != j || !w.undone(j) {
			continue
		}
		for _, dep := range g.Epics[j].DependsOn {
			if i, ok := w.index[dep.ID]; ok && w.undone(i) {
				w.dependents[i] = append(w.dependents[i], j)
				w.waits[j] = true
			}
		}
	}
	return w
}

// undone reports whether the epic at i is not done.
func (w *downstreamWalk) undone(i int) bool {
	return w.states[w.g.Epics[i].ID].Status != EpicStatusDone
}

// weight is what the epic at i adds to a count: 1 for work, 0 for a condition.
func (w *downstreamWalk) weight(i int) int {
	if w.g.Epics[i].Condition {
		return 0
	}
	return 1
}

// longest returns the most work epics on one path depending on i, i
// excluded, and records the path's next epic. A dependency cycle, which lint
// reports, cuts the path where it closes.
func (w *downstreamWalk) longest(i int) int {
	if w.chain[i] >= 0 {
		return w.chain[i]
	}
	w.visiting[i] = true
	best, next := 0, -1
	for _, j := range w.dependents[i] {
		if w.visiting[j] {
			continue
		}
		if c := w.weight(j) + w.longest(j); c > best {
			best, next = c, j
		}
	}
	w.visiting[i] = false
	w.chain[i], w.next[i] = best, next
	return best
}

// reach returns the work epics depending on i, transitively.
func (w *downstreamWalk) reach(i int) epicBits {
	if w.reached[i] != nil {
		return w.reached[i]
	}
	w.visiting[i] = true
	set := newEpicBits(len(w.g.Epics))
	for _, j := range w.dependents[i] {
		if w.visiting[j] {
			continue
		}
		if w.weight(j) > 0 {
			set.add(j)
		}
		set.union(w.reach(j))
	}
	w.visiting[i] = false
	w.reached[i] = set
	return set
}

// epicBits is a set of graph positions.
type epicBits []uint64

func newEpicBits(n int) epicBits { return make(epicBits, (n+63)/64) }

func (b epicBits) add(i int) { b[i/64] |= 1 << (i % 64) }

func (b epicBits) union(o epicBits) {
	for k := range o {
		b[k] |= o[k]
	}
}

// members lists the set's positions in ascending order.
func (b epicBits) members() []int {
	out := []int{}
	for k, word := range b {
		for word != 0 {
			out = append(out, k*64+bits.TrailingZeros64(word))
			word &= word - 1
		}
	}
	return out
}
