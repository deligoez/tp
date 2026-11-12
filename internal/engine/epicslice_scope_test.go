package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// scopeUnitOf returns key's slice's scope:<m> unit and whether it has one.
func scopeUnitOf(t *testing.T, g *model.EpicGraph, key EpicSliceKey, m string) (EpicReviewUnit, bool) {
	t.Helper()
	s, err := EpicReviewSlice(g, key)
	require.NoError(t, err)
	for _, u := range s.Units {
		if u.Key == "scope:"+m {
			return u, true
		}
	}
	return EpicReviewUnit{}, false
}

// TestEpicReviewSlice_ScopeUnitNeedsAThresholdOrAPurpose: a slice that asks
// gate:<m> asks scope:<m> last when the gate has a threshold or a purpose,
// and not when it has neither; an epic slice asks none.
func TestEpicReviewSlice_ScopeUnitNeedsAThresholdOrAPurpose(t *testing.T) {
	g := epicChainGraph()
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}}}
	_, ok := scopeUnitOf(t, g, EpicSliceKey{Milestone: "A"}, "A")
	assert.False(t, ok, "neither a threshold nor a purpose: no scope unit")

	g.Gates["A"] = model.EpicGate{Members: []string{"a"}, Purpose: "  "}
	_, ok = scopeUnitOf(t, g, EpicSliceKey{Milestone: "A"}, "A")
	assert.False(t, ok, "a blank purpose is unset")

	g.Gates["A"] = model.EpicGate{Members: []string{"a"}, Purpose: "prove the chain"}
	_, ok = scopeUnitOf(t, g, EpicSliceKey{Milestone: "A"}, "A")
	assert.True(t, ok, "a purpose alone creates the unit")

	g.Gates["A"] = model.EpicGate{Members: []string{"a"}, Threshold: "ten users"}
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A", "scope:A"}, unitKeys(s.Units))
	assert.Equal(t, "A", s.Units[5].Milestone)

	_, ok = scopeUnitOf(t, g, EpicSliceKey{Epic: "a"}, "A")
	assert.False(t, ok, "an epic slice asks no gate, so no scope unit")
	_, ok = scopeUnitOf(t, g, EpicSliceKey{Epics: []string{"a", "b", "c", "d"}}, "A")
	assert.True(t, ok, "a set slice holding the whole milestone asks its gate and scope")
}

// TestEpicReviewSlice_ScopeUnitIsStatusBlind: the scope unit's hash is the
// scope fingerprint, its block lists the members, deferrables, threshold,
// purpose and blind closure, and closing an epic moves neither.
func TestEpicReviewSlice_ScopeUnitIsStatusBlind(t *testing.T) {
	g := epicChainGraph()
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"b", "a"}, Deferrable: []string{"d"}, Threshold: "ten users", Purpose: "prove the chain"}}
	u, ok := scopeUnitOf(t, g, EpicSliceKey{Milestone: "A"}, "A")
	require.True(t, ok)
	assert.Equal(t, scopeOf(t, g, "A").Fingerprint, u.Hash)
	assert.Equal(t, "scope: A\nmembers: a, b\ndeferrable: d\nthreshold: ten users\npurpose: prove the chain\nclosure: a, b, c, d\n", u.Block)

	g.Epics[2].Done = scopeDone()
	closed, _ := scopeUnitOf(t, g, EpicSliceKey{Milestone: "A"}, "A")
	assert.Equal(t, u.Hash, closed.Hash, "closing an epic is progress, not a scope change")
	assert.Equal(t, u.Block, closed.Block)

	g.Gates["A"] = model.EpicGate{Members: []string{"b", "a"}, Deferrable: []string{"d"}, Threshold: "ten users", Purpose: "prove the whole chain"}
	rewritten, _ := scopeUnitOf(t, g, EpicSliceKey{Milestone: "A"}, "A")
	assert.NotEqual(t, u.Hash, rewritten.Hash, "a rewritten purpose re-asks the scope unit")
	gate := unitHashes(t, g)["gate:A"]
	g.Gates["A"] = model.EpicGate{Members: []string{"b", "a"}, Deferrable: []string{"d"}, Threshold: "ten users", Purpose: "prove the chain"}
	assert.Equal(t, gate, unitHashes(t, g)["gate:A"], "the purpose is no part of the gate unit")
}
