package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResume_NoTaskFileNoArgExit3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "resume")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "no task file found")
	assert.Contains(t, stderr, "or pass a spec path", "the no-file hint stays (spec/0.28.0.md §4.1)")
	assert.NotContains(t, stderr, "multiple task files")
}

// TestResume_SeveralTaskFilesNoArgNamesTheCandidates: with two task files and
// no active pointer, the no-argument form said "no task file found" whatever
// discovery failed on. It must report the discovery error the other commands
// give — both candidates and how to pick one — at the same exit 3.
func TestResume_SeveralTaskFilesNoArgNamesTheCandidates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	for _, base := range []string{"a", "b"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, base+".md"), []byte("# "+base+"\n"), 0o600))
		_, stderr, code := runTP(t, dir, "init", base+".md")
		require.Equal(t, 0, code, stderr)
	}

	_, statusErr, statusCode := runTP(t, dir, "status")
	require.Equal(t, 3, statusCode, statusErr)

	_, stderr, code := runTP(t, dir, "resume")
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "multiple task files", "resume must report the discovery error")
	assert.Contains(t, stderr, "a.tasks.json", "the error names the first candidate")
	assert.Contains(t, stderr, "b.tasks.json", "the error names the second candidate")
	assert.NotContains(t, stderr, "no task file found", "task files exist, so not-found is wrong")
	assert.Equal(t, statusErr, stderr, "resume reports the same discovery error as tp status")
}

// TestResume_MissingExplicitFileNamesThePath: the no-argument form discarded
// the explicit-path failure too, answering "no task file found" when --file or
// TP_FILE named a path that was not there. It must name that path.
func TestResume_MissingExplicitFileNamesThePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, statusErr, statusCode := runTP(t, dir, "--file", "missing.tasks.json", "status")
	require.Equal(t, 3, statusCode, statusErr)

	_, stderr, code := runTP(t, dir, "--file", "missing.tasks.json", "resume")
	assert.Equal(t, 3, code, stderr)
	assert.Contains(t, stderr, "missing.tasks.json", "the error names the path it could not open")
	assert.NotContains(t, stderr, "no task file found", "the caller named a file, so not-found-at-all is wrong")
	assert.Equal(t, statusErr, stderr, "resume reports the same discovery error as tp status")
}

func TestResume_SpecArgumentWinsOverDiscovered(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A discoverable task file that points at disc.md and holds an open task
	// (which would read as implement).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "disc.md"), []byte("# Disc\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "disc.tasks.json"),
		[]byte(`{"spec":"disc.md","tasks":[{"id":"t","title":"T","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"a","source_sections":["x"]}]}`), 0o600))
	// The argument's spec and its adjacent task file (zero tasks).
	require.NoError(t, os.WriteFile(filepath.Join(dir, "arg.md"), []byte("# Arg\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "arg.tasks.json"),
		[]byte(`{"spec":"arg.md","tasks":[]}`), 0o600))

	out, stderr, code := runTP(t, dir, "resume", "arg.md")
	require.Equal(t, 0, code, "resume: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.True(t, strings.HasSuffix(res["spec"].(string), "arg.md"), "the spec argument wins over the discovered spec")
	assert.Equal(t, "review", res["phase"], "the argument's zero-task file with no review state reads as review")
}

func TestResume_AbsentAdjacentTaskFileYieldsReview(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "solo.md"), []byte("# Solo\n"), 0o600))
	// No solo.tasks.json adjacent: an empty task set, phase from review state.
	out, stderr, code := runTP(t, dir, "resume", "solo.md")
	require.Equal(t, 0, code, "resume: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, "review", res["phase"], "an absent adjacent task file with no review state yields review")
}
