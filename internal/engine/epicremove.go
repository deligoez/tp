package engine

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicRemoveRefusedError reports a tp epic remove that §3 refuses: epics
// still depend on the epic (lifted by --force), or a tag_checks entry names it
// as provider (refused even under --force, D8). Nothing was written.
type EpicRemoveRefusedError struct {
	ID         string
	Dependents []string
	TagChecks  []string
}

func (e *EpicRemoveRefusedError) Error() string {
	if len(e.TagChecks) > 0 {
		return fmt.Sprintf("epic %q provides tag_checks %s; repoint or drop them with tp epic set tag_checks=<json> first",
			e.ID, strings.Join(e.TagChecks, ", "))
	}
	return fmt.Sprintf("epics %s depend on %q; --force also drops those edges and drops or narrows those any-of groups", strings.Join(e.Dependents, ", "), e.ID)
}

// RemoveEpic returns the mutation that removes epic id from the graph. Without
// force it refuses an epic another epic depends on, through a plain edge or a
// group membership; with force it also drops the edges into it, its groups
// as dropEpicFromDeps does, and its gate memberships. An epic a tag_checks
// entry names as provider is refused either way. A missing id is an
// *EpicNotFoundError.
func RemoveEpic(id string, force bool) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		refused := &EpicRemoveRefusedError{ID: id, Dependents: epicDependents(g, id), TagChecks: providedTagChecks(g, id)}
		if len(refused.TagChecks) > 0 || (len(refused.Dependents) > 0 && !force) {
			return refused
		}
		removeEpicAt(g, i, true)
		return nil
	}
}

// RemoveEpicRow returns the mutation a tp epic apply remove row makes
// (spec/1.7.0.md §6): only an absent id fails at the row. It removes epic id
// and its gate memberships, and with force also the edges into it and its
// groups, as RemoveEpic does; what it leaves naming the epic, an edge
// without force or a tag_checks provider, waits for the write's lint.
func RemoveEpicRow(id string, force bool) func(*model.EpicGraph) error {
	return func(g *model.EpicGraph) error {
		i := epicIndex(g, id)
		if i < 0 {
			return &EpicNotFoundError{ID: id}
		}
		removeEpicAt(g, i, force)
		return nil
	}
}

// removeEpicAt removes g's i-th epic and its gate memberships and, with
// dropEdges, the edges into it and its groups as dropEpicFromDeps does.
func removeEpicAt(g *model.EpicGraph, i int, dropEdges bool) {
	id, done := g.Epics[i].ID, g.Epics[i].Done != nil
	g.Epics = slices.Delete(g.Epics, i, i+1)
	for j := range g.Epics {
		if e := &g.Epics[j]; dropEdges {
			e.DependsOn = dropEpicFromDeps(e.DependsOn, id, done)
		}
	}
	for name, gate := range g.Gates {
		gate.Members = slices.DeleteFunc(gate.Members, func(m string) bool { return m == id })
		gate.Deferrable = slices.DeleteFunc(gate.Deferrable, func(m string) bool { return m == id })
		g.Gates[name] = gate
	}
}

// dropEpicFromDeps returns deps without epic id (spec/1.7.0.md §7): a plain
// edge to it goes; a group holding it goes whole when id was done, since the
// group was satisfied, and otherwise loses that member, a group left with one
// member becoming a plain edge with the group's why. Entries that then
// repeat another go as implied (dropImpliedDeps). deps naming no id come back
// as they are.
func dropEpicFromDeps(deps []model.EpicDep, id string, done bool) []model.EpicDep {
	if !slices.ContainsFunc(deps, func(d model.EpicDep) bool { return slices.Contains(d.Names(), id) }) {
		return deps
	}
	out := make([]model.EpicDep, 0, len(deps))
	var narrowed []bool
	for _, d := range deps {
		switch {
		case !d.IsGroup() && d.ID == id:
			continue
		case !d.IsGroup() || !slices.Contains(d.AnyOf, id):
			out, narrowed = append(out, d), append(narrowed, false)
		case done:
			continue
		default:
			out, narrowed = append(out, narrowedGroup(d, id)), append(narrowed, true)
		}
	}
	return dropImpliedDeps(out, narrowed)
}

// narrowedGroup returns group d without member id: still a group with two or
// more members, else the plain edge to the one left, keeping d's why.
func narrowedGroup(d model.EpicDep, id string) model.EpicDep {
	members := slices.DeleteFunc(d.Members(), func(m string) bool { return m == id })
	if len(members) == 1 {
		return model.EpicDep{ID: members[0], Why: d.Why}
	}
	return model.EpicDep{AnyOf: members, Why: d.Why}
}

// dropImpliedDeps drops what a narrowing left implied: an entry narrowed into
// a plain edge the list already holds, a group with a member the list has a
// plain edge to, and a group repeating an earlier group's member set. It runs
// only on an epic the removal touched, and keeps every plain edge that was
// there before.
func dropImpliedDeps(deps []model.EpicDep, narrowed []bool) []model.EpicDep {
	plain := map[string]bool{}
	keep := make([]bool, len(deps))
	for j, d := range deps {
		if !d.IsGroup() && !narrowed[j] {
			plain[d.ID], keep[j] = true, true
		}
	}
	for j, d := range deps {
		if !d.IsGroup() && narrowed[j] && !plain[d.ID] {
			plain[d.ID], keep[j] = true, true
		}
	}
	var out []model.EpicDep
	for j, d := range deps {
		implied := d.IsGroup() && (slices.ContainsFunc(d.Members(), func(m string) bool { return plain[m] }) ||
			slices.ContainsFunc(out, d.SameGroup))
		if keep[j] || (d.IsGroup() && !implied) {
			out = append(out, d)
		}
	}
	return out
}

// epicDependents lists, in file order, the epics with an edge into id or a
// group holding it.
func epicDependents(g *model.EpicGraph, id string) []string {
	var out []string
	for i := range g.Epics {
		if slices.ContainsFunc(g.Epics[i].DependsOn, func(d model.EpicDep) bool { return slices.Contains(d.Names(), id) }) {
			out = append(out, g.Epics[i].ID)
		}
	}
	return out
}

// providedTagChecks lists, sorted, the tags whose check id provides.
func providedTagChecks(g *model.EpicGraph, id string) []string {
	var out []string
	for tag, check := range g.TagChecks {
		if check.ProvidedBy == id {
			out = append(out, tag)
		}
	}
	sort.Strings(out)
	return out
}
