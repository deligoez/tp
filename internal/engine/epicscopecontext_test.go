package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// TestEpicScopeContext_EachEpicOnceWithItsMarks: the scope context shows every
// closure and outside epic once, by its brief form and its marks, then the
// shrink rows, deferrable_in_closure, holds_opening and the critical path by
// count.
func TestEpicScopeContext_EachEpicOnceWithItsMarks(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1"}}, Epics: []model.Epic{
		{ID: "a0", Title: "Base", Milestone: "m0", Acceptance: "base ships"},
		{ID: "a9", Title: "Spare", Milestone: "m0"},
		{ID: "z0", Title: "Shipped", Milestone: "m0", Done: scopeDone()},
		{ID: "b1", Title: "Entry", Milestone: "m1", Acceptance: "entry ships", DependsOn: []model.EpicDep{
			{ID: "a0", Why: "b1 reads a0's schema"}, {ID: "z0", Why: "b1 reuses z0's client"}, {ID: "d1", Why: "b1 needs d1's flag"},
			{AnyOf: []string{"x", "y"}, Why: "b1 needs one store"}}},
		{ID: "d1", Title: "Flag", Milestone: "m1"},
		{ID: "x", Title: "Store X", Milestone: "m1"},
		{ID: "y", Title: "Store Y", Milestone: "m1"},
		{ID: "c1", Title: "Probe", Milestone: "m1", Condition: true},
	}, Gates: map[string]model.EpicGate{
		"m0": {Members: []string{"a0", "a9"}},
		"m1": {Members: []string{"b1"}, Deferrable: []string{"d1"}, Threshold: "ten users", Purpose: "prove the entry path"},
	}}
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)

	text := EpicScopeContext(g, states, "m1")
	for _, id := range []string{"a0", "b1", "d1", "x", "y", "c1"} {
		assert.Equal(t, 1, strings.Count(text, "\n- "+id+" "), "%s appears once", id)
	}
	assert.Contains(t, text, "\nmembers: explicit, 1\n", "the members by mode and count; each row carries its mark")
	assert.Contains(t, text, "- b1 [member] Entry\n  acceptance: entry ships\n  depends_on: a0, d1, one of {x, y}\n",
		"edges by id only, the done z0 left out, no why")
	assert.NotContains(t, text, "b1 reads a0's schema")
	assert.Contains(t, text, "- d1 [deferrable] Flag\n")
	assert.Contains(t, text, "- x [any_of: x, y] Store X\n")
	assert.Contains(t, text, "- c1 [outside, condition, no_dependents] Probe\n")
	assert.Contains(t, text, "deferrable_in_closure: d1\n")
	assert.Contains(t, text, "holds_opening: m0 (explicit gate): a9\n")
	assert.Contains(t, text, "critical_path (by count): a0 -> b1\n")
	assert.Contains(t, text, "\n* b1 [member] size 5: a0, b1, d1, x, y\n")
	assert.NotContains(t, text, "Spare", "a9 is outside m1's closure and milestone; it appears only by id")
	assert.NotContains(t, text, "Shipped", "a done epic is not shown")
	assert.NotContains(t, text, "(none)", "a scope with shrink rows prints no empty shrink")
}

// TestEpicScopeContext_ShowsOnlyMilestonesWithClosureEpics: a milestone with
// no closure epic prints no section, a tracked one names its track, an epic
// with no mark and an edge prints both bare, the last epic prints no empty
// edge line, a shrink row with no mark is bare, and a milestone with nothing
// outside its closure prints no outside section.
func TestEpicScopeContext_ShowsOnlyMilestonesWithClosureEpics(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0", "m1"}}, Epics: []model.Epic{
		{ID: "z", Title: "Z", Milestone: "m0"},
		{ID: "a", Title: "A", Milestone: "m1", DependsOn: scopeDeps("b")},
		{ID: "b", Title: "B", Milestone: "m1", DependsOn: scopeDeps("c")},
		{ID: "c", Title: "C", Milestone: "m1"},
	}, Gates: map[string]model.EpicGate{"m1": {Members: []string{"a"}, Threshold: "t"}}}
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)

	require.Len(t, MilestoneScope(g, states, nil, "m1").Closure, 1)
	text := EpicScopeContext(g, states, "m1")
	assert.NotContains(t, text, "--- m0", "m0 holds no closure epic")
	assert.Contains(t, text, "--- m1 (track 0), 3 in the closure\n")
	assert.Contains(t, text, "- b B\n  depends_on: c\n")
	assert.Contains(t, text, "- c C\nshrink", "c has no edge, so no depends_on line")
	assert.Contains(t, text, "* b size 2: b, c\n")
	assert.NotContains(t, text, "outside the closure")
}

// TestEpicScopeContext_TheSmallestScopeInFull: one member epic with no edge
// and nothing to shrink, defer, hold or chain renders exactly so.
func TestEpicScopeContext_TheSmallestScopeInFull(t *testing.T) {
	g := &model.EpicGraph{Version: 1, Milestones: [][]string{{"m0"}}, Epics: []model.Epic{
		{ID: "a", Title: "A", Milestone: "m0"},
	}, Gates: map[string]model.EpicGate{"m0": {Members: []string{"a"}, Threshold: "t"}}}
	states, err := DeriveEpicStates(g, nil)
	require.NoError(t, err)

	assert.Equal(t, "\nScope of gate m0 — the live tp epic scope measurement behind the asked scope unit; its epics are context, not units:\n"+
		"members: explicit, 1\n"+
		"closure, then the milestone's epics outside it, each epic once:\n"+
		"--- m0 (track 0), 1 in the closure\n"+
		"- a [member] A\n"+
		"shrink — the epics that leave the closure when the epic is cut, or dropped from the gate when a member, largest first:\n"+
		"(none)\n"+
		"deferrable_in_closure: none\n"+
		"holds_opening: none\n"+
		"critical_path (by count): a\n", EpicScopeContext(g, states, "m0"))
}
