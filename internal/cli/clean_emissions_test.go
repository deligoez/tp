package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCleanEmissionsKnowsTheGroundInbox is scripts/clean-emissions.sh's half
// of spec/backlog/ground-scratch-in-state-dir.md: the ground scratch file now
// sits in <state dir>/ground-inbox/, so an old one whose round was never
// recorded is a removable emission artifact, and one whose round the state
// directory above it records is kept, as the snapshot and floor are.
func TestCleanEmissionsKnowsTheGroundInbox(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	initGitRepo(t, dir)
	state := filepath.Join(dir, "spec", ".tp-review", "a")
	inbox := filepath.Join(state, "ground-inbox")
	require.NoError(t, os.MkdirAll(inbox, 0o750))
	old := time.Now().Add(-48 * time.Hour)
	for _, name := range []string{"round-1.ndjson", "round-2.ndjson"} {
		path := filepath.Join(inbox, name)
		require.NoError(t, os.WriteFile(path, []byte("{}\n"), 0o600))
		require.NoError(t, os.Chtimes(path, old, old))
	}
	require.NoError(t, os.WriteFile(filepath.Join(state, "ground-round-1.ndjson"), []byte("{}\n"), 0o600))

	cmd := exec.Command(filepath.Join(repoRootDir(t), "scripts", "clean-emissions.sh"), "--root", filepath.Join("spec", ".tp-review"))
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", out)

	assert.Contains(t, string(out), filepath.Join("ground-inbox", "round-2.ndjson"),
		"an old scratch file of a round never recorded is removable")
	assert.NotContains(t, string(out), filepath.Join("ground-inbox", "round-1.ndjson"),
		"one whose round is recorded is kept")
	assert.Contains(t, string(out), "kept: 1 recorded")
}
