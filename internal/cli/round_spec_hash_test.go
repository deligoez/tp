package cli_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A round's spec_hash is the hash of its own emission snapshot — the text its
// prompts were emitted from — and not of the spec as it stands when the round
// is recorded (spec/backlog/reconcile.md §4). These tests are the silent loss
// that rule closes: a round that never read the current text cannot report it
// as read.

const specV1 = "# Spec\n## 1. A\ncontent\n"
const specV2 = "# Spec\n## 1. A\ncontent edited after the round was emitted\n"

// hashSchemeRound is the part of a recorded round entry these tests read.
type hashSchemeRound struct {
	SpecHash   string `json:"spec_hash"`
	HashScheme string `json:"hash_scheme"`
}

func readHashSchemeRounds(t *testing.T, dir string) (review, audit []hashSchemeRound) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st struct {
		ReviewRounds []hashSchemeRound `json:"review_rounds"`
		AuditRounds  []hashSchemeRound `json:"audit_rounds"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	return st.ReviewRounds, st.AuditRounds
}

// sha256Of is the stored spec_hash form: "sha256:<hex>" over a file's bytes.
func sha256Of(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // a path the test built
	require.NoError(t, err)
	return fmt.Sprintf("sha256:%x", sha256.Sum256(data))
}

// loopStatus runs `tp <phase> spec.md --status` and returns its payload.
func loopStatus(t *testing.T, dir, phase string) map[string]any {
	t.Helper()
	stdout, stderr, code := runTP(t, dir, phase, "spec.md", "--status")
	require.Equal(t, 0, code, "%s --status: %s", phase, stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	return out
}

// emptyRecordFile is the zero-byte findings file a clean round records.
func emptyRecordFile(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	return path
}

// specProject is a fresh project holding spec.md at specV1.
func specProject(t *testing.T) (dir, specPath string) {
	t.Helper()
	dir = t.TempDir()
	specPath = filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte(specV1), 0o600))
	return dir, specPath
}

// TestReviewRecord_NoEmissionCannotClearStaleness is BUGS.md's silent-loss
// entry, reproduced: converge, edit the spec, then `--record` an empty file
// with NO emission before it. At HEAD `stale` goes false and `converged` true —
// a round nobody ran against the edited text counts as the round that re-read
// it. The round still records (§4: a round whose snapshot is absent does not
// fail); what it cannot do is answer for text it never read.
func TestReviewRecord_NoEmissionCannotClearStaleness(t *testing.T) {
	t.Parallel()
	dir, specPath := specProject(t)
	empty := emptyRecordFile(t, dir)

	// Converge: two rounds, each emitted and then recorded clean.
	for i := 1; i <= 2; i++ {
		_, stderr, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code, "round %d emission: %s", i, stderr)
		_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", empty)
		require.Equal(t, 0, code, "round %d record: %s", i, stderr)
	}
	pre := loopStatus(t, dir, "review")
	require.Equal(t, false, pre["stale"], "the fixture starts converged over the current text")
	require.Equal(t, true, pre["converged"], "the fixture starts converged over the current text")

	// The edit that owes a round.
	require.NoError(t, os.WriteFile(specPath, []byte(specV2), 0o600))
	require.Equal(t, true, loopStatus(t, dir, "review")["stale"], "the edit makes the recorded rounds stale")

	// The defect: record with no emission before it.
	stdout, stderr, code := runTP(t, dir, "review", "spec.md", "--record", empty)
	require.Equal(t, 0, code, "a round with no snapshot still records: %s", stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &out))
	assert.Equal(t, true, out["stale"], "a round nobody emitted cannot report the edited text as read")
	assert.Equal(t, false, out["converged"], "and cannot re-converge the spec on its own")

	after := loopStatus(t, dir, "review")
	assert.Equal(t, true, after["stale"], "--status agrees with what --record reported")
	assert.Equal(t, false, after["converged"])

	review, _ := readHashSchemeRounds(t, dir)
	require.Len(t, review, 3)
	assert.Equal(t, "no-emission", review[2].HashScheme,
		"the round says on the record that no emission preceded it")
	assert.Equal(t, "snapshot", review[1].HashScheme,
		"while the rounds that were emitted name their own snapshot")
}

// TestReviewRecord_EmittedBeforeTheEditRecordsTheTextItRead is §4 itself: the
// round was emitted from specV1 and recorded after the file became specV2, so
// its spec_hash is the snapshot's — the text its prompts carried — and the spec
// stays stale. At HEAD the round is stamped with specV2's hash and clears it.
func TestReviewRecord_EmittedBeforeTheEditRecordsTheTextItRead(t *testing.T) {
	t.Parallel()
	dir, specPath := specProject(t)
	empty := emptyRecordFile(t, dir)

	_, stderr, code := runTP(t, dir, "review", "spec.md")
	require.Equal(t, 0, code, "emission: %s", stderr)
	snapshot := filepath.Join(dir, ".tp-review", "spec", "snapshot-round-1.md")
	require.FileExists(t, snapshot)

	require.NoError(t, os.WriteFile(specPath, []byte(specV2), 0o600))
	_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", empty)
	require.Equal(t, 0, code, "record: %s", stderr)

	review, _ := readHashSchemeRounds(t, dir)
	require.Len(t, review, 1)
	assert.Equal(t, sha256Of(t, snapshot), review[0].SpecHash,
		"the round records the hash of the text it was emitted from")
	assert.NotEqual(t, sha256Of(t, specPath), review[0].SpecHash,
		"and not the hash of the spec as it stands at record time")
	assert.Equal(t, "snapshot", review[0].HashScheme,
		"and says which of the two meanings its spec_hash carries")
	assert.Equal(t, true, loopStatus(t, dir, "review")["stale"],
		"a round emitted before the edit leaves the spec stale")
}

// TestAuditRecord_EmittedBeforeTheEditRecordsTheTextItRead is the audit half of
// the rule: both phases snapshot at emission, so both record the snapshot's
// hash (§4 *both phases*).
func TestAuditRecord_EmittedBeforeTheEditRecordsTheTextItRead(t *testing.T) {
	t.Parallel()
	dir, specPath := specProject(t)
	empty := emptyRecordFile(t, dir)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "plain.go"), []byte("package main\n"), 0o600))

	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--affected-files", "plain.go")
	require.Equal(t, 0, code, "audit emission: %s", stderr)
	snapshot := filepath.Join(dir, ".tp-review", "spec", "snapshot-audit-round-1.md")
	require.FileExists(t, snapshot)

	require.NoError(t, os.WriteFile(specPath, []byte(specV2), 0o600))
	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--record", empty)
	require.Equal(t, 0, code, "audit record: %s", stderr)

	_, audit := readHashSchemeRounds(t, dir)
	require.Len(t, audit, 1)
	assert.Equal(t, sha256Of(t, snapshot), audit[0].SpecHash,
		"the audit round records the hash of the text it was emitted from")
	assert.Equal(t, true, loopStatus(t, dir, "audit")["stale"],
		"an audit round emitted before the edit leaves the spec stale")
}

// TestReviewRecord_EmitThenRecordUnchangedStillConverges is the control: with
// no edit between emission and record, the snapshot IS the spec, so the two
// hashes agree and the loop converges exactly as before this change.
func TestReviewRecord_EmitThenRecordUnchangedStillConverges(t *testing.T) {
	t.Parallel()
	dir, specPath := specProject(t)
	empty := emptyRecordFile(t, dir)

	for i := 1; i <= 2; i++ {
		_, stderr, code := runTP(t, dir, "review", "spec.md")
		require.Equal(t, 0, code, "round %d emission: %s", i, stderr)
		_, stderr, code = runTP(t, dir, "review", "spec.md", "--record", empty)
		require.Equal(t, 0, code, "round %d record: %s", i, stderr)
	}

	review, _ := readHashSchemeRounds(t, dir)
	require.Len(t, review, 2)
	for i, r := range review {
		assert.Equal(t, sha256Of(t, specPath), r.SpecHash,
			"round %d read the text the spec still holds", i+1)
	}
	status := loopStatus(t, dir, "review")
	assert.Equal(t, false, status["stale"])
	assert.Equal(t, true, status["converged"])
	assert.Equal(t, float64(2), status["consecutive_clean"])
}
