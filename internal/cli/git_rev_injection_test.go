package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Revisions reach git from three directions, and guarding only the one that
// tp done writes leaves the other two open: the task file is also written by
// import and add, and --base is typed by the user. git parses any argument
// starting with "-" as an option, so "--output=<path>" makes git write that
// file. These tests pin the guard at every sink.

const injectionSpec = "# S\n\n## 1. A\n\n1. one thing\n2. two thing\n"

func setupInjectionRepo(t *testing.T, victim string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.md"), []byte(injectionSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	tasks := `{"spec":"s.md","tasks":[{"id":"t1","title":"T","estimate_minutes":5,` +
		`"acceptance":"a.","source_sections":["## 1. A"],"depends_on":[],"status":"done",` +
		`"commit_shas":["--output=` + victim + `"]}]}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"), []byte(tasks), 0o600))
	return dir
}

// TestGitRevInjection_StoredCommitSHA: a commit sha written into the task file
// by import or add never reaches git, so neither --affected-from-tasks nor the
// plain run (which falls back to suggestFilesFromTasks) can be used to write a
// file. resolveCommitSHAs guards only tp done, which is why the sink is guarded.
func TestGitRevInjection_StoredCommitSHA(t *testing.T) {
	victim := filepath.Join(t.TempDir(), "written-by-git")
	dir := setupInjectionRepo(t, victim)

	for _, args := range [][]string{
		{"audit", "s.md", "--affected-from-tasks"},
		{"audit", "s.md"},
	} {
		_, _, _ = runTP(t, dir, args...)
		_, err := os.Stat(victim)
		assert.True(t, os.IsNotExist(err), "%v must not make git write %s", args, victim)
	}
}

// TestGitRevInjection_SkippedSHAIsReported: a sha dropped at the sink is not
// dropped silently. Without the warning and the distinguished message, an
// empty file set is reported as "no done task carries commit_shas" — which is
// false when the task does carry one, and sends the caller after the wrong
// problem.
func TestGitRevInjection_SkippedSHAIsReported(t *testing.T) {
	victim := filepath.Join(t.TempDir(), "written-by-git")
	dir := setupInjectionRepo(t, victim)

	_, stderr, code := runTP(t, dir, "audit", "s.md", "--affected-from-tasks")
	assert.Equal(t, 4, code)
	assert.Contains(t, stderr, "was skipped; git would read it as an option")
	assert.Contains(t, stderr, "every recorded commit sha was skipped as unusable")
	assert.NotContains(t, stderr, "no done task carries commit_shas",
		"a task that carries a rejected sha must not be reported as carrying none")
}

// TestGitRevInjection_BaseFlag: a --base git would read as an option is
// rejected up front with exit 2, rather than silently ignored by the helpers
// that only build diff stats — a silent drop would look like a successful run
// against the wrong base.
func TestGitRevInjection_BaseFlag(t *testing.T) {
	victim := filepath.Join(t.TempDir(), "written-by-git")
	dir := setupInjectionRepo(t, victim)

	_, stderr, code := runTP(t, dir, "audit", "s.md", "--affected-files", "code.go", "--base", "--output="+victim)
	assert.Equal(t, 2, code, "an option-lookalike --base is a usage error")
	assert.Contains(t, stderr, "invalid --base")

	_, err := os.Stat(victim)
	assert.True(t, os.IsNotExist(err), "git never ran with the option-lookalike base")
}

// TestAuditTasksOf_NoWarningWhenTaskFileAbsent: model.ReadTaskFile wraps its
// error with %w and os.IsNotExist does not unwrap, so a guard written with
// os.IsNotExist never fires and the warning prints on every ordinary run of a
// spec that simply has no task file. errors.Is is the working form.
func TestAuditTasksOf_NoWarningWhenTaskFileAbsent(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.md"), []byte(injectionSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "s.md", "--affected-files", "code.go")
	assert.Equal(t, 0, code)
	assert.NotContains(t, stderr, "cannot read task file",
		"a spec with no adjacent task file is the normal case, not a warning")
}

// TestAuditTasksOf_WarnsOnCorruptTaskFile: the other half of the same guard —
// a task file that exists but cannot be parsed must be reported, because
// callers build user-facing claims on the empty result.
func TestAuditTasksOf_WarnsOnCorruptTaskFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.md"), []byte(injectionSpec), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "s.tasks.json"), []byte("{not json"), 0o600))

	_, stderr, _ := runTP(t, dir, "audit", "s.md", "--affected-files", "code.go")
	assert.Contains(t, stderr, "cannot read task file",
		"a corrupt task file is a real error and must not be silently treated as empty")
}
