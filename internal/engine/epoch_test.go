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

