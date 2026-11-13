package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRoundsSinceEpoch_CutsEachPhaseAtTheNewestEpoch: rounds recorded at or
// before the newest epoch's position for a phase are left out, per phase, and
// an older epoch does not cut further than the newest one.
func TestRoundsSinceEpoch_CutsEachPhaseAtTheNewestEpoch(t *testing.T) {
	t.Parallel()
	st := &ReviewState{
		ReviewRounds: []ReviewRound{{Round: 1}, {Round: 2}, {Round: 3}},
		AuditRounds:  []ReviewRound{{Round: 1}, {Round: 2}},
		Epochs: []Epoch{
			{AfterReviewRound: 1, AfterAuditRound: 0, Evidence: "first rewrite"},
			{AfterReviewRound: 2, AfterAuditRound: 1, Evidence: "second rewrite"},
		},
	}
	assert.Equal(t, 2, EpochCut(st, PhaseReview))
	assert.Equal(t, 1, EpochCut(st, PhaseAudit))
	assert.Equal(t, []ReviewRound{{Round: 3}}, RoundsSinceEpoch(st, PhaseReview))
	assert.Equal(t, []ReviewRound{{Round: 2}}, RoundsSinceEpoch(st, PhaseAudit))
}

// TestRoundsSinceEpoch_NoEpochKeepsEveryRound: without an epoch nothing is cut,
// and a nil state has no rounds.
func TestRoundsSinceEpoch_NoEpochKeepsEveryRound(t *testing.T) {
	t.Parallel()
	st := &ReviewState{ReviewRounds: []ReviewRound{{Round: 1}, {Round: 2}}}
	assert.Equal(t, 0, EpochCut(st, PhaseReview))
	assert.Equal(t, st.ReviewRounds, RoundsSinceEpoch(st, PhaseReview))
	assert.Empty(t, RoundsSinceEpoch(nil, PhaseReview))
	assert.Equal(t, 0, EpochCut(nil, PhaseAudit))
}

// TestEpochs_SurviveASaveAndAreAbsentWhenNone: the epochs are a typed field of
// state.json, so a load-and-save keeps them; a state with none writes no key,
// so every state.json recorded before epochs existed stays byte-identical.
func TestEpochs_SurviveASaveAndAreAbsentWhenNone(t *testing.T) {
	t.Parallel()
	specPath := filepath.Join(t.TempDir(), "spec.md")
	require.NoError(t, os.WriteFile(specPath, []byte("# spec\n"), 0o600))
	st, err := EnsureReviewState(specPath)
	require.NoError(t, err)

	raw, err := os.ReadFile(reviewStatePath(specPath))
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "epochs", "no epoch, no key")

	st.Epochs = append(st.Epochs, Epoch{AfterReviewRound: 2, AfterAuditRound: 0, Evidence: "rewritten", At: "2026-11-13T00:00:00Z"})
	require.NoError(t, SaveReviewState(specPath, st))
	again, err := LoadReviewState(specPath)
	require.NoError(t, err)
	require.NoError(t, SaveReviewState(specPath, again))
	final, err := LoadReviewState(specPath)
	require.NoError(t, err)
	assert.Equal(t, st.Epochs, final.Epochs)

	raw, err = os.ReadFile(reviewStatePath(specPath))
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"after_review_round": 2`)
	assert.Contains(t, string(raw), `"after_audit_round": 0`)
	assert.Contains(t, string(raw), `"evidence": "rewritten"`)
	assert.Contains(t, string(raw), `"at": "2026-11-13T00:00:00Z"`)
}
