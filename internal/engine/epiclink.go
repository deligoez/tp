package engine

import (
	"fmt"
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// The actions tp epic link and unlink report (spec/1.4.0.md §1).
const (
	EpicEdgeAdded    = "added"
	EpicEdgeReplaced = "replaced"
	EpicEdgeRemoved  = "removed"
)

// EpicEdgeNotFoundError reports an unlink of an edge the graph does not hold.
type EpicEdgeNotFoundError struct {
	From, To string
}

func (e *EpicEdgeNotFoundError) Error() string {
	return fmt.Sprintf("epic %q has no edge to %q", e.From, e.To)
}

// LinkEpic returns the mutation that adds the edge from -> to with why, or
// replaces the why of that edge when it exists, and records which in action.
// An absent from is an *EpicNotFoundError; an absent to and an empty why are
// left to the write's lint refusal.
func LinkEpic(from, to, why string, action *string) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, from)
		if i < 0 {
			return &EpicNotFoundError{ID: from}
		}
		e := &g.Epics[i]
		for j := range e.DependsOn {
			if e.DependsOn[j].ID == to {
				e.DependsOn[j].Why = why
				*action = EpicEdgeReplaced
				return nil
			}
		}
		e.DependsOn = append(e.DependsOn, model.EpicDep{ID: to, Why: why})
		*action = EpicEdgeAdded
		return nil
	}
}

// UnlinkEpic returns the mutation that removes the edge from -> to. An
// absent from is an *EpicNotFoundError and an absent edge an
// *EpicEdgeNotFoundError.
func UnlinkEpic(from, to string, action *string) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, from)
		if i < 0 {
			return &EpicNotFoundError{ID: from}
		}
		e := &g.Epics[i]
		j := slices.IndexFunc(e.DependsOn, func(d model.EpicDep) bool { return d.ID == to })
		if j < 0 {
			return &EpicEdgeNotFoundError{From: from, To: to}
		}
		e.DependsOn = slices.Delete(e.DependsOn, j, j+1)
		*action = EpicEdgeRemoved
		return nil
	}
}
