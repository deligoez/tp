package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeNow parses a `tp now` payload.
func decodeNow(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "now payload: %s", stdout)
	return out
}

// listTree names every path under dir, so a test can show a read-only command
// left the tree as it found it.
func listTree(t *testing.T, dir string) []string {
	t.Helper()
	var paths []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, _ os.DirEntry, err error) error {
		paths = append(paths, path)
		return err
	}))
	return paths
}

// A project with a .tp/ directory and no task file is still a project: tp now
// exits 0 with task_file and tasks null and every list empty, and writes
// nothing. Outside any project it exits 3, as other commands do.
func TestNow_ProjectWithoutTaskFile(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	project := filepath.Join(dir, "shop")
	require.NoError(t, os.MkdirAll(filepath.Join(project, ".tp"), 0o750))
	before := listTree(t, project)

	stdout, stderr, code := runTP(t, project, "now")
	require.Equal(t, 0, code, "a project with no task file is not an error: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Equal(t, "shop", out["project"])
	assert.Contains(t, out, "task_file")
	assert.Nil(t, out["task_file"])
	assert.Contains(t, out, "tasks")
	assert.Nil(t, out["tasks"])
	assert.Nil(t, out["run"])
	assert.Nil(t, out["epics"])
	for _, key := range []string{"needs_you", "held", "landing", "ready"} {
		assert.Equal(t, []any{}, out[key], "%s is present and empty", key)
	}
	assert.Equal(t, before, listTree(t, project), "tp now writes nothing")

	bare := filepath.Join(dir, "bare")
	require.NoError(t, os.MkdirAll(bare, 0o750))
	_, _, code = runTP(t, bare, "now")
	assert.Equal(t, 3, code, "outside a project tp now exits 3")
}
