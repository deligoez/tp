package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// danglingEpicGraph is spec/1.3.0.md §9 row 2's hand-written file: a with an
// edge to the absent z, and b.
func danglingEpicGraph() *model.EpicGraph {
	return &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}},
		Epics: []model.Epic{
			{ID: "a", Title: "A", Milestone: "A", DependsOn: []model.EpicDep{{ID: "z", Why: "z first"}}},
			{ID: "b", Title: "B", Milestone: "A"},
		},
	}
}

func TestNewEpicErrors_PreExistingDanglingEdgeIsNotNewButASecondOneIs(t *testing.T) {
	before := LintEpicGraph(danglingEpicGraph(), nil)
	require.Len(t, before, 1)

	addC := danglingEpicGraph()
	addC.Epics = append(addC.Epics, model.Epic{ID: "c", Title: "C", Milestone: "A",
		DependsOn: []model.EpicDep{{ID: "b", Why: "b first"}}})
	assert.Empty(t, NewEpicErrors(before, LintEpicGraph(addC, nil)), "adding c -> b adds no error")

	addE := danglingEpicGraph()
	addE.Epics = append(addE.Epics, model.Epic{ID: "e", Title: "E", Milestone: "A",
		DependsOn: []model.EpicDep{{ID: "y", Why: "y first"}}})
	added := NewEpicErrors(before, LintEpicGraph(addE, nil))
	require.Len(t, added, 1, "a second dangling edge is new although its rule is not")
	assert.Equal(t, EpicRuleDanglingReference, added[0].Rule)
	assert.Equal(t, []string{"e", "y"}, added[0].Epics)
}

func TestNewEpicErrors_SecondUnknownTopLevelKeyIsNew(t *testing.T) {
	before := LintEpicGraph(cleanEpicGraph(), []EpicGraphUnknownKey{{Path: "owner", Key: "owner"}})
	after := LintEpicGraph(cleanEpicGraph(), []EpicGraphUnknownKey{
		{Path: "owner", Key: "owner"},
		{Path: "extra", Key: "extra"},
	})
	added := NewEpicErrors(before, after)
	require.Len(t, added, 1, "both keys name no epic; the key name tells them apart")
	assert.Equal(t, EpicRuleUnknownKey, added[0].Rule)
	assert.Contains(t, added[0].Message, "extra")
}

func TestNewEpicErrors_SecondDuplicateMilestoneNameIsNew(t *testing.T) {
	g := cleanEpicGraph()
	g.Milestones = [][]string{{"A", "B"}, {"L1", "B"}}
	before := LintEpicGraph(g, nil)
	g.Milestones = [][]string{{"A", "B", "L1"}, {"L1", "B"}}
	added := NewEpicErrors(before, LintEpicGraph(g, nil))
	require.Len(t, added, 1, "both duplicates name no epic; the milestone name tells them apart")
	assert.Equal(t, EpicRuleDuplicateName, added[0].Rule)
	assert.Contains(t, added[0].Message, `"L1"`)
}

// cycleEpicGraph holds the cycle a <-> b, and b <-> c when wide is set.
func cycleEpicGraph(wide bool) *model.EpicGraph {
	g := &model.EpicGraph{
		Version:    1,
		Milestones: [][]string{{"A"}},
		Epics: []model.Epic{
			{ID: "a", Title: "A", Milestone: "A", DependsOn: []model.EpicDep{{ID: "b", Why: "b first"}}},
			{ID: "b", Title: "B", Milestone: "A", DependsOn: []model.EpicDep{{ID: "a", Why: "a first"}}},
			{ID: "c", Title: "C", Milestone: "A"},
		},
	}
	if wide {
		g.Epics[1].DependsOn = append(g.Epics[1].DependsOn, model.EpicDep{ID: "c", Why: "c first"})
		g.Epics[2].DependsOn = []model.EpicDep{{ID: "b", Why: "b again"}}
	}
	return g
}

