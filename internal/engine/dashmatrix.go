package engine

import "github.com/deligoez/tp/internal/model"

// MatrixCell is one cell of the dashboard's milestone matrix (spec/1.8.0.md
// §4): the dependency edges from the epics of From to the epics of To, each
// member of an any-of group counted as one edge, and how many of those
// dependencies are done.
type MatrixCell struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Edges int    `json:"edges"`
	Done  int    `json:"done"`
}

// MilestoneMatrix counts g's edges between milestones, one cell per pair an
// edge joins, rows and columns in milestoneOrder; an edge inside one
// milestone, or to an epic the graph does not hold, has no cell.
func MilestoneMatrix(g *model.EpicGraph, states map[string]EpicState) []MatrixCell {
	msOf := make(map[string]string, len(g.Epics))
	for i := range g.Epics {
		msOf[g.Epics[i].ID] = g.Epics[i].Milestone
	}
	cells := map[[2]string]*MatrixCell{}
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, d := range e.DependsOn {
			for _, x := range depTargets(d) {
				to, ok := msOf[x]
				if !ok || to == e.Milestone {
					continue
				}
				k := [2]string{e.Milestone, to}
				c := cells[k]
				if c == nil {
					c = &MatrixCell{From: e.Milestone, To: to}
					cells[k] = c
				}
				c.Edges++
				if states[x].Status == EpicStatusDone {
					c.Done++
				}
			}
		}
	}
	order := milestoneOrder(g)
	out := make([]MatrixCell, 0, len(cells))
	for _, from := range order {
		for _, to := range order {
			if c := cells[[2]string{from, to}]; c != nil {
				out = append(out, *c)
			}
		}
	}
	return out
}

// depTargets is the epics one dependency names: its id, or each distinct
// member of its any-of group.
func depTargets(d model.EpicDep) []string {
	if d.IsGroup() {
		return d.Members()
	}
	return []string{d.ID}
}
