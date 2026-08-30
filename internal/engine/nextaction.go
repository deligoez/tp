package engine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// OverSpecificationClass is the un-mechanizable finding class (§5.2). A recurring
// over-specification class may still appear in the frequency-only
// mechanize_candidates array, but next_action branch 3 does not act on it — it is
// excluded and the state falls through to branch 4 (§8.2).
const OverSpecificationClass = "over-specification"

// MechanizePhaseQualifier is §8a.2's phase qualifier on the mechanize advice.
// Registered workflow.checks run in the review phase only, so a check whose
// subject a later phase writes can never verify it — while tp still tells every
// reviewer to stop reporting the mechanized class, so registering early
// suppresses a finding class and verifies nothing. next_action's mechanize
// branch carries this sentence, and skills/tp/SKILL.md's mechanize-candidate
// rule and its next_action step 3 carry it verbatim, so the emitted advice and
// the documented rule cannot drift apart.
const MechanizePhaseQualifier = "only worth registering when the artifact it measures already exists in the review phase"

// ReviewNextAction returns the advisory next_action string for the review loop,
// chosen by the fixed §8.2 precedence, total over reachable states (first match
// wins):
//
//  1. converged → the phase's forward step: decompose, then
//     tp import <base>.tasks.json (<base> resolved from the spec base name).
//  2. a convergence-blocking finding survives in the latest recorded round →
//     revise the spec and re-review. It never names --resolve/--resolve-all/
//     --verify: disposing of a blocking finding is an operator decision, never
//     auto-advised (§3.1, §3.5, Principle 3).
//  3. a mechanizable mechanize_candidates class is present and none is blocking →
//     the compound directive: register a check, then run the next round. It
//     carries MechanizePhaseQualifier, so the driver is told what registering
//     costs when the check's subject is not yet in the spec (§8a.2). The
//     un-mechanizable over-specification class is excluded and does not fire this
//     branch (it falls through to branch 4).
//  4. clean but not yet converged (the lowest-precedence default) → run the next
//     review round.
//
// next_action is advisory/read-only: it changes nothing and gates no exit code
// (§8.1). <spec> is resolved to specPath; <file> stays a literal placeholder
// because tp cannot know the operator's chosen findings filename (§8.2).
func ReviewNextAction(specPath string, converged, blockingUnresolved bool, mechanizeClasses []string) string {
	switch {
	case converged:
		return "decompose the spec into tasks, then tp import " + specTaskBase(specPath)
	case blockingUnresolved:
		return "revise the spec to address the blocking findings, then run the next review round"
	default:
		if cls := firstMechanizableClass(mechanizeClasses); cls != "" {
			return fmt.Sprintf(
				"register a check for the recurring %q class — %s (tp set --workflow checks='[{\"class\":%q,\"cmd\":\"…\"}]'), then run the next review round: tp review %s --record <file>",
				cls, MechanizePhaseQualifier, cls, specPath)
		}
		return "run the next review round: tp review " + specPath + " --record <file>"
	}
}

// AuditNextAction returns the advisory next_action string for the audit loop by
// the §8.2 three-state precedence, using audit's own commands (first match wins):
//
//  1. converged → the terminal proceed-to-release marker; it names no further tp
//     command (release is outside tp).
//  2. non-converged with open non-PASS rows in the latest recorded round → the
//     fix-and-re-audit directive.
//  3. clean but not yet converged (the default) → run the next audit round.
//
// The review revise-and-re-review and mechanize_candidates branches do not apply
// to audit: audit findings are PASS/FAIL rows with no --resolve/--verify path and
// audit --record surfaces no mechanize_candidates. Advisory/read-only; gates no
// exit code (§8.1).
func AuditNextAction(specPath string, converged, latestRoundHasFindings bool) string {
	switch {
	case converged:
		return "converged — implementation verified, proceed to release"
	case latestRoundHasFindings:
		return "address the findings, then re-audit: tp audit " + specPath + " --record <file>"
	default:
		return "run the next audit round: tp audit " + specPath + " --record <file>"
	}
}

// specTaskBase resolves <base>.tasks.json from a spec path — the spec's base name
// (extension stripped) plus the .tasks.json suffix (§8.2).
func specTaskBase(specPath string) string {
	base := filepath.Base(specPath)
	return strings.TrimSuffix(base, filepath.Ext(base)) + ".tasks.json"
}

// firstMechanizableClass returns the first class in the list that is not the
// un-mechanizable over-specification class, or "" when none qualifies — the
// signal that next_action branch 3 fires (§8.2).
func firstMechanizableClass(classes []string) string {
	for _, c := range classes {
		if c != OverSpecificationClass {
			return c
		}
	}
	return ""
}
