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

// exportRepo is a git repository holding spec.md and its task file with one
// described task, and two recorded review rounds with the spec's §1 edited
// between them, so round 2's snapshot differs from round 1's.
func exportRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# S\n\n## 1. X\n\nx first.\n\n## 2. Y\n\ny.\n"), 0o600))
	_, stderr, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code, stderr)
	_, stderr, code = runTP(t, dir, "add", `{"id":"t1","title":"X","description":"the one task","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":["## 1. X"]}`)
	require.Equal(t, 0, code, stderr)
	empty := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, nil, 0o600))
	for _, text := range []string{"x first.", "x rewritten."} {
		require.NoError(t, os.WriteFile(spec, []byte("# S\n\n## 1. X\n\n"+text+"\n\n## 2. Y\n\ny.\n"), 0o600))
		_, stderr, code = runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code, stderr)
		_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", empty)
		require.Equal(t, 0, code, stderr)
	}
	initGitRepo(t, dir)
	return dir
}

// exportJSON runs tp export with args in dir and decodes its payload.
func exportJSON(t *testing.T, dir string, args ...string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, append([]string{"export"}, args...)...)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out), "stdout: %s", stdout)
	return out
}

// exportReviewRounds returns the payload's review rounds.
func exportReviewRounds(t *testing.T, out map[string]any) []any {
	t.Helper()
	loops, ok := out["loops"].(map[string]any)
	require.True(t, ok, "loops: %v", out["loops"])
	review, ok := loops["review"].(map[string]any)
	require.True(t, ok)
	rounds, ok := review["rounds"].([]any)
	require.True(t, ok)
	return rounds
}

// TestExport_PrintsSectionsTasksLoopsAndResume is §16's tp export --file:
// version, the sections with their anchors and paragraphs, the tasks, the
// loops with one entry per recorded round — round 2 carrying the difference
// of its snapshot from round 1's — and the tp resume view.
func TestExport_PrintsSectionsTasksLoopsAndResume(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	out := exportJSON(t, dir, "--file", "spec.tasks.json")

	assert.EqualValues(t, 1, out["version"])
	sections, ok := out["sections"].([]any)
	require.True(t, ok)
	require.Len(t, sections, 3)
	s1 := sections[1].(map[string]any)
	assert.Equal(t, "§1", s1["anchor"])
	require.NotEmpty(t, s1["paragraphs"])
	assert.Equal(t, "§1 ¶1", s1["paragraphs"].([]any)[0].(map[string]any)["anchor"])

	tasks, ok := out["tasks"].([]any)
	require.True(t, ok)
	require.Len(t, tasks, 1)
	task := tasks[0].(map[string]any)
	assert.Equal(t, "ready", task["state"])
	assert.Equal(t, "the one task", task["description"])

	rounds := exportReviewRounds(t, out)
	require.Len(t, rounds, 2)
	assert.Nil(t, rounds[0].(map[string]any)["snapshot_diff"])
	assert.NotNil(t, rounds[1].(map[string]any)["snapshot_diff"])

	resume, ok := out["resume"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "implement", resume["phase"])
	assert.Contains(t, resume["next_action"], "summary")
}

// TestExport_CompactDropsBodiesDescriptionsAndDiffs is §16's --compact: no
// section body or paragraph anchor, no task description, no snapshot
// difference, and the resume view compacted as tp resume --compact is; the
// headings, anchors, task states and loop counts stay.
func TestExport_CompactDropsBodiesDescriptionsAndDiffs(t *testing.T) {
	t.Parallel()
	dir := exportRepo(t)
	out := exportJSON(t, dir, "--file", "spec.tasks.json", "--compact")

	for _, s := range out["sections"].([]any) {
		sec := s.(map[string]any)
		assert.NotContains(t, sec, "paragraphs")
		assert.Contains(t, sec, "anchor")
		assert.Contains(t, sec, "heading")
	}
	task := out["tasks"].([]any)[0].(map[string]any)
	assert.NotContains(t, task, "description")
	assert.Equal(t, "ready", task["state"])
	rounds := exportReviewRounds(t, out)
	require.Len(t, rounds, 2)
	for _, r := range rounds {
		assert.Nil(t, r.(map[string]any)["snapshot_diff"])
	}
	resume := out["resume"].(map[string]any)
	assert.NotContains(t, resume["next_action"], "summary")
	assert.Contains(t, resume, "next_units")
}

