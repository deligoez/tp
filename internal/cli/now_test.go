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

// A wip task claimed by hand is held by you; tasks counts as tp status does,
// and ready lists the startable tasks in tp ready's order.
func TestNow_PlainWipTaskIsHeldByYou(t *testing.T) {
	t.Parallel()
	dir := runProject(t)
	for _, args := range [][]string{
		{"add", `{"id":"grow","title":"Grow","estimate_minutes":5,"acceptance":"Grown.","source_sections":["### 1.1 Seed"],"depends_on":[]}`},
		{"add", `{"id":"trim","title":"Trim","estimate_minutes":5,"acceptance":"Trimmed.","source_sections":["### 1.1 Seed"],"depends_on":["seed"]}`},
		{"claim", "seed"},
	} {
		_, stderr, code := runTP(t, dir, args...)
		require.Equal(t, 0, code, "%v: %s", args, stderr)
	}

	stdout, stderr, code := runTP(t, dir, "now")
	require.Equal(t, 0, code, "tp now: %s", stderr)
	out := decodeNow(t, stdout)
	assert.Equal(t, "spec.tasks.json", out["task_file"])
	assert.Equal(t, "implement", out["phase"])
	assert.Equal(t, map[string]any{"total": 3.0, "done": 0.0, "wip": 1.0, "open": 2.0, "ready": 1.0, "blocked": 1.0, "held": 0.0},
		out["tasks"])
	assert.Equal(t, []any{map[string]any{"id": "grow", "title": "Grow"}}, out["ready"])

	held, ok := out["held"].([]any)
	require.True(t, ok, "held is a list: %v", out["held"])
	require.Len(t, held, 1)
	seed := held[0].(map[string]any)
	assert.Equal(t, "seed", seed["id"])
	assert.Equal(t, "Seed", seed["title"])
	assert.Equal(t, "you", seed["holder"], "a plain wip task with no claim is held by you")
	assert.NotNil(t, seed["since"], "since is the task's started_at")
	assert.Nil(t, seed["worktree"])
	assert.Nil(t, seed["attempt"])
}
