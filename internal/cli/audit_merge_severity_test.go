package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/deligoez/tp/internal/engine"
)

// v0.37.0 §4: `tp audit --merge` gains a by_severity breakdown over the round's
// non-PASS rows. §7 rows 15, 16 and 17 are pinned here.

// auditMergeSeverityRows is row 17's fixture: one PASS row plus seven non-PASS
// rows — an `error`, a `warning`, an `info`, and all four shapes §4 folds into
// the single `unrecognised` bucket (a string outside the enum, JSON null, an
// absent key, and a value that is not a string). Every (role, item_id) pair is
// distinct so the merge's dedup drops none of them.
var auditMergeSeverityRows = []string{
	`{"role":"spec-coverage","item_id":"p","status":"PASS","severity":null}`,
	`{"role":"spec-coverage","item_id":"e","status":"FAIL","severity":"error"}`,
	`{"role":"spec-coverage","item_id":"w","status":"PARTIAL","severity":"warning"}`,
	`{"role":"spec-coverage","item_id":"i","status":"PARTIAL","severity":"info"}`,
	`{"role":"go-safety","item_id":"u1","status":"FAIL","severity":"moderate"}`,
	`{"role":"go-safety","item_id":"u2","status":"FAIL","severity":null}`,
	`{"role":"go-safety","item_id":"u3","status":"FAIL"}`,
	`{"role":"go-safety","item_id":"u4","status":"FAIL","severity":3}`,
}

// auditMergeAdvisoryRows is row 15's discriminating fixture: a round whose only
// non-PASS rows are advisory, so it is unclean under `all` and CLEAN under
// `blocking`. It is what separates a by_severity gated on the round's rows from
// one gated on its cleanliness — the named mutant emits the key here under
// `all` and suppresses it under `blocking`, and a fixture carrying an `error`
// row cannot tell the two apart.
var auditMergeAdvisoryRows = []string{
	`{"role":"spec-coverage","item_id":"p","status":"PASS","severity":null}`,
	`{"role":"spec-coverage","item_id":"w","status":"PARTIAL","severity":"warning"}`,
	`{"role":"go-safety","item_id":"i","status":"PARTIAL","severity":"info"}`,
}

// mergeSummary writes rows to one NDJSON file under dir and returns the decoded
// summary `tp audit --merge` prints for it.
func mergeSummary(t *testing.T, dir string, rows []string, extra ...string) map[string]any {
	t.Helper()
	f := filepath.Join(dir, "rows.ndjson")
	var body []byte
	for _, r := range rows {
		body = append(body, r...)
		body = append(body, '\n')
	}
	require.NoError(t, os.WriteFile(f, body, 0o600))

	args := append([]string{"audit", "--merge", f}, extra...)
	stdout, stderr, code := runTP(t, dir, args...)
	require.Equal(t, 0, code, "audit --merge: %s", stderr)

	var summary map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &summary))
	return summary
}

// decodeAuditRows parses an NDJSON fixture back into the row shape the engine's
// clean predicate grades, so a test can derive its expectations from §2's rule
// rather than restate them as literals.
func decodeAuditRows(t *testing.T, lines []string) []map[string]any {
	t.Helper()
	rows := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var row map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &row))
		rows = append(rows, row)
	}
	return rows
}

// blockingAndAdvisory counts a fixture's non-PASS rows through
// engine.AuditRowsClean, one row at a time — §2's own predicate, so row 17's
// arithmetic is bound to the rule it must not drift from instead of to a
// number written beside the fixture.
func blockingAndAdvisory(t *testing.T, lines []string) (blocking, advisory int) {
	t.Helper()
	for _, row := range decodeAuditRows(t, lines) {
		if row["status"] == "PASS" {
			continue
		}
		if engine.AuditRowsClean([]map[string]any{row}, engine.AuditConvergeOnBlocking) {
			advisory++
			continue
		}
		blocking++
	}
	return blocking, advisory
}

// bySeverityOf extracts the by_severity object, requiring it to be present.
func bySeverityOf(t *testing.T, summary map[string]any) map[string]float64 {
	t.Helper()
	require.Contains(t, summary, "by_severity", "§4: the round holds a non-PASS row")
	raw, ok := summary["by_severity"].(map[string]any)
	require.True(t, ok, "by_severity is an object of counts")
	buckets := make(map[string]float64, len(raw))
	for k, v := range raw {
		n, ok := v.(float64)
		require.True(t, ok, "bucket %q holds a count", k)
		buckets[k] = n
	}
	return buckets
}

// TestAuditMerge_BySeverityEmittedOnRowsNotCleanliness pins §7 row 15:
// by_severity is present on any round holding a non-PASS row — under `all` and
// under `blocking` alike — and absent on an all-PASS round. The mutant that
// must fail it gates emission on the round's cleanliness.
//
// The project config is written because the mutant would read it: --merge takes
// NDJSON inputs and rejects a spec-looking positional, so it has no spec path
// and the only policy it could resolve is the project layer's. §4 says the
// emission condition is a property of the rows and nothing else, and the
// advisory-only fixture under `blocking` is the input where the two differ.
func TestAuditMerge_BySeverityEmittedOnRowsNotCleanliness(t *testing.T) {
	for _, policy := range []string{engine.AuditConvergeOnAll, engine.AuditConvergeOnBlocking} {
		t.Run("advisory-only round under "+policy, func(t *testing.T) {
			blocking, advisory := blockingAndAdvisory(t, auditMergeAdvisoryRows)
			require.Zero(t, blocking, "the fixture must carry no blocking row, or it cannot separate the mutant")
			require.Positive(t, advisory, "and must carry advisory rows, or there is nothing to break down")

			dir := writeProjectConfigDir(t, `{"audit_converge_on":"`+policy+`"}`)
			buckets := bySeverityOf(t, mergeSummary(t, dir, auditMergeAdvisoryRows))
			assert.Equal(t, float64(1), buckets["warning"], "the warning row is broken down under %s", policy)
			assert.Equal(t, float64(1), buckets["info"], "and so is the info row")
		})

		t.Run("blocking round under "+policy, func(t *testing.T) {
			dir := writeProjectConfigDir(t, `{"audit_converge_on":"`+policy+`"}`)
			buckets := bySeverityOf(t, mergeSummary(t, dir, auditMergeSeverityRows))
			assert.NotEmpty(t, buckets, "a round holding non-PASS rows carries the breakdown under %s", policy)
		})
	}

	// --merge has two summary-emitting forms and the workflow uses the one the
	// tests above do not: `tp audit --merge <files> -o merged.ndjson` returns
	// from its own branch before the stdout branch is reached, so a key added
	// on the stdout path alone is missing from exactly the call Workflow D
	// makes. Measured while writing this test: that variant survived every
	// other assertion in this file.
	t.Run("on the -o form the workflow actually uses", func(t *testing.T) {
		dir := t.TempDir()
		merged := filepath.Join(dir, "merged.ndjson")

		stdoutForm := mergeSummary(t, dir, auditMergeSeverityRows)
		fileForm := mergeSummary(t, dir, auditMergeSeverityRows, "-o", merged)
		require.Equal(t, merged, fileForm["output_path"],
			"the -o branch must really be the one taken, or this repeats the test above")

		assert.Equal(t, stdoutForm["by_severity"], fileForm["by_severity"],
			"the breakdown is a property of the rows, so both output forms carry it")
	})

	t.Run("absent on an all-PASS round", func(t *testing.T) {
		dir := t.TempDir()
		summary := mergeSummary(t, dir, []string{
			`{"role":"spec-coverage","item_id":"a","status":"PASS","severity":null}`,
			`{"role":"go-safety","item_id":"b","status":"PASS"}`,
		})
		require.Equal(t, float64(0), summary["findings"], "the fixture must really hold no non-PASS row")
		assert.NotContains(t, summary, "by_severity",
			"an all-PASS round has nothing to break down, so the key is absent rather than empty")
	})
}

