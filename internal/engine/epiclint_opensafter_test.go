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

// TestLintEpicGraph_PartialParallelNamesThePairAndItsEpics is C2.5: R does
// not open after the earlier Q, yet r1 and r2 depend directly on Q epics, so
// exactly one partial-parallel finding names (R, Q) and lists r1 and r2, not
// r4, which reaches Q only through r1. R's entry is explicit and leaves Q out,
// so the operator chose it: severity info. With no R→Q edge there is none.
func TestLintEpicGraph_PartialParallelNamesThePairAndItsEpics(t *testing.T) {
	warnings := findingsOf(t, LintEpicGraph(partialParallelGraph(true), nil), EpicRulePartialParallel)
	require.Len(t, warnings, 1)
	assert.Equal(t, EpicSeverityInfo, warnings[0].Severity)
	assert.Equal(t, []string{"Q", "R"}, warnings[0].Names)
	assert.Equal(t, []string{"r1", "r2"}, warnings[0].Epics)
	assert.Contains(t, warnings[0].Message, `"R"`)
	assert.Contains(t, warnings[0].Message, `"Q"`)

	assert.Empty(t, findingsOf(t, LintEpicGraph(partialParallelGraph(false), nil), EpicRulePartialParallel))
}

// TestLintEpicGraph_LaterMilestoneDependencyKeepsTrackOrder is C2.7: an
// opens_after entry does not reorder a track, so an R→Q edge is no error
// while a Q epic depending on an R epic is still later-milestone-dependency.
func TestLintEpicGraph_LaterMilestoneDependencyKeepsTrackOrder(t *testing.T) {
	g := partialParallelGraph(true)
	assert.Empty(t, opensAfterErrors(g), "an R→Q edge is not a lint error")

	g.OpensAfter["Q"] = []string{}
	g.Epics[1].Root = false
	g.Epics[1].DependsOn = []model.EpicDep{{ID: "r3", Why: "needs r3"}}
	errs := opensAfterErrors(g)
	require.Len(t, errs, 1, "%v", errs)
	assert.Equal(t, EpicRuleLaterMilestoneDependency, errs[0].Rule)
	assert.Equal(t, []string{"q1", "r3"}, errs[0].Epics)
}
