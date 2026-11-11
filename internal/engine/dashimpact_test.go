package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// releaseImpactGraph: a is the epic a spec binds; b waits on a and the done
// c, so a is its last undone dependency; d waits on a and the undone x. M1's
// gate holds a, c, d and x, c alone done; b sits in M2.
func releaseImpactGraph() (g *model.EpicGraph, states map[string]EpicState) {
	dep := func(ids ...string) []model.EpicDep {
		out := make([]model.EpicDep, 0, len(ids))
		for _, id := range ids {
			out = append(out, model.EpicDep{ID: id, Why: "needs it"})
		}
		return out
	}
	g = &model.EpicGraph{Version: 1, Milestones: [][]string{{"M1", "M2"}}, Epics: []model.Epic{
		{ID: "a", Milestone: "M1", Root: true},
		{ID: "c", Milestone: "M1", Root: true},
		{ID: "x", Milestone: "M1", Root: true},
		{ID: "d", Milestone: "M1", DependsOn: dep("a", "x")},
		{ID: "b", Milestone: "M2", DependsOn: dep("a", "c")},
	}}
	states = map[string]EpicState{
		"a": {Status: EpicStatusSpecced}, "c": {Status: EpicStatusDone},
		"x": {Status: EpicStatusPlanned}, "d": {Status: EpicStatusPlanned},
		"b": {Status: EpicStatusPlanned},
	}
	return g, states
}

// TestSpecReleaseImpact_NamesTheFreedEpicAndTheMovedGate is spec/1.8.0.md
// §13: releasing the spec bound to a frees b, whose last undone dependency a
// is, and not d, which still waits on x; M1's gate moves from 1 to 2 of 4
// done, M2's does not move. A mutant freeing every dependent of a fails it.
func TestSpecReleaseImpact_NamesTheFreedEpicAndTheMovedGate(t *testing.T) {
	t.Parallel()
	g, states := releaseImpactGraph()
	assert.Equal(t, ReleaseImpact{
		Frees: []string{"b"},
		Gates: []GateMove{{Milestone: "M1", MembersDone: 1, MembersDoneAfter: 2, Members: 4}},
	}, SpecReleaseImpact(g, states, "a"))

	states["a"] = EpicState{Status: EpicStatusDone}
	assert.Equal(t, ReleaseImpact{Frees: []string{}, Gates: []GateMove{}}, SpecReleaseImpact(g, states, "a"),
		"an epic already done frees nothing")
}

