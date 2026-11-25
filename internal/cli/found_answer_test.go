package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// answerSpecText is the fixture spec a decision answer quotes. Its settling
// sentence spans a line break.
const answerSpecText = "# S\n\nThe default retry count is\nthree.\n"

// undecidedFixture seeds a project with one question-shaped request applied
// to the undecided state, and returns the project dir.
func undecidedFixture(t *testing.T, workflow string) string {
	t.Helper()
	dir := foundApplyFixture(t, workflow)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte(answerSpecText), 0o600))
	stdout, stderr, code := foundAdd(t, dir, nil,
		"--location", "internal/q.go:9", "--reproduction", "go test ./internal -run TestQ",
		"--question", "which default?", "--alternative", "two", "--alternative", "three")
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	return dir
}

// applyToUndecided runs tp found apply and requires F1 to be undecided.
func applyToUndecided(t *testing.T, dir string) {
	t.Helper()
	assert.Equal(t, []any{"F1"}, foundApply(t, dir)["undecided"])
}

func foundAnswer(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	return runTPEnv(t, dir, nil, append([]string{"found", "answer"}, args...)...)
}

// requestState is the stored state of the fixture's request F1.
func requestState(t *testing.T, dir string) string {
	t.Helper()
	reqs, err := engine.ReadFoundRequests(dir, filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	for i := range reqs {
		if reqs[i].ID == "F1" {
			return reqs[i].State
		}
	}
	t.Fatal("request F1 not in the store")
	return ""
}

func readQueue(t *testing.T, dir string) []engine.FoundQueueEntry {
	t.Helper()
	queue, err := engine.ReadFoundQueue(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	return queue
}

// §10 row "A quote must be in the cited file": a task answer quoting a
// sentence absent from the spec is refused and writes nothing.
func TestFoundAnswer_AQuoteMustBeInTheCitedFile(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{}`)
	applyToUndecided(t, dir)

	stdout, stderr, code := foundAnswer(t, dir, "F1", "task",
		"--expected", "three retries", "--observed", "five retries",
		"--cite", "spec.md", "--quote", "The default retry count is five.")

	assert.Equal(t, 2, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Contains(t, stdout+stderr, "the quote is not in spec.md")
	assert.Equal(t, engine.FoundStateUndecided, requestState(t, dir))
	assert.Empty(t, foundTasksIn(t, dir))
	assert.Empty(t, readQueue(t, dir))
}

// §10 row "The quote check normalises whitespace": the quote spans the spec's
// line break. The answer keeps the request's location and reproduction, so it
// also shows the answer is never a duplicate of the request it answers.
func TestFoundAnswer_TheQuoteCheckNormalisesWhitespace(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{}`)
	applyToUndecided(t, dir)

	stdout, stderr, code := foundAnswer(t, dir, "F1", "task",
		"--expected", "three retries", "--observed", "five retries",
		"--cite", "spec.md", "--quote", "The default retry count is three.")

	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	out := parseJSON(t, stdout)
	assert.Equal(t, []any{map[string]any{"request": "F1", "task": "found-f1"}}, out["opened"])
	assert.Nil(t, out["duplicates"])
	assert.Equal(t, []string{"found-f1"}, foundTasksIn(t, dir))
	assert.Equal(t, engine.FoundStateApplied, requestState(t, dir))
}

func TestFoundAnswer_AQuoteFromAFileThatIsNotCitableIsRefused(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{}`)
	applyToUndecided(t, dir)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte(answerSpecText), 0o600))

	_, _, code := foundAnswer(t, dir, "F1", "task",
		"--expected", "three retries", "--observed", "five retries",
		"--cite", "README.md", "--quote", "The default retry count is three.")

	assert.Equal(t, 2, code)
	assert.Equal(t, engine.FoundStateUndecided, requestState(t, dir))
}

func TestFoundAnswer_AConformsAnswerIsDecidedAndOpensNoTask(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{}`)
	applyToUndecided(t, dir)

	stdout, stderr, code := foundAnswer(t, dir, "F1", "conforms",
		"--reproduction", "go test ./internal -run TestRetryDefault",
		"--cite", "spec.md", "--quote", "retry count is three")

	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Empty(t, foundTasksIn(t, dir))
	queue := readQueue(t, dir)
	require.Len(t, queue, 1)
	assert.Equal(t, engine.FoundQueueDecided, queue[0].State)
	assert.Equal(t, engine.FoundAnswerConforms, queue[0].Answer)
	assert.Equal(t, "F1", queue[0].Request)
	assert.Equal(t, "spec.md", queue[0].Cited)
	assert.Equal(t, "retry count is three", queue[0].Quote)
	assert.Equal(t, "go test ./internal -run TestRetryDefault", queue[0].Reproduction)
	assert.Equal(t, "Q1", parseJSON(t, stdout)["entry"])
	assert.Equal(t, engine.FoundStateApplied, requestState(t, dir))
}

func TestFoundAnswer_AHumanAnswerAddsAnOpenEntryWithTheQuestionAndAlternatives(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{}`)
	applyToUndecided(t, dir)

	stdout, stderr, code := foundAnswer(t, dir, "F1", "human")

	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Empty(t, foundTasksIn(t, dir))
	queue := readQueue(t, dir)
	require.Len(t, queue, 1)
	assert.Equal(t, engine.FoundQueueOpen, queue[0].State)
	assert.Equal(t, "which default?", queue[0].Question)
	assert.Equal(t, []string{"two", "three"}, queue[0].Alternatives)
	assert.Equal(t, "internal/q.go:9", queue[0].Location)
	assert.Equal(t, engine.FoundStateApplied, requestState(t, dir))

	_, _, again := foundAnswer(t, dir, "F1", "human")
	assert.Equal(t, 2, again, "an answered request is no longer undecided")
}

// §5: a task answer counts against the cap; past it the answer is an open
// queue entry with its quote and reproduction, and no task opens.
func TestFoundAnswer_ATaskAnswerPastTheCapAddsAnOpenEntry(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{"found_max":1}`)
	fileFixRequest(t, dir, 1)
	out := foundApply(t, dir)
	assert.Len(t, out["opened"], 1)
	assert.Equal(t, []any{"F1"}, out["undecided"])

	stdout, stderr, code := foundAnswer(t, dir, "F1", "task",
		"--expected", "three retries", "--observed", "five retries",
		"--cite", "spec.md", "--quote", "count is three")

	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Nil(t, parseJSON(t, stdout)["opened"])
	assert.Len(t, foundTasksIn(t, dir), 1, "only the Fix-shaped request's task")
	queue := readQueue(t, dir)
	require.Len(t, queue, 1)
	assert.Equal(t, engine.FoundQueueOpen, queue[0].State)
	assert.Equal(t, "F1", queue[0].Request)
	assert.Equal(t, "count is three", queue[0].Quote)
	assert.Equal(t, "go test ./internal -run TestQ", queue[0].Reproduction)
}

// A task answer whose request store cannot be written leaves nothing of the
// answer on disk: the task file and queue it wrote first are put back, so the
// request stays undecided and answering it again opens found-f1, not a second
// found-f1-2 that dedup could not see.
func TestFoundAnswer_AFailedStoreWriteLeavesNoTaskBehind(t *testing.T) {
	t.Parallel()
	dir := undecidedFixture(t, `{}`)
	applyToUndecided(t, dir)
	storeDir := filepath.Dir(engine.FoundRequestsPath(dir, filepath.Join(dir, "spec.tasks.json")))
	require.NoError(t, os.Chmod(storeDir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(storeDir, 0o755) })
	answer := []string{"F1", "task", "--expected", "three retries", "--observed", "five retries",
		"--cite", "spec.md", "--quote", "The default retry count is three."}

	stdout, stderr, code := foundAnswer(t, dir, answer...)
	require.Equal(t, 3, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Empty(t, foundTasksIn(t, dir), "the task written before the store is put back")
	assert.Empty(t, readQueue(t, dir))
	assert.Equal(t, engine.FoundStateUndecided, requestState(t, dir))

	require.NoError(t, os.Chmod(storeDir, 0o755))
	stdout, stderr, code = foundAnswer(t, dir, answer...)
	require.Equal(t, 0, code, "stdout=%s stderr=%s", stdout, stderr)
	assert.Equal(t, []string{"found-f1"}, foundTasksIn(t, dir))
}

// §12.2: tp found answer holds the task-file lock, then the store's. A lock
// held past lock_timeout_seconds at either exits 4 (STATE) with
// LockTimeoutError's message and hint, not the exit 3 of a failed write.
func TestFoundAnswer_LockContentionTimeoutExitsFour(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		target func(dir string) string
	}{
		{"task file", func(dir string) string { return filepath.Join(dir, "spec.tasks.json") }},
		{"request store", func(dir string) string {
			return engine.FoundRequestsPath(dir, filepath.Join(dir, "spec.tasks.json"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := undecidedFixture(t, `{"lock_timeout_seconds":1}`)
			applyToUndecided(t, dir)

			release := holdTaskFileLock(t, tc.target(dir))
			defer release()

			_, stderr, code := foundAnswer(t, dir, "F1", "human")
			assert.Equal(t, 4, code, "a lock held past lock_timeout_seconds exits 4 (STATE): %s", stderr)
			assertLockTimeoutErrorObject(t, stderr)
		})
	}
}
