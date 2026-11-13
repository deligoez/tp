package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// earlierHigh is a carried blocking row from round 1, before the latest.
func earlierHigh() CarriedRow {
	return CarriedRow{Round: 1, File: ".tp-review/spec/review-round-1.ndjson", Index: 0, Severity: "high",
		Row: map[string]any{"severity": "high", "location": "§2", "finding": "Rule 1 is ambiguous"}, Disposition: CarryUnresolved, Earlier: true}
}

// TestReviewNextAction_LatestRoundBranchesCountTheCarriedRows: the cap and
// blocking branches, which name the latest round's file, also count the
// carried blocking rows of earlier rounds, so they are not found a step later.
func TestReviewNextAction_LatestRoundBranchesCountTheCarriedRows(t *testing.T) {
	t.Parallel()
	carried := []CarriedRow{earlierHigh()}
	capped := ReviewNextAction("spec.md", LoopDone{CapReached: true, Undisposed: 1, CarriedBlocking: carried}, true, nil, "r3.ndjson", CheckVerdict{})
	assert.Contains(t, capped, "1 carried blocking finding from earlier rounds")
	open := ReviewNextAction("spec.md", LoopDone{CarriedBlocking: carried}, true, nil, "r3.ndjson", CheckVerdict{})
	assert.Contains(t, open, "1 carried blocking finding from earlier rounds")
}

// TestReviewNextAction_TheCarriedCommandEndsItsClause: the row's aside comes
// before the command, and the clause ends right after "<evidence>".
func TestReviewNextAction_TheCarriedCommandEndsItsClause(t *testing.T) {
	t.Parallel()
	next := ReviewNextAction("spec.md", LoopDone{CarriedBlocking: []CarriedRow{earlierHigh()}}, false, nil, "r3.ndjson", CheckVerdict{})
	aside := strings.Index(next, "high §2 in round 1")
	cmd := strings.Index(next, "tp review .tp-review/spec/review-round-1.ndjson --resolve 0 fixed|wontfix|duplicate \"<evidence>\";")
	assert.Positive(t, aside, next)
	assert.Greater(t, cmd, aside, "the aside precedes the command: %s", next)
}

// TestReviewNextAction_AFixAwaitingARoundNamesTheRound: a carried row marked
// fixed and unverified is cleared by the next round, below the cap, and by
// the operator's raise at it; the resolve command is never offered for it.
func TestReviewNextAction_AFixAwaitingARoundNamesTheRound(t *testing.T) {
	t.Parallel()
	fixed := earlierHigh()
	fixed.Disposition = CarryResolved
	below := ReviewNextAction("spec.md", LoopDone{CarriedBlocking: []CarriedRow{fixed}}, false, nil, "r3.ndjson", CheckVerdict{})
	assert.Contains(t, below, "run the next review round")
	assert.NotContains(t, below, "--resolve")
	atCap := ReviewNextAction("spec.md", LoopDone{CapReached: true, CarriedBlocking: []CarriedRow{fixed}}, false, nil, "r3.ndjson", CheckVerdict{})
	assert.Contains(t, atCap, EscalateRaiseReviewCap)
	assert.NotContains(t, atCap, "--resolve")
	assert.NotContains(t, atCap, "tp import")
}

// TestCarriedBlockingClause_CountsAndWordsEachKind: the clause counts the
// unresolved rows and the fixed ones apart, words each count in the singular
// or the plural, points past the first unresolved row only when there are
// more, and leaves out a kind that has no rows.
func TestCarriedBlockingClause_CountsAndWordsEachKind(t *testing.T) {
	t.Parallel()
	fixed := earlierHigh()
	fixed.Disposition = CarryResolved

	one := CarriedBlockingClause([]CarriedRow{earlierHigh()}, "spec.md", false)
	assert.Contains(t, one, "1 carried blocking finding stands unresolved, and the next round's roles are told not to re-report it:")
	assert.Contains(t, one, "an epoch cuts it instead")
	assert.NotContains(t, one, "more are listed")
	assert.NotContains(t, one, "marked fixed")

	mixed := CarriedBlockingClause([]CarriedRow{earlierHigh(), earlierHigh(), fixed}, "spec.md", false)
	assert.Contains(t, mixed, "2 carried blocking findings stand unresolved, and the next round's roles are told not to re-report them:")
	assert.Contains(t, mixed, "; 1 more are listed by tp review spec.md --status")
	assert.Contains(t, mixed, "an epoch cuts them instead")
	assert.Contains(t, mixed, "; 1 carried blocking finding marked fixed awaits a review round that reads the fix")

	twoFixed := CarriedBlockingClause([]CarriedRow{fixed, fixed}, "spec.md", false)
	assert.True(t, strings.HasPrefix(twoFixed, "2 carried blocking findings marked fixed await a review round that reads the fix"), twoFixed)
	assert.NotContains(t, twoFixed, "unresolved")
}
