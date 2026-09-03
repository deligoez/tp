package engine

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
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

// TestEveryVerdictHasAWrittenAnswerToTheTierRule is the guard the map's own
// comment claims: every verdict's answer is written down, none is the zero
// value of a lookup.
//
// Without it a seventh verdict — or a sixth deleted from the map by an edit
// elsewhere — is silently exempt from §7.2's rule, which is the permissive
// direction and the one nothing else here would notice: TestTheTierRuleIsPerVerdict
// asserts on the four verdicts it names, and a verdict absent from both is
// absent from the record's constraints too.
func TestEveryVerdictHasAWrittenAnswerToTheTierRule(t *testing.T) {
	verdicts := GroundVerdicts()
	for _, v := range verdicts {
		_, listed := groundTierRuleBinds[v]
		assert.True(t, listed,
			"verdict %q has no entry in §7.2's per-verdict table, so its answer is a map default rather than a decision", v)
	}
	assert.Len(t, groundTierRuleBinds, len(verdicts),
		"the table answers §3's six verdicts and nothing else")
}

// TestThePerVerdictTableIsTheOneSection72States binds the map to the artifact
// it is a transcription of.
//
// The classification is read out of §7.2's own per-verdict table rather than
// restated here, so a spec edit moving a verdict between the bound and the
// exempt row fails on the map instead of leaving two documents disagreeing
// about which verdicts the record enforces. The verdicts are taken from the
// left column through ParseGroundVerdict, so the enum is what decides what
// counts as a verdict there rather than this file's idea of one.
//
// Each row must classify as exactly one of bound or exempt. A reworded right
// column that matches neither — or both — fails loudly here rather than being
// quietly read as exempt, which is the permissive direction.
func TestThePerVerdictTableIsTheOneSection72States(t *testing.T) {
	rows := groundSection72VerdictRule(t)
	require.NotEmpty(t, rows, "§7.2 must carry a table classifying verdicts by the tier rule")

	want := make(map[GroundVerdict]bool, len(GroundVerdicts()))
	for _, r := range rows {
		for _, v := range r.verdicts {
			_, seen := want[v]
			require.False(t, seen, "verdict %q is classified by two rows of §7.2's table", v)
			want[v] = r.binds
		}
	}

	// NOT-A-CLAIM is the one verdict the table does not name, and its absence
	// is what makes it exempt: §7.2 makes `kind` and `tier` optional there, and
	// a row asserting nothing about the world has nothing for a tier to be
	// evidence about. Asserting the absence keeps that reading honest — if the
	// spec ever gives NOT-A-CLAIM a row, this stops being an inference.
	_, named := want[VerdictNotAClaim]
	require.False(t, named,
		"NOT-A-CLAIM is exempt because §7.2's per-verdict table does not name it; a row for it would have to be read, not inferred")
	want[VerdictNotAClaim] = false

	require.Len(t, want, len(GroundVerdicts()),
		"§7.2's table plus the one verdict it omits must account for all six")
	for _, v := range GroundVerdicts() {
		assert.Equal(t, want[v], groundTierRuleBinds[v],
			"§7.2 and this package disagree about whether the tier rule binds %q", v)
	}
}

// groundVerdictRuleRow is one row of §7.2's per-verdict table: the verdicts its
// left column names, and whether its right column binds them to §4.1's sets.
type groundVerdictRuleRow struct {
	verdicts []GroundVerdict
	binds    bool
}

// groundSection72VerdictRule reads §7.2's per-verdict table out of the spec.
//
// It selects rows structurally rather than by the table's header: within §7.2,
// a table row whose first cell names at least one of §3's verdicts is a row of
// this table, and the field table above it — whose first cells are lowercase
// field names — has none. So a reworded header does not hide the table.
func groundSection72VerdictRule(t *testing.T) []groundVerdictRuleRow {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "spec", "1.0.0.md"))
	require.NoError(t, err, "this rule is §7.2's table; the spec must be readable for that to be checkable")

	lines := strings.Split(string(data), "\n")
	start := slices.Index(lines, "### 7.2 The row")
	require.GreaterOrEqual(t, start, 0, "§7.2 must be findable by its heading")
	end := start + 1
	for end < len(lines) && !strings.HasPrefix(lines[end], "## ") && !strings.HasPrefix(lines[end], "### ") {
		end++
	}

	token := regexp.MustCompile("`([A-Z][A-Z-]*)`")
	rows := make([]groundVerdictRuleRow, 0, 3)
	for _, line := range lines[start:end] {
		if !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.SplitN(strings.TrimPrefix(line, "|"), "|", 2)
		if len(cells) != 2 {
			continue
		}
		named := make([]GroundVerdict, 0, 3)
		for _, m := range token.FindAllStringSubmatch(cells[0], -1) {
			if v, ok := ParseGroundVerdict(m[1]); ok {
				named = append(named, v)
			}
		}
		if len(named) == 0 {
			continue
		}
		binds := strings.Contains(cells[1], "**must** be acceptable")
		exempt := strings.Contains(cells[1], "no constraint") ||
			strings.Contains(cells[1], "no acceptability constraint")
		require.NotEqual(t, binds, exempt,
			"§7.2's row for %v classifies as neither bound nor exempt, or as both: %q", named, cells[1])
		rows = append(rows, groundVerdictRuleRow{verdicts: named, binds: binds})
	}
	return rows
}

// TestAVerdictOutsideTheSixIsHeldToTheTierRule pins the direction the lookup
// fails in, the way TestAnUnknownKindOrTierIsNeverAcceptable does for the sets.
//
// No such row reaches the validator — ParseGroundVerdict closes the enum before
// it — so this calls the predicate directly. It is held to the rule rather than
// waived from it, and both halves are asserted: an unrecognised verdict is not
// rejected outright, it is required to have reached a tier the kind accepts.
// The permissive default would be the other way round, and a map lookup's zero
// value is exactly that default.
func TestAVerdictOutsideTheSixIsHeldToTheTierRule(t *testing.T) {
	unmatched := &GroundRow{Verdict: "SKIP", Kind: KindBehaviour, Tier: TierRead}
	require.Error(t, validateGroundRowTier(unmatched),
		"a verdict the per-verdict table does not answer is held to the rule, not waived from it")

	matched := &GroundRow{Verdict: "SKIP", Kind: KindBehaviour, Tier: TierRun}
	require.NoError(t, validateGroundRowTier(matched),
		"and held to the rule means exactly that: an acceptable tier still passes")
}
