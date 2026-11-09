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

// generateProgressGraph draws a plan from seed: random tracks, milestones and
// opens_after, two to twelve epics with a quarter of them conditions, edges
// only to earlier epics and never to a later milestone of the same track,
// explicit gate members (conditions included) on some milestones, done
// records on some epics, and bound files on a few work epics, each in
// progress or done.
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
			dp, ep := place[dep.Milestone], place[e.Milestone]
			if r.IntN(3) == 0 && (dp[0] != ep[0] || dp[1] <= ep[1]) {
				e.DependsOn = append(e.DependsOn, model.EpicDep{ID: dep.ID, Why: "it reads the dependency's output"})
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

// progressMoves returns what can move in p: the ready work epics, the
// actionable conditions, and the bound epics that are not done and whose
// direct dependencies are all done, so their task files can be worked and
// closed.
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

// waitCycle reports whether p's undone epics and closed milestones wait on
// each other in a cycle: an undone epic waits on its undone dependencies and,
// without a bound file, on its milestone while it is closed; a closed
// milestone waits on each milestone it opens after that is closed and on that
// milestone's undone gate members. No decision of the operator breaks such a
// cycle: the plan's shape does.
func waitCycle(p *progressGraph) bool {
	gates := MilestoneGates(p.g, p.states)
	after := EffectiveOpensAfter(p.g)
	adj := map[string][]string{}
	undone := func(id string) bool { return p.states[id].Status != EpicStatusDone }
	for i := range p.g.Epics {
		e := &p.g.Epics[i]
		if !undone(e.ID) {
			continue
		}
		node := "e:" + e.ID
		for _, d := range e.DependsOn {
			if undone(d.ID) {
				adj[node] = append(adj[node], "e:"+d.ID)
			}
		}
		if len(p.bound[e.ID]) == 0 && !gates[e.Milestone].Open {
			adj[node] = append(adj[node], "m:"+e.Milestone)
		}
	}
	for m, verdict := range gates {
		if verdict.Open {
			continue
		}
		for _, x := range after[m] {
			if !gates[x].Open {
				adj["m:"+m] = append(adj["m:"+m], "m:"+x)
			}
			members, _ := GateMembers(p.g, x)
			for _, id := range members {
				if undone(id) {
					adj["m:"+m] = append(adj["m:"+m], "e:"+id)
				}
			}
		}
	}
	nodes := make([]string, 0, len(adj))
	for n, next := range adj {
		nodes = append(nodes, n)
		nodes = append(nodes, next...)
	}
	slices.Sort(nodes)
	nodes = slices.Compact(nodes)
	for _, comp := range stronglyConnected(nodes, adj) {
		if len(comp) > 1 || selfLoop(comp[0], adj) {
			return true
		}
	}
	return false
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

// TestEpicProgress_SomethingCanAlwaysMove is the progress invariant over
// generated plans: whenever the whole graph's critical path holds an undone
// epic, some ready or claimed epic, actionable condition or workable bound
// epic exists, so tp epic ready never leaves an unfinished plan with nothing
// to do. A plan whose own shape waits in a cycle (waitCycle) is left out: an
// epic depending across tracks on one whose milestone opens only after the
// dependent's own milestone, which the lint does not report since
// later-milestone-dependency reads track order. No rule on conditions can
// unblock it.
func TestEpicProgress_SomethingCanAlwaysMove(t *testing.T) {
	t.Parallel()
	const seeds = 1000
	checked, cycles := 0, 0
	for seed := range uint64(seeds) {
		p := generateProgressGraph(seed)
		if hasLintError(p.g) {
			continue
		}
		checked++
		if len(GraphCriticalPath(p.g, p.states)) == 0 || len(progressMoves(&p)) > 0 {
			continue
		}
		if waitCycle(&p) {
			cycles++
			continue
		}
		data, _ := json.Marshal(p.g)
		t.Errorf("seed %d: the critical path %v is undone but nothing can move\nstates: %v\nbound: %v\ngraph: %s",
			seed, GraphCriticalPath(p.g, p.states), p.states, p.bound, data)
	}
	if checked < seeds/2 {
		t.Fatalf("only %d of %d generated graphs passed the lint", checked, seeds)
	}
	t.Logf("%d plans checked, %d left out for a wait cycle in their shape", checked, cycles)
}
