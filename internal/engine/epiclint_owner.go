package engine

import (
	"fmt"
	"strings"

	"github.com/deligoez/tp/internal/model"
)

// Owner rules of the epic graph lint (spec/backlog/epic-owner.md).
const (
	// EpicRuleUnknownOwner is the error on an epic naming a handle the
	// owners set does not declare.
	EpicRuleUnknownOwner = "unknown-owner"
	// EpicRuleOwnerHandle is the error on a handle that would let one person
	// be two: blank, with leading or trailing space, declared twice, equal to
	// another declared handle ignoring case, or named twice by one epic.
	EpicRuleOwnerHandle = "owner-handle"
)

// lintEpicOwners reports each handle an epic names that the owners set does
// not declare, once per epic and handle, in graph order.
// lintEpicOwners reports the owners set's bad handles in declaration order,
// then, epic by epic in graph order, each handle an epic names twice and each
// it names that the set does not declare.
func lintEpicOwners(g *model.EpicGraph) []EpicLintFinding {
	out := lintOwnerSet(g.Owners)
	declared := make(map[string]bool, len(g.Owners))
	for _, o := range g.Owners {
		declared[o.Handle] = true
	}
	for i := range g.Epics {
		e := &g.Epics[i]
		named := make(map[string]bool, len(e.Owner))
		for _, h := range e.Owner {
			switch {
			case named[h]:
				out = append(out, epicNamedError(EpicRuleOwnerHandle,
					fmt.Sprintf("epic %q names owner %q twice", e.ID, h), []string{h}, e.ID))
			case !declared[h]:
				out = append(out, epicNamedError(EpicRuleUnknownOwner,
					fmt.Sprintf("epic %q names owner %q, which owners does not declare", e.ID, h), []string{h}, e.ID))
			}
			named[h] = true
		}
	}
	return out
}

// lintOwnerSet reports each declared handle that is blank, carries leading or
// trailing space, repeats an earlier one, or equals an earlier one ignoring
// case; one finding per handle, the first shape that applies.
func lintOwnerSet(owners []model.EpicOwner) []EpicLintFinding {
	var out []EpicLintFinding
	seen := make(map[string]string, len(owners))
	for _, o := range owners {
		h := o.Handle
		folded := strings.ToLower(h)
		first, repeated := seen[folded]
		var msg string
		switch {
		case strings.TrimSpace(h) == "":
			msg = "owners declares a blank handle"
		case strings.TrimSpace(h) != h:
			msg = fmt.Sprintf("owners declares %q, with leading or trailing space", h)
		case repeated && first == h:
			msg = fmt.Sprintf("owners declares %q twice", h)
		case repeated:
			msg = fmt.Sprintf("owners declares %q and %q, equal ignoring case", first, h)
		}
		if !repeated {
			seen[folded] = h
		}
		if msg != "" {
			out = append(out, epicNamedError(EpicRuleOwnerHandle, msg, []string{h}))
		}
	}
	return out
}
