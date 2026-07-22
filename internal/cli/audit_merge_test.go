package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAuditMerge_DedupAndStatusSummary covers tp audit --merge: it combines
// per-role audit-result files, drops exact (role, item_id) duplicates, skips
// blank lines, and reports a status/role breakdown with a non-PASS findings count.
func TestAuditMerge_DedupAndStatusSummary(t *testing.T) {
	dir := t.TempDir()
	r1 := filepath.Join(dir, "r1.ndjson")
	r2 := filepath.Join(dir, "r2.ndjson")
	require.NoError(t, os.WriteFile(r1,
		[]byte(`{"role":"spec-coverage","item_id":"a","status":"PASS"}`+"\n"+
			`{"role":"spec-coverage","item_id":"b","status":"FAIL"}`+"\n"+
			`{"role":"spec-coverage","item_id":"a","status":"PASS"}`+"\n"), 0o600)) // 3rd is a dup of item a
	require.NoError(t, os.WriteFile(r2,
		[]byte(`{"role":"go-safety","item_id":"a","status":"PARTIAL"}`+"\n"+
			"\n"+ // blank line, skipped
			`{"role":"go-safety","item_id":"c","status":"PASS"}`+"\n"), 0o600))

	out := filepath.Join(dir, "merged.ndjson")
	stdout, stderr, code := runTP(t, dir, "audit", "--merge", r1, r2, "-o", out)
	require.Equal(t, 0, code, "merge failed: %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(4), summary["merged_count"], "3 spec-coverage minus 1 dup, plus 2 go-safety = 4 unique")
	assert.Equal(t, float64(1), summary["duplicates_removed"])
	assert.Equal(t, float64(2), summary["findings"], "one FAIL plus one PARTIAL are non-PASS")

	byStatus := summary["by_status"].(map[string]any)
	assert.Equal(t, float64(2), byStatus["PASS"])
	assert.Equal(t, float64(1), byStatus["FAIL"])
	assert.Equal(t, float64(1), byStatus["PARTIAL"])

	data, err := os.ReadFile(out)
	require.NoError(t, err)
	count := 0
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	assert.Equal(t, 4, count, "the merged file holds exactly the 4 unique rows")
}
