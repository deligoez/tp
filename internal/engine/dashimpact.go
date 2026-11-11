package engine

import (
	"maps"
	"path/filepath"

	"github.com/deligoez/tp/internal/model"
)

// ReleaseImpact is what releasing one spec would do to the plan
// (spec/1.8.0.md §13): the epics it frees, in graph order, and the gates
// whose done count moves, in track order. Both lists are empty for a spec
// whose release finishes no epic.
type ReleaseImpact struct {
	Frees []string   `json:"frees"`
	Gates []GateMove `json:"gates"`
}

// GateMove is one gate whose done count a release moves: members done before
// and after, out of its members, deferrable ones left out as MilestoneState
// counts them.
type GateMove struct {
	Milestone        string `json:"milestone"`
	MembersDone      int    `json:"members_done"`
	MembersDoneAfter int    `json:"members_done_after"`
	Members          int    `json:"members"`
}

// SpecReleaseImpact is the impact of epic id becoming done: every epic not
// done that an undone dependency holds now and none would hold after, and
// every tracked milestone whose gate counts one more member done. An epic
// already done, or one the graph does not hold, has an empty impact.
func SpecReleaseImpact(g *model.EpicGraph, states map[string]EpicState, id string) ReleaseImpact {
	impact := ReleaseImpact{Frees: []string{}, Gates: []GateMove{}}
	st, ok := states[id]
	if !ok || st.Status == EpicStatusDone {
		return impact
	}
	after := maps.Clone(states)
	after[id] = EpicState{Status: EpicStatusDone}
	conditions := make(map[string]bool, len(g.Epics))
	for i := range g.Epics {
		conditions[g.Epics[i].ID] = g.Epics[i].Condition
	}
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.ID == id || states[e.ID].Status == EpicStatusDone {
			continue
		}
		before, _ := epicWaitFlags(e, states, conditions)
		still, _ := epicWaitFlags(e, after, conditions)
		if before && !still {
			impact.Frees = append(impact.Frees, e.ID)
		}
	}
	impact.Gates = gateMoves(g, states, after)
	return impact
}

// gateMoves lists every tracked milestone, once, in track order, whose gate
// counts a different number of members done under after than under before.
func gateMoves(g *model.EpicGraph, before, after map[string]EpicState) []GateMove {
	out := []GateMove{}
	seen := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			if seen[m] {
				continue
			}
			seen[m] = true
			done, members := gateMembersCount(g, m, before)
			doneAfter, _ := gateMembersCount(g, m, after)
			if doneAfter != done {
				out = append(out, GateMove{Milestone: m, MembersDone: done, MembersDoneAfter: doneAfter, Members: members})
			}
		}
	}
	return out
}

// releaseImpacts keys every task file of r by its repository path: a file
// bound to an epic of g carries that epic's release impact when every other
// file bound to it is at release, so releasing this one finishes the epic;
// any other file carries an empty impact.
func (r *dashRepo) releaseImpacts(g *model.EpicGraph) map[string]ReleaseImpact {
	epicOf := map[string]string{}
	for id, files := range r.bound {
		for _, f := range files {
			epicOf[f] = id
		}
	}
	out := make(map[string]ReleaseImpact, len(r.files))
	for _, f := range r.files {
		impact := ReleaseImpact{Frees: []string{}, Gates: []GateMove{}}
		if id := epicOf[f]; id != "" && r.othersReleased(id, f) {
			impact = SpecReleaseImpact(g, r.states, id)
		}
		out[r.repoPath(f)] = impact
	}
	return out
}

// othersReleased reports whether every file bound to epic id but f is at
// the release phase.
func (r *dashRepo) othersReleased(id, f string) bool {
	for _, other := range r.bound[id] {
		if other != f && r.phases[other] != PhaseRelease {
			return false
		}
	}
	return true
}

// repoPath is f relative to the repository root, slash-separated.
func (r *dashRepo) repoPath(f string) string {
	if rel, err := filepath.Rel(r.root, f); err == nil {
		return filepath.ToSlash(rel)
	}
	return filepath.ToSlash(f)
}
