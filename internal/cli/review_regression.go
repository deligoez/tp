package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// regressionFixedFindingsCap bounds the previously-fixed block, newest first.
const regressionFixedFindingsCap = 50

// runReviewRegression implements `tp review <spec> --perspective regression`
// standalone. Mode (a): a state directory with at least one recorded round,
// read-only. Mode (b): explicit --diff-from plus --findings, touching no state.
func runReviewRegression(specPath, diffFrom, findingsPath string) error {
	if _, err := os.Stat(specPath); err != nil {
		output.Error(ExitFile, fmt.Sprintf("cannot read spec: %s", specPath))
		os.Exit(ExitFile)
		return nil
	}

	var dr engine.DiffResult
	var sinceLabel, baselinePath string
	fixed := make([]reviewFinding, 0)

	switch {
	case diffFrom != "" && findingsPath != "":
		// Mode (b): neither reads nor writes state
		if _, err := os.Stat(diffFrom); err != nil {
			output.Error(ExitFile, fmt.Sprintf("diff baseline not found: %s", diffFrom))
			os.Exit(ExitFile)
			return nil
		}
		if _, err := os.Stat(findingsPath); err != nil {
			output.Error(ExitUsage, fmt.Sprintf("findings file not found: %s", findingsPath))
			os.Exit(ExitUsage)
			return nil
		}
		dr = engine.DiffSections(diffLinesOf(diffFrom), diffLinesOf(specPath))
		sinceLabel = "baseline " + diffFrom
		baselinePath = diffFrom
		all := parseFindingsFile(findingsPath)
		// newest first: later rows are newer
		for i := len(all) - 1; i >= 0; i-- {
			if all[i].Resolved != nil && all[i].Resolved.Status == "fixed" {
				fixed = append(fixed, all[i])
			}
		}
	case diffFrom != "" || findingsPath != "":
		output.Error(ExitUsage, "standalone regression needs both --diff-from and --findings (or a state directory with a recorded round)")
		os.Exit(ExitUsage)
		return nil
	default:
		// Mode (a): reads state (rounds, snapshots, resolved findings), never writes
		st, err := engine.LoadReviewState(specPath)
		if err != nil {
			exitStateError(err)
			return nil
		}
		if st == nil || len(st.ReviewRounds) == 0 {
			output.Error(ExitUsage, "standalone regression requires a state directory with at least one recorded round, or explicit --diff-from plus --findings")
			os.Exit(ExitUsage)
			return nil
		}
		r := len(st.ReviewRounds) + 1
		if snapRound, snapPath := newestEarlierSnapshot(specPath, r); snapPath != "" {
			dr = engine.DiffSections(diffLinesOf(snapPath), diffLinesOf(specPath))
			sinceLabel = fmt.Sprintf("round %d", snapRound)
			baselinePath = snapPath
		}
		fixed = append(fixed, collectFixedFindings(specPath, st)...)
	}

	// Vacuous input: nothing changed AND nothing was fixed
	if len(dr.Changed) == 0 && len(dr.Removed) == 0 && len(fixed) == 0 {
		output.Error(ExitUsage, "regression review has nothing to check: the section diff is empty and no finding is resolved fixed",
			"the regression prompt applies when the spec changed or fixed findings exist — the same condition that auto-appends it in default review mode")
		os.Exit(ExitUsage)
		return nil
	}

	if len(fixed) > regressionFixedFindingsCap {
		fixed = fixed[:regressionFixedFindingsCap]
	}

	prompt := buildRegressionPrompt(&dr, sinceLabel, baselinePath, fixed)

	// Mechanical checks run in the regression perspective too (§15.3)
	wfChecks, checksTaskFile := engine.ResolveWorkflow(specPath, flagFile)
	var mechChecks []map[string]any
	if len(wfChecks.Checks) > 0 {
		mechChecks, _ = runMechanicalChecks(&wfChecks, checksTaskFile)
		classes := make([]string, 0, len(wfChecks.Checks))
		for i := range wfChecks.Checks {
			classes = append(classes, wfChecks.Checks[i].Class)
		}
		prompt += "\n\nMechanically checked classes — do NOT report findings of these classes: " + strings.Join(classes, ", ")
	}

	result := reviewResult{
		Spec:             specPath,
		Perspective:      "regression",
		MechanicalChecks: mechChecks,
		Prompts: []reviewPrompt{{
			Role:     "regression",
			Category: "regression",
			Prompt:   prompt,
		}},
		ReviewLoop: reviewLoop{
			Round:            0,
			Convergence:      "uncounted delta pass — counted rounds stay full-panel",
			PreviousFindings: len(fixed),
			Instruction:      "Spawn a sub-agent with this prompt. Fix what it reports, then generate the next counted round. This pass records no state.",
			Mode:             "regression",
		},
	}
	return output.JSON(result)
}

// collectFixedFindings returns findings with resolved.status == "fixed"
// across recorded review rounds, newest first.
func collectFixedFindings(specPath string, st *engine.ReviewState) []reviewFinding {
	fixed := make([]reviewFinding, 0)
	if st == nil {
		return fixed
	}
	for i := len(st.ReviewRounds) - 1; i >= 0; i-- {
		rows, found := engine.LoadRoundRows(specPath, &st.ReviewRounds[i])
		if !found {
			output.Info(fmt.Sprintf("round %d file %s is missing; skipping its rows", st.ReviewRounds[i].Round, st.ReviewRounds[i].File))
			continue
		}
		for _, row := range rows {
			f := findingFromRow(row)
			if f.Resolved != nil && f.Resolved.Status == "fixed" {
				fixed = append(fixed, f)
			}
		}
	}
	return fixed
}

// buildRegressionPrompt renders the §11.3 body order: persona, changed
// sections, previously fixed findings, three numbered checks, finding format.
// The regression role accepts no spec-frontmatter override or lens (§5.2, §10.4).
// baselinePath names the snapshot the diff was computed against so a fresh
// sub-agent can read it without the orchestrator injecting the path (§10.3);
// empty when no baseline applies.
func buildRegressionPrompt(dr *engine.DiffResult, sinceLabel, baselinePath string, fixed []reviewFinding) string {
	var b strings.Builder
	b.WriteString("You guard decisions this spec has already settled. Your only job is to find changes that undo them.\n")

	if baselinePath != "" {
		fmt.Fprintf(&b, "\nBaseline snapshot to diff against: %s\n", baselinePath)
	}

	if sinceLabel != "" {
		if block := buildChangedSectionsBlock(dr, sinceLabel); block != "" {
			b.WriteString(block)
		}
	}

	b.WriteString("\n\n## Previously fixed findings\n")
	if len(fixed) == 0 {
		b.WriteString("(none recorded)\n")
	}
	for _, f := range fixed {
		evidence := ""
		if f.Resolved != nil {
			evidence = f.Resolved.Evidence
		}
		fmt.Fprintf(&b, "- %s — %s\n", f.Finding, evidence)
	}

	b.WriteString("\nChecks:\n")
	b.WriteString("1. Does any changed section revert or weaken a fixed finding above?\n")
	b.WriteString("2. Does any changed section contradict an unchanged section?\n")
	b.WriteString("3. Does any change reintroduce a problem that a fixed finding had eliminated in a different section?\n")

	b.WriteString(findingFormat)
	return b.String()
}
