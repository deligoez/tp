package cli_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type roundIDScheme struct {
	IDScheme string `json:"id_scheme"`
}

func readRoundIDSchemes(t *testing.T, dir string) (review, audit []roundIDScheme) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".tp-review", "spec", "state.json"))
	require.NoError(t, err)
	var st struct {
		ReviewRounds []roundIDScheme `json:"review_rounds"`
		AuditRounds  []roundIDScheme `json:"audit_rounds"`
	}
	require.NoError(t, json.Unmarshal(data, &st))
	return st.ReviewRounds, st.AuditRounds
}

// TestIDScheme_RecordedOnAuditOnly: every audit round recorded under this
// release carries id_scheme="slug"; a review round never does, since review
// rows dedup on (location, class) and neither carry nor consume the marker
// (§10.9).
func TestIDScheme_RecordedOnAuditOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	rec := filepath.Join(dir, "empty.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))

	_, _, code := runTP(t, dir, "review", "spec.md", "--record", rec)
	require.Equal(t, 0, code)
	_, _, code = runTP(t, dir, "audit", "spec.md", "--record", rec)
	require.Equal(t, 0, code)

	review, audit := readRoundIDSchemes(t, dir)
	require.Len(t, review, 1)
	require.Len(t, audit, 1)
	assert.Equal(t, "", review[0].IDScheme, "review rounds never carry the marker")
	assert.Equal(t, "slug", audit[0].IDScheme, "an audit round recorded under this release carries the slug marker")
}

// TestIDScheme_LegacyRoundStaysMarkerless: tp never rewrites recorded rounds,
// so a legacy (marker-less) round already on disk stays marker-less when a new
// round is recorded — the index append leaves existing entries untouched, and a
// legacy round keeps its positional ids (§10.9).
func TestIDScheme_LegacyRoundStaysMarkerless(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "spec.md"), []byte("# Spec\n"), 0o600))

	stateDir := filepath.Join(dir, ".tp-review", "spec")
	require.NoError(t, os.MkdirAll(stateDir, 0o755))

	// A round recorded before this release: no id_scheme marker, positional ids.
	legacy := `{
  "spec": "spec.md",
  "review_rounds": [],
  "audit_rounds": [
    {"round": 1, "findings": 0, "clean": true, "recorded_at": "2024-01-01T00:00:00Z", "file": "audit-round-1.ndjson", "spec_hash": "sha256:legacy"}
  ]
}`
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "state.json"), []byte(legacy), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "audit-round-1.ndjson"), []byte(""), 0o600))

	rec := filepath.Join(dir, "results.ndjson")
	require.NoError(t, os.WriteFile(rec, []byte(""), 0o600))
	_, stderr, code := runTP(t, dir, "audit", "spec.md", "--record", rec)
	require.Equal(t, 0, code, "record: %s", stderr)

	_, audit := readRoundIDSchemes(t, dir)
	require.Len(t, audit, 2)
	assert.Equal(t, "", audit[0].IDScheme, "the legacy round stays marker-less — tp never rewrites it")
	assert.Equal(t, "slug", audit[1].IDScheme, "the newly recorded round carries the slug marker")
}
