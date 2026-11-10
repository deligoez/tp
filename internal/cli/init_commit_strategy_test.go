package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestInit_RefusesUnknownCommitStrategy: an unrecognized --commit-strategy is a
// usage error at init (exit 2, naming the valid values), and no task file is
// written; before the fix the value was stored and surfaced only later as a
// warning on the surfaces that read the strategy.
func TestInit_RefusesUnknownCommitStrategy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n\nContent.\n"), 0o600))

	_, stderr, code := runTP(t, dir, "init", "spec.md", "--commit-strategy", "tp")

	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "builtin, auto, hc")
	assert.NoFileExists(t, filepath.Join(dir, "spec.tasks.json"))
}
