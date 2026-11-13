package cli_test

import (
	"fmt"
	"os"
	"path/filepath"
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

// TestAuditUniverse_NoRangeNoOutsideSide: with no tag and a clean tree the
// auto-detected diff is empty by construction, so every task file used to be
// reported "outside the diff" on every run. Without a committed range that
// side says nothing.
func TestAuditUniverse_NoRangeNoOutsideSide(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	sha := commitFile(t, dir, "calc.go", "calc")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	assert.NotContains(t, stderr, "outside the auto-detected diff")
	assert.Nil(t, noticeOfKind(auditNotices(t, stdout), "tasks-differ"))
}

// TestAuditUniverse_TasksDifferIsBounded: each side of tasks-differ names at
// most twenty files and carries its full count.
func TestAuditUniverse_TasksDifferIsBounded(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	git(t, dir, "tag", "v0")
	sha := commitFile(t, dir, "calc.go", "calc")
	for i := range 25 {
		commitFile(t, dir, fmt.Sprintf("stray%02d.go", i), "stray")
	}
	closeTaskFile(t, dir, doneTaskJSON(t, sha))

	stdout, stderr, code := runTP(t, dir, "audit", specPath, "--affected-from-tasks")
	require.Equal(t, 0, code, "stderr: %s", stderr)
	n := noticeOfKind(auditNotices(t, stdout), "tasks-differ")
	require.NotNil(t, n)
	assert.Equal(t, float64(25), n["untasked"], "the full count")
	assert.Equal(t, float64(0), n["outside"], "a committed range exists, so the other side is counted")
	assert.Len(t, n["files"], 20, "and at most twenty names")
}

// TestAuditUniverse_MergeOnlyIsBoundedAndCommittedOnly: merge-only names at
// most twenty files with the full count, and compares the committed range
// alone, so a merged file also edited in the working tree is still named.
func TestAuditUniverse_MergeOnlyIsBoundedAndCommittedOnly(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	git(t, dir, "tag", "v0")
	git(t, dir, "branch", "develop")
	git(t, dir, "checkout", "-q", "-b", "feature")
	commitFile(t, dir, "Mine.go", "mine")
	git(t, dir, "checkout", "-q", "develop")
	for i := range 25 {
		commitFile(t, dir, fmt.Sprintf("Foreign%02d.go", i), "foreign")
	}
	git(t, dir, "checkout", "-q", "feature")
	git(t, dir, "merge", "-q", "--no-edit", "develop")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "Foreign00.go"), []byte("package main\n// edited\n"), 0o600))

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, "stderr: %s", stderr)
	n := noticeOfKind(auditNotices(t, stdout), "merge-only-files")
	require.NotNil(t, n)
	assert.Equal(t, float64(25), n["merge_only"], "all 25, the edited one included")
	files := n["files"].([]any)
	assert.Len(t, files, 20)
	assert.Equal(t, "Foreign00.go", files[0], "the working-tree edit does not take a merged file out of the committed comparison")
}
