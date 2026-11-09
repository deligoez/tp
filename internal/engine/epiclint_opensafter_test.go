package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// opensAfterErrors returns the error findings of g.
func opensAfterErrors(g *model.EpicGraph) []EpicLintFinding {
	var out []EpicLintFinding
	for _, f := range LintEpicGraph(g, nil) {
		if f.Severity == EpicSeverityError {
			out = append(out, f)
		}
	}
	return out
}

// TestLintEpicGraph_OpensAfterUnknownNamesAndCyclesAreOneErrorEach is C2.4:
// on track [P, Q], an unknown name as a key or as a value, the two-cycle
// {P: [Q], Q: [P]} and the self-reference {P: [P]} are one lint error each,
// and a write adding any of them is refused and writes nothing.
func TestLintEpicGraph_OpensAfterUnknownNamesAndCyclesAreOneErrorEach(t *testing.T) {
	cases := map[string]struct {
		opensAfter map[string][]string
		rule       string
		names      []string
	}{
		"unknown key":    {map[string][]string{"N": {"P"}}, EpicRuleUnknownMilestone, []string{"N"}},
		"unknown value":  {map[string][]string{"Q": {"N"}}, EpicRuleUnknownMilestone, []string{"N"}},
		"two-cycle":      {map[string][]string{"P": {"Q"}, "Q": {"P"}}, EpicRuleOpensAfterCycle, []string{"P", "Q"}},
		"self-reference": {map[string][]string{"P": {"P"}}, EpicRuleOpensAfterCycle, []string{"P"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			g := opensAfterGraph([][]string{{"P", "Q"}}, c.opensAfter)
			errs := opensAfterErrors(g)
			require.Len(t, errs, 1, "%v", errs)
			assert.Equal(t, c.rule, errs[0].Rule)
			assert.Equal(t, c.names, errs[0].Names)
			assert.Equal(t, []string{}, errs[0].Epics)

			body, err := json.Marshal(opensAfterGraph([][]string{{"P", "Q"}}, nil))
			require.NoError(t, err)
			tpDir := epicGraphDir(t, string(body))
			before, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
			require.NoError(t, err)
			_, err = UpdateEpicGraph(tpDir, nil, func(g *model.EpicGraph) error { g.OpensAfter = c.opensAfter; return nil })
			var refused *EpicWriteRefusedError
			require.ErrorAs(t, err, &refused)
			after, err := os.ReadFile(filepath.Join(tpDir, EpicGraphFile))
			require.NoError(t, err)
			assert.Equal(t, before, after, "a refused write writes nothing")
		})
	}
}

// partialParallelGraph is C2.1's track [P, Q, R] with R opening after P: r1
// and r2 depend directly on Q's q1 and q2, r4 on Q only through r1, and r3
// on nothing. Without qEdges R's epics depend on no Q epic.
func partialParallelGraph(qEdges bool) *model.EpicGraph {
	g := opensAfterGraph([][]string{{"P", "Q", "R"}}, map[string][]string{"R": {"P"}})
	g.Epics = []model.Epic{
		{ID: "p", Title: "p", Milestone: "P", Root: true},
		{ID: "q1", Title: "q1", Milestone: "Q", Root: true},
		{ID: "q2", Title: "q2", Milestone: "Q", Root: true},
		{ID: "r1", Title: "r1", Milestone: "R", Root: true},
		{ID: "r2", Title: "r2", Milestone: "R", Root: true},
		{ID: "r3", Title: "r3", Milestone: "R", Root: true},
		{ID: "r4", Title: "r4", Milestone: "R", DependsOn: []model.EpicDep{{ID: "r1", Why: "after r1"}}},
	}
	if qEdges {
		g.Epics[3].Root, g.Epics[4].Root = false, false
		g.Epics[3].DependsOn = []model.EpicDep{{ID: "q1", Why: "needs q1"}}
		g.Epics[4].DependsOn = []model.EpicDep{{ID: "q2", Why: "needs q2"}}
	}
	return g
}

