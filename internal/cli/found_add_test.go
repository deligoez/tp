package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// foundFixture seeds a project with one wip task (t1) and one open task (t2)
// and returns the directory and the task file's bytes as written.
func foundFixture(t *testing.T) (dir string, taskFileBytes []byte) {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":{},"tasks":[
		{"id":"t1","title":"T1","status":"wip","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]},
		{"id":"t2","title":"T2","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	taskFileBytes, err = os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	return dir, taskFileBytes
}

// fixFlags are the four Fix fields of one well-formed request.
var fixFlags = []string{
	"--location", "internal/x.go:12",
	"--reproduction", "go test ./internal -run TestX",
	"--expected", "exit 0",
	"--observed", "exit 1",
}

// foundAdd runs tp found add in dir with an environment free of any unit
// identity unless extra supplies one.
func foundAdd(t *testing.T, dir string, extra []string, flags ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runTPEnv(t, dir, extra, append([]string{"found", "add"}, flags...)...)
}

// storedRequests reads what tp found add recorded for the fixture's task file.
func storedRequests(t *testing.T, dir string) []engine.FoundRequest {
	t.Helper()
	reqs, err := engine.ReadFoundRequests(engine.CanonicalProjectRoot(dir), filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	return reqs
}

// assertTaskFileUnchanged holds §1's "it changes no task" on every path.
func assertTaskFileUnchanged(t *testing.T, dir string, before []byte) {
	t.Helper()
	after, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "tp found add changes no task")
}

func TestFoundAdd_AFixShapedRequestRecords(t *testing.T) {
	t.Parallel()
	dir, before := foundFixture(t)

	stdout, stderr, code := foundAdd(t, dir, nil, fixFlags...)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Equal(t, "F1", parseJSON(t, stdout)["id"], "the output names the recorded request's id")

	reqs := storedRequests(t, dir)
	require.Len(t, reqs, 1)
	assert.Equal(t, "F1", reqs[0].ID)
	assert.Equal(t, "internal/x.go:12", reqs[0].Location)
	assert.Equal(t, "go test ./internal -run TestX", reqs[0].Reproduction)
	assert.Equal(t, "exit 0", reqs[0].Expected)
	assert.Equal(t, "exit 1", reqs[0].Observed)
	assert.False(t, reqs[0].Blocks)
	assert.Empty(t, reqs[0].Finder.Task, "a caller working no task files with no finder task")
	assertTaskFileUnchanged(t, dir, before)
}

func TestFoundAdd_ARequestWithoutAReproductionExits2(t *testing.T) {
	t.Parallel()
	dir, before := foundFixture(t)

	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", "internal/x.go:12", "--expected", "exit 0", "--observed", "exit 1")
	assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "reproduction")
	assert.Empty(t, storedRequests(t, dir), "a refused request records nothing")
	assertTaskFileUnchanged(t, dir, before)
}

func TestFoundAdd_ARequestWithNeitherTheFixFieldsNorAQuestionExits2(t *testing.T) {
	t.Parallel()
	dir, before := foundFixture(t)

	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", "internal/x.go:12", "--reproduction", "go test ./internal -run TestX", "--expected", "exit 0")
	assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "question")
	assert.Empty(t, storedRequests(t, dir))
	assertTaskFileUnchanged(t, dir, before)
}

func TestFoundAdd_AQuestionShapedRequestRecordsItsAlternatives(t *testing.T) {
	t.Parallel()
	dir, before := foundFixture(t)

	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", "internal/x.go:12", "--reproduction", "go test ./internal -run TestX",
		"--question", "should x round up?", "--alternative", "up, as REFERENCE says", "--alternative", "down")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)

	reqs := storedRequests(t, dir)
	require.Len(t, reqs, 1)
	assert.Equal(t, "should x round up?", reqs[0].Question)
	assert.Equal(t, []string{"up, as REFERENCE says", "down"}, reqs[0].Alternatives, "an alternative is prose: commas do not split it")
	assertTaskFileUnchanged(t, dir, before)
}

func TestFoundAdd_BlocksWithoutATaskTheCallerWorksExits2(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		env   []string
		flags []string
	}{
		"no unit and no --task":     {},
		"an audit-role unit":        {env: []string{"TP_UNIT_KIND=audit-role", "TP_UNIT_ID=t1"}},
		"--task names an open task": {flags: []string{"--task", "t2"}},
		"--task names no task":      {flags: []string{"--task", "nope"}},
		"--task differs from the implement unit's task": {
			env:   []string{"TP_UNIT_KIND=implement", "TP_UNIT_ID=t1"},
			flags: []string{"--task", "t2"},
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, before := foundFixture(t)
			flags := append(append(append([]string{}, fixFlags...), "--blocks"), tc.flags...)
			stdout, stderr, code := foundAdd(t, dir, tc.env, flags...)
			assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
			assert.Empty(t, storedRequests(t, dir))
			assertTaskFileUnchanged(t, dir, before)
		})
	}
}

func TestFoundAdd_BlocksUnderAnImplementUnitNamesItsTask(t *testing.T) {
	t.Parallel()
	dir, before := foundFixture(t)

	stdout, stderr, code := foundAdd(t, dir, []string{"TP_UNIT_KIND=implement", "TP_UNIT_ID=t1"},
		append(append([]string{}, fixFlags...), "--blocks")...)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)

	reqs := storedRequests(t, dir)
	require.Len(t, reqs, 1)
	assert.True(t, reqs[0].Blocks)
	assert.Equal(t, "t1", reqs[0].Finder.Task)
	assertTaskFileUnchanged(t, dir, before)
}

func TestFoundAdd_BlocksWithTaskNamingAWipTaskRecords(t *testing.T) {
	t.Parallel()
	dir, before := foundFixture(t)

	stdout, stderr, code := foundAdd(t, dir, nil, append(append([]string{}, fixFlags...), "--blocks", "--task", "t1")...)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)

	reqs := storedRequests(t, dir)
	require.Len(t, reqs, 1)
	assert.True(t, reqs[0].Blocks)
	assert.Equal(t, "t1", reqs[0].Finder.Task)
	assertTaskFileUnchanged(t, dir, before)
}
