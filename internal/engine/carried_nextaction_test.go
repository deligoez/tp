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

