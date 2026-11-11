package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// thresholdGraph is spec/1.8.0.md §18 row 5's fixture: unmet gates whose
// thresholds are `[threshold]`, `TBD`, `draft: 80% pass`, a written one and
// none, and a met gate whose threshold is still `TBD`.
func thresholdGraph() *model.EpicGraph {
	met := &model.GateMet{At: time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), Evidence: "measured", Threshold: "TBD"}
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B", "C", "D", "E", "F"}},
		Gates: map[string]model.EpicGate{
			"A": {Threshold: "[threshold]"},
			"B": {Threshold: " TBD "},
			"C": {Threshold: "draft: 80% pass"},
			"D": {Threshold: "90% of transfers reconcile"},
			"E": {},
			"F": {Threshold: "TBD", Met: met},
		},
	}
}

// unwrittenGates is the milestones whose gate the rule warns on, in order.
func unwrittenGates(t *testing.T, pattern string) []string {
	t.Helper()
	m, err := NewPlaceholderMatcher(pattern)
	require.NoError(t, err)
	var out []string
	for _, f := range findingsOf(t, LintEpicThresholds(thresholdGraph(), m), EpicRuleGateThresholdUnwritten) {
		assert.Equal(t, EpicSeverityWarning, f.Severity)
		require.Len(t, f.Names, 1)
		out = append(out, f.Names[0])
	}
	return out
}

// TestLintEpicThresholds_WarnsOnAnUnmetGateWithAPlaceholder is §18 row 5:
// under the default pattern `[threshold]` and `TBD` warn, under the project
// pattern `^draft:` only `draft: 80% pass` does; a written threshold, an
// absent one and a met gate never warn.
func TestLintEpicThresholds_WarnsOnAnUnmetGateWithAPlaceholder(t *testing.T) {
	assert.Equal(t, []string{"A", "B"}, unwrittenGates(t, ""))
	assert.Equal(t, []string{"C"}, unwrittenGates(t, "^draft:"))
}
