package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// EpicCriticalPath is one milestone's critical path (spec/1.5.0.md §7,
// spec/1.5.0-corrections.md C2). Each chain lists epic ids in work order:
// the first can start soonest and the last is a gate member.
type EpicCriticalPath struct {
	// ByCount is the longest chain by count of not-done epics; empty when
	// every gate member is done.
	ByCount []string
	// ByEstimate is the heaviest chain by summed estimate, chosen apart from
	// ByCount; nil when an epic on any chain has no estimate.
	ByEstimate []string
	// EstimateMinutes is ByEstimate's summed estimate.
	EstimateMinutes int
}

// CriticalPath derives milestone's critical path. A chain is a run of
// not-done epics joined by depends_on edges that ends at a not-done member of
// the milestone's gate (GateMembers); a done epic cuts it, and it may cross
// milestones and tracks. states is DeriveEpicStates' result and estimates
// EpicEstimates'. Ties go to the earlier gate member in GateMembers order and
// then to the earlier dependency in the epic's depends_on order.
func CriticalPath(g *model.EpicGraph, states map[string]EpicState, estimates map[string]int, milestone string) EpicCriticalPath {
	w := chainWalk{epics: epicsByID(g), states: states}
	members, _ := GateMembers(g, milestone)
	out := EpicCriticalPath{ByCount: w.best(members, func(string) int { return 1 }).path}
	if w.allEstimated(members, estimates) {
		heavy := w.best(members, func(id string) int { return estimates[id] })
		out.ByEstimate, out.EstimateMinutes = heavy.path, heavy.weight
	}
	return out
}

// chainWalk walks depends_on edges backwards from gate members through
// not-done epics.
type chainWalk struct {
	epics  map[string]*model.Epic
	states map[string]EpicState
}

// chain is one chain in work order and its summed weight.
type chain struct {
	path   []string
	weight int
}

// open reports whether id names an epic of the graph that is not done.
func (w *chainWalk) open(id string) bool {
	_, ok := w.epics[id]
	return ok && w.states[id].Status != EpicStatusDone
}

// best returns the heaviest chain under weight ending at a not-done member.
func (w *chainWalk) best(members []string, weight func(string) int) chain {
	memo := map[string]chain{}
	top := chain{path: []string{}}
	for _, id := range members {
		if !w.open(id) {
			continue
		}
		if c := w.heaviestTo(id, weight, memo, map[string]bool{}); c.weight > top.weight {
			top = c
		}
	}
	return top
}

// heaviestTo returns the heaviest chain ending at id, an epic assumed open.
// A dependency cycle, which lint reports, cuts the chain where it closes.
func (w *chainWalk) heaviestTo(id string, weight func(string) int, memo map[string]chain, visiting map[string]bool) chain {
	if c, ok := memo[id]; ok {
		return c
	}
	visiting[id] = true
	var prev chain
	for _, dep := range w.epics[id].DependsOn {
		if !w.open(dep.ID) || visiting[dep.ID] {
			continue
		}
		if c := w.heaviestTo(dep.ID, weight, memo, visiting); c.weight > prev.weight {
			prev = c
		}
	}
	delete(visiting, id)
	c := chain{path: append(append([]string{}, prev.path...), id), weight: prev.weight + weight(id)}
	memo[id] = c
	return c
}

// allEstimated reports whether every epic on every chain ending at a
// not-done member has an estimate.
func (w *chainWalk) allEstimated(members []string, estimates map[string]int) bool {
	seen := map[string]bool{}
	stack := []string{}
	for _, id := range members {
		if w.open(id) {
			stack = append(stack, id)
		}
	}
	for len(stack) > 0 {
		id := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[id] {
			continue
		}
		seen[id] = true
		if estimates[id] <= 0 {
			return false
		}
		for _, dep := range w.epics[id].DependsOn {
			if w.open(dep.ID) {
				stack = append(stack, dep.ID)
			}
		}
	}
	return true
}

// EpicEstimates returns each epic's estimate (spec/1.5.0.md §7): its
// estimate_minutes when set, else the sum of its bound tasks' estimates when
// it has bound tasks and every one has an estimate. An epic with neither is
// absent. bound is EpicBoundFiles' result; a bound file that cannot be read is
// an error.
func EpicEstimates(g *model.EpicGraph, bound map[string][]string) (map[string]int, error) {
	out := map[string]int{}
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.EstimateMinutes > 0 {
			out[e.ID] = e.EstimateMinutes
			continue
		}
		sum, ok, err := boundTaskEstimate(bound[e.ID])
		if err != nil {
			return nil, err
		}
		if ok {
			out[e.ID] = sum
		}
	}
	return out, nil
}

// boundTaskEstimate sums the estimates of the tasks in files; ok is false
// when there is no task or one has no estimate.
func boundTaskEstimate(files []string) (sum int, ok bool, err error) {
	count := 0
	for _, path := range files {
		tf, err := model.ReadTaskFile(path)
		if err != nil {
			return 0, false, fmt.Errorf("bound task file %s: %w", path, err)
		}
		for j := range tf.Tasks {
			if tf.Tasks[j].EstimateMinutes <= 0 {
				return 0, false, nil
			}
			sum += tf.Tasks[j].EstimateMinutes
			count++
		}
	}
	return sum, count > 0, nil
}
