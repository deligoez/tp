package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDone_OutOfScopeLineAcceptedAndPreserved verifies acceptance §15.9 / §7.2:
// a closure reason carrying a trailing "Out of scope:" line is accepted by
// tp done (the line is not counted as an evidence line) and preserved verbatim
// in closed_reason.
func TestDone_OutOfScopeLineAcceptedAndPreserved(t *testing.T) {
	dir := setupCloseStrategyProject(t, "builtin")
	// A two-criterion task exercises evidence-line counting: the Out of scope:
	// line must not count toward the two required evidence lines.
	runTP(t, dir, "add", `{"id":"two","title":"Two","depends_on":[],"estimate_minutes":5,"acceptance":"First part. Second part.","source_sections":["s1"]}`)

	reason := "- first done at internal/cli/done.go:279\n- second verified at internal/cli/report.go\nOut of scope: typo spotted in README.md"
	_, stderr, code := runTP(t, dir, "done", "two", "--gate-passed", "--", reason)
	require.Equal(t, 0, code, "tp done should accept a trailing Out of scope: line: %s", stderr)

	shown := showTask(t, dir, "two")
	require.NotNil(t, shown["closed_reason"], "closed_reason must be set")
	assert.Equal(t, reason, shown["closed_reason"], "closed_reason preserves the Out of scope: line verbatim")
}

// TestReport_SurfacesOutOfScopeNote verifies §7.2: tp report surfaces the
// out-of-scope note a closing unit recorded, so it reaches a human rather than
// dying in a context window. The note survives --compact for the same reason.
func TestReport_SurfacesOutOfScopeNote(t *testing.T) {
	dir := setupProject(t)

	now := time.Now().UTC()
	start := now.Add(-2 * time.Minute)

	taskFilePath := filepath.Join(dir, "spec.tasks.json")
	content := `{
  "version": 1,
  "spec": "spec.md",
  "tasks": [
    {
      "id": "task-oos",
      "title": "OOS task",
      "status": "done",
      "depends_on": [],
      "estimate_minutes": 5,
      "acceptance": "task-oos acceptance",
      "source_sections": ["s1"],
      "started_at": "` + start.Format(time.RFC3339Nano) + `",
      "closed_at": "` + now.Format(time.RFC3339Nano) + `",
      "closed_reason": "- evidence line one\n- evidence line two\nOut of scope: drift in README.md",
      "gate_passed_at": null,
      "commit_sha": null
    }
  ],
  "workflow": {},
  "coverage": {},
  "updated_at": "` + now.Format(time.RFC3339Nano) + `"
}`
	require.NoError(t, os.WriteFile(taskFilePath, []byte(content), 0o600))

	check := func(args ...string) map[string]any {
		t.Helper()
		stdout, _, code := runTP(t, dir, append([]string{"report"}, args...)...)
		require.Equal(t, 0, code)
		var result map[string]any
		require.NoError(t, json.Unmarshal([]byte(stdout), &result))
		tasks := result["tasks"].([]any)
		require.Len(t, tasks, 1)
		return tasks[0].(map[string]any)
	}

	entry := check()
	assert.Equal(t, "drift in README.md", entry["out_of_scope"],
		"report surfaces the out-of-scope note from closed_reason")

	// The note must reach a human even under --compact.
	entry = check("--compact")
	assert.Equal(t, "drift in README.md", entry["out_of_scope"],
		"out_of_scope stays visible under --compact")
}
