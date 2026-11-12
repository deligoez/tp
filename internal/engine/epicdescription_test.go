package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// epicDescribedGraph is a -> b, a -> one of {c, d}, every field a block
// renders set on a, and no description anywhere.
func epicDescribedGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}},
		Epics: []model.Epic{
			{ID: "a", Title: "A step", Milestone: "A", Tags: []string{"x", "y"}, Acceptance: "a ships\nsecond line",
				DependsOn: []model.EpicDep{{ID: "b", Why: "a reads what b writes"}, {AnyOf: []string{"c", "d"}, Why: "either runs a"}}},
			{ID: "b", Title: "B step", Milestone: "A", Acceptance: "b ships", DependsOn: []model.EpicDep{{ID: "c", Why: "b extends c"}}},
			{ID: "c", Title: "C step", Milestone: "A", Root: true, Condition: true, Acceptance: "c holds"},
			{ID: "d", Title: "D step", Milestone: "A", Root: true},
		},
	}
}

// epicUnitOf returns the unit key names in g's milestone-A slice.
func epicUnitOf(t *testing.T, g *model.EpicGraph, key string) EpicReviewUnit {
	t.Helper()
	s, err := EpicReviewSlice(g, EpicSliceKey{Milestone: "A"})
	require.NoError(t, err)
	for _, u := range s.Units {
		if u.Key == key {
			return u
		}
	}
	require.Failf(t, "unit not found", "the slice holds no unit %q", key)
	return EpicReviewUnit{}
}

// TestEpicUnit_NoDescriptionKeepsTheV1100BlockAndHash pins an undescribed
// epic's block and unit hash to the bytes v1.10.0 rendered (spec/backlog/
// epic-description.md §1): a description that is absent, empty or only
// whitespace moves neither, so no hash moves on upgrade.
func TestEpicUnit_NoDescriptionKeepsTheV1100BlockAndHash(t *testing.T) {
	const block = "epic: a\ntitle: A step\nmilestone: A\ntags: x, y\nacceptance: a ships\nsecond line\ndepends_on:\n- b: a reads what b writes\n- one of {c, d}: either runs a\n"
	const hash = "f3f7baed6f98580ca87e743155f68a2d59a2e7eabe1963d59b6281e2cbb147dd"
	for _, desc := range []string{"", " \n\t"} {
		g := epicDescribedGraph()
		g.Epics[0].Description = desc
		u := epicUnitOf(t, g, "epic:a")
		assert.Equal(t, block, u.Block, "description %q", desc)
		assert.Equal(t, hash, u.Hash, "description %q", desc)
	}
}

// epicDescriptionText is a description holding lines a block parser could
// mistake for a field or an edge.
const epicDescriptionText = "## Purpose\n\ntitle: not a field\n- c: not an edge\n\nOpen: which runner?"

// TestEpicUnit_ADescriptionIsInItsOwnBlockAndHashOnly is the own-block rule:
// b's description is in b's block and hash, every line indented under one
// description: line after the edges, so editing it re-asks b; a, which
// depends on b, keeps its hash, and b as a's context shows no description.
func TestEpicUnit_ADescriptionIsInItsOwnBlockAndHashOnly(t *testing.T) {
	plain := epicDescribedGraph()
	described := epicDescribedGraph()
	described.Epics[1].Description = epicDescriptionText + "\n"

	b := epicUnitOf(t, described, "epic:b")
	assert.Equal(t, "epic: b\ntitle: B step\nmilestone: A\nacceptance: b ships\ndepends_on:\n- c: b extends c\n"+
		"description:\n  ## Purpose\n\n  title: not a field\n  - c: not an edge\n\n  Open: which runner?\n", b.Block)
	assert.NotEqual(t, epicUnitOf(t, plain, "epic:b").Hash, b.Hash, "editing b's description re-asks b")
	assert.Equal(t, epicUnitOf(t, plain, "epic:a").Hash, epicUnitOf(t, described, "epic:a").Hash,
		"a dependent's hash holds its dependency's block without the description")

	s, err := EpicReviewSlice(described, EpicSliceKey{Epic: "a"})
	require.NoError(t, err)
	require.NotEmpty(t, s.Context)
	for _, c := range s.Context {
		assert.NotContains(t, c.Block, "description", "context epic %s", c.ID)
		assert.NotContains(t, c.Brief, "description", "context epic %s", c.ID)
	}
}

// TestEpicScope_IgnoresDescriptions: the scope fingerprint, the scope:<m>
// unit's block and hash and the scope context a gate-scope role reads do not
// move when every epic gains a description: a description is not scope.
func TestEpicScope_IgnoresDescriptions(t *testing.T) {
	plain := epicDescribedGraph()
	plain.Gates = map[string]model.EpicGate{"A": {Members: []string{"a"}, Threshold: "ten users", Purpose: "prove the chain"}}
	described := epicDescribedGraph()
	described.Gates = plain.Gates
	for i := range described.Epics {
		described.Epics[i].Description = "described " + described.Epics[i].ID
	}

	before, after := epicUnitOf(t, plain, "scope:A"), epicUnitOf(t, described, "scope:A")
	assert.Equal(t, before.Block, after.Block)
	assert.Equal(t, before.Hash, after.Hash)
	assert.Equal(t, MilestoneScope(plain, nil, nil, "A").Fingerprint, MilestoneScope(described, nil, nil, "A").Fingerprint)
	ctx := EpicScopeContext(described, nil, "A")
	assert.Equal(t, EpicScopeContext(plain, nil, "A"), ctx)
	assert.NotContains(t, ctx, "described")
}

// TestParseEpicBlock_ReadsADescriptionBackAsOneField: a snapshot diff reads
// the description as one field, its lines un-indented, and neither its
// field-like nor its edge-like lines as a field or an edge.
func TestParseEpicBlock_ReadsADescriptionBackAsOneField(t *testing.T) {
	g := epicDescribedGraph()
	g.Epics[1].Description = epicDescriptionText
	p := parseEpicBlock(epicUnitOf(t, g, "epic:b").Block)

	assert.Equal(t, epicDescriptionText, p.fields["description"])
	assert.Equal(t, "B step", p.fields["title"])
	assert.Equal(t, map[string]string{"c": "b extends c"}, p.edges)
	assert.Equal(t, []string{"c"}, p.edgeOrder)
}
