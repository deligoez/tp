package engine

import (
	"regexp"

	"github.com/deligoez/tp/internal/model"
)

// EpicHandoffContext returns the block of every epic named, as a whole word,
// in the title or acceptance of an asked epic whose own block changed since
// the round before (carry.Before), unless it is asked or already context.
// A fix that hands work to a neighbour is the change the regression role has
// to check the neighbour takes, so it needs the neighbour's block in view.
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
	var texts []string
	for key := range carry.Before {
		if id, ok := epicUnitID(key); ok {
			if i := epicIndex(g, id); i >= 0 {
				texts = append(texts, g.Epics[i].Title+"\n"+g.Epics[i].Acceptance)
			}
		}
	}
	var out []EpicContextBlock
	for i := range g.Epics {
		e := &g.Epics[i]
		if skip[e.ID] || !namedInAny(e.ID, texts) {
			continue
		}
		skip[e.ID] = true
		out = append(out, EpicContextBlock{ID: e.ID, Block: epicBlock(e)})
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
	for _, t := range texts {
		if re.MatchString(t) {
			return true
		}
	}
	return false
}
