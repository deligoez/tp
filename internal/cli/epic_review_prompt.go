package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
)

// epicFindingContract is the row shape every graph role writes: the review
// gate's four required keys, located on a unit of the slice (spec/1.3.0.md §5).
const epicFindingContract = "\n\nOutput contract — write one JSON object per line (NDJSON), each with:\n" +
	"- severity: one of critical, high, medium, low\n" +
	"- finding: what is wrong, in English\n" +
	"- location: \"epic:<id>\" for an asked epic, or \"gate:<milestone>\" for the asked gate; a finding on an edge belongs to its source epic\n" +
	"- evidence: the block text you read to reach this finding\n" +
	"Locate findings on asked units only; a context epic is not reviewed. No finding is a normal result: write an empty file.\n"

// buildEpicReviewPrompts renders one prompt per role, in corpus order, and
// the built-in regression prompt last when the round has something to guard.
// A round asking no unit emits no prompt (§5).
func buildEpicReviewPrompts(run *epicReviewRun, roles []model.Role) []epicReviewPrompt {
	if len(run.slice.Units) == 0 {
		return []epicReviewPrompt{}
	}
	body := epicReviewSliceText(run)
	prompts := make([]epicReviewPrompt, 0, len(roles)+1)
	for i := range roles {
		role := &roles[i]
		var b strings.Builder
		b.WriteString(role.Instructions + "\n\n")
		b.WriteString(body)
		writeEpicFocus(&b, role.Focus)
		prompts = append(prompts, epicPromptWithTail(run, role.ID, &b))
	}
	if run.round >= 2 && run.carry.Regression() {
		prompts = append(prompts, epicRegressionPrompt(run, body))
	}
	return prompts
}

// epicRegressionInstructions frame the built-in regression role over a graph.
const epicRegressionInstructions = "You guard an epic graph against regressions from the round before. " +
	"An asked unit changed since that round or carried a finding that was resolved fixed. " +
	"Check that each change kept what the unit's acceptance, edges and whys needed, and that each fixed finding below " +
	"is resolved in the current text without breaking a neighbour. Report only what a change lost or broke; " +
	"finding nothing is a normal result.\n\n"

// epicRegressionPrompt renders the regression role's prompt: the slice text
// and the findings the round before resolved fixed.
func epicRegressionPrompt(run *epicReviewRun, body string) epicReviewPrompt {
	var b strings.Builder
	b.WriteString(epicRegressionInstructions)
	b.WriteString(body)
	if len(run.carry.Fixed) > 0 {
		fmt.Fprintf(&b, "\nFindings round %d resolved fixed — confirm each fix holds:\n", run.round-1)
		for _, row := range run.carry.Fixed {
			loc, _ := row["location"].(string)
			finding, _ := row["finding"].(string)
			fmt.Fprintf(&b, "- %s: %s\n", loc, finding)
		}
	}
	return epicPromptWithTail(run, engine.RegressionRoleID, &b)
}

// epicPromptWithTail closes a role's prompt with the mechanized exclusion,
// the finding contract and the role's output path.
func epicPromptWithTail(run *epicReviewRun, role string, b *strings.Builder) epicReviewPrompt {
	path := epicRoleOutputPath(run, role)
	b.WriteString(mechanizedExclusionPrefix + strings.Join(engine.EpicErrorRules(), ", "))
	b.WriteString(epicFindingContract)
	fmt.Fprintf(b, "\nWrite this round's findings to: %s\nProduce findings for this round only, write them to that file, then stop.\n", path)
	return epicReviewPrompt{Role: role, Prompt: b.String(), OutputPath: path}
}

// epicReviewSliceText renders the asked units, then the context blocks.
func epicReviewSliceText(run *epicReviewRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This is epic review round %d of %s.\n\nAsked units:\n", run.round, epicSliceLabel(run.key))
	for _, u := range run.slice.Units {
		b.WriteString("---\n" + u.Block)
	}
	b.WriteString("---\n")
	if len(run.slice.Context) > 0 {
		b.WriteString("\nContext — the epics one edge away; read them, do not review them:\n")
		for _, c := range run.slice.Context {
			b.WriteString("---\n" + c.Block)
		}
		b.WriteString("---\n")
	}
	if ids := epicCarriedByID(run); len(ids) > 0 {
		fmt.Fprintf(&b, "\nCarried — unchanged since round %d and not asked again: %s\n", run.round-1, strings.Join(ids, ", "))
	}
	return b.String()
}

// epicCarriedByID returns the carried units no context block shows: a gate,
// or an epic adjacent to no asked unit, which appears by its key only (D5).
func epicCarriedByID(run *epicReviewRun) []string {
	shown := map[string]bool{}
	for _, c := range run.slice.Context {
		shown["epic:"+c.ID] = true
	}
	ids := []string{}
	for _, u := range run.carry.Carried {
		if !shown[u.Key] {
			ids = append(ids, u.Key)
		}
	}
	return ids
}

// writeEpicFocus appends the role's focus questions as a numbered list.
func writeEpicFocus(b *strings.Builder, focus []string) {
	if len(focus) == 0 {
		return
	}
	b.WriteString("\nCheck each of these specifically:\n")
	for i, q := range focus {
		fmt.Fprintf(b, "%d. %s\n", i+1, q)
	}
}

// epicSliceLabel names the slice in prose and in the next_action command.
func epicSliceLabel(key engine.EpicSliceKey) string {
	if key.Milestone != "" {
		return "--milestone " + key.Milestone
	}
	return "--epic " + key.Epic
}

// epicRoleOutputPath is the file one role writes its rows to. Under a run it
// is the role's part file in the round directory; by hand it is cwd-relative
// and carries the start of the state directory's hash, so two slices reviewed
// from one directory never name one file.
func epicRoleOutputPath(run *epicReviewRun, role string) string {
	if roundDir := os.Getenv(engine.EnvRoundDir); roundDir != "" {
		return engine.RoleFindingsPartPath(roundDir, role)
	}
	return fmt.Sprintf("epic-review-%s-r%d-%s.ndjson", filepath.Base(run.stateDir)[:12], run.round, role)
}

// epicReviewPayload assembles the emission from the run and the kept prompts.
func epicReviewPayload(run *epicReviewRun, prompts []epicReviewPrompt) epicReviewResult {
	stateDir := run.stateDir
	if rel, err := filepath.Rel(filepath.Dir(run.tpDir), run.stateDir); err == nil {
		stateDir = rel
	}
	contextIDs := make([]string, 0, len(run.slice.Context))
	for _, c := range run.slice.Context {
		contextIDs = append(contextIDs, c.ID)
	}
	next := fmt.Sprintf("run each prompt, concatenate the rows of every output_path into one NDJSON file, then tp epic review %s --record <file>",
		epicSliceLabel(run.key))
	if len(run.slice.Units) == 0 {
		next = fmt.Sprintf("no unit is asked this round: record an empty file with tp epic review %s --record <empty file>", epicSliceLabel(run.key))
	}
	return epicReviewResult{
		StateDir:          stateDir,
		Round:             run.round,
		Units:             engine.EpicUnitKeys(run.slice.Units),
		Context:           contextIDs,
		Carried:           engine.EpicUnitKeys(run.carry.Carried),
		MechanicalChecks:  run.checks,
		MechanizedClasses: engine.EpicErrorRules(),
		Prompts:           prompts,
		NextAction:        next,
	}
}
