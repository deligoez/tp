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

// scopeAnyOf is one any-of group over ids.
func scopeAnyOf(ids ...string) model.EpicDep {
	return model.EpicDep{AnyOf: ids, Why: "needs one"}
}

// scopeDone is a done record.
func scopeDone() *model.EpicDone {
	return &model.EpicDone{At: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC), Evidence: "shipped"}
}

// closureIDs flattens a scope's closure in its order.
func closureIDs(s *EpicScope) []string {
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

	assert.Equal(t, []string{"a1", "a2"}, closureIDs(&s))
	require.Len(t, s.Closure, 1)
	assert.Equal(t, 2, s.Closure[0].Size)
}

// TestScope_AnyOfGroup: an unsatisfied group brings in every not-done member,
// each marked with its group, since choosing one is the operator's call; an
// epic a plain edge also reaches carries no mark; a satisfied group brings in
// nothing.
func TestScope_AnyOfGroup(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeAnyOf("a2", "a3")),
		scopeEpic("a2", "m0"),
		scopeEpic("a3", "m0"),
		scopeEpic("b1", "m0", scopeAnyOf("b2", "b3")),
		scopeEpic("b2", "m0"),
		scopeEpic("b3", "m0"),
		scopeEpic("c1", "m0", scopeAnyOf("c2", "c3"), scopeDeps("c3")[0]),
		scopeEpic("c2", "m0"),
		scopeEpic("c3", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1", "b1", "c1"}}}}
	g.Epics[4].Done = scopeDone()

	s := scopeOf(t, g, "m0")

	require.Len(t, s.Closure, 1)
	assert.Equal(t, []ScopeEpic{
		{ID: "a1"},
		{ID: "a2", AnyOf: []string{"a2", "a3"}},
		{ID: "a3", AnyOf: []string{"a2", "a3"}},
		{ID: "b1"},
		{ID: "c1"},
		{ID: "c2", AnyOf: []string{"c2", "c3"}},
		{ID: "c3"},
	}, s.Closure[0].Epics)
}

// TestScope_CrossTrackMarked: the closure is composed in milestone order,
// each milestone carrying its track, and a milestone of another track than
// the scope's is marked cross_track as information.
func TestScope_CrossTrackMarked(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1"}, {"n0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m1", scopeDeps("b0", "a0")...),
		scopeEpic("a0", "m0"),
		scopeEpic("b0", "n0"),
	}, Gates: map[string]model.EpicGate{"m1": {Members: []string{"a1"}}}}

	s := scopeOf(t, g, "m1")

	assert.Equal(t, []ScopeMilestone{
		{Milestone: "m0", Track: 0, Size: 1, Epics: []ScopeEpic{{ID: "a0"}}},
		{Milestone: "m1", Track: 0, Size: 1, Epics: []ScopeEpic{{ID: "a1"}}},
		{Milestone: "n0", Track: 1, CrossTrack: true, Size: 1, Epics: []ScopeEpic{{ID: "b0"}}},
	}, s.Closure)
}

// TestScope_Shrink: a non-member's row is what leaves the closure when every
// edge into it is cut, a member's what leaves when it is dropped from the
// gate, marked as a member; an epic that takes only itself prints no row,
// and rows sort by size, then id.
func TestScope_Shrink(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("x", "y", "s")...),
		scopeEpic("b1", "m0", scopeDeps("z", "s")...),
		scopeEpic("x", "m0", scopeDeps("x1", "x2")...),
		scopeEpic("x1", "m0"),
		scopeEpic("x2", "m0"),
		scopeEpic("y", "m0"),
		scopeEpic("z", "m0", scopeDeps("z1")...),
		scopeEpic("z1", "m0"),
		scopeEpic("s", "m0", scopeDeps("s1")...),
		scopeEpic("s1", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1", "b1"}}}}

	s := scopeOf(t, g, "m0")

	assert.Equal(t, []ScopeShrink{
		{Epic: "a1", Member: true, Size: 5, Leaves: []string{"a1", "x", "x1", "x2", "y"}},
		{Epic: "b1", Member: true, Size: 3, Leaves: []string{"b1", "z", "z1"}},
		{Epic: "x", Size: 3, Leaves: []string{"x", "x1", "x2"}},
		{Epic: "s", Size: 2, Leaves: []string{"s", "s1"}},
		{Epic: "z", Size: 2, Leaves: []string{"z", "z1"}},
	}, s.Shrink)
}

// TestScope_OutsideAndNoDependents: outside is the milestone's not-done
// epics the closure misses, and no_dependents the part of it no epic depends
// on, by plain edge or as an any-of member. Under implicit members both are
// empty by definition, and outside_implicit says why.
func TestScope_OutsideAndNoDependents(t *testing.T) {
	cond := scopeEpic("c1", "m0")
	cond.Condition = true
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("a2")...),
		scopeEpic("a2", "m0"),
		scopeEpic("a3", "m0"),
		scopeEpic("a4", "m0"),
		scopeEpic("a5", "m0", scopeDeps("a4")...),
		scopeEpic("a6", "m0"),
		cond,
		scopeEpic("b0", "m1", scopeAnyOf("c1", "a2")),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1"}}}}
	g.Epics[5].Done = scopeDone()

	s := scopeOf(t, g, "m0")
	assert.Equal(t, []string{"a3", "a4", "a5", "c1"}, s.Outside)
	assert.False(t, s.OutsideImplicit)
	assert.Equal(t, []string{"a3", "a5"}, s.NoDependents)

	g.Gates = nil
	s = scopeOf(t, g, "m0")
	assert.Equal(t, []string{}, s.Outside)
	assert.True(t, s.OutsideImplicit)
	assert.Equal(t, []string{}, s.NoDependents)
}
