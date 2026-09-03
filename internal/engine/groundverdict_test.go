package engine

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAllSixVerdictsRoundTripIncludingNotAClaim is §11 row 5.
//
// The mutant it must fail is the earlier draft that shipped five verdicts,
// leaving a floor unit that is not a claim with no legal value — and so every
// spec permanently uncovered. Dropping any of the six from groundVerdictOrder
// makes ParseGroundVerdict answer false for it, and the listing's length wrong.
//
// "Round-trip" is asserted as the property and not as a spelling: the value
// that comes back out is compared against the string that went in, so a
// constant whose spelling drifts from the wire form fails here rather than at
// the first round nobody can read back.
func TestAllSixVerdictsRoundTripIncludingNotAClaim(t *testing.T) {
	for _, wire := range []string{"PASS", "PARTIAL", "FAIL", "UNVERIFIABLE", "QUESTION", "NOT-A-CLAIM"} {
		v, ok := ParseGroundVerdict(wire)
		require.True(t, ok, "%q must be one of §3's six verdicts", wire)
		assert.Equal(t, wire, string(v), "the parsed verdict must render back to the wire form byte for byte")
	}

	assert.Len(t, GroundVerdicts(), 6, "§3 names six verdicts")
	assert.Contains(t, GroundVerdicts(), VerdictNotAClaim,
		"NOT-A-CLAIM is a verdict, not an omission: without it a non-claim has no recordable value")
}

