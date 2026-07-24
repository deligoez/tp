package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNextBrief_ClaimsAndReturnsBrief: tp next --brief claims the next ready
// task (open→wip) and returns the brief for it in one call (§9.2).
func TestNextBrief_ClaimsAndReturnsBrief(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"),
		[]byte("# S\n\n## 1. A\nDo a thing.\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"a","title":"Task A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"UNIT-ACCEPTANCE","source_sections":["## 1. A"]}`)

	out, _, code := runTP(t, dir, "next", "--brief")
	require.Equal(t, 0, code)

	// §4.4: the five brief parts in fixed key order — it is the brief, not the
	// plain tp next task object.
	assert.Equal(t, []string{"identity", "task", "prior_work", "close", "scope"}, topKeys(t, out))
	b := parseJSON(t, out)
	assert.Equal(t, "a", b["task"].(map[string]any)["id"])
	assert.Equal(t, "UNIT-ACCEPTANCE", b["task"].(map[string]any)["acceptance"])

	// §9.2: the claim happened — the task is now wip.
	showOut, _, _ := runTP(t, dir, "show", "a")
	assert.Equal(t, "wip", parseJSON(t, showOut)["status"], "next --brief must claim the task")
}

// TestNextBrief_ComposesWithJSON: under JSON the brief is the structured object
// with the close recipe and scope fence (§9.2 composes with --json).
func TestNextBrief_ComposesWithJSON(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"a","title":"Task A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["s"]}`)

	out, _, code := runTP(t, dir, "next", "--brief", "--json")
	require.Equal(t, 0, code)
	b := parseJSON(t, out)
	assert.Contains(t, b["close"].(string), "tp done")
	assert.Contains(t, b["scope"].(string), "refactor")
}

// TestNextBrief_BriefAndMinimalMutuallyExclusive: §9.2 — one strips context, the
// other assembles it, so together they are a usage error (exit 2).
func TestNextBrief_BriefAndMinimalMutuallyExclusive(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# S\n"), 0o600))
	runTP(t, dir, "init", "spec.md")
	runTP(t, dir, "add", `{"id":"a","title":"A","status":"open","depends_on":[],"estimate_minutes":5,"acceptance":"One.","source_sections":["s"]}`)

	_, _, code := runTP(t, dir, "next", "--brief", "--minimal")
	assert.Equal(t, 2, code)
}
