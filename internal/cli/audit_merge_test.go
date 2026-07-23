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


// TestAuditMerge_EmptyInputSucceeds covers §3.3 row 2 for the audit phase: a
// present-but-empty input file succeeds (exit 0), creates a zero-byte -o file,
// and reports merged_count 0.
func TestAuditMerge_EmptyInputSucceeds(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(empty, []byte{}, 0o600))

	out := filepath.Join(dir, "merged.ndjson")
	stdout, stderr, code := runTP(t, dir, "audit", "--merge", empty, "-o", out)
	require.Equal(t, 0, code, "empty input is a clean result (§3.3): %s", stderr)

	info, err := os.Stat(out)
	require.NoError(t, err, "-o file must be created even when empty")
	assert.Equal(t, int64(0), info.Size(), "-o file must be zero bytes")

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(0), summary["merged_count"])
}

// TestAuditMerge_OnlyMalformedSucceeds covers §3.3 row 3 for the audit phase:
// files holding only malformed/incomplete lines succeed (exit 0), create an
// empty -o file, report merged_count 0, and emit a stderr warning per skipped
// line.
func TestAuditMerge_OnlyMalformedSucceeds(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.ndjson")
	require.NoError(t, os.WriteFile(bad,
		[]byte("not json at all\n"+ // malformed
			`{"role":"go-safety","item_id":"a"}`+"\n"), // incomplete: no status
		0o600))

	out := filepath.Join(dir, "merged.ndjson")
	stdout, stderr, code := runTP(t, dir, "audit", "--merge", bad, "-o", out)
	require.Equal(t, 0, code, "only-malformed input is a clean result (§3.3): %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(0), summary["merged_count"])

	info, err := os.Stat(out)
	require.NoError(t, err)
	assert.Equal(t, int64(0), info.Size(), "no rows survive -> empty output file")

	assert.Contains(t, stderr, "warning: skipping malformed")
	assert.Contains(t, stderr, "warning: skipping incomplete")
}

// TestAuditMerge_NoInputFilesExit2 covers §3.3 row 6: no input files given
// exits 2.
func TestAuditMerge_NoInputFilesExit2(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTPMerge(t, dir, "audit", "--merge")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "at least 1 file required for merge")
}

// TestAuditMerge_MissingFileExit3 covers §3.3 rows 4-5: a missing/unreadable
// input file exits 3.
func TestAuditMerge_MissingFileExit3(t *testing.T) {
	dir := t.TempDir()
	_, stderr, code := runTPMerge(t, dir, "audit", "--merge", filepath.Join(dir, "nope.ndjson"))
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "file not found")
}
