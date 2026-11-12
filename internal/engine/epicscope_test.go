package engine

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// scopeOf derives g's states with no bound file and no estimates and
// returns milestone m's scope.
func scopeOf(t *testing.T, g *model.EpicGraph, m string) EpicScope {
	t.Helper()
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)
	return MilestoneScope(g, states, nil, m)
}

// scopeDeps is one plain edge per id.
func scopeDeps(ids ...string) []model.EpicDep {
	deps := make([]model.EpicDep, 0, len(ids))
	for _, id := range ids {
		deps = append(deps, model.EpicDep{ID: id, Why: "needs it"})
	}
	return deps
}

// scopeDone is a done record.
func scopeDone() *model.EpicDone {
	return &model.EpicDone{At: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), Evidence: "shipped"}
}

// closureIDs flattens a scope's closure in its order.
func closureIDs(s EpicScope) []string {
	out := []string{}
	for _, m := range s.Closure {
		for _, e := range m.Epics {
			out = append(out, e.ID)
		}
	}
	return out
}

// scopeEpic is a work epic of milestone m.
func scopeEpic(id, m string, deps ...model.EpicDep) model.Epic {
	return model.Epic{ID: id, Title: id, Milestone: m, DependsOn: deps}
}

// TestScope_MembersImplicitOrExplicit: an unset gate makes every epic of the
// milestone that is neither a condition nor deferrable a member and says the
// members are implicit; an explicit list is the members as written.
func TestScope_MembersImplicitOrExplicit(t *testing.T) {
	cond := scopeEpic("c1", "m0")
	cond.Condition = true
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0"), scopeEpic("a2", "m0"), cond, scopeEpic("a3", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Deferrable: []string{"a3"}}}}

	s := scopeOf(t, g, "m0")
	assert.Equal(t, []string{"a1", "a2"}, s.Members)
	assert.True(t, s.MembersImplicit)

	g.Gates = map[string]model.EpicGate{"m0": {Members: []string{"a2"}}}
	s = scopeOf(t, g, "m0")
	assert.Equal(t, []string{"a2"}, s.Members)
	assert.False(t, s.MembersImplicit)

	g.Gates = nil
	assert.True(t, scopeOf(t, g, "m0").MembersImplicit, "a missing gate entry is implicit too")
}

// TestScope_ClosureCutAtDoneEpic: the closure walks not-done dependencies
// from the not-done members and stops at a done epic, so what only the done
// epic needed stays out, and a done member brings in nothing.
func TestScope_ClosureCutAtDoneEpic(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("a2")...),
		scopeEpic("a2", "m0", scopeDeps("a3")...),
		scopeEpic("a3", "m0", scopeDeps("a4")...),
		scopeEpic("a4", "m0"),
		scopeEpic("b1", "m0", scopeDeps("b2")...),
		scopeEpic("b2", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1", "b1"}}}}
	g.Epics[2].Done = scopeDone()
	g.Epics[4].Done = scopeDone()

	s := scopeOf(t, g, "m0")

	assert.Equal(t, []string{"a1", "a2"}, closureIDs(s))
	require.Len(t, s.Closure, 1)
	assert.Equal(t, 2, s.Closure[0].Size)
}
