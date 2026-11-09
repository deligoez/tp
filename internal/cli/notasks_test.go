package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noTasksPayload is tp next's answer on a task file holding no task.
type noTasksPayload struct {
	Done    bool    `json:"done"`
	Message string  `json:"message"`
	Phase   string  `json:"phase"`
	Hint    string  `json:"hint"`
	Command *string `json:"command"`
}

// emptyTaskFileDir is a project whose spec.tasks.json holds no task and whose
// spec has no recorded round, so tp resume reports review for it.
func emptyTaskFileDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(`{"version":1,"spec":"spec.md","tasks":[]}`), 0o600))
	return dir
}

// assertNoTasksYet checks tp next's answer on an empty file: not all tasks
// complete, but no tasks yet, naming tp resume's phase and its next step, and
// exit 4 as tp next's other no-task answers.
func assertNoTasksYet(t *testing.T, dir string, args ...string) {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 4, code, "%v stdout: %s stderr: %s", args, stdout, stderr)
	var p noTasksPayload
	require.NoError(t, json.Unmarshal([]byte(stdout), &p), "%v stdout: %s", args, stdout)
	assert.False(t, p.Done, "%v stdout: %s", args, stdout)
	assert.NotContains(t, p.Message, "All tasks complete", "%v", args)
	assert.Contains(t, p.Message, "no tasks yet", "%v", args)
	assert.Equal(t, "review", p.Phase, "%v", args)
	assert.Contains(t, p.Hint, "review round 1", "%v", args)
	require.NotNil(t, p.Command, "%v stdout: %s", args, stdout)
	assert.Contains(t, *p.Command, "tp review", "%v", args)
}

// TestNext_AnEmptyTaskFileAnswersNoTasksYet: tp next and tp next --peek on a
// file with zero tasks do not report all tasks complete.
func TestNext_AnEmptyTaskFileAnswersNoTasksYet(t *testing.T) {
	t.Parallel()
	dir := emptyTaskFileDir(t)
	assertNoTasksYet(t, dir, "next")
	assertNoTasksYet(t, dir, "next", "--peek")
}

// TestNext_AnEmptyEpicBoundFileAnswersNoTasksYet: tp next --epic b on b's
// only bound file, which holds no task, answers as plain tp next does.
func TestNext_AnEmptyEpicBoundFileAnswersNoTasksYet(t *testing.T) {
	t.Parallel()
	dir := epicFileRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b1.md"), []byte("# b1\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b1.tasks.json"), []byte(`{"version":1,"spec":"b1.md","epic":"b","tasks":[]}`), 0o600))
	initGitRepo(t, dir)
	assertNoTasksYet(t, dir, "next", "--epic", "b")
}
