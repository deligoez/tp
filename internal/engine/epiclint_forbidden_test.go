package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// forbiddenRule is a forbidden_edges rule from one selector to another.
func forbiddenRule(from, to model.EpicSelector, why string) model.EpicForbiddenEdge {
	return model.EpicForbiddenEdge{From: from, To: to, Why: why}
}

// TestLintEpicGraph_ForbiddenSelectorsNamingNothingAreDanglingReferences: an
// epic, milestone or track_of selector naming nothing the graph holds is a
// dangling-reference error, one per selector, while a rule whose selectors
// all name something is clean.
func TestLintEpicGraph_ForbiddenSelectorsNamingNothingAreDanglingReferences(t *testing.T) {
	g := cleanEpicGraph()
	g.ForbiddenEdges = []model.EpicForbiddenEdge{
		forbiddenRule(model.EpicSelector{TrackOf: "A"}, model.EpicSelector{Milestone: "L1"}, "track A ships first"),
		forbiddenRule(model.EpicSelector{Epic: "z"}, model.EpicSelector{Milestone: "M9"}, "absent names"),
		forbiddenRule(model.EpicSelector{TrackOf: "Q"}, model.EpicSelector{Epic: "c"}, "absent track"),
	}
	dangling := findingsOf(t, LintEpicGraph(g, nil), EpicRuleDanglingReference)
	require.Len(t, dangling, 3)
	var messages []string
	for _, f := range dangling {
		assert.Equal(t, EpicSeverityError, f.Severity)
		messages = append(messages, f.Message)
	}
	assert.Equal(t, []string{"z"}, dangling[0].Epics)
	assert.Equal(t, []string{
		`forbidden_edges[1].from epic "z" names no epic`,
		`forbidden_edges[1].to milestone "M9" names no milestone`,
		`forbidden_edges[2].from track_of "Q" names no milestone`,
	}, messages)
}

// TestLintEpicGraph_AForbiddenTagNoEpicCarriesIsAWarning: a tag selector no
// epic carries is a dangling-reference warning, so a rule may precede its
// tag, and a carried tag is clean.
func TestLintEpicGraph_AForbiddenTagNoEpicCarriesIsAWarning(t *testing.T) {
	g := cleanEpicGraph()
	g.ForbiddenEdges = []model.EpicForbiddenEdge{
		forbiddenRule(model.EpicSelector{Tag: "ui"}, model.EpicSelector{Tag: "later"}, "ui before later"),
	}
	findings := LintEpicGraph(g, nil)
	dangling := findingsOf(t, findings, EpicRuleDanglingReference)
	require.Len(t, dangling, 1)
	assert.Equal(t, EpicSeverityWarning, dangling[0].Severity)
	assert.Equal(t, `forbidden_edges[0].to tag "later" is carried by no epic`, dangling[0].Message)
	assert.Zero(t, SummarizeEpicLint(findings).Errors, "an uncarried tag is no error")
}

// TestLintEpicGraph_AForbiddenRuleWithoutAWhyIsMissingWhy: a rule whose why
// is empty or blank is a missing-why error.
func TestLintEpicGraph_AForbiddenRuleWithoutAWhyIsMissingWhy(t *testing.T) {
	g := cleanEpicGraph()
	g.ForbiddenEdges = []model.EpicForbiddenEdge{
		forbiddenRule(model.EpicSelector{Epic: "a"}, model.EpicSelector{Epic: "c"}, "a never names c"),
		forbiddenRule(model.EpicSelector{Epic: "a"}, model.EpicSelector{Epic: "b"}, "  "),
	}
	missing := findingsOf(t, LintEpicGraph(g, nil), EpicRuleMissingWhy)
	require.Len(t, missing, 1)
	assert.Equal(t, EpicSeverityError, missing[0].Severity)
	assert.Equal(t, "forbidden_edges[1] has no why", missing[0].Message)
}
