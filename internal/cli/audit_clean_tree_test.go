package cli_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// noticeKinds returns the kind of every notice in an audit payload.
func noticeKinds(t *testing.T, payload map[string]any) []string {
	t.Helper()
	raw, _ := payload["notices"].([]any)
	kinds := make([]string, 0, len(raw))
	for _, n := range raw {
		if m, ok := n.(map[string]any); ok {
			if k, ok := m["kind"].(string); ok {
				kinds = append(kinds, k)
			}
		}
	}
	return kinds
}

// TestAuditCleanTreeFallsBackToTaskCommits: with no tag and every task
// committed, auto-detection finds nothing, and a bare `tp audit` audits the
// files the done tasks' commits touched instead of exiting 4
// (spec/backlog/a-clean-tree-audits-the-task-commits.md).
func TestAuditCleanTreeFallsBackToTaskCommits(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	sha1 := commitFile(t, dir, "a.go", "add a")
	sha2 := commitFile(t, dir, "b.go", "add b")
	closeTaskFile(t, dir, doneTaskJSON(t, sha1, sha2))

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.ElementsMatch(t, []string{"a.go", "b.go"}, toStringSlice(result["files"]))
	assert.Contains(t, noticeKinds(t, result), "tasks-fallback")
	assert.Contains(t, stderr, "no changed files", "the fallback says why it ran")
	assert.Contains(t, stderr, "auditing the 2 files")
	assertNoMeasuredChurn(t, result)
}

// assertNoMeasuredChurn: no comparison covers the fallback set, so no role
// is told a churn figure for its files.
func assertNoMeasuredChurn(t *testing.T, result map[string]any) {
	t.Helper()
	prompts, _ := result["prompts"].([]any)
	require.NotEmpty(t, prompts)
	for _, p := range prompts {
		pm, _ := p.(map[string]any)
		text, _ := pm["prompt"].(string)
		assert.NotContains(t, text, "+0/-0", "role %v", pm["role"])
		files, _ := pm["affected_files"].([]any)
		for _, f := range files {
			fm, _ := f.(map[string]any)
			assert.Empty(t, fm["diff_summary"], "role %v file %v", pm["role"], fm["path"])
		}
	}
}

// TestAuditCleanTreeFallbackKeepsTheAutoDetectCap: the fallback set is capped
// like auto-detection, and the cut is said on stderr and in the payload
// without the two notices disagreeing about the count.
func TestAuditCleanTreeFallbackKeepsTheAutoDetectCap(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	names := make([]string, 0, 51)
	for i := range 51 {
		name := fmt.Sprintf("g%02d.go", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
		names = append(names, name)
	}
	git(t, dir, append([]string{"add"}, names...)...)
	git(t, dir, "commit", "-m", "add fifty-one")
	closeTaskFile(t, dir, doneTaskJSON(t, gitOut(t, dir, "rev-parse", "HEAD")))

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Len(t, toStringSlice(result["files"]), 50)
	assert.Contains(t, noticeKinds(t, result), "auto-detect-cap")
	assert.Contains(t, stderr, "auditing first 50", "the cut is printed, not only kept in the payload")
	assert.Contains(t, stderr, "auditing 50 of the 51 files")
}

// TestAuditCleanTreeFallbackAtTheCapIsNotCut: exactly maxAutoDetectFiles
// files is the whole set, so nothing claims a cut.
func TestAuditCleanTreeFallbackAtTheCapIsNotCut(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	names := make([]string, 0, 50)
	for i := range 50 {
		name := fmt.Sprintf("h%02d.go", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
		names = append(names, name)
	}
	git(t, dir, append([]string{"add"}, names...)...)
	git(t, dir, "commit", "-m", "add fifty")
	closeTaskFile(t, dir, doneTaskJSON(t, gitOut(t, dir, "rev-parse", "HEAD")))

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Len(t, toStringSlice(result["files"]), 50)
	assert.NotContains(t, stderr, "auditing first")
	assert.NotContains(t, noticeKinds(t, result), "auto-detect-cap")
}

// TestAuditTaggedHeadFallsBackToTaskCommits: on a tagged repository whose
// HEAD is the tag, the committed range is empty too.
func TestAuditTaggedHeadFallsBackToTaskCommits(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	sha := commitFile(t, dir, "a.go", "add a")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))
	git(t, dir, "tag", "v1.0.0")

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, []string{"a.go"}, toStringSlice(result["files"]))
}

// TestAuditSkippedTypesOnlyStillExits4: a tree whose only change is a file
// type the audit skips is not a clean tree, so its selection is unchanged.
func TestAuditSkippedTypesOnlyStillExits4(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	sha := commitFile(t, dir, "a.go", "add a")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "notes.md"), []byte("# notes\n"), 0o600))
	git(t, dir, "add", "notes.md")

	_, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 4, code, stderr)
	assert.Contains(t, stderr, "only skipped types changed")
}

// TestAuditCleanTreeFallbackIsCutLikeAutoDetection: the fallback set is not a
// named list, so each code role's list is cut to its usual size rather than
// handed whole to one prompt.
func TestAuditCleanTreeFallbackIsCutLikeAutoDetection(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	names := make([]string, 0, 14)
	for i := range 14 {
		name := fmt.Sprintf("f%02d.go", i)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("package main\n"), 0o600))
		names = append(names, name)
	}
	git(t, dir, append([]string{"add"}, names...)...)
	git(t, dir, "commit", "-m", "add fourteen")
	closeTaskFile(t, dir, doneTaskJSON(t, gitOut(t, dir, "rev-parse", "HEAD")))

	stdout, stderr, code := runTP(t, dir, "audit", specPath)
	require.Equal(t, 0, code, stderr)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	kinds := noticeKinds(t, result)
	assert.Contains(t, kinds, "code-files-cut")
	assert.NotContains(t, kinds, "named-list-one-prompt")
}

// TestAuditEmptyBaseRangeStillExits4: --base names the comparison, so an empty
// range is the caller's answer and the fallback does not replace it.
func TestAuditEmptyBaseRangeStillExits4(t *testing.T) {
	t.Parallel()
	dir, specPath := newAuditRepo(t)
	sha := commitFile(t, dir, "a.go", "add a")
	closeTaskFile(t, dir, doneTaskJSON(t, sha))

	_, stderr, code := runTP(t, dir, "audit", specPath, "--base", "HEAD")
	require.Equal(t, 4, code)

	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(stderr), &payload))
	assert.Equal(t, []string{"a.go"}, toStringSlice(payload["suggested_files"]))
}
