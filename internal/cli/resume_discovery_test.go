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
	dir := t.TempDir()
	_, stderr, code := runTP(t, dir, "resume")
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "no task file found")
}

func TestResume_SpecArgumentWinsOverDiscovered(t *testing.T) {
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
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "solo.md"), []byte("# Solo\n"), 0o600))
	// No solo.tasks.json adjacent: an empty task set, phase from review state.
	out, stderr, code := runTP(t, dir, "resume", "solo.md")
	require.Equal(t, 0, code, "resume: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res))
	assert.Equal(t, "review", res["phase"], "an absent adjacent task file with no review state yields review")
}
