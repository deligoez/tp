package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestReadCycleRunning_OnlyAMissingTaskFileReadsAsEmpty: a cycle before
// decomposition has no task file and reads as an empty task set, as tp resume
// treats it; a task file that is there but does not parse is an error, not
// zero tasks fed to the oracle on every pool refill.
func TestReadCycleRunning_OnlyAMissingTaskFileReadsAsEmpty(t *testing.T) {
	root, spec, taskFile := setupResumeProject(t, oneDoneTask)
	o := &DriverOptions{Root: root, TaskFile: taskFile, Spec: spec}

	require.NoError(t, os.WriteFile(taskFile, []byte("{not json"), 0o600))
	_, err := readCycleRunning(o, nil)
	require.Error(t, err, "a corrupt task file must fail the read")
	assert.Contains(t, err.Error(), "read the task file")

	require.NoError(t, os.Remove(taskFile))
	_, err = readCycleRunning(o, nil)
	require.NoError(t, err, "a missing task file is the pre-decompose cycle")
}

// TestEndSlot_ReportsAKeptWorktreeLeftOnDisk: when the worktree of an
// unfinished unit cannot be removed, endSlot says so instead of dropping the
// error, and the slot is still released.
func TestEndSlot_ReportsAKeptWorktreeLeftOnDisk(t *testing.T) {
	root, _, taskFile, _ := batchProject(t, wipAlpha)
	// A plain directory is not a registered worktree, so the remove fails and
	// the directory stays.
	tree := filepath.Join(t.TempDir(), "not-a-worktree")
	require.NoError(t, os.MkdirAll(tree, 0o750))
	rec, err := NewRunRecorder(root, taskFile, "r1", "implement")
	require.NoError(t, err)
	d := &driver{opts: &DriverOptions{Root: root, TaskFile: taskFile}, runID: "r1", rec: rec, trees: map[string]unitTree{}}
	u := unitTree{id: "alpha", worktree: tree, branch: "tp-unit/c/r1/alpha"}
	p := &pool{running: map[string]*poolSlot{"alpha": {tree: u}}}

	stderr := captureStderr(t, func() { d.endSlot(p, "alpha") })

	assert.Contains(t, stderr, "still on disk: "+tree)
	assert.NotContains(t, p.running, "alpha", "the slot is released")
	assert.NoError(t, p.err)
}
