package engine

import (
	"math/bits"
	"sort"
	"strconv"

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
	// milestones with at least one: tracks in graph order, milestones in track
	// order, then any milestone no track lists in graph order.
	DependentsByMilestone []MilestoneDependents
}

// MilestoneDependents is one milestone's part of an epic's dependents.
type MilestoneDependents struct {
	Milestone  string
	Dependents int
}

// EpicDownstreams returns the downstream of each of ids that names a not-done
// epic of g; the rest are absent. states is DeriveEpicStates' result and
// gates MilestoneGates'. Every id shares one memoized walk over the reverse
// edges, so a large graph is walked once, not once per id.
func EpicDownstreams(g *model.EpicGraph, states map[string]EpicState, gates map[string]MilestoneGate, ids []string) map[string]EpicDownstream {
	w := newDownstreamWalk(g, states)
	order := milestoneOrder(g)
	out := make(map[string]EpicDownstream, len(ids))
	for _, id := range ids {
		i, ok := w.index[id]
		if !ok || !w.undone(i) {
			continue
		}
		d := EpicDownstream{LongestChain: w.longest(i)}
		byMilestone := map[string]int{}
		for _, j := range w.reach(i).members() {
			e := &g.Epics[j]
			d.Dependents++
			byMilestone[e.Milestone]++
			if gates[e.Milestone].Open {
				d.DependentsOpen++
			}
		}
		d.DependentsByMilestone = []MilestoneDependents{}
		for _, m := range order {
			if n := byMilestone[m]; n > 0 {
				d.DependentsByMilestone = append(d.DependentsByMilestone, MilestoneDependents{Milestone: m, Dependents: n})
			}
		}
		out[id] = d
	}
	return out
}

// milestoneOrder lists every milestone once: tracks in graph order and each
// track's milestones in order, then the milestones of epics no track lists,
// in graph order.
func milestoneOrder(g *model.EpicGraph) []string {
	out := []string{}
	seen := map[string]bool{}
	add := func(m string) {
		if !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	for _, track := range g.Milestones {
		for _, m := range track {
			add(m)
		}
	}
	for i := range g.Epics {
		add(g.Epics[i].Milestone)
	}
	return out
}

// GraphCriticalPath returns the longest chain of not-done epics in the whole
// graph by count of work epics, in work order: it starts at a not-done epic
// with no not-done dependency and may cross milestones and tracks. Ties go to
// the earlier epic in graph order; empty when no work epic is left. A
// dependency cycle on the chain is listed whole where the chain enters it,
// its other members in graph order.
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
	for i := head; i >= 0; {
		c := w.comp[i]
		path = append(path, g.Epics[i].ID)
		for _, j := range w.comps[c] {
			if j != i {
				path = append(path, g.Epics[j].ID)
			}
		}
		i = w.compNext[c]
	}
	return path
}

// downstreamWalk walks depends_on edges forwards, from an epic to the
// not-done epics depending on it. Epics are addressed by graph position; a
// repeated id keeps its first. A dependency cycle, which lint reports, is
// one strongly connected component: its members depend on each other, so
// they share one reach and one chain, whatever the order of the epics.
// Chains and reaches are memoized per component.
type downstreamWalk struct {
	g          *model.EpicGraph
	states     map[string]EpicState
	index      map[string]int
	dependents [][]int
	waits      []bool
	comp       []int
	comps      [][]int
	compWeight []int
	compChain  []int
	compNext   []int
	compReach  []epicBits
	compWork   []epicBits
}

// newDownstreamWalk indexes g's reverse edges between not-done epics and
// their strongly connected components.
func newDownstreamWalk(g *model.EpicGraph, states map[string]EpicState) *downstreamWalk {
	n := len(g.Epics)
	w := &downstreamWalk{g: g, states: states, index: make(map[string]int, n),
		dependents: make([][]int, n), waits: make([]bool, n), comp: make([]int, n)}
	for i := range g.Epics {
		if _, ok := w.index[g.Epics[i].ID]; !ok {
			w.index[g.Epics[i].ID] = i
		}
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
	w.condense()
	return w
}

// condense groups the epics into strongly connected components over the
// dependents edges, each component's members in graph order, and sizes the
// per-component memos.
func (w *downstreamWalk) condense() {
	n := len(w.g.Epics)
	nodes := make([]string, n)
	adj := make(map[string][]string, n)
	for i := range n {
		nodes[i] = strconv.Itoa(i)
		for _, j := range w.dependents[i] {
			adj[nodes[i]] = append(adj[nodes[i]], strconv.Itoa(j))
		}
	}
	for c, names := range stronglyConnected(nodes, adj) {
		members := make([]int, 0, len(names))
		weight := 0
		for _, name := range names {
			i, _ := strconv.Atoi(name)
			members = append(members, i)
			w.comp[i] = c
			weight += w.weight(i)
		}
		sort.Ints(members)
		w.comps = append(w.comps, members)
		w.compWork = append(w.compWork, w.workOf(members))
		w.compWeight = append(w.compWeight, weight)
		w.compChain = append(w.compChain, -1)
		w.compNext = append(w.compNext, -1)
		w.compReach = append(w.compReach, nil)
	}
}

// workOf returns the work epics of a cycle's members as a set; nil for a
// single epic, which addWork adds by itself, so an acyclic graph keeps no
// set per epic.
func (w *downstreamWalk) workOf(members []int) epicBits {
	if len(members) == 1 {
		return nil
	}
	set := newEpicBits(len(w.g.Epics))
	for _, i := range members {
		if w.weight(i) > 0 {
			set.add(i)
		}
	}
	return set
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
// excluded: the other work members of i's cycle, if any, then the longest
// chain on from the cycle.
func (w *downstreamWalk) longest(i int) int {
	c := w.comp[i]
	return w.compWeight[c] - w.weight(i) + w.chainFrom(c)
}

// chainFrom returns the most work epics on one path of components depending
// on component c, c excluded, and records the epic the path enters next.
func (w *downstreamWalk) chainFrom(c int) int {
	if w.compChain[c] >= 0 {
		return w.compChain[c]
	}
	best, next := 0, -1
	for _, i := range w.comps[c] {
		for _, j := range w.dependents[i] {
			if d := w.comp[j]; d != c {
				if n := w.compWeight[d] + w.chainFrom(d); n > best {
					best, next = n, j
				}
			}
		}
	}
	w.compChain[c], w.compNext[c] = best, next
	return best
}

// reach returns the work epics depending on i, transitively, i excluded.
func (w *downstreamWalk) reach(i int) epicBits {
	c := w.comp[i]
	if len(w.comps[c]) == 1 {
		return w.reachFrom(c)
	}
	set := newEpicBits(len(w.g.Epics))
	set.union(w.reachFrom(c))
	w.addWork(set, c)
	set.remove(i)
	return set
}

// addWork adds component c's work epics to set.
func (w *downstreamWalk) addWork(set epicBits, c int) {
	if w.compWork[c] != nil {
		set.union(w.compWork[c])
	} else if i := w.comps[c][0]; w.weight(i) > 0 {
		set.add(i)
	}
}

// reachFrom returns the work epics of the components depending on component
// c, transitively, c's own excluded.
func (w *downstreamWalk) reachFrom(c int) epicBits {
	if w.compReach[c] != nil {
		return w.compReach[c]
	}
	set := newEpicBits(len(w.g.Epics))
	for _, i := range w.comps[c] {
		for _, j := range w.dependents[i] {
			if d := w.comp[j]; d != c {
				w.addWork(set, d)
				set.union(w.reachFrom(d))
			}
		}
	}
	w.compReach[c] = set
	return set
}

// epicBits is a set of graph positions.
type epicBits []uint64

func newEpicBits(n int) epicBits { return make(epicBits, (n+63)/64) }

func (b epicBits) add(i int) { b[i/64] |= 1 << (i % 64) }

func (b epicBits) remove(i int) { b[i/64] &^= 1 << (i % 64) }

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
