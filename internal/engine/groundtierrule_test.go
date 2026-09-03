package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// groundTierRuleRow builds a row carrying one (kind, tier, verdict) triple,
// adding whatever else that verdict requires so the triple is the only thing
// the row is rejected or accepted for.
//
// The base is the ordinary claim row, NOT-A-CLAIM included: §7.2 makes `kind`,
// `tier` and `evidence` optional there rather than forbidden, so one base
// serves all six verdicts and no case differs from another in more than the
// triple under test.
func groundTierRuleRow(t *testing.T, kind, tier, verdict string) []byte {
	t.Helper()
	set := map[string]any{
		"verdict":  verdict,
		"kind":     kind,
		"tier":     tier,
		"evidence": "what this tier was reached by",
	}
	switch verdict {
	case "PARTIAL":
		set["partial_kind"] = "two-readings"
	case "QUESTION":
		set["causes"] = groundThreeCauses()
	}
	return groundWireRow(t, groundClaimRow(), set, nil)
}

// TestTheTierRuleIsPerVerdict is §11 rows 6, 7 and 7b at the record boundary:
// §7.2's per-verdict table decides which verdicts have to satisfy §4.1's sets.
//
// Three mutants this table must fail, and the case that kills each:
//
//   - Requiring the MISMATCH for QUESTION — the mutant §11 row 7b names.
//     `{behaviour, run, QUESTION}` is the unit that ran the shipped command and
//     got an unclear result, which has no other legal verdict; that mutant
//     rejects it while accepting everything else here.
//   - Holding QUESTION to the rule the way PASS is held. Killed by the other
//     QUESTION case, `{behaviour, read, QUESTION}` — §3's `tier-unreached`
//     shape, which is the shape this release exists for.
//   - Binding the rule to every verdict, or to none. The first is killed by
//     both QUESTION cases and by `{mechanism, read, UNVERIFIABLE}`; the second
//     by the three rejected rows.
//
// PARTIAL and FAIL each carry both an accepted and a rejected case, so a mutant
// binding only PASS reddens on them: §7.2 puts PARTIAL inside the rule because
// outside it a unit that could not carry a FAIL at the required tier could
// downgrade to PARTIAL and keep a read standing in for a run.
//
// `matched` is asserted rather than assumed. It records what §4.1 says about
// each case's (kind, tier) pair, and the subtest checks it against
// TierAcceptableFor before touching the row — so a change to §4.1's sets that
// made `{behaviour, read}` acceptable would fail here loudly instead of turning
// every "unmatched" case into a row that tests nothing.
func TestTheTierRuleIsPerVerdict(t *testing.T) {
	cases := []struct {
		name     string
		kind     GroundKind
		tier     GroundTier
		verdict  GroundVerdict
		matched  bool
		accepted bool
	}{
		{"PASS on a tier the kind accepts", KindBehaviour, TierRun, VerdictPass, true, true},
		{"PASS on a tier the kind does not", KindBehaviour, TierRead, VerdictPass, false, false},
		{"PASS on a document read", KindDocument, TierRead, VerdictPass, true, true},
		{"PASS on a document run, which an ordering would admit", KindDocument, TierRun, VerdictPass, false, false},

		{"PARTIAL on a tier the kind accepts", KindDefect, TierRedGreen, VerdictPartial, true, true},
		{"PARTIAL on a tier the kind does not", KindDefect, TierRun, VerdictPartial, false, false},

		{"FAIL on a tier the kind accepts", KindCorpus, TierQuery, VerdictFail, true, true},
		{"FAIL on a tier the kind does not", KindCorpus, TierRead, VerdictFail, false, false},

		// §11 row 7b's own pair: QUESTION takes either relation, and which one
		// holds is the row's shape rather than its legality.
		{"QUESTION on a tier the kind accepts — ambiguous", KindBehaviour, TierRun, VerdictQuestion, true, true},
		{"QUESTION on a tier the kind does not — tier-unreached", KindBehaviour, TierRead, VerdictQuestion, false, true},

		// §3: `tier` records the deepest attempt, and the point of the verdict
		// is that no acceptable tier was reachable at all.
		{"UNVERIFIABLE naming the deepest attempt", KindMechanism, TierRead, VerdictUnverifiable, false, true},
		{"UNVERIFIABLE on a tier the kind accepts", KindMechanism, TierProbe, VerdictUnverifiable, true, true},

		// The verdict §7.2's per-verdict table does not name. The trio is
		// optional there and carries no acceptability constraint, so a mutant
		// that binds the rule to every verdict it can see reddens here.
		{"NOT-A-CLAIM carrying a kind and a tier that do not match", KindBehaviour, TierRead, VerdictNotAClaim, false, true},
		{"NOT-A-CLAIM carrying a kind and a tier that match", KindBehaviour, TierRun, VerdictNotAClaim, true, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.matched, TierAcceptableFor(tc.kind, tc.tier),
				"the case claims {%s, %s} is a %v under §4.1's sets; if that is no longer true the case tests nothing",
				tc.kind, tc.tier, tc.matched)

			line := groundTierRuleRow(t, string(tc.kind), string(tc.tier), string(tc.verdict))
			row, err := ParseGroundRow(line)
			if tc.accepted {
				require.NoError(t, err)
				assert.Equal(t, tc.tier, row.Tier, "an accepted row keeps the tier it recorded")
				return
			}

			require.Error(t, err)
			var rowErr *GroundRowError
			require.ErrorAs(t, err, &rowErr, "a cell failure is rejected as a GroundRowError")
			assert.Equal(t, "tier", rowErr.Field,
				"the cell that failed is `tier`: the kind is one of §4.1's seven and the tier one of its six")
		})
	}
}
