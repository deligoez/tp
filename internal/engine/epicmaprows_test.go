package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/deligoez/tp/internal/model"
)

// rowsGraph is one track of milestones and the epics as given.
func rowsGraph(milestones []string, epics ...model.Epic) *model.EpicGraph {
	return &model.EpicGraph{Version: 1, Milestones: [][]string{milestones}, Epics: epics}
}

// rowsEpic is epic id in milestone m depending on deps, a root when it has
// none.
func rowsEpic(id, m string, deps ...string) model.Epic {
	e := model.Epic{ID: id, Title: id, Milestone: m, Root: len(deps) == 0}
	for _, d := range deps {
		e.DependsOn = append(e.DependsOn, model.EpicDep{ID: d, Why: "w"})
	}
	return e
}

// crossings counts the pairs of edges from M1 to M2 that cross when each
// milestone's epics are drawn in rows' order.
func crossings(g *model.EpicGraph, rows map[string][]string) int {
	row := map[string]int{}
	for _, m := range []string{"M1", "M2"} {
		for i, id := range rows[m] {
			row[id] = i
		}
	}
	type edge struct{ a, b int }
	var edges []edge
	for i := range g.Epics {
		e := &g.Epics[i]
		if e.Milestone != "M2" {
			continue
		}
		for _, d := range e.DependsOn {
			if g.Epics[epicIndex(g, d.ID)].Milestone == "M1" {
				edges = append(edges, edge{row[d.ID], row[e.ID]})
			}
		}
	}
	n := 0
	for i := range edges {
		for j := i + 1; j < len(edges); j++ {
			if (edges[i].a-edges[j].a)*(edges[i].b-edges[j].b) < 0 {
				n++
			}
		}
	}
	return n
}

// graphRows is each milestone's epics in graph-file order.
func graphRows(g *model.EpicGraph) map[string][]string {
	out := map[string][]string{}
	for i := range g.Epics {
		out[g.Epics[i].Milestone] = append(out[g.Epics[i].Milestone], g.Epics[i].ID)
	}
	return out
}

// TestEpicMapRows_CrossFewerEdgesThanGraphOrder: M1's a, b and c each depend
// on the root r and M2's x, y and z on c, b and a; in graph order every M1-M2
// pair of edges crosses, three crossings, and the layout crosses none.
func TestEpicMapRows_CrossFewerEdgesThanGraphOrder(t *testing.T) {
	g := rowsGraph([]string{"M1", "M2"},
		rowsEpic("r", "M1"), rowsEpic("a", "M1", "r"), rowsEpic("b", "M1", "r"), rowsEpic("c", "M1", "r"),
		rowsEpic("x", "M2", "c"), rowsEpic("y", "M2", "b"), rowsEpic("z", "M2", "a"),
	)
	rows := EpicMapRows(g, EpicMapLanes(g))
	assert.Equal(t, 3, crossings(g, graphRows(g)))
	assert.Equal(t, 0, crossings(g, rows))
}

// TestEpicMapRows_RootsFollowTheirDependentsAcrossMilestones: M1 holds only
// roots, so no barycenter orders it; each root takes its first dependent's row.
func TestEpicMapRows_RootsFollowTheirDependentsAcrossMilestones(t *testing.T) {
	g := rowsGraph([]string{"M1", "M2"},
		rowsEpic("a", "M1"), rowsEpic("b", "M1"), rowsEpic("c", "M1"),
		rowsEpic("x", "M2", "c"), rowsEpic("y", "M2", "b"), rowsEpic("z", "M2", "a"),
	)
	rows := EpicMapRows(g, EpicMapLanes(g))
	assert.Equal(t, 3, crossings(g, graphRows(g)))
	assert.Equal(t, 0, crossings(g, rows))
}

// TestEpicMapRows_AnEpicWithNoDependencySitsNextToItsFirstDependent: p and r
// have no dependency; p's first dependent is a and r's is c, listed after b,
// so r moves from the end of the graph order to just before c.
func TestEpicMapRows_AnEpicWithNoDependencySitsNextToItsFirstDependent(t *testing.T) {
	g := rowsGraph([]string{"M"},
		rowsEpic("p", "M"), rowsEpic("a", "M", "p"), rowsEpic("b", "M", "p"),
		rowsEpic("r", "M"), rowsEpic("c", "M", "r"), rowsEpic("d", "M", "r"),
	)
	assert.Equal(t, map[string][]string{"M": {"p", "a", "b", "r", "c", "d"}}, EpicMapRows(g, EpicMapLanes(g)))

	g = rowsGraph([]string{"M"},
		rowsEpic("a", "M", "r"), rowsEpic("b", "M", "p"), rowsEpic("p", "M"), rowsEpic("r", "M"),
	)
	assert.Equal(t, map[string][]string{"M": {"r", "a", "p", "b"}}, EpicMapRows(g, EpicMapLanes(g)))
}

