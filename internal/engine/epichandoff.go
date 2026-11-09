package engine

import (
	"regexp"
	"slices"

	"github.com/deligoez/tp/internal/model"
)

// EpicHandoffContext returns the block of every epic the round's change newly
// names: named, as a whole word, in the title or acceptance of an asked epic
// whose own block changed since the round before (carry.Before), and not named
// in that epic's block as the round before read it; unless it is asked or
// already context. A fix that hands work to a neighbour is the change the
// regression role has to check the neighbour takes. An old hand-off is left
// out: showing every name the text holds turns each round into an audit of
// ownership the change did not touch.
func EpicHandoffContext(g *model.EpicGraph, carry *EpicCarry, context []EpicContextBlock) []EpicContextBlock {
	skip := map[string]bool{}
	for _, u := range carry.Asked {
		if id, ok := epicUnitID(u.Key); ok {
			skip[id] = true
		}
	}
	for _, c := range context {
		skip[c.ID] = true
	}
	type change struct{ now, before string }
	var changes []change
	for key, before := range carry.Before {
		if id, ok := epicUnitID(key); ok {
			if i := epicIndex(g, id); i >= 0 {
				changes = append(changes, change{now: g.Epics[i].Title + "\n" + g.Epics[i].Acceptance, before: before})
			}
		}
	}
	var out []EpicContextBlock
	for i := range g.Epics {
		e := &g.Epics[i]
		if skip[e.ID] {
			continue
		}
		for _, c := range changes {
			if namedInAny(e.ID, []string{c.now}) && !namedInAny(e.ID, []string{c.before}) {
				skip[e.ID] = true
				out = append(out, EpicContextBlock{ID: e.ID, Block: epicBlock(e)})
				break
			}
		}
	}
	return out
}

// namedInAny reports whether id occurs in one of texts with no letter, digit,
// '-' or '_' on either side.
func namedInAny(id string, texts []string) bool {
	if len(texts) == 0 {
		return false
	}
	re := regexp.MustCompile(`(^|[^\p{L}\p{N}_-])` + regexp.QuoteMeta(id) + `($|[^\p{L}\p{N}_-])`)
	return slices.ContainsFunc(texts, re.MatchString)
}
