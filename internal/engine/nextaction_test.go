package engine

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// The review precedence is total over reachable states: exactly one branch fires
// per state, in the fixed order done > at-cap > blocking > mechanize > next-round.
// Non-Goal 7 keeps the verbatim per-branch string out of the acceptance, so these
// assert the branch/command kind, not full-byte equality.

var (
	doneConverged = LoopDone{Done: true, By: DoneByConverged}
	doneAtCap     = LoopDone{Done: true, By: DoneByCap, CapReached: true}
	openAtCap     = LoopDone{CapReached: true, Undisposed: 2}
	notDone       = LoopDone{}
)

const roundFile = ".tp-review/spec/review-round-3.ndjson"

func TestReviewNextAction_Converged(t *testing.T) {
	got := ReviewNextAction("spec.md", doneConverged, false, nil, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp import spec.tasks.json", "branch 1 names the decompose-then-import forward step")
	assert.NotContains(t, got, "--resolve", "branch 1 never advises disposal")
}

// TestReviewNextAction_ConvergedWinsOverEverything: converged is the highest
// precedence — even with a blocking finding and a mechanize class present (an
// unreachable overlap in practice, but it pins the ordering), branch 1 wins.
func TestReviewNextAction_ConvergedWinsOverEverything(t *testing.T) {
	got := ReviewNextAction("spec.md", doneConverged, true /*blocking*/, []string{"naming"}, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp import", "converged outranks blocking and mechanize")
	assert.NotContains(t, got, "revise the spec")
	assert.NotContains(t, got, "tp set --workflow")
}

// TestReviewNextAction_DoneAtTheCap: a loop the cap ended goes forward like a
// converged one, and says why, since no round graded it clean twice.
func TestReviewNextAction_DoneAtTheCap(t *testing.T) {
	got := ReviewNextAction("spec.md", doneAtCap, false, nil, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp import spec.tasks.json")
	assert.Contains(t, got, "cap")
}

// TestReviewNextAction_AtTheCap names the only way out at the cap: disposition
// what is left in the recorded round file, never another round.
func TestReviewNextAction_AtTheCap(t *testing.T) {
	got := ReviewNextAction("spec.md", openAtCap, true /*blocking*/, []string{"naming"}, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp review "+roundFile+" --resolve", "the command names the recorded round file")
	assert.Contains(t, got, "operator", "accepting a blocking finding is named as the operator's decision")
	assert.Contains(t, got, "tp import spec.tasks.json", "and the step after it")
	assert.NotContains(t, got, "run the next review round", "the cap admits no further round")
	assert.NotContains(t, got, "tp set --workflow", "the cap outranks the mechanize advice")
}

// TestReviewNextAction_Blocking names disposition as an exit equal to a spec
// change: a finding leaves a round either as an edit or as a --resolve, and a
// directive naming only the edit is what grew every spec it touched.
func TestReviewNextAction_Blocking(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, true /*blockingUnresolved*/, []string{"naming"}, roundFile, CheckVerdict{})
	assert.Contains(t, got, "revise the spec", "branch 2 still names the spec change")
	assert.Contains(t, got, "tp review "+roundFile+" --resolve", "and names disposition beside it")
	assert.Contains(t, got, "operator", "accepting a blocking finding is the operator's decision")
	assert.NotContains(t, got, "--resolve-all", "blanket disposal of blocking findings is never advised")
	assert.NotContains(t, got, "--verify")
}

func TestReviewNextAction_Mechanize(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, false, []string{"naming"}, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp set --workflow checks", "branch 3 names the register-a-check command")
	assert.Contains(t, got, "naming", "branch 3 names the recurring class")
	assert.Contains(t, got, "tp review spec.md --record", "branch 3 is compound: register, then next round")
}

// TestReviewNextAction_OverSpecificationExcluded: a recurring over-specification
// class is un-mechanizable, so branch 3 does NOT fire on it — the state falls
// through to branch 4's plain next-round command.
func TestReviewNextAction_OverSpecificationExcluded(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, false, []string{"over-specification"}, roundFile, CheckVerdict{})
	assert.NotContains(t, got, "tp set --workflow checks", "over-specification does not trigger branch 3")
	assert.Contains(t, got, "run the next review round", "falls through to branch 4")
	assert.Contains(t, got, "tp review spec.md --record")
}

// TestReviewNextAction_MechanizeSkipsOverSpecToNextClass: over-specification is
// skipped, but a genuinely mechanizable class in the same list still fires
// branch 3 (firstMechanizableClass picks it).
func TestReviewNextAction_MechanizeSkipsOverSpecToNextClass(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, false, []string{"over-specification", "naming"}, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp set --workflow checks")
	assert.Contains(t, got, "naming")
}

// TestReviewNextAction_MechanizePhaseQualifier guards §8a.2: branch 3 names the
// class AND states that a check is only worth registering when the artifact it
// measures already exists in the review phase. The qualifier belongs to that
// branch alone — no other reachable state advises registering a check, so no
// other state may carry the qualifier.
func TestReviewNextAction_MechanizePhaseQualifier(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, false, []string{"naming"}, roundFile, CheckVerdict{})
	assert.Contains(t, got, "naming", "the qualified advice still names the recurring class")
	assert.Contains(t, got, MechanizePhaseQualifier,
		"branch 3 qualifies the registration by phase")

	for name, other := range map[string]string{
		"branch 1 (converged)":          ReviewNextAction("spec.md", doneConverged, false, []string{"naming"}, roundFile, CheckVerdict{}),
		"at the cap":                    ReviewNextAction("spec.md", openAtCap, false, []string{"naming"}, roundFile, CheckVerdict{}),
		"branch 2 (blocking)":           ReviewNextAction("spec.md", notDone, true, []string{"naming"}, roundFile, CheckVerdict{}),
		"branch 4 (no class)":           ReviewNextAction("spec.md", notDone, false, nil, roundFile, CheckVerdict{}),
		"branch 4 (over-specification)": ReviewNextAction("spec.md", notDone, false, []string{"over-specification"}, roundFile, CheckVerdict{}),
	} {
		assert.NotContains(t, other, MechanizePhaseQualifier,
			"%s advises no registration, so it carries no registration qualifier", name)
	}
}

func TestReviewNextAction_CleanNotConverged(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, false, nil, roundFile, CheckVerdict{})
	assert.Contains(t, got, "run the next review round", "branch 4 is the lowest-precedence default")
	assert.Contains(t, got, "tp review spec.md --record <file>")
	assert.NotContains(t, got, "tp set --workflow", "no mechanize class present")
	// <file> stays a literal placeholder; <spec> is resolved.
	assert.Contains(t, got, "<file>")
}

// TestReviewNextAction_BaseResolution: <base> resolves to the spec's base name
// even for a pathed, dotted spec name.
func TestReviewNextAction_BaseResolution(t *testing.T) {
	got := ReviewNextAction("spec/0.31.0.md", doneConverged, false, nil, roundFile, CheckVerdict{})
	assert.Contains(t, got, "tp import 0.31.0.tasks.json")
}

// Audit precedence: done > at-cap > latest round unclean > next-round. Since
// v0.37.0 §2 the branch input is the round's `clean` verdict and the non-PASS
// count is a separate argument, because `blocking` separates them.

const auditRoundFile = ".tp-review/spec/audit-round-3.ndjson"

func TestAuditNextAction_Converged(t *testing.T) {
	got := AuditNextAction("spec.md", doneConverged, true /*clean*/, 0, auditRoundFile)
	assert.Contains(t, got, "proceed to release", "converged names the terminal release marker")
	assert.NotContains(t, got, "tp audit", "the terminal marker names no further tp command")
}

func TestAuditNextAction_DoneAtTheCap(t *testing.T) {
	got := AuditNextAction("spec.md", doneAtCap, false, 1, auditRoundFile)
	assert.Contains(t, got, "proceed to release")
	assert.Contains(t, got, "cap", "it says the cap ended the loop, not two clean rounds")
	assert.NotContains(t, got, "converged —")
}

func TestAuditNextAction_AtTheCap(t *testing.T) {
	got := AuditNextAction("spec.md", openAtCap, false, 2, auditRoundFile)
	assert.Contains(t, got, "tp audit "+auditRoundFile+" --resolve", "the command names the recorded round file")
	assert.Contains(t, got, "operator")
	assert.NotContains(t, got, "--record", "the cap admits no further round")
}

func TestAuditNextAction_CleanNotConverged(t *testing.T) {
	got := AuditNextAction("spec.md", notDone, true /*clean*/, 0 /*no non-PASS rows*/, auditRoundFile)
	assert.Contains(t, got, "run the next audit round")
	assert.Contains(t, got, "tp audit spec.md --record <file>")
}

func TestAuditNextAction_NonPassRowsPresent(t *testing.T) {
	got := AuditNextAction("spec.md", notDone, false /*unclean*/, 1, auditRoundFile)
	assert.Contains(t, got, "address the findings", "names the fix-and-re-audit directive")
	assert.Contains(t, got, "tp audit spec.md --record <file>")
	assert.Contains(t, got, "tp audit "+auditRoundFile+" --resolve", "and a finding needing no code change can be dispositioned")
}

// TestAuditNextAction_ConvergedWinsOverFindings pins the audit ordering: a
// converged state names the forward step even if the round is (unreachably) unclean.
func TestAuditNextAction_ConvergedWinsOverFindings(t *testing.T) {
	got := AuditNextAction("spec.md", doneConverged, false, 1, auditRoundFile)
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
		name  string
		done  LoopDone
		empty string
	}{
		{"converged", doneConverged, AuditNextAction("spec.md", doneConverged, true, 0, auditRoundFile)},
		{"clean not converged", notDone, AuditNextAction("spec.md", notDone, true, 0, auditRoundFile)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := AuditNextAction("spec.md", tc.done, true /*clean*/, 3, auditRoundFile)
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
	assert.Contains(t, AuditNextAction("spec.md", notDone, true, 1, auditRoundFile), "1 accepted row ")
	assert.Contains(t, AuditNextAction("spec.md", notDone, true, 2, auditRoundFile), "2 accepted rows")
	assert.NotContains(t, AuditNextAction("spec.md", notDone, true, 0, auditRoundFile), "accepted",
		"an empty round names no count at all rather than naming zero")
	assert.NotContains(t, AuditNextAction("spec.md", doneConverged, true, 0, auditRoundFile), "accepted")
}

// TestAuditNextAction_UncleanRoundIsSilentAboutTheCount pins the boundary of
// this release: §2 changes two of the three branches, and the fix-and-re-audit
// branch is not one of them. A count rendered there would be reporting rows the
// directive is already sending the reader to.
func TestAuditNextAction_UncleanRoundIsSilentAboutTheCount(t *testing.T) {
	assert.Equal(t,
		AuditNextAction("spec.md", notDone, false, 1, auditRoundFile),
		AuditNextAction("spec.md", notDone, false, 7, auditRoundFile))
}

// TestFirstMechanizableClass covers the over-specification skip directly.
func TestFirstMechanizableClass(t *testing.T) {
	assert.Equal(t, "naming", firstMechanizableClass([]string{"naming"}))
	assert.Equal(t, "naming", firstMechanizableClass([]string{"over-specification", "naming"}))
	assert.Equal(t, "", firstMechanizableClass([]string{"over-specification"}))
	assert.Equal(t, "", firstMechanizableClass(nil))
	assert.Equal(t, OverSpecificationClass, "over-specification")
	// sanity: none of the review directives leak an audit command and vice versa.
	assert.False(t, strings.Contains(ReviewNextAction("s.md", notDone, false, nil, roundFile, CheckVerdict{}), "tp audit"))
}

// TestNextAction_ABlockingFixedAtTheCap: the one state at the cap only the
// operator can end is named as such in both phases, with the acceptance
// command pointed at the recorded round file.
func TestNextAction_ABlockingFixedAtTheCap(t *testing.T) {
	d := LoopDone{CapReached: true, BlockingFixedAtCap: 2}
	review := ReviewNextAction("spec.md", d, true, nil, roundFile, CheckVerdict{})
	assert.Contains(t, review, "2 blocking findings were marked fixed")
	assert.Contains(t, review, "raises the cap by one")
	assert.Contains(t, review, "tp review "+roundFile+" --resolve")

	audit := AuditNextAction("spec.md", LoopDone{CapReached: true, BlockingFixedAtCap: 1}, false, 1, auditRoundFile)
	assert.Contains(t, audit, "1 blocking finding was marked fixed")
	assert.Contains(t, audit, "tp audit "+auditRoundFile+" --resolve")
}

// failingCheck is a verdict holding one registered check that could not run.
var failingCheck = CheckVerdict{Failing: []CheckFailure{{Class: "naming", Outcome: "exited 2"}}}

// TestReviewNextAction_AFailingCheckReplacesEveryImportStep: every branch that
// names import — done by convergence, done by the cap, and both cap states —
// names the failing check instead, because `--status --check` exits 1 on it.
func TestReviewNextAction_AFailingCheckReplacesEveryImportStep(t *testing.T) {
	for name, d := range map[string]LoopDone{
		"converged":                 doneConverged,
		"done at the cap":           doneAtCap,
		"open at the cap":           openAtCap,
		"blocking fixed at the cap": {CapReached: true, BlockingFixedAtCap: 1},
	} {
		got := ReviewNextAction("spec.md", d, false, nil, roundFile, failingCheck)
		assert.NotContains(t, got, "tp import", "%s: no import step while a check fails", name)
		assert.NotContains(t, got, "decompose", name)
		assert.Contains(t, got, `fix the registered "naming" check — it exited 2`, name)
		assert.Contains(t, got, "tp review spec.md --status --check", "%s: it names the gate the check holds shut", name)
	}
}

// TestReviewNextAction_AFailingCheckLeavesTheLoopBranchesAlone: below done and
// the cap no step names import, so the loop's own next step still comes first.
func TestReviewNextAction_AFailingCheckLeavesTheLoopBranchesAlone(t *testing.T) {
	assert.Equal(t,
		ReviewNextAction("spec.md", notDone, true, nil, roundFile, CheckVerdict{}),
		ReviewNextAction("spec.md", notDone, true, nil, roundFile, failingCheck), "blocking")
	assert.Equal(t,
		ReviewNextAction("spec.md", notDone, false, nil, roundFile, CheckVerdict{}),
		ReviewNextAction("spec.md", notDone, false, nil, roundFile, failingCheck), "next round")
}

// TestReviewNextAction_ACandidateWhoseCheckDidNotRun: a recurring class whose
// registered check did not run is named with that check to fix, never with a
// check to register; another candidate class still gets the register advice.
func TestReviewNextAction_ACandidateWhoseCheckDidNotRun(t *testing.T) {
	got := ReviewNextAction("spec.md", notDone, false, []string{"naming"}, roundFile, failingCheck)
	assert.Contains(t, got, `fix the registered "naming" check — it exited 2`)
	assert.NotContains(t, got, "tp set --workflow", "a check already exists")
	assert.NotContains(t, got, MechanizePhaseQualifier)
	assert.Contains(t, got, "tp review spec.md --record <file>", "then the next round")

	other := ReviewNextAction("spec.md", notDone, false, []string{"other"}, roundFile, failingCheck)
	assert.Contains(t, other, "tp set --workflow checks", "a class with no registered check is still to be registered")
}

// TestReviewNextAction_UnverifiedChecksGateEveryImportStep: a caller that ran
// no registered check names --status --check ahead of each import step, and
// ahead of decomposition, rather than claiming the gate passes.
func TestReviewNextAction_UnverifiedChecksGateEveryImportStep(t *testing.T) {
	unverified := CheckVerdict{Unverified: true}
	for name, d := range map[string]LoopDone{
		"converged":       doneConverged,
		"done at the cap": doneAtCap,
		"open at the cap": openAtCap,
	} {
		got := ReviewNextAction("spec.md", d, false, nil, roundFile, unverified)
		gate := strings.Index(got, "tp review spec.md --status --check")
		assert.GreaterOrEqual(t, gate, 0, "%s: %s", name, got)
		assert.Less(t, gate, strings.Index(got, "tp import"), "%s: the gate comes before import", name)
		if at := strings.Index(got, "decompose"); at >= 0 {
			assert.Less(t, gate, at, "%s: and before decomposition", name)
		}
	}
	assert.Equal(t,
		ReviewNextAction("spec.md", notDone, false, nil, roundFile, CheckVerdict{}),
		ReviewNextAction("spec.md", notDone, false, nil, roundFile, unverified),
		"below done and the cap nothing names import, so nothing is gated")
}
