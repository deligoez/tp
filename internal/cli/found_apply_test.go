package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// foundApplyFixture seeds a project whose task file's workflow is the given
// JSON object, with one open spec task.
func foundApplyFixture(t *testing.T, workflow string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	writeTaskFile(t, dir, `{"version":1,"spec":"spec.md","workflow":`+workflow+`,"tasks":[
		{"id":"t1","title":"T1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["# S"]}
	]}`)
	return dir
}

// fileFixRequest files a Fix-shaped request at a location of its own.
func fileFixRequest(t *testing.T, dir string, n int) {
	t.Helper()
	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", fmt.Sprintf("internal/f%d.go:3", n),
		"--reproduction", fmt.Sprintf("go test ./internal -run TestF%d", n),
		"--expected", "exit 0", "--observed", "exit 1")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
}

func foundApply(t *testing.T, dir string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTPEnv(t, dir, nil, "found", "apply")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	return parseJSON(t, stdout)
}

func foundTasksIn(t *testing.T, dir string) []string {
	t.Helper()
	tf, err := model.ReadTaskFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	ids := []string{}
	for i := range tf.Tasks {
		if tf.Tasks[i].Origin != nil {
			ids = append(ids, tf.Tasks[i].ID)
		}
	}
	return ids
}

// §10 row "The cap holds across a restart": five found tasks applied by one
// apply, then a sixth request applied by a later process, is queued. The
// mutant that counts found tasks per run (per apply) opens it.
func TestFoundApply_TheCapHoldsAcrossARestart(t *testing.T) {
	t.Parallel()
	dir := foundApplyFixture(t, `{}`)
	for n := 1; n <= 5; n++ {
		fileFixRequest(t, dir, n)
	}
	first := foundApply(t, dir)
	assert.Len(t, first["opened"], 5)
	assert.EqualValues(t, 5, first["found_tasks"])
	assert.EqualValues(t, 5, first["found_max"], "found_max defaults to 5")

	fileFixRequest(t, dir, 6)
	second := foundApply(t, dir)

	assert.Nil(t, second["opened"], "the sixth Fix-shaped request opens no task")
	assert.Equal(t, []any{map[string]any{"request": "F6", "entry": "Q1"}}, second["queued"])
	assert.Len(t, foundTasksIn(t, dir), 5)
	queue, err := engine.ReadFoundQueue(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	require.Len(t, queue, 1)
	assert.Equal(t, engine.FoundQueueOpen, queue[0].State)
	assert.Equal(t, "internal/f6.go:3", queue[0].Location)
	assert.Equal(t, "go test ./internal -run TestF6", queue[0].Reproduction)

	// The fixture has no coverage block, so validate judges coverage on its
	// own; the found tasks must pass every check about the tasks themselves.
	stdout, _, _ := runTP(t, dir, "validate")
	report := parseJSON(t, stdout)
	checks, ok := report["checks"].(map[string]any)
	require.True(t, ok, stdout)
	for _, check := range []string{"schema", "references", "uniqueness", "cycles"} {
		assert.Equal(t, "pass", checks[check], check)
	}
	assert.NotContains(t, stdout, `found-f`, "no finding names a found task")
}

// §8: at found_max=0 the sixth Fix-shaped request opens a found task.
func TestFoundApply_FoundMaxZeroOpensTheSixth(t *testing.T) {
	t.Parallel()
	dir := foundApplyFixture(t, `{"found_max":0}`)
	for n := 1; n <= 6; n++ {
		fileFixRequest(t, dir, n)
	}

	out := foundApply(t, dir)

	assert.Len(t, out["opened"], 6)
	assert.Nil(t, out["queued"])
	assert.Len(t, foundTasksIn(t, dir), 6)
}

// A request that needs a choice goes to the decision unit, and a second apply
// finds nothing left to apply.
func TestFoundApply_AQuestionIsUndecidedAndASecondApplyIsANoOp(t *testing.T) {
	t.Parallel()
	dir := foundApplyFixture(t, `{}`)
	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", "internal/q.go:9", "--reproduction", "go test ./internal -run TestQ",
		"--question", "which default?", "--alternative", "a", "--alternative", "b")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	fileFixRequest(t, dir, 1)

	out := foundApply(t, dir)
	assert.Equal(t, []any{"F1"}, out["undecided"])
	assert.Equal(t, []any{map[string]any{"request": "F2", "task": "found-f2"}}, out["opened"])

	before, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	again := foundApply(t, dir)
	assert.Nil(t, again["opened"])
	assert.Nil(t, again["undecided"])
	after, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "nothing is left to apply")
}

// An apply whose request store cannot be written leaves nothing of the apply
// on disk: the task file it wrote first is put back, so the request stays
// recorded and applying again opens found-f1, not a second found-f1-2 that
// dedup could not see.
func TestFoundApply_AFailedStoreWriteLeavesNoTaskBehind(t *testing.T) {
	t.Parallel()
	dir := foundApplyFixture(t, `{}`)
	fileFixRequest(t, dir, 1)
	store := engine.FoundRequestsPath(dir, filepath.Join(dir, "spec.tasks.json"))
	storeDir := filepath.Dir(store)
	// The store's lock directory exists before its parent is made read-only,
	// so apply gets past the lock and writes the task file, and only the
	// store write that follows fails.
	require.NoError(t, os.MkdirAll(filepath.Dir(engine.LockFilePath(store)), 0o755))
	require.NoError(t, os.Chmod(storeDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(storeDir, 0o755) }) //nolint:gosec // restores a test directory

	stdout, stderr, code := runTPEnv(t, dir, nil, "found", "apply")
	require.Equal(t, 3, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Empty(t, foundTasksIn(t, dir), "the task written before the store is put back")

	require.NoError(t, os.Chmod(storeDir, 0o755)) //nolint:gosec // restores a test directory
	assert.Equal(t, []any{map[string]any{"request": "F1", "task": "found-f1"}}, foundApply(t, dir)["opened"])
	assert.Equal(t, []string{"found-f1"}, foundTasksIn(t, dir))
}

// §12.2: tp found apply holds the task-file lock, then the store's, and tp
// found add the store's. A lock held past lock_timeout_seconds exits 4
// (STATE) with LockTimeoutError's message and hint, not the exit 3 of a
// failed write.
func TestFoundApplyAndAdd_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	taskFile := func(dir string) string { return filepath.Join(dir, "spec.tasks.json") }
	store := func(dir string) string { return engine.FoundRequestsPath(dir, taskFile(dir)) }
	apply := []string{"found", "apply"}
	add := []string{"found", "add", "--location", "internal/f2.go:3", "--reproduction", "go test ./internal -run TestF2",
		"--expected", "exit 0", "--observed", "exit 1"}
	for _, tc := range []struct {
		name   string
		target func(dir string) string
		args   []string
	}{
		{"apply, task file", taskFile, apply},
		{"apply, request store", store, apply},
		{"add, request store", store, add},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := foundApplyFixture(t, `{"lock_timeout_seconds":1}`)
			fileFixRequest(t, dir, 1)
			foundApply(t, dir)

			release := holdTaskFileLock(t, tc.target(dir))
			defer release()

			_, stderr, code := runTPEnv(t, dir, nil, tc.args...)
			assert.Equal(t, 4, code, "a lock held past lock_timeout_seconds exits 4 (STATE): %s", stderr)
			assertLockTimeoutErrorObject(t, stderr)
		})
	}
}
