package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// opensLaterGraph is tracks [[A], [B]] with B opening after A, holding epics.
func opensLaterGraph(epics ...model.Epic) *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}, {"B"}},
		OpensAfter: map[string][]string{"B": {"A"}},
		Epics:      epics,
	}
}

// dependsOn is an epic of milestone m depending on ids, root when it has none.
func dependsOn(id, m string, ids ...string) model.Epic {
	e := model.Epic{ID: id, Title: id, Milestone: m, Root: len(ids) == 0}
	for _, d := range ids {
		e.DependsOn = append(e.DependsOn, model.EpicDep{ID: d, Why: id + " needs " + d})
	}
	return e
}

// TestLintEpicGraph_DependencyOpensLaterMemberIsAnError is fixture (i): a,
// a member of A's gate, depends on b in B, and B opens after A, so A's gate
// can never go green before B opens: one error naming a and b.
func TestLintEpicGraph_DependencyOpensLaterMemberIsAnError(t *testing.T) {
	g := opensLaterGraph(dependsOn("a", "A", "b"), dependsOn("b", "B"))
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityError, found[0].Severity)
	assert.Equal(t, []string{"a", "b"}, found[0].Epics)
	assert.Equal(t, []string{"A", "B"}, found[0].Names)
	assert.Contains(t, found[0].Message, "a -> b")
}

// TestLintEpicGraph_DependencyOpensLaterThroughADeferrableIsAnError is
// fixture (ii): the member a reaches b through c, a deferrable epic of A, so
// A's gate still waits on B: one error with the path, and no warning for c.
func TestLintEpicGraph_DependencyOpensLaterThroughADeferrableIsAnError(t *testing.T) {
	g := opensLaterGraph(dependsOn("a", "A", "c"), dependsOn("c", "A", "b"), dependsOn("b", "B"))
	g.Gates = map[string]model.EpicGate{"A": {Deferrable: []string{"c"}}}
	found := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDependencyOpensLater)
	require.Len(t, found, 1, "%v", found)
	assert.Equal(t, EpicSeverityError, found[0].Severity)
	assert.Equal(t, []string{"a", "b"}, found[0].Epics)
	assert.Contains(t, found[0].Message, "a -> c -> b")
}

