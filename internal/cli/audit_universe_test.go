package cli_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergedBranchRepo builds the field report's shape: feature commits Mine.go,
// develop commits Foreign.go, feature merges develop, and develop-stale is
// left at develop's commit before Foreign.go.
func mergedBranchRepo(t *testing.T) (dir, specPath string) {
	t.Helper()
	dir, specPath = newAuditRepo(t)
	git(t, dir, "branch", "develop-stale")
	git(t, dir, "branch", "develop")
	git(t, dir, "checkout", "-q", "-b", "feature")
	commitFile(t, dir, "Mine.go", "mine")
	git(t, dir, "checkout", "-q", "develop")
	commitFile(t, dir, "Foreign.go", "foreign")
	git(t, dir, "checkout", "-q", "feature")
	git(t, dir, "merge", "-q", "--no-edit", "develop")
	return dir, specPath
}

// TestAuditUniverse_MergeOnlyFilesAreNamed: decision 5(a). Against a base
// older than the merged branch, Foreign.go is in the range only through the
// merge, so `--first-parent` sees fewer files than the audit does. The notice
// names it on stderr and in the payload.
func TestAuditUniverse_MergeOnlyFilesAreNamed(t *testing.T) {
	t.Parallel()
	dir, specPath := mergedBranchRepo(t)

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--base", "develop-stale")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "1 file reaches develop-stale...HEAD only through a merge")
	n := noticeOfKind(auditNotices(t, stdout), "merge-only-files")
	require.NotNil(t, n, "and the payload says so")
	assert.Equal(t, []any{"Foreign.go"}, n["files"])
}

// TestAuditUniverse_NoMergeOnlyNoticeAgainstTheMergedBase: against the
// branch that was merged, nothing reaches the range through the merge alone.
func TestAuditUniverse_NoMergeOnlyNoticeAgainstTheMergedBase(t *testing.T) {
	t.Parallel()
	dir, specPath := mergedBranchRepo(t)

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--base", "develop")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, stderr, "through a merge")
	assert.Nil(t, noticeOfKind(auditNotices(t, stdout), "merge-only-files"))
}

// TestAuditUniverse_TasksAndDiffDiffer: decision 5(b). --affected-from-tasks
// audits the files the plan's tasks touched; stray.go changed on the branch
// and belongs to no task, and it used to be dropped without a word.
func TestAuditUniverse_TasksAndDiffDiffer(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	git(t, dir, "tag", "v0")
	sha := commitFile(t, dir, "calc.go", "calc")
	commitFile(t, dir, "stray.go", "stray")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.Contains(t, stderr, "1 changed file belongs to no task")
	n := noticeOfKind(auditNotices(t, stdout), "tasks-differ")
	require.NotNil(t, n, "and the payload says so")
	assert.Equal(t, []any{"stray.go"}, n["files"])
}

// TestAuditUniverse_TasksCoverTheDiff: when every changed file belongs to a
// task and every task file is in the diff, nothing is said.
func TestAuditUniverse_TasksCoverTheDiff(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	git(t, dir, "tag", "v0")
	sha := commitFile(t, dir, "calc.go", "calc")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, stderr, "belongs to no task")
	assert.Nil(t, noticeOfKind(auditNotices(t, stdout), "tasks-differ"))
}
