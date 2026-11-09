package cli

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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
// A round asking no unit emits no prompt (§5). Each prompt reads task, asked
// units, round inputs, contract, then context: on a large slice the context is
// most of the prompt, and a reader that stops early must already have the
// contract.
func buildEpicReviewPrompts(run *epicReviewRun, roles []model.Role) []epicReviewPrompt {
	if len(run.slice.Units) == 0 {
		return []epicReviewPrompt{}
	}
	asked := epicAskedText(run) + epicDecidedText(run)
	prompts := make([]epicReviewPrompt, 0, len(roles)+1)
	if run.round >= 2 && run.carry.VerifyOnly {
		// The round before fixed only non-blocking findings and left nothing
		// open: the change is read, by regression alone.
		return append(prompts, epicRegressionPrompt(run, asked))
	}
	for i := range roles {
		role := &roles[i]
		var b strings.Builder
		b.WriteString(role.Instructions + "\n\n")
		b.WriteString(asked)
		writeEpicFocus(&b, role.Focus)
		prompts = append(prompts, epicPromptWithTail(run, role.ID, &b, epicContextText(run, role.Context)))
	}
	if run.round >= 2 && run.carry.Regression() {
		prompts = append(prompts, epicRegressionPrompt(run, asked))
	}
	return prompts
}

// epicRegressionInstructions frame the built-in regression role over a graph.
const epicRegressionInstructions = "You guard an epic graph against regressions from the round before. " +
	"An asked unit changed since that round or carried a finding that was resolved fixed. " +
	"Check that each change kept what the unit's acceptance, edges and whys needed, and that each fixed finding below " +
	"is resolved in the current text without breaking a neighbour. Report only what a change lost or broke; " +
	"finding nothing is a normal result.\n\n"

// epicRegressionPrompt renders the regression role's prompt: the asked units,
// the findings the round before resolved fixed, and the full context.
func epicRegressionPrompt(run *epicReviewRun, asked string) epicReviewPrompt {
	var b strings.Builder
	b.WriteString(epicRegressionInstructions)
	b.WriteString(asked)
	b.WriteString(epicChangedText(run))
	if len(run.handoff) > 0 {
		b.WriteString("\nNamed by a changed unit — the epics its text hands work to; check each takes what it is handed, but do not review it:\n")
		for _, c := range run.handoff {
			b.WriteString("---\n" + c.Block)
		}
		b.WriteString("---\n")
	}
	if len(run.carry.Fixed) > 0 {
		fmt.Fprintf(&b, "\nFindings round %d resolved fixed — confirm each fix holds:\n", run.round-1)
		for _, row := range run.carry.Fixed {
			loc, _ := row["location"].(string)
			finding, _ := row["finding"].(string)
			fmt.Fprintf(&b, "- %s: %s\n", loc, finding)
		}
	}
	return epicPromptWithTail(run, engine.RegressionRoleID, &b, epicContextText(run, ""))
}

// epicPromptWithTail closes a role's prompt with the mechanized exclusion,
// the finding contract and the role's output path, then the context, and
// repeats the row keys and the path on the last line.
func epicPromptWithTail(run *epicReviewRun, role string, b *strings.Builder, context string) epicReviewPrompt {
	path := epicRoleOutputPath(run, role)
	b.WriteString(mechanizedExclusionPrefix + strings.Join(engine.EpicErrorRules(), ", "))
	b.WriteString(epicFindingContract)
	fmt.Fprintf(b, "\nWrite this round's findings to: %s\nProduce findings for this round only, write them to that file, then stop.\n", path)
	b.WriteString(context)
	fmt.Fprintf(b, "\nReminder: write one NDJSON row per finding with severity, finding, location and evidence to %s, then stop.\n", path)
	return epicReviewPrompt{Role: role, Prompt: b.String(), OutputPath: path}
}

// epicAskedText renders the round's header and the asked units' blocks.
func epicAskedText(run *epicReviewRun) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This is epic review round %d of %s.\n\nAsked units:\n", run.round, epicSliceLabel(run.key))
	gate := false
	for _, u := range run.slice.Units {
		b.WriteString("---\n" + u.Block)
		gate = gate || strings.HasPrefix(u.Key, "gate:")
	}
	b.WriteString("---\n")
	if !gate {
		b.WriteString("\nNo gate unit is asked this round: every instruction or question about a gate does not apply; judge the epics only.\n")
	}
	return b.String()
}

// epicDecidedText lists the earlier rounds' findings on an asked unit that
// were accepted wontfix or duplicate, each with its location and the
// decision's evidence; with none it is empty.
func epicDecidedText(run *epicReviewRun) string {
	if len(run.carry.Decided) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nAlready decided — do not raise these again unless the cause they name changed; \"this round\" in a decision's evidence means the round it names:\n")
	for _, d := range run.carry.Decided {
		loc, _ := d.Row["location"].(string)
		finding, _ := d.Row["finding"].(string)
		resolved, _ := d.Row["resolved"].(map[string]any)
		status, _ := resolved["status"].(string)
		evidence, _ := resolved["evidence"].(string)
		fmt.Fprintf(&b, "- %s: %s — round %d, %s: %s\n", loc, finding, d.Round, status, evidence)
	}
	return b.String()
}

// epicChangedText shows, for each asked unit whose own block changed since
// the round before, the block that round read and the block now, so the
// regression role reads the change rather than reconstructing it.
func epicChangedText(run *epicReviewRun) string {
	var b strings.Builder
	for _, u := range run.slice.Units {
		before, ok := run.carry.Before[u.Key]
		if !ok {
			continue
		}
		if b.Len() == 0 {
			fmt.Fprintf(&b, "\nChanged since round %d — each unit as that round read it, then as it reads now:\n", run.round-1)
		}
		fmt.Fprintf(&b, "--- %s, round %d\n%s--- %s, now\n%s", u.Key, run.round-1, before, u.Key, u.Block)
	}
	return b.String()
}

// epicContextText renders the context blocks a role is shown — every
// neighbour, or only the asked units' dependencies under
// model.RoleContextDependencies — then each carried gate's block, read only,
// and the carried units no block shows, by key.
func epicContextText(run *epicReviewRun, scope string) string {
	var b strings.Builder
	var blocks []engine.EpicContextBlock
	for _, c := range run.slice.Context {
		if c.Dependency || scope != model.RoleContextDependencies {
			blocks = append(blocks, c)
		}
	}
	if len(blocks) > 0 {
		b.WriteString("\nContext — the epics one edge away; read them, do not review them:\n")
		for _, c := range blocks {
			b.WriteString("---\n" + c.Block)
		}
		b.WriteString("---\n")
	}
	for _, u := range run.carry.Carried {
		if strings.HasPrefix(u.Key, "gate:") {
			fmt.Fprintf(&b, "\nCarried gate — unchanged since round %d; read it, do not review it:\n---\n%s---\n", run.round-1, u.Block)
		}
	}
	if ids := epicCarriedByID(run); len(ids) > 0 {
		fmt.Fprintf(&b, "\nCarried — unchanged since round %d and not asked again: %s\n", run.round-1, strings.Join(ids, ", "))
	}
	return b.String()
}

// epicCarriedByID returns the carried epics no context block shows: an epic
// adjacent to no asked unit, which appears by its key only (D5). A carried
// gate is shown by its block instead.
func epicCarriedByID(run *epicReviewRun) []string {
	shown := map[string]bool{}
	for _, c := range run.slice.Context {
		shown["epic:"+c.ID] = true
	}
	ids := []string{}
	for _, u := range run.carry.Carried {
		if !shown[u.Key] && !strings.HasPrefix(u.Key, "gate:") {
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

// epicSliceLabel names the slice in prose and in the next_action command; a
// set is written as one CSV --epics value, so an id holding a comma is quoted.
func epicSliceLabel(key engine.EpicSliceKey) string {
	switch {
	case key.Milestone != "":
		return "--milestone " + key.Milestone
	case len(key.Epics) > 0:
		var b strings.Builder
		w := csv.NewWriter(&b)
		_ = w.Write(key.Epics)
		w.Flush()
		label := "--epics " + strconv.Quote(strings.TrimRight(b.String(), "\n"))
		if key.Name != "" {
			label += " --name " + strconv.Quote(key.Name)
		}
		return label
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

// epicReviewStateDirOut renders the slice's state directory relative to the
// project root, as every tp epic review payload prints it.
func epicReviewStateDirOut(run *epicReviewRun) string {
	if rel, err := filepath.Rel(filepath.Dir(run.tpDir), run.stateDir); err == nil {
		return rel
	}
	return run.stateDir
}

// epicReviewPayload assembles the emission from the run and the kept prompts.
func epicReviewPayload(run *epicReviewRun, prompts []epicReviewPrompt) epicReviewResult {
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
		StateDir:          epicReviewStateDirOut(run),
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
