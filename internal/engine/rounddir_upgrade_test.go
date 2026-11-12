package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestARoundInFlightAcrossTheUpgradeIsRespawned pins what the driver does
// with a round collected under the pre-1.12.0 round-directory layout
// (spec/backlog/ground-scratch-in-state-dir.md): a nested task file's role
// files under the old .tp/rounds/<base>/ are not read, so the oracle spawns
// every role unit of that round again into the new
// .tp/rounds/<dir>/<base>/, and the old files are left where they are for
// scripts/clean-emissions.sh to report. Nothing recorded is lost, since a
// round's findings only count once its record unit has run. A task file at
// the root keeps its old directory, so its collected roles still count.
func TestARoundInFlightAcrossTheUpgradeIsRespawned(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, ".git"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "x"), 0o755))
	spec := filepath.Join(root, "x", "a.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n"), 0o600))
	taskFile := filepath.Join(root, "x", "a.tasks.json")
	require.NoError(t, os.WriteFile(taskFile, []byte(`{"spec":"a.md","tasks":[]}`), 0o600))

	panel, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, nil, nil, nil)
	require.NotEmpty(t, panel)
	done := panel[0].ID

	// The role the old driver finished, under the old layout.
	old := filepath.Join(root, ".tp", "rounds", "a", "review-r1", "role-"+done+".ndjson")
	require.NoError(t, os.MkdirAll(filepath.Dir(old), 0o755))
	require.NoError(t, os.WriteFile(old, []byte(`{"id":"f"}`+"\n"), 0o600))

	units, _ := BuildNextUnits(root, taskFile, spec, PhaseReview, nil, nil, nil)
	assert.Equal(t, unitIDs(panel), unitIDs(units), "the new driver respawns every role of the round, the finished one included")
	assert.FileExists(t, old, "and leaves the old file for clean-emissions to report")

	// A root task file's directory did not move.
	top := filepath.Join(root, "x.tasks.json")
	topSpec := filepath.Join(root, "x.md")
	require.NoError(t, os.WriteFile(topSpec, []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(top, []byte(`{"spec":"x.md","tasks":[]}`), 0o600))
	assert.Equal(t, filepath.Join(root, ".tp", "rounds", "x", "review-r1"), RoundDir(root, top, PhaseReview, 1))
}
