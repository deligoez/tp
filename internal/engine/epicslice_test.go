package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// epicChainGraph is spec/1.3.0.md §9's "chain": a -> b -> c -> d, all in
// milestone A of tracks [[A, B], [L1]], every why distinct.
func epicChainGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A", "B"}, {"L1"}},
		Epics: []model.Epic{
			{ID: "a", Title: "A step", Milestone: "A", DependsOn: []model.EpicDep{{ID: "b", Why: "a reads what b writes"}}},
			{ID: "b", Title: "B step", Milestone: "A", DependsOn: []model.EpicDep{{ID: "c", Why: "b extends c's model"}}},
			{ID: "c", Title: "C step", Milestone: "A", DependsOn: []model.EpicDep{{ID: "d", Why: "c ships on d's runner"}}},
			{ID: "d", Title: "D step", Milestone: "A"},
		},
	}
}

func unitKeys(units []EpicReviewUnit) []string {
	keys := make([]string, 0, len(units))
	for _, u := range units {
		keys = append(keys, u.Key)
	}
	return keys
}

func unitHashes(t *testing.T, g *model.EpicGraph) map[string]string {
	t.Helper()
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	hashes := map[string]string{}
	for _, u := range s.Units {
		hashes[u.Key] = u.Hash
	}
	return hashes
}

// §9 row 6 with D11: b is the only asked unit, the context holds a and c, and
// d appears only as an edge target id inside c's block.
func TestEpicReviewSlice_EpicAsksItselfWithNeighboursAsContext(t *testing.T) {
	s, err := EpicReviewSlice(epicChainGraph(), EpicSliceKey{Epic: "b"})
	require.NoError(t, err)

	assert.Equal(t, []string{"epic:b"}, unitKeys(s.Units))
	ctx := make([]string, 0, len(s.Context))
	for _, c := range s.Context {
		ctx = append(ctx, c.ID)
	}
	assert.Equal(t, []string{"a", "c"}, ctx)

	assert.NotContains(t, s.Units[0].Block, "D step", "b's own block holds no block of d")
	assert.NotContains(t, s.Context[0].Block, "D step")
	assert.Contains(t, s.Context[1].Block, "- d: c ships on d's runner", "d is an edge target id in c's block")
	assert.NotContains(t, s.Context[1].Block, "D step", "d's own block never appears")
}

func TestEpicReviewSlice_EditingAWhyChangesTheSourceAndItsDependents(t *testing.T) {
	before := unitHashes(t, epicChainGraph())
	g := epicChainGraph()
	g.Epics[1].DependsOn[0].Why = "b needs c's schema"
	after := unitHashes(t, g)

	assert.NotEqual(t, before["epic:a"], after["epic:a"], "a's hash covers b's block")
	assert.NotEqual(t, before["epic:b"], after["epic:b"])
	assert.Equal(t, before["epic:c"], after["epic:c"])
	assert.Equal(t, before["epic:d"], after["epic:d"])
}

func TestEpicReviewSlice_GateUnitShowsTheGatesEntry(t *testing.T) {
	g := epicChainGraph()
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a", "b"}, Deferrable: []string{"c"}}}
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A"}, unitKeys(s.Units))
	assert.Empty(t, s.Context)
	gate := s.Units[4]
	assert.Contains(t, gate.Block, "members: a, b")
	assert.Contains(t, gate.Block, "deferrable: c")

	g.Gates["A"] = model.EpicGate{Members: []string{"a"}, Deferrable: []string{"c"}}
	s2, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.NotEqual(t, gate.Hash, s2.Units[4].Hash, "the gate hash covers its members")
}

func TestEpicReviewSlice_UnknownEpicOrMilestoneIsAnError(t *testing.T) {
	_, err := EpicReviewSlice(epicChainGraph(), EpicSliceKey{Epic: "zz"})
	require.Error(t, err)
	_, err = EpicReviewSlice(epicChainGraph(), EpicSliceKey{Milestone: "Q"})
	require.Error(t, err)
}

// TestEpicReviewSlice_ABlockCarriesTheAcceptance: dod-satisfiability reads an
// epic's acceptance from its block, so an epic with one shows it and an epic
// without one prints no empty acceptance line.
func TestEpicReviewSlice_ABlockCarriesTheAcceptance(t *testing.T) {
	g := epicChainGraph()
	g.Epics[1].Acceptance = "b passes the ledger checker"
	s, err := EpicReviewSlice(g, EpicSliceKey{Epic: "b"})
	require.NoError(t, err)
	require.Len(t, s.Units, 1)
	assert.Contains(t, s.Units[0].Block, "acceptance: b passes the ledger checker\n")
	for _, c := range s.Context {
		assert.NotContains(t, c.Block, "acceptance:", "%s has no acceptance", c.ID)
	}
}

// TestEpicReviewSlice_BlocksShowConditionAndThreshold: spec/1.5.0.md §8 — an
// epic block shows condition when true, a gate block shows its threshold, and
// neither prints a line when unset.
func TestEpicReviewSlice_BlocksShowConditionAndThreshold(t *testing.T) {
	g := epicChainGraph()
	g.Epics[1].Condition = true
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}, Threshold: "p95 under 200ms"}}
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	require.Len(t, s.Units, 5)

	assert.Contains(t, s.Units[1].Block, "condition: true\n")
	assert.NotContains(t, s.Units[0].Block, "condition:", "a is not a condition")
	assert.Contains(t, s.Units[4].Block, "threshold: p95 under 200ms\n")

	g.Gates["A"] = model.EpicGate{Members: []string{"a"}}
	s, err = EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.NotContains(t, s.Units[4].Block, "threshold:", "a gate without a threshold prints none")
}

// TestEpicReviewSlice_EditingAThresholdChangesTheGateHash: the gate unit's
// hash covers its threshold, so editing it re-asks the unit.
func TestEpicReviewSlice_EditingAThresholdChangesTheGateHash(t *testing.T) {
	g := epicChainGraph()
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}, Threshold: "p95 under 200ms"}}
	before := unitHashes(t, g)
	g.Gates["A"] = model.EpicGate{Members: []string{"a"}, Threshold: "p95 under 100ms"}
	after := unitHashes(t, g)

	assert.NotEqual(t, before["gate:A"], after["gate:A"])
	assert.Equal(t, before["epic:a"], after["epic:a"])
}

// TestEpicReviewSlice_EditingOpensAfterChangesTheGateHash is spec/1.6.0.md
// §1: a gate block shows its milestone's opens_after entry, so editing it
// re-asks the unit; a milestone opens_after does not name shows its default.
func TestEpicReviewSlice_EditingOpensAfterChangesTheGateHash(t *testing.T) {
	g := epicChainGraph()
	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}}}
	before := unitHashes(t, g)
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Contains(t, s.Units[4].Block, "opens_after: []\n", "a track's first milestone opens after none")

	g.OpensAfter = map[string][]string{"A": {"L1"}}
	s, err = EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Contains(t, s.Units[4].Block, "opens_after: L1\n")
	after := unitHashes(t, g)
	assert.NotEqual(t, before["gate:A"], after["gate:A"])
	assert.Equal(t, before["epic:a"], after["epic:a"])

	g.OpensAfter["A"] = []string{}
	s, err = EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Contains(t, s.Units[4].Block, "opens_after: []\n")
}

// TestEpicReviewSlice_EveryMilestoneOfTheSliceHasAGateUnit: every milestone
// has a gate, implicit when gates names none, so a milestone slice asks it
// with its effective members and effective opens_after, and a set slice asks
// the gate of each milestone whose every epic it lists.
func TestEpicReviewSlice_EveryMilestoneOfTheSliceHasAGateUnit(t *testing.T) {
	g := epicChainGraph()
	g.Epics[3].Condition = true
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A"}, unitKeys(s.Units))
	assert.Equal(t, "gate: A\nmembers: a, b, c\nopens_after: []\n", s.Units[4].Block,
		"the implicit gate's members leave the condition out")

	s, err = EpicReviewSlice(g, EpicSliceKey{Milestone: "B"})
	require.NoError(t, err)
	assert.Equal(t, []string{"gate:B"}, unitKeys(s.Units))
	assert.Equal(t, "gate: B\nmembers: \nopens_after: A\n", s.Units[0].Block, "B opens after A by default")

	s, err = EpicReviewSlice(g, EpicSliceKey{Epics: []string{"a", "b", "c", "d"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"epic:a", "epic:b", "epic:c", "epic:d", "gate:A"}, unitKeys(s.Units))

	g.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}}}
	g.OpensAfter = map[string][]string{"A": {"L1"}}
	s, err = EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Equal(t, "gate: A\nmembers: a\nopens_after: L1\n", s.Units[4].Block,
		"a gates entry with members and an opens_after entry keeps its 1.6.0 block")
}

// TestEpicReviewSlice_ASetOfOneAsksItsMilestonesGateAndAnEpicSliceNone: l is
// the only epic of L1. The set {l} lists every epic of L1, so it asks L1's
// gate too; the epic slice of l asks l alone.
func TestEpicReviewSlice_ASetOfOneAsksItsMilestonesGateAndAnEpicSliceNone(t *testing.T) {
	g := epicChainGraph()
	g.Epics = append(g.Epics, model.Epic{ID: "l", Title: "L step", Milestone: "L1"})
	s, err := EpicReviewSlice(g, EpicSliceKey{Epics: []string{"l"}})
	require.NoError(t, err)
	assert.Equal(t, []string{"epic:l", "gate:L1"}, unitKeys(s.Units))

	s, err = EpicReviewSlice(g, EpicSliceKey{Epic: "l"})
	require.NoError(t, err)
	assert.Equal(t, []string{"epic:l"}, unitKeys(s.Units))
}

// TestEpicReviewSlice_ABlockShowsASingleTagAndNoEmptyLists: spec/1.3.0.md §5
// puts an epic's tags in its block, so b's one tag shows; like acceptance, an
// epic without tags prints no tags line, and the root d no depends_on line.
func TestEpicReviewSlice_ABlockShowsASingleTagAndNoEmptyLists(t *testing.T) {
	g := epicChainGraph()
	g.Epics[1].Tags = []string{"ui"}
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	assert.Contains(t, s.Units[1].Block, "tags: ui\n")
	assert.NotContains(t, s.Units[0].Block, "tags:", "a has no tags")
	assert.NotContains(t, s.Units[3].Block, "depends_on:", "d depends on nothing")
}
