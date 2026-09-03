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

// TestTheThreeEnumsRejectEverythingOutsideThem pins the closed half of "closed
// enum". A near-miss is rejected rather than trimmed or case-folded, and the
// rejected value comes back as the zero string so a caller that ignores the ok
// cannot end up holding a plausible-looking verdict.
func TestTheThreeEnumsRejectEverythingOutsideThem(t *testing.T) {
	for _, s := range []string{"", "pass", "Pass", " PASS", "PASS ", "NOT A CLAIM", "NOTACLAIM", "SKIP"} {
		v, ok := ParseGroundVerdict(s)
		assert.False(t, ok, "verdict %q is outside §3's six", s)
		assert.Empty(t, string(v), "a rejected verdict comes back empty, never partially parsed")
	}
	for _, s := range []string{"", "Document", "code structure", "behavior", "claim", "prose"} {
		k, ok := ParseGroundKind(s)
		assert.False(t, ok, "kind %q is outside §4.1's seven", s)
		assert.Empty(t, string(k))
	}
	for _, s := range []string{"", "Read", "red green", "redgreen", "break-and-controls", "inspect"} {
		tier, ok := ParseGroundTier(s)
		assert.False(t, ok, "tier %q is outside §4.1's six", s)
		assert.Empty(t, string(tier))
	}
}

// TestTheKindAndTierEnumsAreSevenAndSixInTableOrder pins the two cardinalities
// this task's title states, the §4.1 table order the listings render in, and
// that the listings are copies — a caller reordering what it got back must not
// reorder the package's own table for the next reader.
func TestTheKindAndTierEnumsAreSevenAndSixInTableOrder(t *testing.T) {
	assert.Equal(t, []GroundKind{
		KindDocument, KindCodeStructure, KindCorpus, KindBehaviour, KindMechanism, KindDefect, KindGuard,
	}, GroundKinds(), "§4.1's kind table, in its own order")
	assert.Equal(t, []GroundTier{
		TierRead, TierQuery, TierRun, TierProbe, TierRedGreen, TierBreakAndControl,
	}, GroundTiers(), "§4.1's tier table, in its own order — a printing order that carries no rank")

	GroundKinds()[0] = "mutated"
	GroundTiers()[0] = "mutated"
	GroundVerdicts()[0] = "mutated"
	assert.Equal(t, KindDocument, GroundKinds()[0])
	assert.Equal(t, TierRead, GroundTiers()[0])
	assert.Equal(t, VerdictPass, GroundVerdicts()[0])
}

// TestKindTierAcceptabilityIsASetAndNotAnOrder is §11 row 6.
//
// The mutant it must fail treats the tiers as ordered — read, query, run,
// probe, red-green, break-and-control — and accepts anything at or above the
// kind's first listed tier. Three of these five pairs cannot tell that mutant
// from the shipped rule; the last two are the whole point of the row, and each
// is named here as its own subtest so a failure says which reading broke:
//
//   - {document, run}: an ordering starts `document` at `read` and so admits
//     every tier above it. Under the sets, running a command says nothing about
//     what a text contains.
//   - {behaviour, probe}: an ordering starts `behaviour` at `run` and so admits
//     `probe`. Under the sets a probe is evidence about an artifact the unit
//     built, not about the shipped command's behaviour — which is the
//     falsifying instance §4 opens with, a `-type d` pipeline that read
//     perfectly and changed nothing when run.
func TestKindTierAcceptabilityIsASetAndNotAnOrder(t *testing.T) {
	cases := []struct {
		name string
		kind GroundKind
		tier GroundTier
		want bool
	}{
		{"behaviour+read is rejected: reading a command is not running it", KindBehaviour, TierRead, false},
		{"behaviour+run is accepted", KindBehaviour, TierRun, true},
		{"document+read is accepted", KindDocument, TierRead, true},
		{"document+run is rejected, which an ordering would admit", KindDocument, TierRun, false},
		{"behaviour+probe is rejected, which an ordering would admit", KindBehaviour, TierProbe, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, TierAcceptableFor(c.kind, c.tier))
		})
	}
}

// TestSetMembershipHoldsAtBothEdgesForOneAndTwoTierKinds is §11 row 7.
//
// The mutant it must fail accepts any tier for a kind listing more than one.
// That mutant passes every single-tier kind, so a table asserting only
// `document`, `corpus`, `mechanism`, `defect` and `guard` cannot see it: it is
// distinguished exclusively by a rejected tier on `code-structure` or
// `behaviour`. Membership is therefore asserted on both sides for a one-tier
// kind (`corpus`) and for the two-tier kind, with `code-structure`'s rejected
// `run` the pair that kills it.
func TestSetMembershipHoldsAtBothEdgesForOneAndTwoTierKinds(t *testing.T) {
	assert.False(t, TierAcceptableFor(KindCorpus, TierRead), "corpus rejects read: it is a query over the corpus")
	assert.True(t, TierAcceptableFor(KindCorpus, TierQuery))

	assert.True(t, TierAcceptableFor(KindCodeStructure, TierRead))
	assert.True(t, TierAcceptableFor(KindCodeStructure, TierQuery),
		"a call-graph or dead-code tool is a query over the tree")
	assert.False(t, TierAcceptableFor(KindCodeStructure, TierRun),
		"the two-tier kind still rejects a third tier — the pair that separates a set from 'any tier will do'")
}

// TestEveryOneOfTheFortyTwoKindTierPairsMatchesSection41 states §4.1's third
// column in full and walks the whole (kind, tier) space.
//
// Rows 6 and 7 pin the pairs their two named mutants turn on; this pins the
// other thirty-five, so an edit to any single cell of the table fails a test.
// The expected sets are written out here from §4.1 rather than derived from the
// implementation, and the walk is over GroundKinds() × GroundTiers() so a kind
// or tier added without an entry is a failure rather than an untested pair —
// §3's own claim that the space has forty-two pairs is asserted, not assumed.
// Membership on the expected side is the standard library's, so the expectation
// is never computed by anything the implementation shares.
func TestEveryOneOfTheFortyTwoKindTierPairsMatchesSection41(t *testing.T) {
	acceptable := map[GroundKind][]GroundTier{
		KindDocument:      {TierRead},
		KindCodeStructure: {TierRead, TierQuery},
		KindCorpus:        {TierQuery},
		KindBehaviour:     {TierRun, TierRedGreen},
		KindMechanism:     {TierProbe},
		KindDefect:        {TierRedGreen},
		KindGuard:         {TierBreakAndControl},
	}

	kinds, tiers := GroundKinds(), GroundTiers()
	require.Len(t, acceptable, len(kinds), "every kind in the enum needs a row in §4.1's table")

	pairs := 0
	for _, kind := range kinds {
		want, listed := acceptable[kind]
		require.True(t, listed, "kind %q has no §4.1 entry here", kind)
		for _, tier := range tiers {
			pairs++
			assert.Equal(t, slices.Contains(want, tier), TierAcceptableFor(kind, tier),
				"{%s, %s}", kind, tier)
		}
	}
	assert.Equal(t, 42, pairs, "§3 counts forty-two (kind, tier) pairs")
}

