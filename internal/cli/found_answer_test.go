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
