package engine

import "github.com/deligoez/tp/internal/model"

// ThresholdRewrites lists, in track order and each once, the tracked
// milestones whose gate has no met record and whose threshold is a
// placeholder by matcher's verdict (spec/1.8.0.md §7): the operator's
// decision there is to write the threshold, whether or not the members are
// done.
func ThresholdRewrites(g *model.EpicGraph, matcher *PlaceholderMatcher) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, track := range g.Milestones {
		for _, m := range track {
			gate := g.Gates[m]
			if seen[m] || gate.Met != nil || !matcher.IsPlaceholder(gate.Threshold) {
				continue
			}
			seen[m] = true
			out = append(out, m)
		}
	}
	return out
}

// WithoutPlaceholderGates keeps the milestones of gates whose threshold is
// not a placeholder: a gate awaiting --met has a written threshold to be
// measured against (spec/1.8.0.md §7).
func WithoutPlaceholderGates(g *model.EpicGraph, gates []string, matcher *PlaceholderMatcher) []string {
	out := make([]string, 0, len(gates))
	for _, m := range gates {
		if !matcher.IsPlaceholder(g.Gates[m].Threshold) {
			out = append(out, m)
		}
	}
	return out
}
