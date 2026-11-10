package engine

import (
	"fmt"
	"slices"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// EpicRuleAnyOfShape is the error on a depends_on entry that is no sound
// edge or group (spec/1.7.0.md §7): a group with fewer than two distinct
// members, an entry with both or neither of id and any_of, a group beside a
// plain edge to one of its members, or a second group with an earlier
// group's member set.
const EpicRuleAnyOfShape = "any-of-shape"

// epicDepLabel names a depends_on entry in a message: a plain edge by its
// quoted id, a group as one of {A, B}.
func epicDepLabel(d model.EpicDep) string {
	if d.IsGroup() {
		return "one of {" + strings.Join(d.Members(), ", ") + "}"
	}
	return fmt.Sprintf("%q", d.ID)
}

// lintEpicAnyOfShape reports each depends_on entry of each epic whose shape
// any-of-shape refuses, one finding per entry.
func lintEpicAnyOfShape(g *model.EpicGraph) []EpicLintFinding {
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		for j := range e.DependsOn {
			if msg, ids := anyOfEntryShape(e, j); msg != "" {
				out = append(out, epicError(EpicRuleAnyOfShape,
					fmt.Sprintf("epic %q depends_on[%d] %s", e.ID, j, msg), append([]string{e.ID}, ids...)...))
			}
		}
	}
	return out
}

// anyOfEntryShape words what is wrong with e's j-th entry and the ids the
// finding names, or returns "" for a sound entry.
func anyOfEntryShape(e *model.Epic, j int) (msg string, ids []string) {
	d := e.DependsOn[j]
	switch {
	case d.IsGroup() && d.ID != "":
		return "carries both id and any_of", d.Names()
	case !d.IsGroup() && d.ID == "":
		return "carries neither id nor any_of", nil
	case !d.IsGroup():
		return "", nil
	case len(d.Members()) < 2:
		return "is an any_of group with fewer than two distinct members", d.Names()
	}
	if plain := plainEdgesTo(e, d.Members()); len(plain) > 0 {
		return fmt.Sprintf("is the group %s beside a plain edge to %s", epicDepLabel(d), strings.Join(plain, ", ")), plain
	}
	if k := slices.IndexFunc(e.DependsOn[:j], d.SameGroup); k >= 0 {
		return fmt.Sprintf("is the group %s, the member set of depends_on[%d]", epicDepLabel(d), k), d.Names()
	}
	return "", nil
}

// plainEdgesTo returns the members, in order, that e also has a plain edge to.
func plainEdgesTo(e *model.Epic, members []string) []string {
	var out []string
	for _, m := range members {
		if slices.ContainsFunc(e.DependsOn, func(o model.EpicDep) bool { return !o.IsGroup() && o.ID == m }) {
			out = append(out, m)
		}
	}
	return out
}
