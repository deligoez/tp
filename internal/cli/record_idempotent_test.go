package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Section 6.3 / test 57: recording is idempotent — re-recording a round already
// recorded rewrites that round's entry rather than adding one, so a retry after
// a partial failure converges on the same state. The round a record unit is
// recording is its own id, which the driver hands it as TP_ROUND (§3.1.1).
//
// Both --record paths are covered: §6.3 speaks of "a record unit" without
// distinguishing review from audit, and both kinds exist (`review-record`,
// `audit-record`).

// auditFailRow is one non-PASS audit result — a finding for parseAuditRows.
const auditFailRow = `{"role":"spec-coverage","item_id":"i1","status":"FAIL","location":"L1","finding":"missing","suggestion":"add it"}` + "\n"

// recordUnitRound records one merged file the way a record unit does, with the
// driver's child environment supplied by extra ("TP_ROUND=2", ...).
func recordUnitRound(t *testing.T, dir, verb, content string, extra []string) (out map[string]any, stderr string, code int) {
	t.Helper()
	f := filepath.Join(dir, "merged.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(content), 0o600))
	var stdout string
	stdout, stderr, code = runTPEnv(t, dir, extra, verb, "spec.md", "--record", f)
	if code == 0 {
		require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	}
	return out, stderr, code
}

func specOnlyProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\ncontent\n"), 0o600))
	return dir
}

func roundEntries(t *testing.T, dir, key string) []map[string]any {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st map[string]any
	require.NoError(t, json.Unmarshal(data, &st))
	raw, _ := st[key].([]any)
	entries := make([]map[string]any, 0, len(raw))
	for _, r := range raw {
		entry, ok := r.(map[string]any)
		require.True(t, ok, "%s holds a round object", key)
		entries = append(entries, entry)
	}
	return entries
}

func roundFiles(t *testing.T, dir, prefix string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".tp-review", "spec", prefix+"-round-*.ndjson"))
	require.NoError(t, err)
	return matches
}

func TestRecordIdempotent_ReviewRewritesRecordedRound(t *testing.T) {
	dir := specOnlyProject(t)

	_, stderr, code := recordUnitRound(t, dir, "review", dirtyRow, nil)
	require.Equal(t, 0, code, "round 1: %s", stderr)
	out, stderr, code := recordUnitRound(t, dir, "review", dirtyRow, nil)
	require.Equal(t, 0, code, "round 2: %s", stderr)
	require.Equal(t, float64(2), out["round"])

	// The record unit for round 2 runs again — same round, different content.
	out, stderr, code = recordUnitRound(t, dir, "review", "", []string{"TP_ROUND=2"})
	require.Equal(t, 0, code, "re-record: %s", stderr)
	assert.Equal(t, float64(2), out["round"], "a re-record reports the round it rewrote")
	assert.Equal(t, true, out["clean"], "the rewritten entry carries the new content's verdict")

	entries := roundEntries(t, dir, "review_rounds")
	require.Len(t, entries, 2, "re-recording round 2 rewrites its entry rather than adding one")
	assert.Equal(t, float64(2), entries[1]["round"])
	assert.Equal(t, float64(0), entries[1]["findings"], "the rewritten entry replaces the old count")
	assert.Equal(t, "review-round-2.ndjson", entries[1]["file"])

	assert.Len(t, roundFiles(t, dir, "review"), 2, "no third round file appears")
	body, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "review-round-2.ndjson"))
	require.NoError(t, err)
	assert.Empty(t, string(body), "the round file is rewritten with the re-recorded content")
}

func TestRecordIdempotent_AuditRewritesRecordedRound(t *testing.T) {
	dir := specOnlyProject(t)

	_, stderr, code := recordUnitRound(t, dir, "audit", auditFailRow, nil)
	require.Equal(t, 0, code, "round 1: %s", stderr)
	out, stderr, code := recordUnitRound(t, dir, "audit", auditFailRow, nil)
	require.Equal(t, 0, code, "round 2: %s", stderr)
	require.Equal(t, float64(2), out["round"])

	out, stderr, code = recordUnitRound(t, dir, "audit", "", []string{"TP_ROUND=2"})
	require.Equal(t, 0, code, "re-record: %s", stderr)
	assert.Equal(t, float64(2), out["round"], "a re-record reports the round it rewrote")
	assert.Equal(t, true, out["clean"])

	entries := roundEntries(t, dir, "audit_rounds")
	require.Len(t, entries, 2, "re-recording round 2 rewrites its entry rather than adding one")
	assert.Equal(t, float64(2), entries[1]["round"])
	assert.Equal(t, float64(0), entries[1]["findings"])
	assert.Equal(t, "audit-round-2.ndjson", entries[1]["file"])
	assert.Len(t, roundFiles(t, dir, "audit"), 2, "no third round file appears")
}

// The story §6.3 tells: a record unit writes its round, the attempt fails after
// that write, and the retry lands on the same state instead of inflating the
// round count. The merged file carries a disposition the earlier phase added,
// which the retry must preserve rather than re-open.
func TestRecordIdempotent_RetryAfterPartialFailureConverges(t *testing.T) {
	const disposed = `{"severity":"high","category":"c","location":"L1","finding":"f","suggestion":"s",` +
		`"resolved":{"status":"wontfix","evidence":"verifier: false positive"}}` + "\n"

	dir := specOnlyProject(t)
	first, stderr, code := recordUnitRound(t, dir, "review", disposed, []string{"TP_ROUND=1"})
	require.Equal(t, 0, code, "first attempt: %s", stderr)
	require.Equal(t, float64(1), first["round"])
	beforeEntry := roundEntries(t, dir, "review_rounds")[0]
	beforeBody, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
	require.NoError(t, err)

	// The retry re-records the same merged file under the same round.
	retry, stderr, code := recordUnitRound(t, dir, "review", disposed, []string{"TP_ROUND=1"})
	require.Equal(t, 0, code, "retry: %s", stderr)

	for _, key := range []string{"round", "findings", "clean", "consecutive_clean", "converged", "stale"} {
		assert.Equal(t, first[key], retry[key], "the retry reports the same %s", key)
	}
	entries := roundEntries(t, dir, "review_rounds")
	require.Len(t, entries, 1, "a retry converges on the same state, it does not add a round")
	for _, field := range []string{"round", "findings", "clean", "file", "spec_hash"} {
		assert.Equal(t, beforeEntry[field], entries[0][field], "the rewritten entry keeps the same %s", field)
	}
	afterBody, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "review-round-1.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, string(beforeBody), string(afterBody), "the wontfix disposition survives the retry")
	assert.Len(t, roundFiles(t, dir, "review"), 1)
}

// Outside a run TP_ROUND is absent, and a hand `--record` cannot say which round
// it means — so it must stay additive. Anything that names no recorded round is
// treated the same way.
func TestRecordIdempotent_HandRecordingIsNeverDestructive(t *testing.T) {
	for _, tc := range []struct {
		name string
		env  []string
	}{
		{"TP_ROUND absent", nil},
		{"TP_ROUND empty", []string{"TP_ROUND="}},
		{"TP_ROUND not a number", []string{"TP_ROUND=r2"}},
		{"TP_ROUND zero", []string{"TP_ROUND=0"}},
		{"TP_ROUND past the recorded rounds", []string{"TP_ROUND=9"}},
		{"TP_ROUND names the round about to be created", []string{"TP_ROUND=2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := specOnlyProject(t)
			_, stderr, code := recordUnitRound(t, dir, "review", dirtyRow, nil)
			require.Equal(t, 0, code, "round 1: %s", stderr)

			out, stderr, code := recordUnitRound(t, dir, "review", dirtyRow, tc.env)
			require.Equal(t, 0, code, "round 2: %s", stderr)
			assert.Equal(t, float64(2), out["round"], "the second record appends")
			assert.Len(t, roundEntries(t, dir, "review_rounds"), 2)
			assert.Len(t, roundFiles(t, dir, "review"), 2)
		})
	}
}

// A rewrite adds no round, so it cannot exhaust a budget the recorded history
// has not already exhausted. Without this the guarantee would fail at exactly
// the round a retry is most likely: the last one, sitting at the cap.
func TestRecordIdempotent_RewriteAtRoundCapNotRefused(t *testing.T) {
	for _, tc := range []struct{ verb, field, key string }{
		{"review", "review_max_rounds", "review_rounds"},
		{"audit", "audit_max_rounds", "audit_rounds"},
	} {
		t.Run(tc.verb, func(t *testing.T) {
			dir := t.TempDir()
			require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
			_, _, code := runTP(t, dir, "init", "spec.md")
			require.Equal(t, 0, code)
			_, _, code = runTP(t, dir, "set", "--workflow", tc.field+"=1")
			require.Equal(t, 0, code)

			row := dirtyRow
			if tc.verb == "audit" {
				row = auditFailRow
			}
			_, stderr, code := recordUnitRound(t, dir, tc.verb, row, nil)
			require.Equal(t, 0, code, "round 1 at the cap: %s", stderr)

			// Appending past the cap stays refused.
			_, stderr, code = recordUnitRound(t, dir, tc.verb, row, nil)
			require.Equal(t, 4, code, "a new round past the cap is still refused")
			assert.Contains(t, stderr, "budget exhausted")

			out, stderr, code := recordUnitRound(t, dir, tc.verb, row, []string{"TP_ROUND=1"})
			require.Equal(t, 0, code, "re-recording round 1 is not a new round: %s", stderr)
			assert.Equal(t, float64(1), out["round"])
			assert.Len(t, roundEntries(t, dir, tc.key), 1)
		})
	}
}
