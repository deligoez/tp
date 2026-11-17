package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// Owner rules of the epic graph lint (spec/backlog/epic-owner.md).
const (
	// EpicRuleUnknownOwner is the error on an epic naming a handle the
	// owners set does not declare.
	EpicRuleUnknownOwner = "unknown-owner"
)

// lintEpicOwners reports each handle an epic names that the owners set does
// not declare, once per epic and handle, in graph order.
func lintEpicOwners(g *model.EpicGraph) []EpicLintFinding {
	declared := make(map[string]bool, len(g.Owners))
	for _, o := range g.Owners {
		declared[o.Handle] = true
	}
	var out []EpicLintFinding
	for i := range g.Epics {
		e := &g.Epics[i]
		for _, h := range e.Owner {
			if !declared[h] {
				out = append(out, epicNamedError(EpicRuleUnknownOwner,
					fmt.Sprintf("epic %q names owner %q, which owners does not declare", e.ID, h), []string{h}, e.ID))
			}
		}
	}
	return out
}
