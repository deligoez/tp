package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResume_UnparseableAdjacentTaskFileExits3: an adjacent task file that
// does not parse was treated as an empty task set, so tp resume <spec> sent
// the agent back to review over a plan it could not read. Only an absent file
// is an empty task set; a parse failure exits 3 naming the error, as the
// no-argument form does.
func TestResume_UnparseableAdjacentTaskFileExits3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, "spec"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec", "s.md"), []byte("# S\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec", "s.tasks.json"), []byte(`{"spec":"s.md","tasks":[`), 0o600))

	out, stderr, code := runTP(t, dir, "resume", "spec/s.md")
	assert.Equal(t, 3, code, "stdout: %s stderr: %s", out, stderr)
	assert.Contains(t, stderr, "parse task file", "the error names the parse failure")
	assert.NotContains(t, out, `"phase"`, "no phase is reported over an unreadable plan")
}
