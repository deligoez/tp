package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImport_RefusesAnUnreadableTarget: a task file that exists and cannot be
// read carries an epic binding and a workflow no import can see, so even
// --force refuses rather than writing over them.
func TestImport_RefusesAnUnreadableTarget(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\n## 1. A\n\nText.\n"), 0o600))
	broken := []byte(`{"version":1,"spec":"spec.md","epic":"e1",`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), broken, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "plan.json"),
		[]byte(`[{"id":"x1","title":"X","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":["## 1. A"]}]`), 0o600))

	_, stderr, code := runTP(t, dir, "import", "plan.json", "--spec", "spec.md", "--force")
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "spec.tasks.json")
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.Equal(t, broken, data, "the refused import wrote nothing")
}
