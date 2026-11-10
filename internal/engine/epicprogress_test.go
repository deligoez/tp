package engine

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/deligoez/tp/internal/model"
)

// progressGraph is one generated plan: the graph, every epic's derived
// state, and the bound-file index.
type progressGraph struct {
	g      *model.EpicGraph
	states map[string]EpicState
	bound  map[string][]string
}

// progressMilestones draws one to three tracks of one to three milestones
// each, and opens_after entries that only name milestones drawn earlier, so
// the effective order has no cycle: a random [] on some, a random set of
// earlier milestones, from any track, on others.
func progressMilestones(r *rand.Rand) (tracks [][]string, after map[string][]string, order []string) {
	after = map[string][]string{}
	for t := range 1 + r.IntN(3) {
		track := []string{}
		for m := range 1 + r.IntN(3) {
			name := fmt.Sprintf("T%dM%d", t, m)
			track = append(track, name)
			switch r.IntN(4) {
			case 0:
				after[name] = []string{}
			case 1:
				if len(order) > 0 {
					after[name] = []string{order[r.IntN(len(order))]}
				}
			}
			order = append(order, name)
		}
		tracks = append(tracks, track)
	}
	return tracks, after, order
}

// trackPlace returns each milestone's track and its position in it.
func trackPlace(tracks [][]string) map[string][2]int {
	out := map[string][2]int{}
	for t, track := range tracks {
		for i, m := range track {
			out[m] = [2]int{t, i}
		}
	}
	return out
}

// admissible reports whether an epic of milestone ep may depend on one of
// milestone dp without a later-milestone-dependency: another track, or no
// later place in the same one.
func admissible(dp, ep [2]int) bool {
	return dp[0] != ep[0] || dp[1] <= ep[1]
}

// drawGroup draws an any-of group of two or three distinct earlier epics e
// has no plain edge to. A member may sit in a later milestone of e's track
// only when another member is admissible, as later-milestone-dependency
// allows (spec/1.7.0.md §7); false when no such group can be drawn.
func drawGroup(r *rand.Rand, earlier []model.Epic, e *model.Epic, place map[string][2]int) (model.EpicDep, bool) {
	var pool []string
	for j := range earlier {
		if !slices.ContainsFunc(e.DependsOn, func(d model.EpicDep) bool { return d.ID == earlier[j].ID }) {
			pool = append(pool, earlier[j].ID)
		}
	}
	k := 2 + r.IntN(2)
	if len(pool) < k {
		return model.EpicDep{}, false
	}
	r.Shuffle(len(pool), func(a, b int) { pool[a], pool[b] = pool[b], pool[a] })
	members := pool[:k]
	ep := place[e.Milestone]
	byID := map[string]string{}
	for j := range earlier {
		byID[earlier[j].ID] = earlier[j].Milestone
	}
	if !slices.ContainsFunc(members, func(id string) bool { return admissible(place[byID[id]], ep) }) {
		return model.EpicDep{}, false
	}
	return model.EpicDep{AnyOf: slices.Clone(members), Why: "either one serves the dependent"}, true
}

// generateProgressGraph draws a plan from seed: random tracks, milestones and
// opens_after, two to twelve epics with a quarter of them conditions, edges
// only to earlier epics and never to a later milestone of the same track,
// any-of groups of two or three earlier epics on some (drawGroup), explicit
// gate members (conditions included) on some milestones, done records on
// some epics, and bound files on a few work epics, each in progress or done.
func generateProgressGraph(seed uint64) progressGraph {
	r := rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))
	tracks, after, order := progressMilestones(r)
	place := trackPlace(tracks)
	g := &model.EpicGraph{Version: 1, Milestones: tracks, OpensAfter: after, Gates: map[string]model.EpicGate{}}
	states := map[string]EpicState{}
	bound := map[string][]string{}
	for i := range 2 + r.IntN(11) {
		e := model.Epic{ID: fmt.Sprintf("e%d", i), Title: fmt.Sprintf("E%d", i), Milestone: order[r.IntN(len(order))]}
		e.Condition = r.IntN(4) == 0
		if e.Condition {
			e.Acceptance = "the operator decides"
		}
		for j := range i {
			dep := &g.Epics[j]
			if r.IntN(3) == 0 && admissible(place[dep.Milestone], place[e.Milestone]) {
				e.DependsOn = append(e.DependsOn, model.EpicDep{ID: dep.ID, Why: "it reads the dependency's output"})
			}
		}
		if r.IntN(3) == 0 {
			if d, ok := drawGroup(r, g.Epics, &e, place); ok {
				e.DependsOn = append(e.DependsOn, d)
			}
		}
		e.Root = len(e.DependsOn) == 0
		status := EpicStatusPlanned
		switch {
		case !e.Condition && r.IntN(6) == 0:
			bound[e.ID] = []string{"/repo/" + e.ID + ".tasks.json"}
			status = EpicStatusInProgress
			if r.IntN(2) == 0 {
				status = EpicStatusDone
			}
		case r.IntN(3) == 0:
			e.Done = &model.EpicDone{Evidence: "recorded"}
			status = EpicStatusDone
		}
		states[e.ID] = EpicState{Status: status}
		g.Epics = append(g.Epics, e)
	}
	for _, m := range order {
		if r.IntN(3) != 0 {
			continue
		}
		for i := range g.Epics {
			if g.Epics[i].Milestone == m && r.IntN(2) == 0 {
				gate := g.Gates[m]
				gate.Members = append(gate.Members, g.Epics[i].ID)
				g.Gates[m] = gate
			}
		}
	}
	return progressGraph{g: g, states: states, bound: bound}
}

// fieldShapePlan is the field shape any-of exists for: one track [P, Q],
// both opening after nothing, conditions a in P and b in Q, work x in P
// depending on {a, b}, and z in Q depending on x, nothing done.
func fieldShapePlan() progressGraph {
	cond := func(id, m string) model.Epic {
		return model.Epic{ID: id, Title: id, Milestone: m, Root: true, Condition: true, Acceptance: "the operator decides"}
	}
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"P", "Q"}},
		OpensAfter: map[string][]string{"P": {}, "Q": {}},
		Epics: []model.Epic{
			cond("a", "P"), cond("b", "Q"),
			{ID: "x", Title: "x", Milestone: "P", DependsOn: []model.EpicDep{{AnyOf: []string{"a", "b"}, Why: "either decision opens x"}}},
			{ID: "z", Title: "z", Milestone: "Q", DependsOn: []model.EpicDep{{ID: "x", Why: "z reads x"}}},
		},
	}
	states := map[string]EpicState{}
	for i := range g.Epics {
		states[g.Epics[i].ID] = EpicState{Status: EpicStatusPlanned}
	}
	return progressGraph{g: g, states: states, bound: map[string][]string{}}
}

// progressMoves returns what can move in p: the ready work epics, the
// actionable conditions, and the bound epics that are not done and whose
// direct dependencies are all satisfied, a group once one member is done,
// so their task files can be worked and closed.
func progressMoves(p *progressGraph) []string {
	r := EpicReady(p.g, p.states, p.bound, false)
	moves := slices.Clone(r.Ready)
	moves = append(moves, r.Claimed...)
	moves = append(moves, r.Conditions...)
	for i := range p.g.Epics {
		e := &p.g.Epics[i]
		if len(p.bound[e.ID]) > 0 && p.states[e.ID].Status != EpicStatusDone && epicDepsDone(e, p.states) {
			moves = append(moves, e.ID)
		}
	}
	return moves
}

// undoneLive lists, in graph order, the epics of p that are neither done nor
// moot (MootEpics), conditions included: what a plan still owes. The one
// class left out is a condition that has dependents, every one of them done
// or moot: no live work waits on it, so nothing has to record it, and
// ConditionsNotActionable gives it no dependent to make it actionable. A moot
// epic leaves every count, and a satisfied group holds no dependent, as
// actionDependents reads it (spec/1.7.0.md §7).
func undoneLive(p *progressGraph) []string {
	moot := MootEpics(p.g, p.states)
	dependents := map[string][]string{}
	for i := range p.g.Epics {
		e := &p.g.Epics[i]
		for _, d := range e.DependsOn {
			if d.IsGroup() && epicDepSatisfied(d, p.states) {
				continue
			}
			for _, id := range d.Names() {
				dependents[id] = append(dependents[id], e.ID)
			}
		}
	}
	allSettled := func(ids []string) bool {
		return !slices.ContainsFunc(ids, func(id string) bool { return p.states[id].Status != EpicStatusDone && !moot[id] })
	}
	var out []string
	for i := range p.g.Epics {
		e := &p.g.Epics[i]
		if p.states[e.ID].Status == EpicStatusDone || moot[e.ID] {
			continue
		}
		if e.Condition && len(dependents[e.ID]) > 0 && allSettled(dependents[e.ID]) {
			continue
		}
		out = append(out, e.ID)
	}
	return out
}

// hasLintError reports whether g carries an error-severity lint finding: a
// graph the lint refuses is no plan to make progress on.
func hasLintError(g *model.EpicGraph) bool {
	for _, f := range LintEpicGraph(g, nil) {
		if f.Severity == EpicSeverityError {
			return true
		}
	}
	return false
}

// stuckPlan words why p is a counterexample to the progress invariant, or
// returns "" when p is no counterexample: the lint refuses it, nothing
// undone and not moot is left, or something can move.
func stuckPlan(p *progressGraph) string {
	if hasLintError(p.g) {
		return ""
	}
	live := undoneLive(p)
	if len(live) == 0 || len(progressMoves(p)) > 0 {
		return ""
	}
	data, _ := json.Marshal(p.g)
	return fmt.Sprintf("%v are undone and not moot but nothing can move\nstates: %v\nbound: %v\ngraph: %s", live, p.states, p.bound, data)
}

// TestEpicProgress_SomethingCanAlwaysMove is the progress invariant over
// generated plans, any-of groups included: every lint-clean plan can always
// make progress. Whenever an epic, work or condition, is neither done nor
// moot (undoneLive), some ready or claimed epic, actionable condition or
// workable bound epic exists, so tp epic ready never leaves an unfinished
// plan with nothing to do; recording an actionable condition is a move. A
// plan the lint reports an error on is left out: dependency-opens-later is
// the error on a plan whose own shape waits on itself.
func TestEpicProgress_SomethingCanAlwaysMove(t *testing.T) {
	t.Parallel()
	const seeds = 1000
	field := fieldShapePlan()
	if hasLintError(field.g) {
		t.Fatalf("the field shape fails the lint: %v", LintEpicGraph(field.g, nil))
	}
	if msg := stuckPlan(&field); msg != "" {
		t.Errorf("the field shape: %s", msg)
	}
	checked, grouped, owing := 0, 0, 0
	for seed := range uint64(seeds) {
		p := generateProgressGraph(seed)
		if hasLintError(p.g) {
			continue
		}
		checked++
		if slices.ContainsFunc(p.g.Epics, func(e model.Epic) bool {
			return slices.ContainsFunc(e.DependsOn, model.EpicDep.IsGroup)
		}) {
			grouped++
		}
		if owesACondition(&p) {
			owing++
		}
		if msg := stuckPlan(&p); msg != "" {
			t.Errorf("seed %d: %s", seed, msg)
		}
	}
	if checked < seeds/2 {
		t.Fatalf("only %d of %d generated graphs passed the lint", checked, seeds)
	}
	if grouped < checked/10 {
		t.Fatalf("only %d of %d lint-clean plans carry an any-of group", grouped, checked)
	}
	t.Logf("%d lint-clean plans checked, %d with an any-of group, %d owing a condition", checked, grouped, owing)
}

// owesACondition reports whether undoneLive lists a condition of p.
func owesACondition(p *progressGraph) bool {
	byID := epicsByID(p.g)
	return slices.ContainsFunc(undoneLive(p), func(id string) bool { return byID[id].Condition })
}
