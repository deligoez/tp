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
	return fmt.Sprintf("epics %s depend on %q; --force also drops those edges", strings.Join(e.Dependents, ", "), e.ID)
}

// RemoveEpic returns the mutation that removes epic id from the graph. Without
// force it refuses an epic another epic depends on; with force it also drops
// the edges into it and its gate memberships. An epic a tag_checks entry names
// as provider is refused either way. A missing id is an *EpicNotFoundError.
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
		g.Epics = slices.Delete(g.Epics, i, i+1)
		for j := range g.Epics {
			e := &g.Epics[j]
			e.DependsOn = slices.DeleteFunc(e.DependsOn, func(d model.EpicDep) bool { return d.ID == id })
		}
		for name, gate := range g.Gates {
			gate.Members = slices.DeleteFunc(gate.Members, func(m string) bool { return m == id })
			gate.Deferrable = slices.DeleteFunc(gate.Deferrable, func(m string) bool { return m == id })
			g.Gates[name] = gate
		}
		return nil
	}
}

// epicDependents lists, in file order, the epics with an edge into id.
func epicDependents(g *model.EpicGraph, id string) []string {
	var out []string
	for i := range g.Epics {
		if slices.ContainsFunc(g.Epics[i].DependsOn, func(d model.EpicDep) bool { return d.ID == id }) {
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
