package cli_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initGitRepo initializes a git repo in dir with an initial commit.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "add", "-A"},
		{"git", "commit", "-m", "initial", "--allow-empty"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git command %v failed: %s", args, string(out))
	}
}

// setupCommitProject creates a project dir with spec, task file, git repo, and a task.
func setupCommitProject(t *testing.T, taskID string) string {
	t.Helper()
	dir := t.TempDir()

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n\nSome content.\n"), 0o600))

	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	taskJSON := `{"id":"` + taskID + `","title":"Test task","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"` + taskID + ` acceptance met","source_sections":["s1"]}`
	_, _, code = runTP(t, dir, "add", taskJSON)
	require.Equal(t, 0, code)

	initGitRepo(t, dir)
	return dir
}

func TestCommitBasic(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	// Create a dirty file
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "commit", "t1", "implemented the thing")
	require.Equal(t, 0, code, "commit should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "t1", result["id"])
	assert.NotEmpty(t, result["sha"])
	assert.Contains(t, result["message"].(string), "feat(t1)")
	assert.Contains(t, result["message"].(string), "Test task")
}

func TestCommitStructuredMessage(t *testing.T) {
	dir := setupCommitProject(t, "auth-model")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "model.go"), []byte("package model\n"), 0o600))

	_, _, code := runTP(t, dir, "commit", "auth-model", "Auth model created")
	require.Equal(t, 0, code)

	// Check git log for structured message
	cmd := exec.Command("git", "log", "-1", "--format=%B")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)

	msg := string(out)
	assert.Contains(t, msg, "feat(auth-model): Test task")
	assert.Contains(t, msg, "Auth model created")
	assert.Contains(t, msg, "Task: auth-model")
	assert.Contains(t, msg, "Acceptance:")
}

func TestCommitRecordsSHA(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.go"), []byte("package f\n"), 0o600))
	_, _, code := runTP(t, dir, "commit", "t1")
	require.Equal(t, 0, code)

	// Show task — should have commit_sha set
	stdout, _, _ := runTP(t, dir, "show", "t1")
	var show map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &show))
	assert.NotNil(t, show["commit_sha"], "commit should record SHA on task")
}

func TestCommitImplicitClaim(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "file.go"), []byte("package f\n"), 0o600))

	// Task is open — commit should auto-claim to wip
	_, _, code := runTP(t, dir, "commit", "t1")
	require.Equal(t, 0, code)

	stdout, _, _ := runTP(t, dir, "show", "t1")
	var show map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &show))
	assert.Equal(t, "wip", show["status"])
	assert.NotNil(t, show["started_at"])
}

func TestCommitNoChanges(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	// No dirty files — should fail
	_, stderr, code := runTP(t, dir, "commit", "t1")
	assert.Equal(t, 4, code, "commit with no changes should fail")
	assert.Contains(t, stderr, "no changes")
}

func TestCommitSelectiveFiles(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	// Create two files
	require.NoError(t, os.WriteFile(filepath.Join(dir, "include.go"), []byte("package a\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "exclude.txt"), []byte("skip\n"), 0o600))

	// Only commit .go files
	_, _, code := runTP(t, dir, "commit", "t1", "--files", "*.go")
	require.Equal(t, 0, code)

	// exclude.txt should still be untracked
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "exclude.txt", "non-matching file should remain unstaged")
}

func TestCommitDoneTask(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o600))
	runTP(t, dir, "claim", "t1")
	runTP(t, dir, "close", "t1", "t1 acceptance met completely")

	// Try commit on done task
	_, stderr, code := runTP(t, dir, "commit", "t1")
	assert.NotEqual(t, 0, code)
	assert.Contains(t, stderr, "already done")
}

func TestDoneAutoCommit(t *testing.T) {
	dir := setupCommitProject(t, "t1")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "f.go"), []byte("package f\n"), 0o600))

	// tp done with --auto-commit
	stdout, stderr, code := runTP(t, dir, "done", "t1", "t1 acceptance met completely", "--gate-passed", "--auto-commit")
	require.Equal(t, 0, code, "done --auto-commit should succeed: %s", stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "t1", result["closed"])

	// Verify commit was made
	cmd := exec.Command("git", "log", "-1", "--format=%s")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "feat(t1)")

	// Verify SHA recorded
	stdout, _, _ = runTP(t, dir, "show", "t1")
	var show map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &show))
	assert.NotNil(t, show["commit_sha"])
}

func TestCommitSequentialTasks(t *testing.T) {
	dir := t.TempDir()

	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)

	addTask(t, dir, `{"id":"t1","title":"First","depends_on":[],"estimate_minutes":5,"acceptance":"First done","source_sections":["s1"]}`)
	addTask(t, dir, `{"id":"t2","title":"Second","depends_on":["t1"],"estimate_minutes":5,"acceptance":"Second done","source_sections":["s1"]}`)

	initGitRepo(t, dir)

	// Task 1: implement + commit
	require.NoError(t, os.WriteFile(filepath.Join(dir, "first.go"), []byte("package first\n"), 0o600))
	stdout, _, code := runTP(t, dir, "commit", "t1", "first task done")
	require.Equal(t, 0, code)
	var r1 map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &r1))
	sha1 := r1["sha"].(string)

	// Task 1: close
	runTP(t, dir, "done", "t1", "First done completely", "--gate-passed", "--commit", sha1)

	// Task 2: implement + commit (t2 depends on t1, now unblocked)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "second.go"), []byte("package second\n"), 0o600))
	stdout, _, code = runTP(t, dir, "commit", "t2", "second task done")
	require.Equal(t, 0, code)
	var r2 map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &r2))
	sha2 := r2["sha"].(string)

	// Verify different SHAs
	assert.NotEqual(t, sha1, sha2)

	// Verify git log has 2 task commits (+ initial)
	cmd := exec.Command("git", "log", "--oneline")
	cmd.Dir = dir
	out, err := cmd.Output()
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	assert.GreaterOrEqual(t, len(lines), 3, "should have initial + 2 task commits")
	assert.Contains(t, string(out), "feat(t1)")
	assert.Contains(t, string(out), "feat(t2)")
}
