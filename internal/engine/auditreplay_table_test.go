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

// TestAuditReplayCellsNeedTheFieldResolvedBeforeRoundOne measures §2.1's
// precondition rather than leaving it to the reader. Because §2 stamps `clean`
// at record time, a round recorded before the knob resolves keeps the verdict
// `all` gave it, so the round the field resolves at is an input to every cell
// above — and the cells are the best column of that input.
//
// The property asserted is that delaying resolution never converges earlier and
// somewhere converges later. §2.1 states this as an arithmetic floor —
// "setting it at round k converges no earlier than k + audit_clean_rounds" —
// and **that floor is false on this corpus under either reading of k**: v0.35.0
// rounds 8 and 9 are clean under `all` as well, so a replay resolving at round 9
// converges at round 8 for audit_clean_rounds 1 and at 9 for 2, both below the
// floor. The floor holds only while every pre-resolution round is unclean, which
// is a condition §2.1 does not state; monotonicity is the part that is true of
// the whole corpus, and it carries the same conclusion about the table.
//
// Monotonicity alone would also hold for a replay that ignored resolvesAt, so
// the strict-degradation guard is what makes this a test of the precondition.
func TestAuditReplayCellsNeedTheFieldResolvedBeforeRoundOne(t *testing.T) {
	for _, tc := range auditReplayTable {
		t.Run(fmt.Sprintf("v%s/audit_clean_rounds=%d", tc.cycle, tc.cleanRounds), func(t *testing.T) {
			rounds := loadAuditReplayRounds(t, tc.cycle)
			n := len(rounds)

			best := auditReplayConvergence(rounds, AuditConvergeOnBlocking, tc.cleanRounds, 1)
			require.Equal(t, tc.blocking, best,
				"the table's cell must be the column this test calls the best one")

			worse, previous := 0, auditReplayRank(best, n)
			for resolvesAt := 2; resolvesAt <= n; resolvesAt++ {
				got := auditReplayRank(
					auditReplayConvergence(rounds, AuditConvergeOnBlocking, tc.cleanRounds, resolvesAt), n)
				assert.GreaterOrEqualf(t, got, previous,
					"resolving at round %d converged earlier than resolving at round %d did",
					resolvesAt, resolvesAt-1)
				if got > auditReplayRank(best, n) {
					worse++
				}
				previous = got
			}
			require.NotZero(t, worse,
				"the precondition is untested unless some later resolution loses the table's cell")
		})
	}
}

// TestAuditReplayNeverCellIsNoRoundReachingZero pins the reason behind §2.1's
// two "never" cells. The table does not say v0.36.0 ran out of rounds; it says
// "no round reaches zero", and the two readings differ — a cycle whose final
// round were clean would converge under `all` had it run one round longer.
// Asserting the reason keeps the cell from being satisfied by the wrong fact.
func TestAuditReplayNeverCellIsNoRoundReachingZero(t *testing.T) {
	for i, rows := range loadAuditReplayRounds(t, "0.36.0") {
		assert.Falsef(t, AuditRowsClean(rows, AuditConvergeOnAll),
			"v0.36.0 round %d holds no non-PASS row, so the cycle does reach zero", i+1)
	}

	// And the contrast the same predicate must draw: v0.35.0's `all` column is a
	// pair of rounds that do reach zero — round 8, its cell at
	// audit_clean_rounds 1, and round 9, its cell at 2.
	rounds := loadAuditReplayRounds(t, "0.35.0")
	require.Len(t, rounds, 9)
	assert.True(t, AuditRowsClean(rounds[7], AuditConvergeOnAll), "v0.35.0 round 8")
	assert.True(t, AuditRowsClean(rounds[8], AuditConvergeOnAll), "v0.35.0 round 9")
}
