package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The review precedence is total over reachable states: exactly one branch fires
// per state, in the fixed order converged > blocking > mechanize > next-round.
// Non-Goal 7 keeps the verbatim per-branch string out of the acceptance, so these
// assert the branch/command kind, not full-byte equality.

func TestReviewNextAction_Converged(t *testing.T) {
	got := ReviewNextAction("spec.md", true /*converged*/, false, nil)
	assert.Contains(t, got, "tp import spec.tasks.json", "branch 1 names the decompose-then-import forward step")
	assert.NotContains(t, got, "--resolve", "branch 1 never advises disposal")
}

// TestReviewNextAction_ConvergedWinsOverEverything: converged is the highest
// precedence — even with a blocking finding and a mechanize class present (an
// unreachable overlap in practice, but it pins the ordering), branch 1 wins.
func TestReviewNextAction_ConvergedWinsOverEverything(t *testing.T) {
	got := ReviewNextAction("spec.md", true /*converged*/, true /*blocking*/, []string{"naming"})
	assert.Contains(t, got, "tp import", "converged outranks blocking and mechanize")
	assert.NotContains(t, got, "revise the spec")
	assert.NotContains(t, got, "tp set --workflow")
}

func TestReviewNextAction_Blocking(t *testing.T) {
	got := ReviewNextAction("spec.md", false, true /*blockingUnresolved*/, []string{"naming"})
	assert.Contains(t, got, "revise the spec", "branch 2 names the revise-and-re-review directive")
	// The canonical response to a blocking finding is never auto-disposal.
	assert.NotContains(t, got, "--resolve")
	assert.NotContains(t, got, "--resolve-all")
	assert.NotContains(t, got, "--verify")
}

func TestReviewNextAction_Mechanize(t *testing.T) {
	got := ReviewNextAction("spec.md", false, false, []string{"naming"})
	assert.Contains(t, got, "tp set --workflow checks", "branch 3 names the register-a-check command")
	assert.Contains(t, got, "naming", "branch 3 names the recurring class")
	assert.Contains(t, got, "tp review spec.md --record", "branch 3 is compound: register, then next round")
}

// TestReviewNextAction_OverSpecificationExcluded: a recurring over-specification
// class is un-mechanizable, so branch 3 does NOT fire on it — the state falls
// through to branch 4's plain next-round command.
func TestReviewNextAction_OverSpecificationExcluded(t *testing.T) {
	got := ReviewNextAction("spec.md", false, false, []string{"over-specification"})
	assert.NotContains(t, got, "tp set --workflow checks", "over-specification does not trigger branch 3")
	assert.Contains(t, got, "run the next review round", "falls through to branch 4")
	assert.Contains(t, got, "tp review spec.md --record")
}

// TestReviewNextAction_MechanizeSkipsOverSpecToNextClass: over-specification is
// skipped, but a genuinely mechanizable class in the same list still fires
// branch 3 (firstMechanizableClass picks it).
func TestReviewNextAction_MechanizeSkipsOverSpecToNextClass(t *testing.T) {
	got := ReviewNextAction("spec.md", false, false, []string{"over-specification", "naming"})
	assert.Contains(t, got, "tp set --workflow checks")
	assert.Contains(t, got, "naming")
}

// TestReviewNextAction_MechanizePhaseQualifier guards §8a.2: branch 3 names the
// class AND states that a check is only worth registering when the artifact it
// measures already exists in the review phase. The qualifier belongs to that
// branch alone — no other reachable state advises registering a check, so no
// other state may carry the qualifier.
func TestReviewNextAction_MechanizePhaseQualifier(t *testing.T) {
	got := ReviewNextAction("spec.md", false, false, []string{"naming"})
	assert.Contains(t, got, "naming", "the qualified advice still names the recurring class")
	assert.Contains(t, got, MechanizePhaseQualifier,
		"branch 3 qualifies the registration by phase")

	for name, other := range map[string]string{
		"branch 1 (converged)":          ReviewNextAction("spec.md", true, false, []string{"naming"}),
		"branch 2 (blocking)":           ReviewNextAction("spec.md", false, true, []string{"naming"}),
		"branch 4 (no class)":           ReviewNextAction("spec.md", false, false, nil),
		"branch 4 (over-specification)": ReviewNextAction("spec.md", false, false, []string{"over-specification"}),
	} {
		assert.NotContains(t, other, MechanizePhaseQualifier,
			"%s advises no registration, so it carries no registration qualifier", name)
	}
}

func TestReviewNextAction_CleanNotConverged(t *testing.T) {
	got := ReviewNextAction("spec.md", false, false, nil)
	assert.Contains(t, got, "run the next review round", "branch 4 is the lowest-precedence default")
	assert.Contains(t, got, "tp review spec.md --record <file>")
	assert.NotContains(t, got, "tp set --workflow", "no mechanize class present")
	// <file> stays a literal placeholder; <spec> is resolved.
	assert.Contains(t, got, "<file>")
}

// TestReviewNextAction_BaseResolution: <base> resolves to the spec's base name
// even for a pathed, dotted spec name.
func TestReviewNextAction_BaseResolution(t *testing.T) {
	got := ReviewNextAction("spec/0.31.0.md", true, false, nil)
	assert.Contains(t, got, "tp import 0.31.0.tasks.json")
}

// Audit precedence: converged > latest round unclean > next-round. Since
// v0.37.0 §2 the branch input is the round's stamped `clean` verdict and the
// non-PASS count is a separate argument, because `blocking` separates them.

func TestAuditNextAction_Converged(t *testing.T) {
	got := AuditNextAction("spec.md", true /*converged*/, true /*clean*/, 0)
	assert.Contains(t, got, "proceed to release", "converged names the terminal release marker")
	assert.NotContains(t, got, "tp audit", "the terminal marker names no further tp command")
}

func TestAuditNextAction_CleanNotConverged(t *testing.T) {
	got := AuditNextAction("spec.md", false, true /*clean*/, 0 /*no non-PASS rows*/)
	assert.Contains(t, got, "run the next audit round")
	assert.Contains(t, got, "tp audit spec.md --record <file>")
}

func TestAuditNextAction_NonPassRowsPresent(t *testing.T) {
	got := AuditNextAction("spec.md", false, false /*unclean*/, 1)
	assert.Contains(t, got, "address the findings", "names the fix-and-re-audit directive")
	assert.Contains(t, got, "tp audit spec.md --record <file>")
}

// TestAuditNextAction_ConvergedWinsOverFindings pins the audit ordering: a
// converged state names the forward step even if the round is (unreachably) unclean.
func TestAuditNextAction_ConvergedWinsOverFindings(t *testing.T) {
	got := AuditNextAction("spec.md", true, false, 1)
	assert.Contains(t, got, "proceed to release")
	assert.NotContains(t, got, "address the findings")
}

// TestAuditNextAction_AcceptedCountOnBothChangedBranches is §7 rows 10 and 10b
// at the unit level: the two branches §2's table names each render the count as
// a numeral and each read differently from the same branch on an empty round.
// The count and the verdict are supplied independently here, which is the state
// `blocking` produces and `all` never can — a clean round holding rows.
func TestAuditNextAction_AcceptedCountOnBothChangedBranches(t *testing.T) {
	for _, tc := range []struct {
		name      string
		converged bool
		empty     string
	}{
		{"converged", true, AuditNextAction("spec.md", true, true, 0)},
		{"clean not converged", false, AuditNextAction("spec.md", false, true, 0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AuditNextAction("spec.md", tc.converged, true /*clean*/, 3)
			assert.Contains(t, got, "3", "the accepted count is rendered as a numeral")
			assert.NotEqual(t, tc.empty, got,
				"a round closing over accepted rows reads differently from an empty one")
		})
	}
}

// TestAuditNextAction_AcceptedCountAgreesWithItsNoun covers the one boundary the
// rendering has. A count is a plural by default and 1 is the value that makes
// the default wrong, so it is asserted rather than assumed; 0 is asserted from
// the other side, as the absence of the clause entirely.
func TestAuditNextAction_AcceptedCountAgreesWithItsNoun(t *testing.T) {
	assert.Contains(t, AuditNextAction("spec.md", false, true, 1), "1 accepted row ")
	assert.Contains(t, AuditNextAction("spec.md", false, true, 2), "2 accepted rows")
	assert.NotContains(t, AuditNextAction("spec.md", false, true, 0), "accepted",
		"an empty round names no count at all rather than naming zero")
	assert.NotContains(t, AuditNextAction("spec.md", true, true, 0), "accepted")
}

// TestFirstMechanizableClass covers the over-specification skip directly.
func TestFirstMechanizableClass(t *testing.T) {
	assert.Equal(t, "naming", firstMechanizableClass([]string{"naming"}))
	assert.Equal(t, "naming", firstMechanizableClass([]string{"over-specification", "naming"}))
	assert.Equal(t, "", firstMechanizableClass([]string{"over-specification"}))
	assert.Equal(t, "", firstMechanizableClass(nil))
	assert.Equal(t, OverSpecificationClass, "over-specification")
	// sanity: none of the review directives leak an audit command and vice versa.
	assert.False(t, strings.Contains(ReviewNextAction("s.md", false, false, nil), "tp audit"))
}
