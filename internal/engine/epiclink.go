package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// The actions tp epic link and unlink report (spec/1.4.0.md §1).
const (
	EpicEdgeAdded    = "added"
	EpicEdgeReplaced = "replaced"
	EpicEdgeRemoved  = "removed"
)

// EpicEdgeNotFoundError reports an unlink of an edge the graph does not hold:
// a plain edge to To, or, when AnyOf is set, a group with that member set.
// Groups lists the member lists of From's groups that hold To, so a plain
// unlink of a group member can name the group to unlink instead.
type EpicEdgeNotFoundError struct {
	From, To string
	AnyOf    []string
	Groups   [][]string
}

func (e *EpicEdgeNotFoundError) Error() string {
	if e.AnyOf != nil {
		return fmt.Sprintf("epic %q has no any_of group %s", e.From, epicDepLabel(model.EpicDep{AnyOf: e.AnyOf}))
	}
	var msg strings.Builder
	fmt.Fprintf(&msg, "epic %q has no edge to %q", e.From, e.To)
	for _, members := range e.Groups {
		fmt.Fprintf(&msg, "; %q is a member of its group %s", e.To, epicDepLabel(model.EpicDep{AnyOf: members}))
	}
	return msg.String()
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
			if !e.DependsOn[j].IsGroup() && e.DependsOn[j].ID == to {
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
		j := slices.IndexFunc(e.DependsOn, func(d model.EpicDep) bool { return !d.IsGroup() && d.ID == to })
		if j < 0 {
			return &EpicEdgeNotFoundError{From: from, To: to, Groups: groupsHolding(e, to)}
		}
		e.DependsOn = slices.Delete(e.DependsOn, j, j+1)
		*action = EpicEdgeRemoved
		return nil
	}
}

// groupsHolding returns the members of each of e's groups that hold id.
func groupsHolding(e *model.Epic, id string) [][]string {
	var out [][]string
	for _, d := range e.DependsOn {
		if d.IsGroup() && slices.Contains(d.AnyOf, id) {
			out = append(out, d.Members())
		}
	}
	return out
}

// LinkEpicAnyOf returns the mutation that adds the any-of group of members
// to from with why, or replaces the why of the group from already has with
// that member set (spec/1.7.0.md §7), and records which in action. Members
// are stored distinct, in the order given. An absent from is an
// *EpicNotFoundError; a member set the shape rule refuses, an absent member
// and an empty why are left to the write's lint refusal.
func LinkEpicAnyOf(from string, members []string, why string, action *string) func(*model.EpicGraph) error {
	group := model.EpicDep{AnyOf: append([]string{}, members...), Why: why}
	group.AnyOf = group.Members()
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, from)
		if i < 0 {
			return &EpicNotFoundError{ID: from}
		}
		e := &g.Epics[i]
		if j := slices.IndexFunc(e.DependsOn, group.SameGroup); j >= 0 {
			e.DependsOn[j].Why = why
			*action = EpicEdgeReplaced
			return nil
		}
		e.DependsOn = append(e.DependsOn, group)
		*action = EpicEdgeAdded
		return nil
	}
}

// UnlinkEpicAnyOf returns the mutation that removes from's group with the
// member set of members, in any order. An absent from is an
// *EpicNotFoundError and no such group an *EpicEdgeNotFoundError.
func UnlinkEpicAnyOf(from string, members []string, action *string) func(*model.EpicGraph) error {
	group := model.EpicDep{AnyOf: append([]string{}, members...)}
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, from)
		if i < 0 {
			return &EpicNotFoundError{ID: from}
		}
		e := &g.Epics[i]
		j := slices.IndexFunc(e.DependsOn, group.SameGroup)
		if j < 0 {
			return &EpicEdgeNotFoundError{From: from, AnyOf: group.Members()}
		}
		e.DependsOn = slices.Delete(e.DependsOn, j, j+1)
		*action = EpicEdgeRemoved
		return nil
	}
}
