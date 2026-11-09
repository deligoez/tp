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

