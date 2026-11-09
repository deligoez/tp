package engine

import (
	"github.com/deligoez/tp/internal/model"
)

// AncestryChange is one milestone whose ancestry, the milestones it opens
// after transitively over the effective opens_after, a write changed: the
// names it lost and gained, each in track order.
type AncestryChange struct {
	Milestone string   `json:"milestone"`
	Lost      []string `json:"lost"`
	Gained    []string `json:"gained"`
}

// EpicAncestryChanges compares every milestone's ancestry in before and after
// (spec/1.6.0.md §1), so a writer of opens_after or milestones shows what an
// edit did beyond the entry it named. Milestones come in after's track order,
// then those only before lists; lost names in before's track order, gained in
// after's. Only changed milestones are listed; never nil.
func EpicAncestryChanges(before, after *model.EpicGraph) []AncestryChange {
	was, now := EffectiveOpensAfter(before), EffectiveOpensAfter(after)
	beforeOrder, afterOrder := milestoneUnion(before, after), milestoneUnion(after, before)
	out := []AncestryChange{}
	for _, m := range afterOrder {
		old, cur := opensAfterClosure(was, m), opensAfterClosure(now, m)
		lost, gained := namesNotIn(beforeOrder, old, cur), namesNotIn(afterOrder, cur, old)
		if len(lost) > 0 || len(gained) > 0 {
			out = append(out, AncestryChange{Milestone: m, Lost: lost, Gained: gained})
		}
	}
	return out
}

// milestoneUnion lists, once each, the milestones first's tracks name in
// order, then those only second's tracks name.
func milestoneUnion(first, second *model.EpicGraph) []string {
	seen := map[string]bool{}
	var out []string
	for _, g := range []*model.EpicGraph{first, second} {
		for _, track := range g.Milestones {
			for _, m := range track {
				if !seen[m] {
					seen[m] = true
					out = append(out, m)
				}
			}
		}
	}
	return out
}

// namesNotIn returns, in order's order, the names in has and not in lacks.
func namesNotIn(order []string, has, lacks map[string]bool) []string {
	out := []string{}
	for _, m := range order {
		if has[m] && !lacks[m] {
			out = append(out, m)
		}
	}
	return out
}
