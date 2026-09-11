package engine

import (
	"fmt"
	"path/filepath"
	"strconv"
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
// chosen by a fixed precedence, total over reachable states (first match wins):
//
//  1. done (converged, or ended at the cap with every finding dispositioned) →
//     the phase's forward step: decompose, then tp import <base>.tasks.json.
//  2. the cap is reached and a finding still carries no disposition → the only
//     way out: disposition it in the recorded round file, then import. No
//     further round is named, because the cap admits none.
//  3. a convergence-blocking finding survives in the latest recorded round →
//     revise the spec, or disposition the finding in the recorded round file,
//     then re-review. Both exits are named: a directive naming only the edit is
//     what turned every finding into spec text. Accepting a blocking finding is
//     named as the operator's decision, and --resolve-all is never advised.
//  4. a mechanizable mechanize_candidates class is present and none is blocking →
//     the compound directive: register a check, then run the next round. It
//     carries MechanizePhaseQualifier (§8a.2). The un-mechanizable
//     over-specification class does not fire this branch.
//  5. clean but not yet converged (the lowest-precedence default) → run the next
//     review round.
//
// next_action is advisory/read-only: it changes nothing and gates no exit code
// (§8.1). <spec> is resolved to specPath and roundFile names the latest recorded
// round's file; <file> stays a literal placeholder because tp cannot know the
// operator's chosen findings filename (§8.2).
func ReviewNextAction(specPath string, done LoopDone, blockingUnresolved bool, mechanizeClasses []string, roundFile string) string {
	importStep := "tp import " + specTaskBase(specPath)
	switch {
	case done.Done && done.By == DoneByCap:
		return "decompose the spec into tasks, then " + importStep + " — the round cap ended review with every finding dispositioned"
	case done.Done:
		return "decompose the spec into tasks, then " + importStep
	case done.CapReached && done.BlockingFixedAtCap > 0:
		return blockingFixedAtCap(done.BlockingFixedAtCap) + " — tp review " + roundFile + " --resolve <index> wontfix \"<evidence>\" --force — then " + importStep
	case done.CapReached:
		return "the review round cap is reached: disposition each remaining finding in " + roundFile +
			" — tp review " + roundFile + " --resolve <index> fixed|wontfix|duplicate \"<evidence>\"; accepting a critical or high finding is the operator's decision — then " + importStep
	case blockingUnresolved:
		return "revise the spec where a blocking finding is a defect, or disposition it — tp review " + roundFile +
			" --resolve <index> wontfix|duplicate \"<evidence>\", the operator's decision for a critical or high finding — then run the next review round"
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
// a fixed precedence, using audit's own commands (first match wins):
//
//  1. done → the terminal proceed-to-release marker; it names no further tp
//     command. A loop the cap ended says so instead of "converged".
//  2. the cap is reached and a finding still carries no disposition →
//     disposition it in the recorded round file; no further round is named.
//  3. the latest recorded round is unclean → fix and re-audit, or, for a
//     finding that needs no code change, disposition it (the operator's call).
//  4. clean but not yet converged (the default) → run the next audit round.
//
// latestRoundClean is the round's live verdict; latestRoundFindings is the count
// of its non-PASS rows, which under `blocking` is positive on rounds that are
// clean, and branches 1 and 4 render it as a numeral (v0.37.0 §2). Under the
// default `all` the two agree and the numeral never appears. Advisory/read-only;
// gates no exit code (§8.1).
func AuditNextAction(specPath string, done LoopDone, latestRoundClean bool, latestRoundFindings int, roundFile string) string {
	switch {
	case done.Done && done.By == DoneByCap:
		return "the audit round cap ended the loop with every finding dispositioned — proceed to release"
	case done.Done:
		if latestRoundFindings > 0 {
			return "converged over " + acceptedRows(latestRoundFindings) +
				" — implementation verified, proceed to release"
		}
		return "converged — implementation verified, proceed to release"
	case done.CapReached && done.BlockingFixedAtCap > 0:
		return blockingFixedAtCap(done.BlockingFixedAtCap) + " — tp audit " + roundFile + " --resolve <role:item_id> wontfix \"<evidence>\" --force"
	case done.CapReached:
		return "the audit round cap is reached: disposition each remaining finding in " + roundFile +
			" — tp audit " + roundFile + " --resolve <role:item_id> fixed|wontfix|duplicate \"<evidence>\"; accepting a finding without a code change is the operator's decision"
	case !latestRoundClean:
		return "address the findings, then re-audit: tp audit " + specPath + " --record <file> — or, for a finding that needs no code change, tp audit " +
			roundFile + " --resolve <role:item_id> wontfix|duplicate \"<evidence>\" (the operator's decision)"
	default:
		if latestRoundFindings > 0 {
			return acceptedRows(latestRoundFindings) + " carried forward — run the next audit round: tp audit " +
				specPath + " --record <file>"
		}
		return "run the next audit round: tp audit " + specPath + " --record <file>"
	}
}

// blockingFixedAtCap names the one state at the cap that only the operator can
// end: blocking findings marked fixed that no round will re-read.
func blockingFixedAtCap(n int) string {
	noun := "findings were"
	if n == 1 {
		noun = "finding was"
	}
	return fmt.Sprintf("%d blocking %s marked fixed at the round cap and no round re-read them: the operator accepts them with evidence or raises the cap by one for a verification round", n, noun)
}

// LatestRoundFile returns the recorded round file of the latest round in
// rounds, as the path an agent passes to --resolve, or "" with no round.
func LatestRoundFile(specPath string, rounds []ReviewRound) string {
	if len(rounds) == 0 {
		return ""
	}
	return filepath.Join(ReviewStateDir(specPath), rounds[len(rounds)-1].File)
}

// acceptedRows renders the accepted-row count as the numeral §2 asks for, with
// the noun agreeing with it. The numeral is the whole point: it is what makes
// the rendered string differ observably from the one an empty round produces,
// which a mutant branching on `clean` alone cannot do.
func acceptedRows(n int) string {
	if n == 1 {
		return "1 accepted row"
	}
	return strconv.Itoa(n) + " accepted rows"
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
