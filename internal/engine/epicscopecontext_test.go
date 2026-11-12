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
		{ID: "b1", Title: "Entry", Milestone: "m1", Acceptance: "entry ships", DependsOn: []model.EpicDep{
			{ID: "a0", Why: "b1 reads a0's schema"}, {ID: "d1", Why: "b1 needs d1's flag"}, {AnyOf: []string{"x", "y"}, Why: "b1 needs one store"}}},
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
	assert.Contains(t, text, "- b1 [member] Entry\n  acceptance: entry ships\n  depends_on: a0: b1 reads a0's schema; d1: b1 needs d1's flag; one of {x, y}: b1 needs one store\n")
	assert.Contains(t, text, "- d1 [deferrable] Flag\n")
	assert.Contains(t, text, "- x [any_of: x, y] Store X\n")
	assert.Contains(t, text, "- c1 [outside, condition, no_dependents] Probe\n")
	assert.Contains(t, text, "deferrable_in_closure: d1\n")
	assert.Contains(t, text, "holds_opening: m0: a9\n")
	assert.Contains(t, text, "critical_path (by count): a0 -> b1\n")
	assert.Contains(t, text, "\n* b1 [member] size 5: a0, b1, d1, x, y\n")
	assert.NotContains(t, text, "Spare", "a9 is outside m1's closure and milestone; it appears only by id")
}
