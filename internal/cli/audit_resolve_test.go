package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deligoez/tp/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test 53 (§3.3): `tp audit --resolve` and `--resolve-all` dispose audit rows
// with the same dispositions and flags as their review counterparts, and
// additionally accept a `role:item_id` selector; an audit-fix unit that disposes
// its row without a code change satisfies its predicate.

// auditRows is the round's merged results file as an audit-record unit writes
// it: one PASS row and two findings, each carrying the (role, item_id) pair the
// selector names.
const auditRows = `{"role":"spec-coverage","item_id":"item-1","status":"PASS"}
{"role":"go-safety","item_id":"item-4","status":"FAIL","finding":"unchecked error"}
{"role":"ax-contract","item_id":"item-9","status":"PARTIAL","finding":"null slice"}
`

// writeAuditResults writes the merged results file into a round directory, the
// shape §3.3 names: $TP_ROUND_DIR/merged.ndjson.
func writeAuditResults(t *testing.T, roundDir string) string {
	t.Helper()
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	path := engine.MergedFindingsPath(roundDir)
	require.NoError(t, os.WriteFile(path, []byte(auditRows), 0o600))
	return path
}

// readAuditRows reads the results file back as rows, in file order.
func readAuditRows(t *testing.T, path string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	rows := make([]map[string]any, 0)
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row), "line %q", line)
		rows = append(rows, row)
	}
	return rows
}

func resolvedOf(t *testing.T, row map[string]any) map[string]any {
	t.Helper()
	r, ok := row["resolved"].(map[string]any)
	require.True(t, ok, "row %v carries no resolved object", row)
	return r
}

// TestAuditResolve_IndexSelector: the 0-based index selector its review
// counterpart takes disposes exactly that row and leaves the others alone.
func TestAuditResolve_IndexSelector(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeAuditResults(t, filepath.Join(dir, "round"))

	stdout, stderr, code := runTP(t, dir, "audit", path, "--resolve", "1", "fixed", "added the check")
	require.Equal(t, 0, code, "index selector must resolve: %s", stderr)
	assert.Contains(t, stdout, `"status": "fixed"`)

	rows := readAuditRows(t, path)
	require.Len(t, rows, 3)
	_, disposed := rows[0]["resolved"]
	assert.False(t, disposed, "row 0 must be untouched")
	res := resolvedOf(t, rows[1])
	assert.Equal(t, "fixed", res["status"])
	assert.Equal(t, "added the check", res["evidence"])
	assert.NotEmpty(t, res["resolved_at"])
	_, disposed = rows[2]["resolved"]
	assert.False(t, disposed, "row 2 must be untouched")
}

// TestAuditResolve_RoleItemIDSelector: the audit-side addition — a unit names
// its own row by the `role:item_id` key the oracle handed it, without first
// locating an index.
func TestAuditResolve_RoleItemIDSelector(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeAuditResults(t, filepath.Join(dir, "round"))

	_, stderr, code := runTP(t, dir, "audit", path, "--resolve", "ax-contract:item-9", "duplicate", "same as item-4")
	require.Equal(t, 0, code, "role:item_id selector must resolve: %s", stderr)

	rows := readAuditRows(t, path)
	res := resolvedOf(t, rows[2])
	assert.Equal(t, "duplicate", res["status"])
	assert.Equal(t, "same as item-4", res["evidence"])
	_, disposed := rows[1]["resolved"]
	assert.False(t, disposed, "only the named row is disposed")
}

// sharedKeyRows is a round whose one (role, item_id) holds three rows: a
// PASS from one shard and two findings from others. --merge keeps disagreeing
// verdicts, so a key can name more than one row.
const sharedKeyRows = `{"role":"go-safety","item_id":"item-4","status":"PASS","evidence_file":"a.go"}
{"role":"go-safety","item_id":"item-4","status":"FAIL","severity":"error","evidence_file":"b.go"}
{"role":"go-safety","item_id":"item-4","status":"PARTIAL","evidence_file":"c.go"}
{"role":"go-safety","item_id":"item-7","status":"FAIL"}
`

// TestAuditResolve_KeyDisposesEveryFindingUnderIt: the role:item_id selector
// disposes every non-PASS row under the key, never a PASS row, and never a
// row under another key. Disposing only the first match left the second
// shard's FAIL open while the unit's predicate read the first row and said
// done.
func TestAuditResolve_KeyDisposesEveryFindingUnderIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	roundDir := filepath.Join(dir, "round")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	path := engine.MergedFindingsPath(roundDir)
	require.NoError(t, os.WriteFile(path, []byte(sharedKeyRows), 0o600))

	target := engine.UnitTarget{RoundDir: roundDir, ID: "go-safety:item-4"}
	require.False(t, engine.UnitAuditFix.DurableWrite(target))

	stdout, stderr, code := runTP(t, dir, "audit", path, "--resolve", "go-safety:item-4", "fixed", "checked both")
	require.Equal(t, 0, code, "%s", stderr)
	assert.Contains(t, stdout, `"disposed": 2`)

	rows := readAuditRows(t, path)
	assert.NotContains(t, rows[0], "resolved", "a PASS row is not a finding")
	assert.Equal(t, "fixed", resolvedOf(t, rows[1])["status"])
	assert.Equal(t, "fixed", resolvedOf(t, rows[2])["status"])
	assert.NotContains(t, rows[3], "resolved", "another key's row is untouched")
	assert.True(t, engine.UnitAuditFix.DurableWrite(target), "every finding under the key is disposed")
}

// TestAuditFixUnit_DoneOnlyWhenEveryFindingUnderTheKeyIsDisposed: the
// audit-fix predicate reads every row under its key. Reading the first match
// was wrong both ways: a PASS first kept the unit open forever, and a disposed
// FAIL first closed it over an open one.
func TestAuditFixUnit_DoneOnlyWhenEveryFindingUnderTheKeyIsDisposed(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	roundDir := filepath.Join(dir, "round")
	require.NoError(t, os.MkdirAll(roundDir, 0o755))
	path := engine.MergedFindingsPath(roundDir)
	target := engine.UnitTarget{RoundDir: roundDir, ID: "go-safety:item-4"}

	passFirst := `{"role":"go-safety","item_id":"item-4","status":"PASS"}
{"role":"go-safety","item_id":"item-4","status":"FAIL","resolved":{"status":"fixed","evidence":"e"}}
`
	require.NoError(t, os.WriteFile(path, []byte(passFirst), 0o600))
	assert.True(t, engine.UnitAuditFix.DurableWrite(target), "a PASS before the disposed finding does not keep it open")

	disposedFirst := `{"role":"go-safety","item_id":"item-4","status":"FAIL","resolved":{"status":"fixed","evidence":"e"}}
{"role":"go-safety","item_id":"item-4","status":"PARTIAL"}
`
	require.NoError(t, os.WriteFile(path, []byte(disposedFirst), 0o600))
	assert.False(t, engine.UnitAuditFix.DurableWrite(target), "a disposed first row does not close an open second")
}

// TestAuditFixUnit_DisposesWithoutACodeChange is test 53's second half: an
// audit-fix unit whose whole output is a disposition — no code change at all —
// satisfies §3.3's durable-write predicate for its own row.
func TestAuditFixUnit_DisposesWithoutACodeChange(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	roundDir := filepath.Join(dir, "round")
	path := writeAuditResults(t, roundDir)

	target := engine.UnitTarget{RoundDir: roundDir, ID: "go-safety:item-4"}
	require.False(t, engine.UnitAuditFix.DurableWrite(target), "undisposed row: the unit has not finished")

	_, stderr, code := runTP(t, dir, "audit", path, "--resolve", "go-safety:item-4", "wontfix", "the caller cannot fail here")
	require.Equal(t, 0, code, "audit-fix must be able to close its row: %s", stderr)

	assert.True(t, engine.UnitAuditFix.DurableWrite(target),
		"a disposition with no code change is the whole durable write")
}

// TestAuditResolve_ForceFlag: an already-disposed row is refused (exit 1) and
// --force re-disposes it — the same two behaviours as tp review --resolve.
func TestAuditResolve_ForceFlag(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeAuditResults(t, filepath.Join(dir, "round"))

	_, stderr, code := runTP(t, dir, "audit", path, "--resolve", "go-safety:item-4", "wontfix", "first")
	require.Equal(t, 0, code, "%s", stderr)

	_, stderr, code = runTP(t, dir, "audit", path, "--resolve", "go-safety:item-4", "fixed", "second")
	require.Equal(t, 1, code, "an already-disposed row is refused without --force: %s", stderr)
	assert.Contains(t, stderr, "already resolved")
	assert.Contains(t, stderr, "--force")
	assert.Equal(t, "wontfix", resolvedOf(t, readAuditRows(t, path)[1])["status"],
		"the refusal must not have rewritten the disposition")

	_, stderr, code = runTP(t, dir, "audit", path, "--resolve", "go-safety:item-4", "fixed", "second", "--force")
	require.Equal(t, 0, code, "--force re-disposes: %s", stderr)
	res := resolvedOf(t, readAuditRows(t, path)[1])
	assert.Equal(t, "fixed", res["status"])
	assert.Equal(t, "second", res["evidence"])
}

// TestAuditResolveAll_DisposesEveryUndisposedRow mirrors tp review --resolve-all:
// undisposed findings take the status, already-disposed ones are skipped,
// --force overwrites them, and a PASS row is never touched.
func TestAuditResolveAll_DisposesEveryUndisposedRow(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeAuditResults(t, filepath.Join(dir, "round"))

	_, stderr, code := runTP(t, dir, "audit", path, "--resolve", "1", "fixed", "already done")
	require.Equal(t, 0, code, "%s", stderr)

	stdout, stderr, code := runTP(t, dir, "audit", path, "--resolve-all", "wontfix", "out of scope for this round")
	require.Equal(t, 0, code, "%s", stderr)
	assert.Contains(t, stdout, `"resolved_count": 1`)
	assert.Contains(t, stdout, `"skipped_count": 1`)

	rows := readAuditRows(t, path)
	assert.NotContains(t, rows[0], "resolved", "a PASS row is not a finding and takes no disposition")
	assert.Equal(t, "fixed", resolvedOf(t, rows[1])["status"], "an already-disposed row is skipped")
	assert.Equal(t, "already done", resolvedOf(t, rows[1])["evidence"])
	assert.Equal(t, "wontfix", resolvedOf(t, rows[2])["status"])

	_, stderr, code = runTP(t, dir, "audit", path, "--resolve-all", "duplicate", "all dupes", "--force")
	require.Equal(t, 0, code, "%s", stderr)
	for i, row := range readAuditRows(t, path)[1:] {
		assert.Equal(t, "duplicate", resolvedOf(t, row)["status"], "row %d under --force", i+1)
	}
}

// TestAuditResolve_UsageErrors covers the exit-2 surface: a disposition outside
// the three, a selector that is neither an index nor a role:item_id key, an
// out-of-range index, a key naming no row, and a spec-looking positional.
func TestAuditResolve_UsageErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := writeAuditResults(t, filepath.Join(dir, "round"))
	spec := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(spec, []byte("# Spec\n## 1. A\nbody\n"), 0o600))

	cases := []struct {
		name string
		args []string
		want string
	}{
		{"invalid disposition", []string{"audit", path, "--resolve", "1", "deferred", "why"}, "invalid status"},
		{"selector is neither", []string{"audit", path, "--resolve", "abc", "fixed", "why"}, "invalid selector"},
		{"index out of range", []string{"audit", path, "--resolve", "9", "fixed", "why"}, "out of range"},
		{"key names no row", []string{"audit", path, "--resolve", "go-safety:item-99", "fixed", "why"}, "no row matches"},
		{"spec positional", []string{"audit", spec, "--resolve", "0", "fixed", "why"}, "looks like a spec"},
		{"spec positional, all", []string{"audit", spec, "--resolve-all", "fixed", "why"}, "looks like a spec"},
		{"invalid disposition, all", []string{"audit", path, "--resolve-all", "deferred", "why"}, "invalid status"},
		{"too few args", []string{"audit", path, "--resolve", "1"}, "usage:"},
		{"too few args, all", []string{"audit", "--resolve-all"}, "usage:"},
		{"resolve with --record", []string{"audit", path, "--resolve", "1", "fixed", "why", "--record", path}, "cannot be combined"},
		{"resolve with --merge", []string{"audit", path, "--resolve", "1", "fixed", "why", "--merge"}, "cannot be combined"},
		{"both resolve modes", []string{"audit", path, "--resolve", "--resolve-all", "fixed", "why"}, "mutually exclusive"},
		// --force is also the emission's discard of an in-flight round, so a
		// bare `audit <spec> --force` emits; the modes that write no emission
		// and resolve nothing still refuse it.
		{"force on --status", []string{"audit", spec, "--status", "--force"}, "--force requires"},
		{"force on --record", []string{"audit", spec, "--record", path, "--force"}, "--force requires"},
		{"force on --merge", []string{"audit", path, "--merge", "--force"}, "--force requires"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runTP(t, dir, tc.args...)
			assert.Equal(t, 2, code, "%v must be a usage error: %s", tc.args, stderr)
			assert.Contains(t, stderr, tc.want)
		})
	}

	// None of the refusals may have written a disposition.
	for i, row := range readAuditRows(t, path) {
		_, disposed := row["resolved"]
		assert.False(t, disposed, "row %d must be untouched by a refused call", i)
	}
}

// TestAuditResolve_MissingFileExitsThree: a results file that is not there is a
// file error, as it is for tp review --resolve.
func TestAuditResolve_MissingFileExitsThree(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	missing := filepath.Join(dir, "nope.ndjson")

	_, stderr, code := runTP(t, dir, "audit", missing, "--resolve", "0", "fixed", "why")
	assert.Equal(t, 3, code, "missing results file: %s", stderr)

	_, stderr, code = runTP(t, dir, "audit", missing, "--resolve-all", "fixed", "why")
	assert.Equal(t, 3, code, "missing results file: %s", stderr)
}

// TestAuditResolve_HelpNamesTheSelector: the help text states both selector
// forms, so a unit reading --help learns it can name its own row.
func TestAuditResolve_HelpNamesTheSelector(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stdout, _, code := runTP(t, dir, "audit", "--help")
	require.True(t, code == 0 || code == 2, "help should not hard-fail: code=%d", code)
	assert.Contains(t, stdout, "0-based")
	assert.Contains(t, stdout, "role:item_id")
}
