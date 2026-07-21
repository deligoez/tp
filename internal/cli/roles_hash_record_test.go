package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundRolesHash struct {
	RolesHash string `json:"roles_hash"`
}

func readRoundHashes(t *testing.T, dir string) (review, audit []roundRolesHash) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st struct {
		ReviewRounds []roundRolesHash `json:"review_rounds"`
		AuditRounds  []roundRolesHash `json:"audit_rounds"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	return st.ReviewRounds, st.AuditRounds
}

// TestRolesHash_StoredAtRecordPerPhase stores the phase's corpus hash on each
// recorded round: the reviewer hash on a review round, the auditor hash on an
// audit round, independently (§9.2).
func TestRolesHash_StoredAtRecordPerPhase(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	// A user reviewer corpus (hashed), but no auditor corpus (builtin).
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(revDir, "r.json"),
		[]byte(`{"id":"r","title":"T","instructions":"I"}`), 0o600))

	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))

	_, stderr, code := runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code, "review record: %s", stderr)
	_, stderr, code = runTP(t, dir, "audit", "spec.md", "--record", rec)
	require.Equal(t, 0, code, "audit record: %s", stderr)

	review, audit := readRoundHashes(t, dir)
	require.Len(t, review, 1)
	require.Len(t, audit, 1)
	assert.Contains(t, review[0].RolesHash, "sha256:", "the reviewer corpus hash is stored on the review round")
	assert.Equal(t, "builtin", audit[0].RolesHash, "no auditor corpus -> builtin on the audit round")
}

// TestRolesHash_SequenceIndependence: editing a reviewer changes the next review
// round's stored hash but never the audit round's — the sequences are independent
// (§9.2).
func TestRolesHash_SequenceIndependence(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))
	revDir := filepath.Join(dir, ".tp", "reviewers")
	require.NoError(t, os.MkdirAll(revDir, 0o755))
	rolePath := filepath.Join(revDir, "r.json")
	require.NoError(t, os.WriteFile(rolePath, []byte(`{"id":"r","title":"T","instructions":"I"}`), 0o600))

	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))

	// Round 1: review + audit.
	_, _, code := runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "audit", "spec.md", "--record", rec)
	require.Equal(t, 0, code)

	// Edit the reviewer, then record review round 2.
	require.NoError(t, os.WriteFile(rolePath, []byte(`{"id":"r","title":"T EDITED","instructions":"I"}`), 0o600))
	_, _, code = runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)

	review, audit := readRoundHashes(t, dir)
	require.Len(t, review, 2)
	require.Len(t, audit, 1)
	assert.NotEqual(t, review[0].RolesHash, review[1].RolesHash, "a reviewer edit flips the review round hash")
	assert.Equal(t, "builtin", audit[0].RolesHash, "editing a reviewer never touches the audit sequence")
}
