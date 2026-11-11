package engine

import (
	"fmt"

	"github.com/deligoez/tp/internal/model"
)

// LintEpicThresholds reports gate-threshold-unwritten (spec/1.8.0.md §8):
// a warning on each unmet gate, in milestone name order, whose trimmed
// threshold matcher calls a placeholder. An absent or blank threshold is not
// one. It is apart from LintEpicGraph because it reads the project's
// pattern; being a warning, it never makes a writer refuse.
func LintEpicThresholds(g *model.EpicGraph, matcher *PlaceholderMatcher) []EpicLintFinding {
	var out []EpicLintFinding
	for _, m := range sortedMapKeys(g.Gates) {
		gate := g.Gates[m]
		if gate.Met != nil || !matcher.IsPlaceholder(gate.Threshold) {
			continue
		}
		f := epicNamedError(EpicRuleGateThresholdUnwritten,
			fmt.Sprintf("gate %q threshold %q is a placeholder: write the exit condition (tp epic set gates.%s.threshold=<text>)", m, gate.Threshold, m),
			[]string{m})
		f.Severity = EpicSeverityWarning
		out = append(out, f)
	}
	return out
}
