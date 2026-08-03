package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readRoundNotes reads the verbatim harness_note stored on each recorded round
// per phase from state.json.
func readRoundNotes(t *testing.T, dir string) (review, audit []string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st struct {
		ReviewRounds []struct {
			HarnessNote string `json:"harness_note"`
		} `json:"review_rounds"`
		AuditRounds []struct {
			HarnessNote string `json:"harness_note"`
		} `json:"audit_rounds"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	for _, r := range st.ReviewRounds {
		review = append(review, r.HarnessNote)
	}
	for _, r := range st.AuditRounds {
		audit = append(audit, r.HarnessNote)
	}
	return review, audit
}

func writeHarnessSpec(t *testing.T, dir string) string {
	t.Helper()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))
	return rec
}

// TestHarnessNote_StoredVerbatimPerPhase: --harness-note stores the note
// verbatim on the just-recorded review and audit round independently.
func TestHarnessNote_StoredVerbatimPerPhase(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "  review note  ")
	require.Equal(t, 0, code, "review record: %s", stderr)
	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "audit note")
	require.Equal(t, 0, code, "audit record: %s", stderr)

	review, audit := readRoundNotes(t, dir)
	require.Len(t, review, 1)
	require.Len(t, audit, 1)
	assert.Equal(t, "  review note  ", review[0], "review note stored verbatim, untrimmed")
	assert.Equal(t, "audit note", audit[0])
}

// TestHarnessNote_OmittedStoresNothing: omitting --harness-note stores no note
// (behavior exactly as before the field existed).
func TestHarnessNote_OmittedStoresNothing(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	_, _, code := runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	review, _ := readRoundNotes(t, dir)
	require.Len(t, review, 1)
	assert.Equal(t, "", review[0], "no flag -> empty note (omitempty in state.json)")
}

// TestHarnessNote_RequiresRecord: --harness-note without --record is a usage
// error (exit 2) on both review and audit.
func TestHarnessNote_RequiresRecord(t *testing.T) {
	dir := t.TempDir()
	writeHarnessSpec(t, dir)

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--status", "--harness-note", "x")
	assert.Equal(t, 2, code, "review --harness-note without --record must be a usage error")
	assert.Contains(t, stderr, "requires --record")

	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--status", "--harness-note", "x")
	assert.Equal(t, 2, code, "audit --harness-note without --record must be a usage error")
	assert.Contains(t, stderr, "requires --record")
}

// TestHarnessStale_RecordAndStatus: harness_stale is true only once two
// non-empty differing notes are recorded; --record computes it after storing
// the current round; --status agrees; harness_note is the verbatim latest note
// and is omitted when not stale.
func TestHarnessStale_RecordAndStatus(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	// Round 1: single round -> not stale, no harness_note key.
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "first")
	require.Equal(t, 0, code, "%s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["harness_stale"], "one recorded round is never stale")
	_, hasNote := out["harness_note"]
	assert.False(t, hasNote, "harness_note omitted when not stale")

	// Round 2: differing note -> stale computed AFTER storing this round.
	stdout, stderr, code = runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "second")
	require.Equal(t, 0, code, "%s", stderr)
	out = map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["harness_stale"], "two differing notes -> stale")
	assert.Equal(t, "second", out["harness_note"], "harness_note is the verbatim latest note")

	// --status agrees and surfaces the same values read-only.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	out = map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["harness_stale"])
	assert.Equal(t, "second", out["harness_note"])

	// Round 3: identical note to round 2 -> no longer stale, note omitted.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "second")
	require.Equal(t, 0, code)
	out = map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["harness_stale"], "identical trailing notes -> not stale")
	_, hasNote = out["harness_note"]
	assert.False(t, hasNote, "harness_note omitted when not stale")
}

// TestHarnessStale_PerPhaseIsolation: a review round's note is never compared
// against an audit round's. An audit round recorded between two identical
// review notes does not make review stale.
func TestHarnessStale_PerPhaseIsolation(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	_, _, code := runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "same")
	require.Equal(t, 0, code)
	// An audit round with a DIFFERENT note lands between the two review rounds.
	_, _, code = runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "audit-different")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "same")
	require.Equal(t, 0, code)

	// Review compares only review rounds: [same, same] -> not stale.
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["harness_stale"], "review ignores the interleaved audit round")

	// Audit compares only audit rounds: a single round -> not stale.
	stdout, _, code = runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	out = map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, false, out["harness_stale"], "single audit round is not stale")
	_, hasNote := out["harness_note"]
	assert.False(t, hasNote)
}

// TestHarnessStale_AuditStatus: the audit phase surfaces harness_stale/note on
// its own recorded rounds.
func TestHarnessStale_AuditStatus(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	_, _, code := runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "a1")
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "a2")
	require.Equal(t, 0, code)

	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["harness_stale"])
	assert.Equal(t, "a2", out["harness_note"])
}
