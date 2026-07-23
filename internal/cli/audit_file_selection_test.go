package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gitOut runs a git command in dir and returns its trimmed stdout.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2020-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2020-01-01T00:00:00Z")
	out, err := cmd.Output()
	require.NoError(t, err, "git %v: %s", args, out)
	return strings.TrimSpace(string(out))
}

// commitFile writes a Go source file name in dir, commits it, and returns the sha.
func commitFile(t *testing.T, dir, name, msg string) string {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
	git(t, dir, "add", name)
	git(t, dir, "commit", "-m", msg)
	return gitOut(t, dir, "rev-parse", "HEAD")
}

// writeTaskFileRaw writes a spec-adjacent task file from a raw tasks-array JSON.
func writeTaskFileRaw(t *testing.T, dir, tasksArrayJSON string) {
	t.Helper()
	data := `{"version":1,"spec":"spec.md","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","workflow":{},"coverage":{"total_sections":0,"mapped_sections":0,"context_only":[],"unmapped":[]},"tasks":` +
		tasksArrayJSON + `}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.tasks.json"), []byte(data), 0o600))
}

// newAuditRepo creates a git repo with spec.md and an empty task file, committed
// so the working tree starts clean.
func newAuditRepo(t *testing.T) (dir, specPath string) {
	t.Helper()
	dir = t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\nEmpty.\n"), 0o600))
	writeTaskFileRaw(t, dir, `[]`)
	initGitRepo(t, dir)
	return dir, specPath
}

// doneTaskJSON builds a single done task (id t1) carrying the given commit_shas.
func doneTaskJSON(t *testing.T, shas ...string) string {
	t.Helper()
	parts := make([]string, len(shas))
	for i, s := range shas {
		parts[i] = fmt.Sprintf("%q", s)
	}
	return fmt.Sprintf(
		`[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":[],"commit_shas":[%s],"closed_at":"2020-01-01T00:00:00Z","gate_passed_at":"2020-01-01T00:00:00Z"}]`,
		strings.Join(parts, ","))
}

func toStringSlice(v any) []string {
	arr, _ := v.([]any)
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// closeTaskFile writes the task file and commits it so the tree is clean again.
func closeTaskFile(t *testing.T, dir, tasksArrayJSON string) {
	t.Helper()
	writeTaskFileRaw(t, dir, tasksArrayJSON)
	git(t, dir, "add", "spec.tasks.json")
	git(t, dir, "commit", "-m", "close task")
}

// TestAuditSuggestedFilesFromCommitSHAs: a clean tree with a done task whose
// commit_shas reference commits touching a.go and b.go — audit exits 4 with
// suggested_files naming exactly those paths (§11.1).
func TestAuditSuggestedFilesFromCommitSHAs(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	sha1 := commitFile(t, dir, "a.go", "add a")
	sha2 := commitFile(t, dir, "b.go", "add b")
	closeTaskFile(t, dir, doneTaskJSON(t, sha1, sha2))

	_, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 4, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Equal(t, float64(4), payload["code"])
	assert.ElementsMatch(t, []string{"a.go", "b.go"}, toStringSlice(payload["suggested_files"]))

	hint, _ := payload["hint"].(string)
	assert.Contains(t, hint, "--affected-files")
	assert.Contains(t, hint, "--affected-from-tasks")
}

// TestAuditSuggestedFilesEmpty: a done task with no commit_shas (covered-by /
// legacy close) — audit still exits 4 with suggested_files: [] (§11.1).
func TestAuditSuggestedFilesEmpty(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	closeTaskFile(t, dir, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":[],"closed_at":"2020-01-01T00:00:00Z","gate_passed_at":"2020-01-01T00:00:00Z"}]`)

	_, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 4, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	suggested, ok := payload["suggested_files"].([]any)
	require.True(t, ok, "suggested_files must be a JSON array")
	assert.Empty(t, suggested, "no done-task commits → suggested_files is []")
}

// TestAuditSuggestedFilesTypeFilter: a commit touching .go, .md, and
// .tasks.json — only the .go file survives into suggested_files (§11.1 type
// filtering parity with auto-detection).
func TestAuditSuggestedFilesTypeFilter(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "x.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# doc\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "other.tasks.json"), []byte("{}"), 0o600))
	git(t, dir, "add", "x.go", "doc.md", "other.tasks.json")
	git(t, dir, "commit", "-m", "mixed")
	sha := gitOut(t, dir, "rev-parse", "HEAD")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))

	_, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 4, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Equal(t, []string{"x.go"}, toStringSlice(payload["suggested_files"]),
		"binary/.md/.tasks.json paths are filtered out")
}

// TestAuditSuggestedFilesSurvivesCompact: suggested_files is decision-critical
// and must survive --compact (§8.4).
func TestAuditSuggestedFilesSurvivesCompact(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	sha1 := commitFile(t, dir, "a.go", "add a")
	sha2 := commitFile(t, dir, "b.go", "add b")
	closeTaskFile(t, dir, doneTaskJSON(t, sha1, sha2))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--compact")
	require.Equal(t, 4, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.ElementsMatch(t, []string{"a.go", "b.go"}, toStringSlice(payload["suggested_files"]))
}

// TestAuditAffectedFromTasks: --affected-from-tasks audits the derived file set
// directly (exit 0), so the common post-implementation case needs no manual
// file list (§11.2).
func TestAuditAffectedFromTasks(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	sha1 := commitFile(t, dir, "a.go", "add a")
	sha2 := commitFile(t, dir, "b.go", "add b")
	closeTaskFile(t, dir, doneTaskJSON(t, sha1, sha2))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks")
	require.Equal(t, 0, code, stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.ElementsMatch(t, []string{"a.go", "b.go"}, toStringSlice(result["files"]))
}

// TestAuditAffectedFromTasksEmpty: when the derivation yields nothing,
// --affected-from-tasks exits 4 with suggested_files: [] (§11.1, §11.2).
func TestAuditAffectedFromTasksEmpty(t *testing.T) {
	dir, specPath := newAuditRepo(t)
	closeTaskFile(t, dir, `[{"id":"t1","title":"T","status":"done","depends_on":[],"estimate_minutes":5,"acceptance":"x done","source_sections":[],"closed_at":"2020-01-01T00:00:00Z","gate_passed_at":"2020-01-01T00:00:00Z"}]`)

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks")
	require.Equal(t, 4, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	suggested, ok := payload["suggested_files"].([]any)
	require.True(t, ok)
	assert.Empty(t, suggested)
}

// TestAuditAffectedFromTasksConflicts: --affected-from-tasks is a file-source
// selector exclusive of --affected-files and --base, and rejected by
// --record/--status.
func TestAuditAffectedFromTasksConflicts(t *testing.T) {
	dir, specPath := newAuditRepo(t)

	_, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks", "--affected-files", "x.go")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--affected-from-tasks cannot be combined")

	_, stderr, code = runTP(t, dir, "audit", specPath, "--affected-from-tasks", "--base", "main")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--affected-from-tasks cannot be combined")

	_, stderr, code = runTP(t, dir, "audit", specPath, "--affected-from-tasks", "--status")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "--record/--status reject")
}
