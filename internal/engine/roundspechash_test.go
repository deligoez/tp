package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoundSpecHash_PrefersTheRoundsOwnSnapshot: the hash a round records is of
// the text its prompts were emitted from — the snapshot the emission wrote —
// and not of the spec as it stands when the round is recorded.
func TestRoundSpecHash_PrefersTheRoundsOwnSnapshot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# emitted\n"), 0o600))
	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseReview, 1, []byte("# emitted\n")))

	emitted, err := SpecHash(specPath)
	require.NoError(t, err)

	// The spec moves on after the emission.
	require.NoError(t, os.WriteFile(specPath, []byte("# edited\n"), 0o600))
	edited, err := SpecHash(specPath)
	require.NoError(t, err)
	require.NotEqual(t, emitted, edited, "the fixture must really have changed the spec")

	hash, scheme, err := RoundSpecHash(specPath, PhaseReview, 1)
	require.NoError(t, err)
	assert.Equal(t, emitted, hash, "the round names the text it read")
	assert.Equal(t, HashSchemeSnapshot, scheme)

	// Per phase: the audit round 1 of the same spec has no snapshot of its own.
	_, auditScheme, err := RoundSpecHash(specPath, PhaseAudit, 1)
	require.NoError(t, err)
	assert.Equal(t, HashSchemeNoEmission, auditScheme, "a review snapshot is not an audit round's text")
}

// TestRoundSpecHash_NoSnapshotFallsBackToTheSpec is reconcile.md §4's fallback
// (row 12): a round whose snapshot is absent hashes the spec file, as before,
// and does not fail — the live case of a --record with no preceding emission.
// The marker is what keeps that hash from being read as text the round saw.
func TestRoundSpecHash_NoSnapshotFallsBackToTheSpec(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	want, err := SpecHash(specPath)
	require.NoError(t, err)

	hash, scheme, err := RoundSpecHash(specPath, PhaseReview, 1)
	require.NoError(t, err)
	assert.Equal(t, want, hash)
	assert.Equal(t, HashSchemeNoEmission, scheme)
}

// TestStateStale_MarkerlessRoundKeepsItsRecordTimeMeaning: a round recorded
// before hash_scheme existed carries no marker, and tp reads it exactly as it
// was written — its spec_hash is the spec as it stood when the round was
// recorded. Nothing recomputes or rewrites it (reconcile.md Non-Goals 4 and 6).
func TestStateStale_MarkerlessRoundKeepsItsRecordTimeMeaning(t *testing.T) {
	t.Parallel()
	legacy := []ReviewRound{{Round: 1, Clean: true, SpecHash: "sha256:a"}}

	assert.False(t, StateStale(legacy, "sha256:a"), "unchanged since the round was recorded")
	assert.True(t, StateStale(legacy, "sha256:b"), "edited since the round was recorded")
	assert.True(t, Converged(legacy, 1, "sha256:a"), "and it still converges as it did")
}

// TestStateStale_ARoundThatReadNoTextAnswersForNone: a round recorded with no
// emission before it read nothing, so it neither clears staleness nor creates
// it — the latest round that did read text is the one the current spec is
// compared against.
func TestStateStale_ARoundThatReadNoTextAnswersForNone(t *testing.T) {
	t.Parallel()
	rounds := []ReviewRound{
		{Round: 1, Clean: true, SpecHash: "sha256:old", HashScheme: HashSchemeSnapshot},
		{Round: 2, Clean: true, SpecHash: "sha256:new", HashScheme: HashSchemeNoEmission},
	}

	assert.True(t, StateStale(rounds, "sha256:new"),
		"round 2 read nothing, so round 1's text is still the last one read")
	assert.False(t, Converged(rounds, 2, "sha256:new"),
		"and two clean rounds do not converge a spec no round has read")

	// With NO round that ever read text there is no reading to compare with,
	// so the comparison falls back to the last recorded round's stored hash —
	// the answer tp gave before the marker existed. A history recorded entirely
	// by hand keeps reporting an edit after its last record.
	only := rounds[1:]
	assert.False(t, StateStale(only, "sha256:new"), "unchanged since that record")
	assert.True(t, StateStale(only, "sha256:other"), "edited since that record")
}
