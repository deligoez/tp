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
	t.Parallel()
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
	for l := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	assert.Equal(t, 4, count, "the merged file holds exactly the 4 unique rows")
}

// TestAuditMerge_DisagreeingRowsOnOneItemAreKept: two shards gave one
// (role, item_id) two verdicts. Keeping the first row chose, silently, which
// verdict was the round's: an error-severity FAIL vanished behind a PASS and
// the round recorded clean. Rows collapse only when their verdict (status,
// severity, disposition) agrees; every disagreeing row is kept and the group
// is named under conflicts.
func TestAuditMerge_DisagreeingRowsOnOneItemAreKept(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.ndjson")
	b := filepath.Join(dir, "b.ndjson")
	require.NoError(t, os.WriteFile(a,
		[]byte(`{"role":"go-safety","item_id":"x","status":"PASS","evidence_file":"one.go"}`+"\n"+
			`{"role":"go-safety","item_id":"y","status":"FAIL","severity":"warning"}`+"\n"+
			`{"role":"go-safety","item_id":"z","status":"PASS"}`+"\n"), 0o600))
	require.NoError(t, os.WriteFile(b,
		[]byte(`{"role":"go-safety","item_id":"x","status":"FAIL","severity":"error","evidence_file":"two.go"}`+"\n"+
			`{"role":"go-safety","item_id":"y","status":"FAIL","severity":"error"}`+"\n"+
			`{"role":"go-safety","item_id":"z","status":"PASS","notes":"same verdict, other words"}`+"\n"), 0o600))

	out := filepath.Join(dir, "merged.ndjson")
	stdout, stderr, code := runTP(t, dir, "audit", "--merge", a, b, "-o", out)
	require.Equal(t, 0, code, "merge failed: %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.Equal(t, float64(5), summary["merged_count"], "x and y keep both rows; z collapses")
	assert.Equal(t, float64(1), summary["duplicates_removed"], "only z's agreeing pair collapses")
	assert.Equal(t, float64(3), summary["findings"], "the error FAIL behind the PASS survives")

	conflicts, ok := summary["conflicts"].([]any)
	require.True(t, ok, "disagreeing groups are reported: %s", stdout)
	require.Len(t, conflicts, 2)
	first := conflicts[0].(map[string]any)
	assert.Equal(t, "go-safety", first["role"])
	assert.Equal(t, "x", first["item_id"])
	assert.Equal(t, float64(2), first["rows"])
	assert.Contains(t, stderr, "2 (role, item_id) groups carry disagreeing verdicts")
}

// TestAuditMerge_NoConflictsNoKey: a merge with no disagreeing group carries
// no conflicts key at all.
func TestAuditMerge_NoConflictsNoKey(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	a := filepath.Join(dir, "a.ndjson")
	require.NoError(t, os.WriteFile(a,
		[]byte(`{"role":"go-safety","item_id":"x","status":"PASS"}`+"\n"+
			`{"role":"go-safety","item_id":"x","status":"PASS"}`+"\n"), 0o600))
	stdout, stderr, code := runTP(t, dir, "audit", "--merge", a, "-o", filepath.Join(dir, "m.ndjson"))
	require.Equal(t, 0, code, "merge failed: %s", stderr)
	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	assert.NotContains(t, summary, "conflicts")
}

// TestAuditMerge_EmptyInputSucceeds covers §3.3 row 2 for the audit phase: a
// present-but-empty input file succeeds (exit 0), creates a zero-byte -o file,
// and reports merged_count 0.
func TestAuditMerge_EmptyInputSucceeds(t *testing.T) {
	t.Parallel()
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

// TestAuditMerge_OnlyMalformedFails covers §8a.4 for the audit phase, reversing
// the old "only-malformed is a clean result" contract: files holding only
// malformed/incomplete lines now exit 1, while still creating the -o file,
// reporting merged_count 0, and emitting a stderr warning per skipped line.
func TestAuditMerge_OnlyMalformedFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.ndjson")
	require.NoError(t, os.WriteFile(bad,
		[]byte("not json at all\n"+ // malformed
			`{"role":"go-safety","item_id":"a"}`+"\n"), // incomplete: no status
		0o600))

	out := filepath.Join(dir, "merged.ndjson")
	stdout, stderr, code := runTP(t, dir, "audit", "--merge", bad, "-o", out)
	require.Equal(t, 1, code, "an only-malformed input is a dropped role (§8a.4): %s", stderr)

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
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTPMerge(t, dir, "audit", "--merge")
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "at least 1 file required for merge")
}

// TestAuditMerge_MissingFileExit3 covers §3.3 rows 4-5: a missing/unreadable
// input file exits 3.
func TestAuditMerge_MissingFileExit3(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_, stderr, code := runTPMerge(t, dir, "audit", "--merge", filepath.Join(dir, "nope.ndjson"))
	assert.Equal(t, 3, code)
	assert.Contains(t, stderr, "file not found")
}

// TestAuditMerge_RejectsSpecPositionalExit2 covers §4.1 for the audit phase:
// --merge takes only its explicit NDJSON inputs, so a spec-looking positional
// (a .md) among them is rejected at entry with exit 2. Today the spec is
// silently parsed (input_files counts it, one warning per spec line) — this
// guards against that regression.
func TestAuditMerge_RejectsSpecPositionalExit2(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\nbody\n"), 0o600))
	a := filepath.Join(dir, "a.ndjson")
	require.NoError(t, os.WriteFile(a, []byte(`{"role":"spec-coverage","item_id":"a","status":"PASS"}`+"\n"), 0o600))
	b := filepath.Join(dir, "b.ndjson")
	require.NoError(t, os.WriteFile(b, []byte(`{"role":"go-safety","item_id":"b","status":"FAIL"}`+"\n"), 0o600))

	_, stderr, code := runTPMerge(t, dir, "audit", "--merge", spec, a, b)
	assert.Equal(t, 2, code)
	assert.Contains(t, stderr, "looks like a spec")
	assert.Contains(t, stderr, "--merge takes NDJSON input files only")
	assert.NotContains(t, stderr, "warning:", "the spec must not be parsed as an input file")
}
