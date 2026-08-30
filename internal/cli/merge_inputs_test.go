package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mergeInputsOf pulls the §8a.4 per-input accounting out of a --merge JSON
// summary, keyed by path so a test asserts on the file it wrote rather than on
// an array position.
func mergeInputsOf(t *testing.T, stdout string) map[string][2]int {
	t.Helper()
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary), "merge summary must be JSON: %s", stdout)

	raw, ok := summary["inputs"]
	require.True(t, ok, "merge summary must carry inputs (§8a.4): %s", stdout)
	entries, ok := raw.([]any)
	require.True(t, ok, "inputs must be an array, never null (§8a.4): %v", raw)

	byPath := make(map[string][2]int, len(entries))
	for _, e := range entries {
		m, ok := e.(map[string]any)
		require.True(t, ok, "each inputs entry is an object: %v", e)
		path, ok := m["path"].(string)
		require.True(t, ok, "inputs entry needs a path: %v", m)
		parsed, ok := m["parsed"].(float64)
		require.True(t, ok, "inputs entry needs parsed: %v", m)
		skipped, ok := m["skipped"].(float64)
		require.True(t, ok, "inputs entry needs skipped: %v", m)
		byPath[path] = [2]int{int(parsed), int(skipped)}
	}
	return byPath
}

const (
	goodFinding1 = `{"role":"implementer","severity":"high","class":"gap","location":"§1","finding":"missing bound","suggestion":"state it"}`
	goodFinding2 = `{"role":"implementer","severity":"low","class":"nit","location":"§2","finding":"wording","suggestion":"reword"}`
	goodFinding3 = `{"role":"tester","severity":"medium","class":"test","location":"§3","finding":"no boundary test","suggestion":"add one"}`

	goodAuditRow1 = `{"role":"go-safety","item_id":"a","status":"PASS"}`
	goodAuditRow2 = `{"role":"go-safety","item_id":"b","status":"FAIL","finding":"unchecked error"}`
	goodAuditRow3 = `{"role":"spec-coverage","item_id":"c","status":"PASS"}`
)

// TestReviewMerge_InputsReportPerFileCounts covers test 32 for the review
// merge: the payload names every input with its own parsed and skipped counts,
// and a file that lost some lines but kept others still exits 0.
func TestReviewMerge_InputsReportPerFileCounts(t *testing.T) {
	dir := t.TempDir()

	f1 := writeFindingsFile(t, dir, "f1.ndjson", []string{
		goodFinding1,
		`{"severity":"high","finding":"no location at all"}`, // incomplete
		goodFinding2,
		`{not json`, // malformed
	})
	f2 := writeFindingsFile(t, dir, "f2.ndjson", []string{goodFinding3})

	stdout, stderr, code := runTPMerge(t, dir, "review", "--merge", "--json", f1, f2)
	require.Equal(t, 0, code, "an input that parsed at least one line is not a dropped role: %s", stderr)

	inputs := mergeInputsOf(t, stdout)
	assert.Len(t, inputs, 2, "one entry per input file")
	assert.Equal(t, [2]int{2, 2}, inputs[f1], "f1: two findings parsed, two lines skipped")
	assert.Equal(t, [2]int{1, 0}, inputs[f2], "f2: one finding parsed, nothing skipped")
}

