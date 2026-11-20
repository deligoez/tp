package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Repairs from the v1.29.0 review round (spec/backlog/filled-pool.md).

func TestHeld_DoneDoesNotCountAHeldTaskAsReady(t *testing.T) {
	t.Parallel()
	dir := heldProject(t)
	stdout, stderr, code := runTP(t, dir, "done", "free", "free acceptance criteria met completely", "--gate-passed")
	require.Equal(t, 0, code, stderr)
	got := parseJSON(t, stdout)
	assert.InDelta(t, 0, got["remaining"].(map[string]any)["ready"], 0, "the held task is not ready")
	assert.Equal(t, false, got["has_next"])
}

func TestHeld_CommitOnAHeldTaskClearsTheHold(t *testing.T) {
	t.Parallel()
	dir := setupCloseStrategyProject(t, "builtin")
	_, stderr, code := runTP(t, dir, "set", "t1", "waiting=the release tag")
	require.Equal(t, 0, code, stderr)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "impl.go"), []byte("package impl\n"), 0o600))
	_, _, _ = runTP(t, dir, "commit", "t1", "implemented t1")
	assert.Nil(t, showTask(t, dir, "t1")["waiting"], "the implicit claim ends the hold, as every claim does")
}

func TestHeld_NextNamesHeldTasksApartFromBlocked(t *testing.T) {
	t.Parallel()
	dir := setupProject(t)
	addTaskWithEstimate(t, dir, "gated", 1)
	_, _, code := runTP(t, dir, "set", "gated", "waiting=the release tag")
	require.Equal(t, 0, code)
	stdout, _, code := runTP(t, dir, "next")
	require.Equal(t, 4, code)
	got := parseJSON(t, stdout)
	assert.Equal(t, []any{"gated"}, got["held"])
	assert.Empty(t, got["blocked"], "a held task is not blocked by a dependency")
}
