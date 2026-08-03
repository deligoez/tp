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

// TestCompact_ReviewOmitsHarnessKeepsNextAction: under --compact, review
// --status and --record omit harness_note and harness_stale while retaining
// next_action; non-compact output still carries both harness fields (§8.4).
func TestCompact_ReviewOmitsHarnessKeepsNextAction(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	// Two differing notes -> harness_stale would be true.
	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "first")
	require.Equal(t, 0, code, "%s", stderr)
	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "second")
	require.Equal(t, 0, code, "%s", stderr)

	// --status --compact: harness fields omitted, next_action retained.
	stdout, _, code := runTP(t, dir, "review", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	_, hasStale := out["harness_stale"]
	assert.False(t, hasStale, "harness_stale omitted under --compact")
	_, hasNote := out["harness_note"]
	assert.False(t, hasNote, "harness_note omitted under --compact")
	assert.Contains(t, out, "next_action", "next_action retained under --compact")

	// --status (non-compact): both harness fields present per their rules.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status")
	require.Equal(t, 0, code)
	full := parseStatusJSON(t, stdout)
	assert.Equal(t, true, full["harness_stale"], "harness_stale present without --compact")
	assert.Equal(t, "second", full["harness_note"], "harness_note present without --compact")

	// --record --compact: a differing note keeps the round stale; still omitted.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "third", "--compact")
	require.Equal(t, 0, code)
	rout := parseStatusJSON(t, stdout)
	_, hasStale = rout["harness_stale"]
	assert.False(t, hasStale, "harness_stale omitted on --record --compact")
	_, hasNote = rout["harness_note"]
	assert.False(t, hasNote, "harness_note omitted on --record --compact")
	assert.Contains(t, rout, "next_action", "next_action retained on --record --compact")

	// --record without --compact: the differing note surfaces both fields.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--record", rec, "--harness-note", "fourth")
	require.Equal(t, 0, code)
	rfull := parseStatusJSON(t, stdout)
	assert.Equal(t, true, rfull["harness_stale"], "harness_stale present on --record without --compact")
	assert.Equal(t, "fourth", rfull["harness_note"])
}

// TestCompact_AuditOmitsHarnessKeepsNextAction: the same --compact behavior
// applies to audit --status and --record — harness_note/harness_stale omitted,
// next_action retained (§8.4). nonblocking_open is review-only and never here.
func TestCompact_AuditOmitsHarnessKeepsNextAction(t *testing.T) {
	dir := t.TempDir()
	rec := writeHarnessSpec(t, dir)

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "a1")
	require.Equal(t, 0, code, "%s", stderr)
	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "a2")
	require.Equal(t, 0, code, "%s", stderr)

	// --status --compact
	stdout, _, code := runTP(t, dir, "audit", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	out := parseStatusJSON(t, stdout)
	_, hasStale := out["harness_stale"]
	assert.False(t, hasStale, "audit harness_stale omitted under --compact")
	_, hasNote := out["harness_note"]
	assert.False(t, hasNote, "audit harness_note omitted under --compact")
	assert.Contains(t, out, "next_action", "audit next_action retained under --compact")

	// --status (non-compact): both present.
	stdout, _, code = runTP(t, dir, "audit", "spec.md", "--status")
	require.Equal(t, 0, code)
	full := parseStatusJSON(t, stdout)
	assert.Equal(t, true, full["harness_stale"])
	assert.Equal(t, "a2", full["harness_note"])

	// --record --compact: differing note keeps it stale; still omitted.
	stdout, _, code = runTP(t, dir, "audit", "spec.md", "--record", rec, "--harness-note", "a3", "--compact")
	require.Equal(t, 0, code)
	rout := parseStatusJSON(t, stdout)
	_, hasStale = rout["harness_stale"]
	assert.False(t, hasStale, "audit harness_stale omitted on --record --compact")
	_, hasNote = rout["harness_note"]
	assert.False(t, hasNote, "audit harness_note omitted on --record --compact")
	assert.Contains(t, rout, "next_action", "audit next_action retained on --record --compact")
}

// TestCompact_ReviewRetainsNonBlockingOpen: nonblocking_open is decision-critical
// and survives --compact on both review --record and --status; accepted_open is
// never emitted (§4.2, §8.4).
func TestCompact_ReviewRetainsNonBlockingOpen(t *testing.T) {
	dir := setupConvergeOnProject(t) // a single clean round converges
	medium := `{"severity":"medium","category":"ambiguity","location":"L1","finding":"soft","suggestion":"clarify"}` + "\n"
	f := filepath.Join(dir, "findings.ndjson")
	require.NoError(t, os.WriteFile(f, []byte(medium), 0o600))

	// --record --compact: a clean accepted-open round retains nonblocking_open.
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", f, "--compact")
	require.Equal(t, 0, code, "%s", stderr)
	out := parseStatusJSON(t, stdout)
	assert.Equal(t, float64(1), out["nonblocking_open"], "nonblocking_open retained under --compact")
	_, hasAccepted := out["accepted_open"]
	assert.False(t, hasAccepted, "accepted_open never emitted")

	// --status --compact retains it too.
	stdout, _, code = runTP(t, dir, "review", "spec.md", "--status", "--compact")
	require.Equal(t, 0, code)
	status := parseStatusJSON(t, stdout)
	assert.Equal(t, float64(1), status["nonblocking_open"], "nonblocking_open retained on --status --compact")
	_, hasAccepted = status["accepted_open"]
	assert.False(t, hasAccepted, "accepted_open never emitted on --status")
}
