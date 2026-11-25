package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// §2 acceptance "A request inside an open task's files opens no task and
// shows on that task": t1's touches hold internal/x/, so a request at
// internal/x/y.go opens nothing, leaves the found-task count at zero, and
// shows in both tp show and tp brief for t1.
func TestFoundApply_ARequestInsideAnOpenTasksFilesShowsOnThatTask(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":{"found_max":1},"tasks":[
		{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"],"touches":["internal/x/"]}
	]}`)
	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", "internal/x/y.go:12", "--reproduction", "go test ./internal/x -run TestY",
		"--expected", "exit 0", "--observed", "exit 1")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)

	out := foundApply(t, dir)

	assert.Nil(t, out["opened"], "an attached request opens no task")
	assert.Nil(t, out["queued"])
	assert.Equal(t, []any{map[string]any{"request": "F1", "task": "t1"}}, out["attached"])
	assert.EqualValues(t, 0, out["found_tasks"], "the attachment does not count against the cap")
	assert.Empty(t, foundTasksIn(t, dir))

	note := map[string]any{
		"request":      "F1",
		"location":     "internal/x/y.go:12",
		"reproduction": "go test ./internal/x -run TestY",
		"expected":     "exit 0",
		"observed":     "exit 1",
	}
	stdout, stderr, code = runTP(t, dir, "show", "t1")
	require.Equal(t, 0, code, stderr)
	assert.Equal(t, []any{note}, parseJSON(t, stdout)["found_notes"])

	for _, args := range [][]string{{"brief", "t1"}, {"--compact", "brief", "t1"}} {
		stdout, stderr, code = runTP(t, dir, args...)
		require.Equal(t, 0, code, stderr)
		task, ok := parseJSON(t, stdout)["task"].(map[string]any)
		require.True(t, ok, stdout)
		assert.Equal(t, []any{note}, task["found_notes"], args)
	}

	// The cap is one and still free: a plain request elsewhere opens a task.
	fileFixRequest(t, dir, 1)
	again := foundApply(t, dir)
	assert.Equal(t, []any{map[string]any{"request": "F2", "task": "found-f2"}}, again["opened"])
}
