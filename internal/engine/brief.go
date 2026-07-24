package engine

import (
	"fmt"
	"strings"
)

// OutScopePrefix marks the optional trailing out-of-scope line in a closure
// reason (§7.2). A closing unit appends it to report a finding it noticed
// outside the scope fence; tp preserves the line verbatim in closed_reason
// and surfaces it in tp report so it reaches a human.
const OutScopePrefix = "Out of scope:"

// ScopeFenceText returns the scope-fence statement every implementation
// brief carries in its Scope section (§7.1). The acceptance criteria are the
// boundary: only they are implemented; code outside them is not touched; the
// task file and .tp-review are not hand-edited; and a real out-of-fence
// problem is reported (a trailing "Out of scope:" line) rather than fixed.
// tp brief assembles this text; it is a statement, not a commit-path check.
func ScopeFenceText() string {
	return `Implement only this task's acceptance criteria. Do not refactor, rename, reformat, or "clean up" code outside them. Do not hand-edit the task file or anything under .tp-review/. If you find a real problem outside the fence, report it in the closure evidence as a trailing "Out of scope:" line instead of fixing it.`
}

// ExtractOutOfScope returns the out-of-scope note a closure reason carries in
// its "Out of scope:" line (§7.2), or "" when the reason has none. Only the
// text following the prefix is returned, trimmed; the prefix itself is not, so
// a report field named for the concept does not duplicate it. tp report uses
// this to surface out-of-fence findings a closing unit recorded.
func ExtractOutOfScope(closedReason string) string {
	for _, line := range strings.Split(closedReason, "\n") {
		if !strings.HasPrefix(line, OutScopePrefix) {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, OutScopePrefix))
	}
	return ""
}

// CloseRecipeText builds the "How to close" section text an implementation
// brief carries (§8). effectiveStrategy is the already-resolved concrete
// committing behavior — CommitStrategyBuiltin or CommitStrategyHC; an "auto"
// name is resolved to one of these by the caller via EffectiveCommitStrategy
// before this call, so the recipe never holds an "auto" branch (§8.1).
// qualityGate is the resolved quality_gate command verbatim (§8.2). The recipe
// states the exact close command, the red-gate rule, the evidence contract,
// the "--" separator, and — under hc — the rejected invocations (§8.4). tp
// brief assembles this text; it is a statement of the close path, not a check
// tp runs.
func CloseRecipeText(effectiveStrategy, qualityGate string) string {
	command, rejected := closeRecipeCommand(effectiveStrategy)

	var b strings.Builder
	b.WriteString("How to close this unit.\n\n")

	// §8.2: the resolved gate verbatim, and the rule a red gate is never closed over.
	b.WriteString("1. Run the gate and confirm it is green before closing. A red gate is never closed over: --skip-gate is a human decision, never the unit's.\n")
	if qualityGate != "" {
		fmt.Fprintf(&b, "   quality_gate: %s\n", qualityGate)
	} else {
		b.WriteString("   quality_gate: (none configured)\n")
	}
	b.WriteString("\n")

	// §8.1: the exact close command for the effective commit_strategy.
	b.WriteString("2. Close the task with the exact command for the effective commit_strategy:\n")
	b.WriteString(command)
	b.WriteString("\n")

	// §8.3: the closure reason is an evidence contract.
	b.WriteString("3. Write the closure reason as an evidence contract:\n")
	b.WriteString("   - One line per acceptance criterion, each stating what was implemented and how it was verified.\n")
	b.WriteString("   - No bare \"done\", \"wip\", or \"deferred\".\n")
	b.WriteString("   - Written in English.\n")
	b.WriteString("   - The first line becomes the next unit's summary of this work, so lead with the substantive claim.\n")
	b.WriteString("   - An optional trailing \"Out of scope:\" line reports a finding outside the fence instead of fixing it.\n\n")

	// §8.4: the "--" separator precedes the reason in both recipes.
	b.WriteString("The \"--\" separator precedes the reason because a reason often starts with \"- \" (a bullet), which a flag parser would otherwise treat as a flag.\n")

	// §8.4: under hc a bare tp done, tp commit, and --auto-commit are rejected.
	if rejected != "" {
		b.WriteString("\n")
		b.WriteString(rejected)
		b.WriteString("\n")
	}

	return b.String()
}

// closeRecipeCommand returns the exact close command text for the effective
// strategy (§8.1) and, for hc only, the rejected-invocation note (§8.4). An
// unrecognized strategy resolves to the builtin recipe, matching
// EffectiveCommitStrategy's default-to-builtin behavior.
func closeRecipeCommand(effectiveStrategy string) (command, rejected string) {
	switch effectiveStrategy {
	case CommitStrategyHC:
		command = "   commit with hc, then:\n   tp done <id> --commit <sha> -- \"<evidence>\"   (repeat --commit per sha)\n"
		rejected = "Under hc, a bare \"tp done\", \"tp commit\", and \"--auto-commit\" are all rejected: make the commit with hc, then record the SHA with \"tp done <id> --commit <sha>\"."
	default:
		command = "   tp done <id> --auto-commit -- \"<evidence>\"\n"
		rejected = ""
	}
	return command, rejected
}
