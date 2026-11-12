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

// scopeTrack is a closure milestone's track index.
func scopeTrack(n int) *int {
	return &n
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
		{Milestone: "m0", Track: scopeTrack(0), Size: 1, Epics: []ScopeEpic{{ID: "a0"}}},
		{Milestone: "m1", Track: scopeTrack(0), Size: 1, Epics: []ScopeEpic{{ID: "a1"}}},
		{Milestone: "n0", Track: scopeTrack(1), CrossTrack: true, Size: 1, Epics: []ScopeEpic{{ID: "b0"}}},
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
// epics the closure misses, each deferrable or condition one marked, and
// no_dependents the part of it no epic depends on, by plain edge or as an
// any-of member. Implicit members leave the deferrable and condition epics
// out, so outside is computed the same way under them and is not empty.
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
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1"}, Deferrable: []string{"a3"}}}}
	g.Epics[5].Done = scopeDone()

	s := scopeOf(t, g, "m0")
	assert.Equal(t, []ScopeOutside{{ID: "a3", Deferrable: true}, {ID: "a4"}, {ID: "a5"}, {ID: "c1", Condition: true}}, s.Outside)
	assert.Equal(t, []string{"a3", "a5"}, s.NoDependents)

	g.Gates = map[string]model.EpicGate{"m0": {Deferrable: []string{"a3"}}}
	s = scopeOf(t, g, "m0")
	assert.Equal(t, []ScopeOutside{{ID: "a3", Deferrable: true}, {ID: "c1", Condition: true}}, s.Outside)
	assert.Equal(t, []string{"a3"}, s.NoDependents)
}

// TestScope_DeferrableInClosure: a deferrable epic the members require is in
// the closure, so it cannot actually be deferred, and is listed as such; one
// only an any-of alternative needs is optional and not listed, and one no
// member reaches stays outside. A gate with no deferrable epics lists none,
// as [] rather than null.
func TestScope_DeferrableInClosure(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("d1")[0], scopeAnyOf("d3", "a2")),
		scopeEpic("a2", "m0"),
		scopeEpic("d1", "m0"),
		scopeEpic("d2", "m0"),
		scopeEpic("d3", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1"}, Deferrable: []string{"d1", "d2", "d3"}}}}

	s := scopeOf(t, g, "m0")

	assert.Equal(t, []string{"d1"}, s.DeferrableInClosure)
	assert.Equal(t, []ScopeOutside{{ID: "d2", Deferrable: true}}, s.Outside)

	g.Gates = map[string]model.EpicGate{"m0": {Members: []string{"a1"}}}
	assert.Equal(t, []string{}, scopeOf(t, g, "m0").DeferrableInClosure)
}

// TestScope_UnneededEarlier: for each milestone before the scope's in its
// track, the not-done members of its gate no other gate's closure contains,
// naming the other gates consulted that were implicit; a milestone with none
// prints no row.
func TestScope_UnneededEarlier(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1", "m2"}, {"n0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0"),
		scopeEpic("a2", "m0"),
		scopeEpic("a3", "m0"),
		scopeEpic("b1", "m1", scopeDeps("a1")...),
		scopeEpic("c1", "m2", scopeDeps("b1")...),
		scopeEpic("d1", "n0"),
	}, Gates: map[string]model.EpicGate{
		"m0": {Members: []string{"a1", "a2", "a3"}},
		"m1": {Members: []string{"b1"}},
	}}
	g.Epics[2].Done = scopeDone()

	s := scopeOf(t, g, "m2")

	assert.Equal(t, []ScopeUnneeded{
		{Milestone: "m0", Epics: []string{"a2"}, ImplicitGates: []string{"m2", "n0"}},
	}, s.UnneededEarlier, "m1's b1 is needed by m2, so m1 prints no row")
}

// TestScope_UnneededEarlierCountsOtherTracks: an earlier gate's member whose
// only consumer is a gate of another track is needed, so it is not reported.
func TestScope_UnneededEarlierCountsOtherTracks(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1"}, {"n0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0"),
		scopeEpic("b1", "m1"),
		scopeEpic("d1", "n0", scopeDeps("a1")...),
	}, Gates: map[string]model.EpicGate{
		"m0": {Members: []string{"a1"}},
		"m1": {Members: []string{"b1"}},
		"n0": {Members: []string{"d1"}},
	}}

	assert.Equal(t, []ScopeUnneeded{}, scopeOf(t, g, "m1").UnneededEarlier)

	g.Epics[2].DependsOn = nil
	assert.Equal(t, []ScopeUnneeded{{Milestone: "m0", Epics: []string{"a1"}, ImplicitGates: []string{}}},
		scopeOf(t, g, "m1").UnneededEarlier, "without n0's need a1 is reported")
}

// TestScope_Fingerprint: the fingerprint hashes a status-blind closure, so
// closing an epic does not move it; it is blind to the members' order, moves
// with a rewritten threshold, and moves with the graph's edges.
func TestScope_Fingerprint(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("a2")...),
		scopeEpic("a2", "m0", scopeDeps("a3")...),
		scopeEpic("a3", "m0"),
		scopeEpic("a4", "m0"),
		scopeEpic("a5", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1", "a4"}, Threshold: "ten users"}}}

	first := scopeOf(t, g, "m0").Fingerprint
	assert.Len(t, first, 64)
	assert.Equal(t, first, scopeOf(t, g, "m0").Fingerprint, "stable on a rerun")

	g.Epics[1].Done = scopeDone()
	assert.Equal(t, first, scopeOf(t, g, "m0").Fingerprint, "closing an epic is no change")

	g.Gates["m0"] = model.EpicGate{Members: []string{"a4", "a1"}, Threshold: "ten users"}
	assert.Equal(t, first, scopeOf(t, g, "m0").Fingerprint, "reordering the members is no change")

	g.Gates["m0"] = model.EpicGate{Members: []string{"a4", "a1"}, Threshold: "twenty users"}
	rewritten := scopeOf(t, g, "m0").Fingerprint
	assert.NotEqual(t, first, rewritten, "a rewritten threshold is a change")

	g.Epics[3].DependsOn = scopeDeps("a5")
	assert.NotEqual(t, rewritten, scopeOf(t, g, "m0").Fingerprint, "a new edge in the closure is a change")
}

// TestScope_ShrinkMarksAnyOfAlternative: the row of an epic an any-of group
// alone brought in carries the group, since cutting it is choosing the
// other alternative; size counts the epic itself.
func TestScope_ShrinkMarksAnyOfAlternative(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeAnyOf("x", "y")),
		scopeEpic("x", "m0", scopeDeps("x1")...),
		scopeEpic("x1", "m0"),
		scopeEpic("y", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1"}}}}

	assert.Equal(t, []ScopeShrink{
		{Epic: "a1", Member: true, Size: 4, Leaves: []string{"a1", "x", "x1", "y"}},
		{Epic: "x", AnyOf: []string{"x", "y"}, Size: 2, Leaves: []string{"x", "x1"}},
	}, scopeOf(t, g, "m0").Shrink)
}

// TestScope_ShrinkMemberAnotherMemberNeeds: dropping a member another member
// depends on leaves the closure as it was, so it prints no row, whatever its
// own dependencies weigh.
func TestScope_ShrinkMemberAnotherMemberNeeds(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("b1")...),
		scopeEpic("b1", "m0", scopeDeps("b2")...),
		scopeEpic("b2", "m0", scopeDeps("b3")...),
		scopeEpic("b3", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1", "b1"}}}}

	assert.Equal(t, []ScopeShrink{
		{Epic: "b2", Size: 2, Leaves: []string{"b2", "b3"}},
	}, scopeOf(t, g, "m0").Shrink)
}

// TestScope_ControlEpicLeavesWithItsChain is a synthetic scenario: a
// condition hangs from a chain a member reaches, so the shrink row of the
// chain's head carries it; once the edge into that chain is removed, the
// condition leaves the closure with the chain and shows outside, marked.
func TestScope_ControlEpicLeavesWithItsChain(t *testing.T) {
	cond := scopeEpic("c1", "m0")
	cond.Condition = true
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeDeps("x", "y")...),
		scopeEpic("x", "m0", scopeDeps("c1")...),
		scopeEpic("y", "m0"),
		cond,
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1"}}}}

	before := scopeOf(t, g, "m0")
	assert.Contains(t, closureIDs(&before), "c1")
	assert.Contains(t, before.Shrink, ScopeShrink{Epic: "x", Size: 2, Leaves: []string{"c1", "x"}})

	g.Epics[0].DependsOn = scopeDeps("y")
	after := scopeOf(t, g, "m0")
	assert.Equal(t, []string{"a1", "y"}, closureIDs(&after))
	assert.Equal(t, []ScopeOutside{{ID: "x"}, {ID: "c1", Condition: true}}, after.Outside)
}

// TestScope_LaterGateChangeFreesEarlierMembers is a synthetic scenario: a
// later gate drops the member that was the only consumer of an earlier
// gate's member, which then shows in unneeded_earlier.
func TestScope_LaterGateChangeFreesEarlierMembers(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0"),
		scopeEpic("a2", "m0"),
		scopeEpic("b1", "m1", scopeDeps("a1")...),
		scopeEpic("b2", "m1", scopeDeps("a2")...),
	}, Gates: map[string]model.EpicGate{
		"m0": {Members: []string{"a1", "a2"}},
		"m1": {Members: []string{"b1", "b2"}},
	}}
	assert.Equal(t, []ScopeUnneeded{}, scopeOf(t, g, "m1").UnneededEarlier)

	g.Gates["m1"] = model.EpicGate{Members: []string{"b1"}}
	assert.Equal(t, []ScopeUnneeded{{Milestone: "m0", Epics: []string{"a2"}, ImplicitGates: []string{}}},
		scopeOf(t, g, "m1").UnneededEarlier)
}

// TestScope_OptionalEpicInheritsItsAlternativesMark: an epic that only an
// any-of alternative needs, through a plain edge, is optional too and carries
// the alternative's group; what the members need through plain edges alone
// carries none.
func TestScope_OptionalEpicInheritsItsAlternativesMark(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}, {"n0"}}, Epics: []model.Epic{
		scopeEpic("b1", "m0", scopeAnyOf("b3", "c0"), scopeDeps("b2")[0]),
		scopeEpic("b2", "m0"),
		scopeEpic("b3", "m0", scopeDeps("x")...),
		scopeEpic("x", "m0"),
		scopeEpic("c0", "n0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"b1"}}}}

	s := scopeOf(t, g, "m0")

	group := []string{"b3", "c0"}
	assert.Equal(t, []ScopeMilestone{
		{Milestone: "m0", Track: scopeTrack(0), Size: 4, Epics: []ScopeEpic{{ID: "b1"}, {ID: "b2"}, {ID: "b3", AnyOf: group}, {ID: "x", AnyOf: group}}},
		{Milestone: "n0", Track: scopeTrack(1), CrossTrack: true, Size: 1, Epics: []ScopeEpic{{ID: "c0", AnyOf: group}}},
	}, s.Closure)
}

// TestScope_MootEpicsAreNotOutside: the lost alternative of a satisfied
// group, and what only it needed, are moot (MootEpics): no work waits on
// them, so outside leaves them out.
func TestScope_MootEpicsAreNotOutside(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeAnyOf("x", "y")),
		scopeEpic("x", "m0"),
		scopeEpic("y", "m0", scopeDeps("y1")...),
		scopeEpic("y1", "m0"),
		scopeEpic("z", "m0"),
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a1"}}}}
	g.Epics[1].Done = scopeDone()

	assert.Equal(t, []ScopeOutside{{ID: "z"}}, scopeOf(t, g, "m0").Outside)
}

// TestScope_ImplicitGateHoldsALostAlternative settles whether a lost
// alternative is moot under implicit members: it is not. An implicit gate
// holds every work epic of its milestone, the lost alternative y and its
// dependency y1 included, so MootEpics does not mark them moot and the gate
// stays red until they are done (GateGreen). The scope reports the gate as
// it is: both are members, in the closure.
func TestScope_ImplicitGateHoldsALostAlternative(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		scopeEpic("a1", "m0", scopeAnyOf("x", "y")),
		scopeEpic("x", "m0"),
		scopeEpic("y", "m0", scopeDeps("y1")...),
		scopeEpic("y1", "m0"),
	}}
	g.Epics[1].Done = scopeDone()
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)
	require.Empty(t, MootEpics(g, states), "an implicit gate holds y and y1, so neither is moot")

	s := scopeOf(t, g, "m0")

	assert.Equal(t, []string{"a1", "x", "y", "y1"}, s.Members)
	assert.Equal(t, []string{"a1", "y", "y1"}, closureIDs(&s))
	assert.False(t, GateGreen(g, "m0", states), "the gate waits on y and y1 too")
}
