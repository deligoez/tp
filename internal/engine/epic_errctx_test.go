package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEpicEdgeHistory_AnUnparsableSnapshotNamesItsFile: the hint of exit 3
// says to repair "the file the message names", so the message must name the
// snapshot that did not parse.
func TestEpicEdgeHistory_AnUnparsableSnapshotNamesItsFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, epicRoundFile(1)), []byte("{}\n"), 0o600))
	snapshot := filepath.Join(dir, epicSnapshotFile(1))
	require.NoError(t, os.WriteFile(snapshot, []byte("not json"), 0o600))

	_, err := EpicEdgeHistory([]EpicSliceDir{{Name: "s", Path: dir}}, "e1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), snapshot)
}

// TestEpicFileStates_AnUnparsableGroundStateNamesTheBoundFile: a bound file
// whose ground state does not parse is named, as an unreadable task file is.
func TestEpicFileStates_AnUnparsableGroundStateNamesTheBoundFile(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(specPath, []byte(exportSpec), 0o600))
	tasksPath := filepath.Join(dir, "s.tasks.json")
	require.NoError(t, os.WriteFile(tasksPath, []byte(exportTasksJSON), 0o600))
	writeStateFile(t, specPath, "snapshot-ground-round-1.md", exportSpec)
	writeStateFile(t, specPath, "floor-ground-round-1.txt", "u1 §0 0123456789ab #1 24B\n")
	writeStateFile(t, specPath, "ground-round-1.ndjson", "not json\n")

	_, err := EpicFileStates(dir, map[string][]string{"e1": {tasksPath}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), tasksPath)
}

// TestEpicFileStates_AnUnparsableReviewStateNamesTheBoundFile: the same for a
// review state index that does not parse.
func TestEpicFileStates_AnUnparsableReviewStateNamesTheBoundFile(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "s.md")
	require.NoError(t, os.WriteFile(specPath, []byte(exportSpec), 0o600))
	tasksPath := filepath.Join(dir, "s.tasks.json")
	require.NoError(t, os.WriteFile(tasksPath, []byte(exportTasksJSON), 0o600))
	_, err := EnsureReviewState(specPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(reviewStatePath(specPath), []byte("not json"), 0o600))

	_, err = EpicFileStates(dir, map[string][]string{"e1": {tasksPath}})

	require.Error(t, err)
	assert.Contains(t, err.Error(), tasksPath)
}
