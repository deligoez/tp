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

