package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/model"
)

// taskDeps reads each task's depends_on from the fixture's task file.
func taskDeps(t *testing.T, dir string) map[string][]string {
	t.Helper()
	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	deps := map[string][]string{}
	for i := range tf.Tasks {
		deps[tf.Tasks[i].ID] = tf.Tasks[i].DependsOn
	}
	return deps
}

// §4 through tp found apply: a non-blocking request's found task depends on
// its finder, and after a blocking request the finder depends on the found
// task. The task file still validates.
func TestFoundApply_PlacesFoundTasksOnTheFindersEdge(t *testing.T) {
	t.Parallel()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","tasks":[
		{"id":"t1","title":"T1","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	for _, args := range [][]string{
		{"--location", "a.go:1", "--reproduction", "go test -run TestA", "--expected", "exit 0", "--observed", "exit 1", "--task", "t1"},
		{"--location", "b.go:1", "--reproduction", "go test -run TestB", "--expected", "exit 0", "--observed", "exit 1", "--task", "t1", "--blocks"},
	} {
		stdout, stderr, code := foundAdd(t, dir, nil, args...)
		require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	}

	out := foundApply(t, dir)

	assert.Equal(t, []any{
		map[string]any{"request": "F2", "task": "found-f2"},
		map[string]any{"request": "F1", "task": "found-f1"},
	}, out["opened"])
	deps := taskDeps(t, dir)
	assert.Equal(t, []string{"t1"}, deps["found-f1"], "the non-blocking found task depends on its finder")
	assert.Equal(t, []string{}, deps["found-f2"], "the blocking one does not")
	assert.Equal(t, []string{"found-f2"}, deps["t1"], "the finder depends on the blocking request's task")
	stdout, _, _ := runTP(t, dir, "validate")
	checks, ok := parseJSON(t, stdout)["checks"].(map[string]any)
	require.True(t, ok, stdout)
	assert.Equal(t, "pass", checks["cycles"], stdout)
	assert.Equal(t, "pass", checks["references"], stdout)
}
