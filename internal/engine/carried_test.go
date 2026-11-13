package engine

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The rows spec/backlog/carried-rows-block.md's decision lines are tested
// over: one finding at §2, open and later fixed, and a second finding at the
// same location.
const (
	carriedOpen    = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 1 is ambiguous"}`
	carriedFixed   = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 1 is ambiguous","resolved":{"status":"fixed","evidence":"reworded"}}`
	carriedOtherAt = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 2 contradicts rule 1"}`
)

// TestReviewCarried_AnUnresolvedHighBlocksPastEmptyRounds: the BUGS.md
// sequence at the engine, uncapped: two empty rounds after an open high leave
// the loop open, with the row named as what holds it.
func TestReviewCarried_AnUnresolvedHighBlocksPastEmptyRounds(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{}, []string{})}

	d := ReviewLoopDone(spec, st, 2, 0, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done)
	require.Len(t, d.CarriedBlocking, 1)
	assert.Equal(t, 1, d.CarriedBlocking[0].Round)
	assert.Equal(t, 0, d.CarriedBlocking[0].Index)
	assert.False(t, ReviewConverged(spec, st, 2, "h", ReviewConvergeOnBlocking))
}

// TestReviewCarried_TheLatestDispositionWins: an early unresolved copy of a
// finding does not hold the loop open once a later round's copy is fixed.
func TestReviewCarried_TheLatestDispositionWins(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{carriedFixed}, []string{}, []string{})}

	d := ReviewLoopDone(spec, st, 2, 0, "h", ReviewConvergeOnBlocking)
	assert.Empty(t, d.CarriedBlocking, "the fixed copy is the row's latest occurrence")
	assert.True(t, d.Done)
	assert.Equal(t, DoneByConverged, d.By)
}

// TestReviewCarried_TwoFindingsAtOneLocationStayTwo: resolving one finding
// at a location never hides another one there.
func TestReviewCarried_TwoFindingsAtOneLocationStayTwo(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{ReviewRounds: loopRounds(t, spec, []string{carriedOtherAt, carriedFixed}, []string{}, []string{})}

	d := ReviewLoopDone(spec, st, 2, 0, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done)
	require.Len(t, d.CarriedBlocking, 1)
	assert.Equal(t, "Rule 2 contradicts rule 1", d.CarriedBlocking[0].Row["finding"])
}

// TestReviewCarried_AnEpochCutsTheRow: a row an epoch cut is no longer carried,
// so roles may raise it again and it does not block.
func TestReviewCarried_AnEpochCutsTheRow(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{
		ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{}, []string{}),
		Epochs:       []Epoch{{AfterReviewRound: 1, Evidence: "rewrote §2"}},
	}

	assert.Empty(t, ReviewCarriedBlocking(spec, st, ReviewConvergeOnBlocking))
	assert.True(t, ReviewConverged(spec, st, 2, "h", ReviewConvergeOnBlocking))
}

// TestReviewCarried_CapDoesNotEndTheLoopPastACarriedRow: at the cap with the
// latest round fully dispositioned, a carried blocking row still keeps the
// loop open.
func TestReviewCarried_CapDoesNotEndTheLoopPastACarriedRow(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{}, []string{})}

	d := ReviewLoopDone(spec, st, 2, 3, "h", ReviewConvergeOnBlocking)
	assert.True(t, d.CapReached)
	assert.False(t, d.Done)
	assert.Empty(t, d.By)
	assert.Len(t, d.CarriedBlocking, 1)
}

// A fix of carriedOpen written while three review rounds were recorded.
const carriedFixedAfter3 = `{"severity":"high","category":"clarity","location":"§2","finding":"Rule 1 is ambiguous","resolved":{"status":"fixed","evidence":"reworded","resolved_after_round":3}}`

// TestReviewCarried_AFixCountsOnceARoundVerifiesIt (§2.6): a blocking row
// marked fixed keeps blocking until a round numbered above the fix's order is
// recorded, at the cap too; that round verifies it.
func TestReviewCarried_AFixCountsOnceARoundVerifiesIt(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{carriedFixedAfter3}, []string{}, []string{}, []string{})

	st := &ReviewState{ReviewRounds: rounds[:3]}
	d := ReviewLoopDone(spec, st, 2, 3, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done, "no round read the fix, and the cap does not end the loop past it")
	require.Len(t, d.CarriedBlocking, 1)
	assert.Equal(t, CarriedStateFixedUnverified, d.CarriedBlocking[0].State())

	st = &ReviewState{ReviewRounds: rounds}
	d = ReviewLoopDone(spec, st, 2, 0, "h", ReviewConvergeOnBlocking)
	assert.Empty(t, d.CarriedBlocking, "round 4 was emitted after the fix and verifies it")
	assert.True(t, d.Done)
}

// TestReviewCarried_AFixWithNoRecordedOrderIsVerifiedByALaterRound: a
// disposition with no recorded order counts as verified if a round later than
// its row's round is recorded.
func TestReviewCarried_AFixWithNoRecordedOrderIsVerifiedByALaterRound(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	rounds := loopRounds(t, spec, []string{carriedFixed}, []string{})

	assert.Len(t, ReviewCarriedBlocking(spec, &ReviewState{ReviewRounds: rounds[:1]}, ReviewConvergeOnBlocking), 1,
		"no later round: the fix is unverified")
	assert.Empty(t, ReviewCarriedBlocking(spec, &ReviewState{ReviewRounds: rounds}, ReviewConvergeOnBlocking),
		"round 2 is later than the row's round")
}

// TestReviewCarried_ALaterLowCopyDoesNotDowngradeAHigh: a finding graded high
// and re-reported low, neither resolved, blocks at the highest severity since
// its last resolving disposition.
func TestReviewCarried_ALaterLowCopyDoesNotDowngradeAHigh(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	low := `{"severity":"low","category":"clarity","location":"§2","finding":"Rule 1 is ambiguous"}`
	st := &ReviewState{ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{low}, []string{})}

	d := ReviewLoopDone(spec, st, 2, 0, "h", ReviewConvergeOnBlocking)
	assert.False(t, d.Done)
	require.Len(t, d.CarriedBlocking, 1)
	assert.Equal(t, "high", d.CarriedBlocking[0].Severity)
}

// TestReviewCarried_TheCapEndingPastAnEpochCutSaysSo: a loop the cap ended
// after an epoch cut its unresolved blocking row reports that the epoch, not a
// disposition, let it end.
func TestReviewCarried_TheCapEndingPastAnEpochCutSaysSo(t *testing.T) {
	t.Parallel()
	spec := filepath.Join(t.TempDir(), "spec.md")
	st := &ReviewState{
		ReviewRounds: loopRounds(t, spec, []string{carriedOpen}, []string{mediumOpen}, []string{mediumWontfix}),
		Epochs:       []Epoch{{AfterReviewRound: 1, Evidence: "rewrote §2"}},
	}
	d := ReviewLoopDone(spec, st, 2, 3, "changed", ReviewConvergeOnBlocking)
	require.True(t, d.Done)
	assert.Equal(t, DoneByCap, d.By)
	assert.True(t, d.EpochCutBlocking)
	assert.NotContains(t, ReviewNextAction("spec.md", d, false, nil, "", CheckVerdict{}), "every finding dispositioned")
}
