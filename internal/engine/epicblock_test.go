package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/deligoez/tp/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// blockChain is §10 row 3's fixture: the chain a→b→c→d with d done, and c
// done too when cDone is set. b is the epic a task file binds.
func blockChain(cDone bool) *model.EpicGraph {
	g := statusChain()
	if !cDone {
		g.Epics[2].Done = nil
	}
	return g
}

// writeBlockGraph writes g as root's .tp/epics.json.
func writeBlockGraph(t *testing.T, root string, g *model.EpicGraph) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp"), 0o750))
	data, err := json.Marshal(g)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", EpicGraphFile), data, 0o600))
}

// TestEpicBlockFor_NamesDepsNotDoneUntilADoneRecord is spec/1.5.0.md §6: a
// file bound to b is blocked by c while c is not done, and c's done record
// clears it. A block that ignored the graph, or read c's own dependency d,
// fails.
func TestEpicBlockFor_NamesDepsNotDoneUntilADoneRecord(t *testing.T) {
	root := boundFilesRepo(t)
	path := filepath.Join(root, "b.tasks.json")
	tf := &model.TaskFile{Epic: "b"}

	writeBlockGraph(t, root, blockChain(false))
	block, err := EpicBlockFor(tf, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Equal(t, []string{"c"}, block.BlockedBy)
	assert.Empty(t, block.Notice)

	g := blockChain(true)
	g.Epics[3].Done = nil // d's state must not matter: dependencies are direct
	g.Epics[2].Done = &model.EpicDone{At: time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC), Evidence: "operator"}
	writeBlockGraph(t, root, g)
	block, err = EpicBlockFor(tf, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Empty(t, block.BlockedBy)
}

// TestEpicBlockFor_AbsentEpicOrNoGraphIsUnblockedAndSaysSo is spec/1.5.0.md
// §6: a file bound to an epic the graph does not hold, or with no graph file,
// behaves as unbound and says so.
func TestEpicBlockFor_AbsentEpicOrNoGraphIsUnblockedAndSaysSo(t *testing.T) {
	root := boundFilesRepo(t)
	path := filepath.Join(root, "z.tasks.json")

	block, err := EpicBlockFor(&model.TaskFile{Epic: "z"}, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Empty(t, block.BlockedBy)
	assert.Contains(t, block.Notice, EpicGraphFile)

	writeBlockGraph(t, root, blockChain(false))
	block, err = EpicBlockFor(&model.TaskFile{Epic: "z"}, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Empty(t, block.BlockedBy)
	assert.Contains(t, block.Notice, `"z"`)
}

// TestEpicBlockFor_UnboundFileReadsNoGraph is spec/1.5.0.md §6: an unbound
// file behaves as at v1.4.2, so not even a malformed graph beside it is read.
func TestEpicBlockFor_UnboundFileReadsNoGraph(t *testing.T) {
	root := boundFilesRepo(t)
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".tp"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, ".tp", EpicGraphFile), []byte("{"), 0o600))

	block, err := EpicBlockFor(&model.TaskFile{}, filepath.Join(root, "u.tasks.json"))
	require.NoError(t, err)
	assert.Nil(t, block)
}

// TestEpicBlockFor_ADependencyWithoutADoneRecordIsDoneOnceItsFilesAre is
// spec/1.5.0.md §6: a, the graph's first epic, is blocked by b, which has no
// done record, until b's bound file converges its audit; b then derives done
// and clears the block. A block that read only done records, or skipped the
// first epic of the graph, fails.
func TestEpicBlockFor_ADependencyWithoutADoneRecordIsDoneOnceItsFilesAre(t *testing.T) {
	root := boundFilesRepo(t)
	writeBlockGraph(t, root, statusChain())
	path := filepath.Join(root, "a.tasks.json")
	tf := &model.TaskFile{Epic: "a"}
	spec := writeStatusFile(t, root, "b", model.StatusDone)

	block, err := EpicBlockFor(tf, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Empty(t, block.Notice)
	assert.Equal(t, []string{"b"}, block.BlockedBy, "b's audit has not converged")

	recordRounds(t, spec, 0, 2, true)
	block, err = EpicBlockFor(tf, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Empty(t, block.BlockedBy, "b derives done from its bound file")
}

// TestEpicBlockFor_ReadsEachDependencyWhereverTheGraphHoldsIt: a dependency
// is read at any position of the graph, the first included, and an id the
// graph does not hold is not done. An undone condition at the head of the
// graph is a blocker and a condition; a done dependency there blocks nothing.
func TestEpicBlockFor_ReadsEachDependencyWhereverTheGraphHoldsIt(t *testing.T) {
	root := boundFilesRepo(t)
	path := filepath.Join(root, "b.tasks.json")
	tf := &model.TaskFile{Epic: "b"}
	dependent := model.Epic{ID: "b", Title: "B", Milestone: "M", DependsOn: []model.EpicDep{{ID: "c", Why: "c first"}, {ID: "ghost", Why: "never written"}}}

	condition := model.Epic{ID: "c", Title: "C", Milestone: "M", Root: true, Condition: true, Acceptance: "the operator agrees"}
	writeBlockGraph(t, root, &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{condition, dependent}})
	block, err := EpicBlockFor(tf, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Equal(t, []string{"c", "ghost"}, block.BlockedBy)
	assert.Equal(t, []string{"c"}, block.Conditions)

	done := model.Epic{ID: "c", Title: "C", Milestone: "M", Root: true, Done: &model.EpicDone{At: time.Date(2026, 1, 6, 9, 0, 0, 0, time.UTC), Evidence: "shipped"}}
	dependent.DependsOn = dependent.DependsOn[:1]
	writeBlockGraph(t, root, &model.EpicGraph{Version: 1, Milestones: [][]string{{"M"}}, Epics: []model.Epic{done, dependent}})
	block, err = EpicBlockFor(tf, path)
	require.NoError(t, err)
	require.NotNil(t, block)
	assert.Empty(t, block.BlockedBy)
}

// TestEpicBlock_OperatorActionOnlyWhenEveryBlockerIsACondition pins
// OperatorAction's contract at its edges: nothing blocked and any work
// blocker give no action; one condition or several give one record each.
func TestEpicBlock_OperatorActionOnlyWhenEveryBlockerIsACondition(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		blockedBy  []string
		conditions []string
		commands   []string
	}{
		{"nothing blocked", []string{}, []string{}, nil},
		{"one work blocker", []string{"w"}, []string{}, nil},
		{"a condition beside work", []string{"c", "w"}, []string{"c"}, nil},
		{"one condition", []string{"c"}, []string{"c"}, []string{"tp epic done c --evidence <evidence>"}},
		{"two conditions", []string{"c1", "c2"}, []string{"c1", "c2"}, []string{
			"tp epic done c1 --evidence <evidence>", "tp epic done c2 --evidence <evidence>",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			b := &EpicBlock{Epic: "e", BlockedBy: tc.blockedBy, Conditions: tc.conditions}
			op := b.OperatorAction()
			if tc.commands == nil {
				assert.Nil(t, op)
				return
			}
			require.NotNil(t, op)
			assert.Equal(t, EscalateEpicDone, op.Decision)
			assert.Equal(t, tc.commands, op.Commands)
		})
	}
}
