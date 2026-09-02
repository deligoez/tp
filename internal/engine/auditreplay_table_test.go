package engine

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// auditReplayNever is the convergence round of a cycle that never reaches the
// required trailing run of clean rounds — §2.1's "never" cell. Zero is not a
// legal round ordinal (loadAuditReplayRounds requires 1 or above), so it cannot
// be confused with an answer.
const auditReplayNever = 0

// auditReplayConvergence replays one recorded cycle and returns the round it
// converges at under convergeOn, or auditReplayNever.
//
// Two rules from §2.1 are encoded here rather than left to the callers.
//
// consecutive_clean is a *trailing* run: an unclean round resets it, and
// convergence is the first round at which the run reaches cleanRounds. Counting
// the run as a running total instead is §7 row 11's rejected mutant, and it is
// rejected because it cannot fail — a total and a trailing run first reach
// cleanRounds at the same round, since they can only diverge after a clean round
// has been followed by an unclean one, which is past the first convergence.
//
// resolvesAt is the first round graded under convergeOn, and it is part of the
// answer rather than an implementation detail: §2 stamps `clean` at record time,
// so a round recorded before the knob is set keeps for ever the verdict `all`
// gave it. The table's cells are the resolvesAt 1 column — the field resolves
// before round 1 is recorded — which is §2.1's stated precondition and what
// TestAuditReplayCellsNeedTheFieldResolvedBeforeRoundOne measures.
func auditReplayConvergence(rounds [][]map[string]any, convergeOn string, cleanRounds, resolvesAt int) int {
	streak := 0
	for i, rows := range rounds {
		policy := AuditConvergeOnAll
		if i+1 >= resolvesAt {
			policy = convergeOn
		}
		if AuditRowsClean(rows, policy) {
			streak++
		} else {
			streak = 0
		}
		if streak >= cleanRounds {
			return i + 1
		}
	}
	return auditReplayNever
}

// auditReplayRank orders convergence answers worst-last so that two of them can
// be compared: a cycle that never converges is worse than one that converges on
// its final round. Ordering the raw values would rank auditReplayNever best.
func auditReplayRank(convergesAt, roundsRecorded int) int {
	if convergesAt == auditReplayNever {
		return roundsRecorded + 1
	}
	return convergesAt
}

// auditReplayTable is §2.1's table, plus the sentence beneath it that gives the
// same two cycles at audit_clean_rounds 1. The `all` column is what tp counts
// today and the `blocking` column is what this release offers; the gap between
// them is the whole of §2.1's argument, and this file is its only guard.
var auditReplayTable = []struct {
	cycle       string
	cleanRounds int
	blocking    int
	all         int
}{
	{cycle: "0.35.0", cleanRounds: 2, blocking: 3, all: 9},
	{cycle: "0.36.0", cleanRounds: 2, blocking: 3, all: auditReplayNever},
	{cycle: "0.35.0", cleanRounds: 1, blocking: 2, all: 8},
	{cycle: "0.36.0", cleanRounds: 1, blocking: 2, all: auditReplayNever},
}

// TestAuditReplayReproducesTheTableThatDecidesTheDefault replays tp's own two
// recorded audit cycles through engine's clean predicate and requires the eight
// cells §2.1 states. §2.1 is where the release argues its default, and the
// argument is arithmetic over these rounds — so a predicate that graded severity
// differently would move the table without moving a word of the prose.
func TestAuditReplayReproducesTheTableThatDecidesTheDefault(t *testing.T) {
	for _, tc := range auditReplayTable {
		t.Run(fmt.Sprintf("v%s/audit_clean_rounds=%d", tc.cycle, tc.cleanRounds), func(t *testing.T) {
			rounds := loadAuditReplayRounds(t, tc.cycle)
			assert.Equal(t, tc.blocking,
				auditReplayConvergence(rounds, AuditConvergeOnBlocking, tc.cleanRounds, 1),
				"the round v%s converges at under blocking", tc.cycle)
			assert.Equal(t, tc.all,
				auditReplayConvergence(rounds, AuditConvergeOnAll, tc.cleanRounds, 1),
				"the round v%s converges at under all", tc.cycle)
		})
	}
}

