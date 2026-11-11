package engine

import (
	"cmp"
	"math"
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// EpicMapRows orders the epics inside each tracked milestone for the map
// (spec/1.8.0.md §9), returning each milestone's epic ids top to bottom.
//
// Milestones are taken by column, then lane, then graph-file order. An epic
// with a dependency is placed by the mean row of its dependencies in the
// milestones already taken, the classic barycenter that cuts crossings; one
// none of whose dependencies is placed yet keeps graph-file order after
// them. An epic with no dependency then sits next to its first dependent (the
// first epic in the graph file naming it): just before it in the same
// milestone, or at its row in another; one nothing depends on goes last.
// Ties keep graph-file order, so the same graph always yields the same rows.
func EpicMapRows(g *model.EpicGraph, lanes MapLanes) map[string][]string {
	byMilestone := map[string][]*model.Epic{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if _, tracked := lanes.Columns[e.Milestone]; tracked {
			byMilestone[e.Milestone] = append(byMilestone[e.Milestone], e)
		}
	}
	row := map[string]int{}
	base := map[string][]string{}
	for _, m := range mapMilestoneOrder(lanes) {
		base[m] = barycenterOrder(byMilestone[m], row)
		for i, id := range base[m] {
			row[id] = i
		}
	}
	first := firstDependents(g)
	out := map[string][]string{}
	for m, epics := range byMilestone {
		out[m] = placeRoots(epics, base[m], first, row)
	}
	return out
}

// mapMilestoneOrder lists the lanes' milestones by column, then lane, then
// their place in the lane.
func mapMilestoneOrder(lanes MapLanes) []string {
	var order []string
	lane := map[string]int{}
	for i, l := range lanes.Lanes {
		for _, m := range l.Milestones {
			lane[m] = i
			order = append(order, m)
		}
	}
	slices.SortStableFunc(order, func(a, b string) int {
		return cmp.Or(cmp.Compare(lanes.Columns[a], lanes.Columns[b]), cmp.Compare(lane[a], lane[b]))
	})
	return order
}

// barycenterOrder orders a milestone's epics with a dependency by the mean
// row of their placed dependencies; those with none placed follow in
// graph-file order.
func barycenterOrder(epics []*model.Epic, row map[string]int) []string {
	var withDeps []rowKey
	for _, e := range epics {
		if len(e.DependsOn) == 0 {
			continue
		}
		sum, n := 0, 0
		for _, d := range e.DependsOn {
			for _, id := range d.Names() {
				if r, placed := row[id]; placed {
					sum += r
					n++
				}
			}
		}
		key := math.Inf(1)
		if n > 0 {
			key = float64(sum) / float64(n)
		}
		withDeps = append(withDeps, rowKey{e.ID, key})
	}
	return idsByKey(withDeps)
}

// rowKey is an epic id and the key that orders it in its milestone.
type rowKey struct {
	id  string
	key float64
}

// idsByKey returns the ids of keys by ascending key, equal keys in the
// order given.
func idsByKey(keys []rowKey) []string {
	slices.SortStableFunc(keys, func(a, b rowKey) int { return cmp.Compare(a.key, b.key) })
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.id
	}
	return out
}

// firstDependents maps each epic id to the id of the first epic in the graph
// file that names it in a dependency.
func firstDependents(g *model.EpicGraph) map[string]string {
	out := map[string]string{}
	for i := range g.Epics {
		for _, d := range g.Epics[i].DependsOn {
			for _, id := range d.Names() {
				if _, seen := out[id]; !seen {
					out[id] = g.Epics[i].ID
				}
			}
		}
	}
	return out
}

// placeRoots inserts a milestone's epics with no dependency into its base
// order, each keyed just before its first dependent's row: right before that
// dependent when it is in the same milestone, before the epic at its row when
// it is elsewhere; last when nothing tracked depends on the epic.
func placeRoots(epics []*model.Epic, base []string, first map[string]string, row map[string]int) []string {
	all := make([]rowKey, 0, len(epics))
	for i, id := range base {
		all = append(all, rowKey{id, float64(i)})
	}
	for _, e := range epics {
		if len(e.DependsOn) > 0 {
			continue
		}
		key := math.Inf(1)
		if r, placed := row[first[e.ID]]; placed {
			key = float64(r) - 0.5
		}
		all = append(all, rowKey{e.ID, key})
	}
	return idsByKey(all)
}
