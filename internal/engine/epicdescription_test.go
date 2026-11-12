package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

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

// TestEpicBuiltinRoles_ReadADescriptionAsThePrimaryText is the role rule:
// dod-satisfiability, why-auditor and phase-economist each read a
// description, when the epic has one, as its primary text for their own
// question, keep the acceptance as the gate's contract, report a
// contradiction, and report neither a missing description nor a listed open
// question; gate-scope's text does not change and names no description.
func TestEpicBuiltinRoles_ReadADescriptionAsThePrimaryText(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	roles, err := ResolveEpicRoleCorpus(root)
	require.NoError(t, err)
	own := map[string]string{
		"dod-satisfiability": "whether the acceptance satisfies the purpose and scope it states",
		"why-auditor":        "whether each edge's why agrees with the decisions it records",
		"phase-economist":    "whether the epic's placement fits the purpose it states",
	}
	for i := range roles {
		text := roles[i].Instructions
		question, reads := own[roles[i].ID]
		if !reads {
			assert.NotContains(t, text, "description", roles[i].ID)
			continue
		}
		for _, want := range []string{
			"When an asked epic's block carries a description, read it as the primary text of that epic",
			question,
			"the acceptance stays the contract the gate checks",
			"A contradiction between the description and the acceptance or an edge is a finding",
			"A missing description is not a finding",
			"an open question the description lists is not one to report back",
		} {
			assert.Contains(t, text, want, roles[i].ID)
		}
	}
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

// blockDiff is the snapshot diff of one epic unit whose block went from
// before to after.
func blockDiff(before, after string) EpicGraphDiff {
	return diffEpicSnapshots(
		epicSnapshot{Units: map[string]string{"epic:b": "h1"}, Blocks: map[string]string{"epic:b": before}},
		epicSnapshot{Units: map[string]string{"epic:b": "h2"}, Blocks: map[string]string{"epic:b": after}})
}

// TestEpicDescription_AwkwardTextRoundTripsAsOneField: each description a
// block parser could misread renders, parses back to the text it holds (its
// trailing newline dropped), and reads in --history as exactly one field
// changed, no edge or why touched.
func TestEpicDescription_AwkwardTextRoundTripsAsOneField(t *testing.T) {
	for name, desc := range map[string]string{
		"blank lines inside":  "first\n\n\nlast",
		"a description: line": "intro\ndescription:\nmore",
		"field and edge":      "title: x\n- c: y\nacceptance: z",
		"unit headers":        "epic: x\ngate: m\n---\nscope: m",
		"tabs":                "\tindented\nmid\tdle\t",
		"trailing newline":    "ends with a newline\n",
	} {
		plain := epicDescribedGraph()
		described := epicDescribedGraph()
		described.Epics[1].Description = desc
		block := epicUnitOf(t, described, "epic:b").Block
		want := strings.TrimRight(desc, "\n")
		if name == "tabs" {
			want = strings.TrimRightFunc(desc, unicode.IsSpace)
		}

		p := parseEpicBlock(block)
		assert.Equal(t, want, p.fields["description"], name)
		assert.Equal(t, "b ships", p.fields["acceptance"], name)
		assert.Equal(t, map[string]string{"c": "b extends c"}, p.edges, name)
		d := blockDiff(epicUnitOf(t, plain, "epic:b").Block, block)
		require.Len(t, d.FieldsChanged, 1, name)
		assert.Equal(t, "description", d.FieldsChanged[0].Field, name)
		assert.Equal(t, want, d.FieldsChanged[0].After, name)
		assert.Empty(t, d.EdgesAdded, name)
		assert.Empty(t, d.EdgesRemoved, name)
		assert.Empty(t, d.WhysChanged, name)
	}
}

// TestParseEpicBlock_ADescriptionLineInAnAcceptanceStaysAContinuation: a
// description: line opens a description only as the block's last section,
// so one inside a multi-line acceptance continues it, and --history reads
// the acceptance whole.
func TestParseEpicBlock_ADescriptionLineInAnAcceptanceStaysAContinuation(t *testing.T) {
	plain := epicDescribedGraph()
	edited := epicDescribedGraph()
	edited.Epics[1].Acceptance = "ships\ndescription:\nfoo"
	block := epicUnitOf(t, edited, "epic:b").Block

	p := parseEpicBlock(block)
	assert.Equal(t, "ships\ndescription:\nfoo", p.fields["acceptance"])
	assert.NotContains(t, p.fields, "description")
	d := blockDiff(epicUnitOf(t, plain, "epic:b").Block, block)
	require.Len(t, d.FieldsChanged, 1)
	assert.Equal(t, EpicFieldChange{Unit: "epic:b", Field: "acceptance", Before: "b ships", After: "ships\ndescription:\nfoo"}, d.FieldsChanged[0])
}

// TestWriteEpicGraph_NormalisesCRLFDescriptions: the same text saved with
// CRLF and with LF line ends is one description, so it hashes identically.
func TestWriteEpicGraph_NormalisesCRLFDescriptions(t *testing.T) {
	hashOf := func(desc string) (string, string) {
		dir := t.TempDir()
		g := epicDescribedGraph()
		g.Epics[1].Description = desc
		require.NoError(t, WriteEpicGraph(dir, g))
		loaded, _, err := LoadEpicGraph(dir)
		require.NoError(t, err)
		return loaded.Epics[1].Description, epicUnitOf(t, loaded, "epic:b").Hash
	}
	crlfText, crlf := hashOf("## Purpose\r\nfirst\r\n\r\nsecond\r\n")
	lfText, lf := hashOf("## Purpose\nfirst\n\nsecond\n")
	assert.Equal(t, lf, crlf)
	assert.Equal(t, lfText, crlfText)
	assert.NotContains(t, crlfText, "\r")
}

// TestEpicHandoffContext_ADescriptionDoesNotHideANewlyNamedEpic: the
// hand-off compares the changed title and acceptance with the ones the round
// before read, so a description that already named d does not hide the
// acceptance newly handing d work.
func TestEpicHandoffContext_ADescriptionDoesNotHideANewlyNamedEpic(t *testing.T) {
	before := epicDescribedGraph()
	before.Epics[1].Description = "d will take the export later"
	beforeBlock := epicUnitOf(t, before, "epic:b").Block
	now := epicDescribedGraph()
	now.Epics[1].Description = before.Epics[1].Description
	now.Epics[1].Acceptance = "b ships and hands the export to d"
	carry := EpicCarry{Asked: []EpicReviewUnit{epicUnitOf(t, now, "epic:b")}, Before: map[string]string{"epic:b": beforeBlock}}

	out := EpicHandoffContext(now, &carry, nil)
	ids := make([]string, 0, len(out))
	for _, c := range out {
		ids = append(ids, c.ID)
	}
	assert.Equal(t, []string{"d"}, ids)
}

// describedDiff renders b's description diff from before to now.
func describedDiff(t *testing.T, before, now string) string {
	t.Helper()
	g := epicDescribedGraph()
	g.Epics[1].Description = before
	block := epicUnitOf(t, g, "epic:b").Block
	g.Epics[1].Description = now
	u := epicUnitOf(t, g, "epic:b")
	return EpicDescriptionDiff(block, &u)
}

// TestEpicDescriptionDiff_ShowsHunksWithOneContextLine: a description-only
// change renders as hunks, each change with one unchanged line on either
// side and one … line for each run of unchanged lines left out.
func TestEpicDescriptionDiff_ShowsHunksWithOneContextLine(t *testing.T) {
	assert.Equal(t, "…\n  c\n- d\n+ X\n  e\n…\n  i\n+ added\n",
		describedDiff(t, "a\nb\nc\nd\ne\nf\ng\nh\ni", "a\nb\nc\nX\ne\nf\ng\nh\ni\nadded"))
	assert.Equal(t, "+ first\n+ second\n", describedDiff(t, "", "first\nsecond"), "a first description is all added")
	assert.Equal(t, "  keep\n- old line\n+ new line\n  tail\n", describedDiff(t, "keep\nold line\ntail", "keep\nnew line\ntail"))
}
