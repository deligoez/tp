package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteSnapshotAtomic_AppliesAndLeavesNoTmp(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseReview, 1, []byte("# Spec\nround 1\n")))

	data, err := os.ReadFile(filepath.Join(ReviewStateDir(specPath), "snapshot-round-1.md"))
	require.NoError(t, err)
	assert.Equal(t, "# Spec\nround 1\n", string(data), "review snapshot keeps the legacy snapshot-round-N.md name")

	entries, err := os.ReadDir(ReviewStateDir(specPath))
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "no .tmp leftover after a successful atomic write: %s", e.Name())
	}
}

func TestWriteSnapshotAtomic_AuditPhase_Namespaced(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseAudit, 1, []byte("# Spec\nround 1\n")))

	_, err := os.Stat(filepath.Join(ReviewStateDir(specPath), "snapshot-audit-round-1.md"))
	assert.NoError(t, err, "audit snapshot is written to snapshot-audit-round-N.md")
	_, err = os.Stat(filepath.Join(ReviewStateDir(specPath), "snapshot-round-1.md"))
	assert.ErrorIs(t, err, os.ErrNotExist, "audit snapshot must NOT pollute the review snapshot-round-N.md namespace")
}

func TestInFlightRound(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	assert.Equal(t, 0, InFlightRound(specPath, PhaseReview, 0), "no state dir → no in-flight round")

	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseReview, 1, []byte("# Spec\n")))
	assert.Equal(t, 1, InFlightRound(specPath, PhaseReview, 0), "snapshot-round-1.md with 0 recorded rounds → in-flight 1")
	assert.Equal(t, 0, InFlightRound(specPath, PhaseReview, 1), "next round (2) has no snapshot → not in-flight")

	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseReview, 2, []byte("# Spec v2\n")))
	assert.Equal(t, 2, InFlightRound(specPath, PhaseReview, 1), "snapshot-round-2.md with 1 recorded round → in-flight 2")
}

// TestInFlightRound_PhaseScoped_NoCrossPhaseLeak guards §10.2: a review
// snapshot-round-N.md must NOT read as an in-flight audit round (and an audit
// snapshot must not read as an in-flight review round). InFlightRound only
// inspects the next round (recordedRounds+1), so the leak surfaces when one
// phase has written a snapshot for round N and the other phase's recorded
// count is N-1: the shared snapshot-round-N.md namespace made the oracle
// report a phantom round for the wrong phase — `tp resume` pointed a fresh
// driver at recording a bogus empty round, faking convergence.
func TestInFlightRound_PhaseScoped_NoCrossPhaseLeak(t *testing.T) {
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# Spec\n"), 0o600))

	// Review recorded 3 rounds, then started round 4 (snapshot written, round
	// file absent) — the legitimate review in-flight round.
	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseReview, 4, []byte("# Spec\n")))
	assert.Equal(t, 4, InFlightRound(specPath, PhaseReview, 3), "review sees its in-flight round 4")
	// Audit (0 recorded) checks its round 1; it must consult
	// snapshot-audit-round-1.md (absent), NOT the review snapshot-round-4.md.
	assert.Equal(t, 0, InFlightRound(specPath, PhaseAudit, 0), "a review snapshot must not read as a phantom audit round")

	// The live reproduction: review round 2's snapshot read as in-flight audit
	// round 2 once audit had recorded 1 round.
	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseReview, 2, []byte("# Spec\n")))
	assert.Equal(t, 2, InFlightRound(specPath, PhaseReview, 1), "review sees its in-flight round 2")
	assert.Equal(t, 0, InFlightRound(specPath, PhaseAudit, 1), "snapshot-round-2.md (review) must not read as in-flight audit round 2")

	// Mirror: an audit snapshot must not read as an in-flight review round.
	require.NoError(t, WriteSnapshotAtomic(specPath, PhaseAudit, 5, []byte("# Spec\n")))
	assert.Equal(t, 5, InFlightRound(specPath, PhaseAudit, 4), "audit sees its in-flight round 5")
	assert.Equal(t, 0, InFlightRound(specPath, PhaseReview, 4), "an audit snapshot must not read as a phantom review round")
}

func TestHasStateArtifacts_IgnoresTmpLeftover(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "snapshot-round-1.md.tmp"), []byte("partial"), 0o600))
	assert.False(t, hasStateArtifacts(dir), "a lone .tmp crash-leftover is not a state artifact")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "snapshot-round-1.md"), []byte("full"), 0o600))
	assert.True(t, hasStateArtifacts(dir), "a real review snapshot file is a state artifact")
}

func TestHasStateArtifacts_RecognizesAuditSnapshot(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "snapshot-audit-round-1.md"), []byte("full"), 0o600))
	assert.True(t, hasStateArtifacts(dir), "a phase-scoped audit snapshot is also a state artifact")
}
