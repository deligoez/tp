package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// foundTaskJSON is a found task: origin in place of source_sections, with
// source_lines spanning the whole spec (spec/1.31.0.md §3).
const foundTaskJSON = `{"id":"f1","title":"Found f1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"f1 acceptance met","source_lines":"1-12","origin":{"finder_task":"t1","location":"a.go:1"}}`

// coverageFigures returns the task file's coverage block and the coverage
// and line-coverage findings tp validate reports.
func coverageFigures(t *testing.T, dir string) (block any, findings []any) {
	t.Helper()
	out, stderr, code := runTP(t, dir, "validate")
	require.Equal(t, 0, code, "validate: %s", stderr)
	var res map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &res), out)
	all, _ := res["findings"].([]any) // absent when validate finds nothing
	for _, f := range all {
		switch f.(map[string]any)["rule"] {
		case "coverage", "line-coverage":
			findings = append(findings, f)
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, "spec.tasks.json"))
	require.NoError(t, err)
	var tf map[string]any
	require.NoError(t, json.Unmarshal(data, &tf))
	return tf["coverage"], findings
}

// TestSpecCoverageIsTheSameAfterAFoundTaskIsAdded: a found task whose
// source_lines would cover the section no spec task anchors leaves spec
// coverage's figures as they were.
func TestSpecCoverageIsTheSameAfterAFoundTaskIsAdded(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n\n## A\n\nText a.\n\n## B\n\nText b.\n"), 0o600))
	_, _, code := runTP(t, dir, "init", "spec.md")
	require.Equal(t, 0, code)
	_, stderr, code := runTP(t, dir, "add", `{"id":"t1","title":"Task t1","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"t1 acceptance met","source_sections":["## A"]}`)
	require.Equal(t, 0, code, "add t1: %s", stderr)

	beforeBlock, beforeFindings := coverageFigures(t, dir)
	require.NotEmpty(t, beforeFindings, "section B is uncovered before the found task")

	_, stderr, code = runTP(t, dir, "add", foundTaskJSON)
	require.Equal(t, 0, code, "add f1: %s", stderr)

	afterBlock, afterFindings := coverageFigures(t, dir)
	assert.Equal(t, beforeBlock, afterBlock)
	assert.Equal(t, beforeFindings, afterFindings)
}

// TestTaskAudit_AFoundTaskIsReachedThroughItsFiles: a found task has no spec
// section, yet the task audit lists the files its commit touched.
func TestTaskAudit_AFoundTaskIsReachedThroughItsFiles(t *testing.T) {
	t.Parallel()
	dir := taskAuditProject(t, "t1")
	_, stderr, code := runTP(t, dir, "add", foundTaskJSON)
	require.Equal(t, 0, code, "add f1: %s", stderr)
	commitAll(t, dir)
	closeWith(t, dir, "t1", commitFiles(t, dir, "a.go"))
	closeWith(t, dir, "f1", commitFiles(t, dir, "found.go"))

	ta := emitTaskAudit(t, dir)["task_audit"].(map[string]any)
	assert.Equal(t, []any{"a.go", "found.go"}, ta["files"])
	assert.Equal(t, []any{"t1", "f1"}, ta["tasks"])
}
