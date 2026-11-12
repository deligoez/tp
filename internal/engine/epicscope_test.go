package engine

import (
	"testing"

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
