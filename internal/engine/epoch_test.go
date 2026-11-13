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

// snap is a round recorded from its own emission snapshot of the text hash.
func snap(round int, hash string) ReviewRound {
	return ReviewRound{Round: round, SpecHash: hash, HashScheme: HashSchemeSnapshot}
}

// TestEpochPosition_IsTheLastRoundOverAnOlderTextPerPhase: each phase is cut
// after its own last round that read an older text, and rounds over the
// current text stay uncut — a review round over the new text strands no
// audit round over the old one.
func TestEpochPosition_IsTheLastRoundOverAnOlderTextPerPhase(t *testing.T) {
	t.Parallel()
	st := &ReviewState{
		ReviewRounds: []ReviewRound{snap(1, "H1"), snap(2, "H1"), snap(3, "H2")},
		AuditRounds:  []ReviewRound{snap(1, "H1"), snap(2, "H1")},
	}
	assert.Equal(t, 2, EpochPosition(st, PhaseReview, "H2"))
	assert.Equal(t, 2, EpochPosition(st, PhaseAudit, "H2"))
	assert.Equal(t, 0, EpochPosition(nil, PhaseReview, "H2"))
}

// TestEpochPosition_TheFirstRoundAloneOverAnOlderText: the walk back reaches
// the first round, so a history whose only old-text round is round 1 is cut
// after it, and a history that read only the current text cuts nothing.
func TestEpochPosition_TheFirstRoundAloneOverAnOlderText(t *testing.T) {
	t.Parallel()
	st := &ReviewState{ReviewRounds: []ReviewRound{snap(1, "H1"), snap(2, "H2")}}
	assert.Equal(t, 1, EpochPosition(st, PhaseReview, "H2"))
	current := &ReviewState{ReviewRounds: []ReviewRound{snap(1, "H2")}}
	assert.Equal(t, 0, EpochPosition(current, PhaseReview, "H2"))
}

// TestEpochPosition_ARoundWithNoSnapshotHashReadAnOlderText: a round whose
// read text cannot be established — unmarked (recorded before the marker
// existed) or marked no-emission — counts as having read an older text,
// whatever its stored hash, so the cut lands after the last such round or
// old-hash round.
func TestEpochPosition_ARoundWithNoSnapshotHashReadAnOlderText(t *testing.T) {
	t.Parallel()
	st := &ReviewState{ReviewRounds: []ReviewRound{
		snap(1, "H1"),
		{Round: 2, SpecHash: "H2"},
		snap(3, "H2"),
		{Round: 4, SpecHash: "H2", HashScheme: HashSchemeNoEmission},
		snap(5, "H2"),
		snap(6, "H2"),
	}}
	assert.Equal(t, 4, EpochPosition(st, PhaseReview, "H2"))
	st.ReviewRounds = st.ReviewRounds[:4]
	st.ReviewRounds[3] = snap(4, "H2")
	assert.Equal(t, 2, EpochPosition(st, PhaseReview, "H2"), "an unmarked round counts as older even when its hash is the current one")
}

// TestEpochPosition_ARevertedSpecCutsTheEarlyRoundsToo: text A with rounds,
// rewritten to B with rounds, reverted to A — the position is the last B
// round, so the early A rounds are cut with it although they read what is
// current again. The position is one number, and the conservative result
// errs toward more review.
func TestEpochPosition_ARevertedSpecCutsTheEarlyRoundsToo(t *testing.T) {
	t.Parallel()
	st := &ReviewState{ReviewRounds: []ReviewRound{snap(1, "A"), snap(2, "A"), snap(3, "B"), snap(4, "B")}}
	assert.Equal(t, 4, EpochPosition(st, PhaseReview, "A"))
	st.ReviewRounds = append(st.ReviewRounds, snap(5, "A"))
	assert.Equal(t, 4, EpochPosition(st, PhaseReview, "A"))
}

// TestEpochPosition_NeverMovesBackwards: an existing epoch's position is a
// floor, so a phase whose rounds all read the current text keeps it.
func TestEpochPosition_NeverMovesBackwards(t *testing.T) {
	t.Parallel()
	st := &ReviewState{
		ReviewRounds: []ReviewRound{{Round: 1, SpecHash: "H1"}, snap(2, "H2"), snap(3, "H2")},
		Epochs:       []Epoch{{AfterReviewRound: 2}},
	}
	assert.Equal(t, 2, EpochPosition(st, PhaseReview, "H2"))
	st.Epochs[0].AfterReviewRound = 3
	assert.Equal(t, 3, EpochPosition(st, PhaseReview, "H2"))
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

// TestRegressionDue_CountsNoFixedRowFromBeforeTheEpoch: tp run's oracle spawns
// a regression unit on the same inputs the emission reads, so a fixed row
// recorded before the epoch does not make a round due. The spec is unchanged
// since round 1's snapshot here, so the fixed row is the only reason it could be.
func TestRegressionDue_CountsNoFixedRowFromBeforeTheEpoch(t *testing.T) {
	t.Parallel()
	specPath := filepath.Join(t.TempDir(), "spec.md")
	body := []byte("# spec\n\n## A\n\ntext\n")
	require.NoError(t, os.WriteFile(specPath, body, 0o600))
	stateDir := ReviewStateDir(specPath)
	require.NoError(t, os.MkdirAll(stateDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "snapshot-round-1.md"), body, 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(stateDir, "review-round-1.ndjson"),
		[]byte(`{"finding":"f","resolved":{"status":"fixed","evidence":"e"}}`+"\n"), 0o600))
	st := &ReviewState{ReviewRounds: []ReviewRound{{Round: 1, File: "review-round-1.ndjson"}}}
	require.True(t, RegressionDue(specPath, 2, st), "the fixture must make the predicate hold without an epoch")

	st.Epochs = []Epoch{{AfterReviewRound: 1, Evidence: "rewritten"}}
	assert.False(t, RegressionDue(specPath, 2, st))
}
