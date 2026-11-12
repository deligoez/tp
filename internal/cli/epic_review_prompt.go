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

// epicFindingContract is the row shape a graph role writes: the review gate's
// four required keys, located on a unit of the kinds it is asked
// (spec/1.3.0.md §5).
func epicFindingContract(kinds map[string]bool) string {
	var locs []string
	for _, k := range epicUnitKinds {
		if kinds[k.kind] {
			locs = append(locs, k.location)
		}
	}
	loc := strings.Join(locs, ", ")
	if n := len(locs); n > 1 {
		loc = strings.Join(locs[:n-1], ", ") + ", or " + locs[n-1]
	}
	if kinds[model.RoleUnitEpics] {
		loc += "; a finding on an edge belongs to its source epic"
	}
	return "\n\nOutput contract — write one JSON object per line (NDJSON), each with:\n" +
		"- severity: one of critical, high, medium, low\n" +
		"- finding: what is wrong, in English\n" +
		"- location: " + loc + "\n" +
		"- evidence: the block text you read to reach this finding\n" +
		"Locate findings on asked units only; a context epic is not reviewed. No finding is a normal result: write an empty file.\n"
}

// buildEpicReviewPrompts renders one prompt per role, in corpus order, and
// the built-in regression prompt last when the round has something to guard.
// A round asking no unit emits no prompt (§5); a role asked no unit of its
// kinds gets none either and is returned skipped with reason no-units. Each
// prompt reads task, asked units, round inputs, contract, then context: on a
// large slice the context is most of the prompt, and a reader that stops
// early must already have the contract. It refuses a round in which a unit
// it lists is asked of no prompt (epicUnaskedError): recording such a round
// would converge a unit nobody read.
func buildEpicReviewPrompts(run *epicReviewRun, roles []model.Role) ([]epicReviewPrompt, []engine.SkippedRole, *epicRunError) {
	prompts := []epicReviewPrompt{}
	skipped := []engine.SkippedRole{}
	units, _ := epicEmittedUnits(run, roles)
	if len(units) == 0 {
		return prompts, skipped, nil
	}
	// The round before fixed only non-blocking findings on epic and gate
	// units and left nothing open there: that change is read by regression
	// alone, beside the roles asked a scope unit or a changed purpose.
	verifyOnly := run.round >= 2 && run.carry.VerifyOnly
	carried := map[string]bool(nil)
	if verifyOnly {
		carried = map[string]bool{model.RoleUnitScope: true}
	}
	asked := map[string]bool{}
	for i := range roles {
		role := &roles[i]
		ask := epicAskOf(run, role.UnitKinds(), role.Purpose, carried)
		switch {
		case len(ask.units) > 0:
			prompts = append(prompts, epicRolePrompt(run, role, &ask))
			ask.markAsked(asked)
		case !verifyOnly:
			skipped = append(skipped, engine.SkippedRole{Role: role.ID, Reason: engine.SkipNoUnits})
		}
	}
	if run.round >= 2 && (verifyOnly || run.carry.Regression()) {
		ask := epicAskOf(run, engine.EpicRegressionKinds, false, nil)
		if len(ask.units) == 0 {
			skipped = append(skipped, engine.SkippedRole{Role: engine.RegressionRoleID, Reason: engine.SkipNoUnits})
		} else {
			prompts = append(prompts, epicRegressionPrompt(run, &ask))
			ask.markAsked(asked)
		}
	}
	return prompts, skipped, epicUnaskedError(units, asked)
}

// epicUnaskedError is the refusal of a round listing a unit no prompt asks,
// or nil when every unit has a reader (the H1 invariant).
func epicUnaskedError(units []engine.EpicReviewUnit, asked map[string]bool) *epicRunError {
	var missing []string
	for _, u := range units {
		if !asked[u.Key] {
			missing = append(missing, u.Key)
		}
	}
	if len(missing) == 0 {
		return nil
	}
	return &epicRunError{code: ExitState, msg: "epic review round asks no role these units: " + strings.Join(missing, ", "),
		hint: "add a graph role whose units key lists each unit's kind to .tp/epic-reviewers/, or remove the unit from the slice"}
}

// epicRolePrompt renders one corpus role's prompt over what it is asked.
func epicRolePrompt(run *epicReviewRun, role *model.Role, ask *epicRoleAsk) epicReviewPrompt {
	var b strings.Builder
	b.WriteString(role.Instructions + "\n\n")
	b.WriteString(epicAskedText(run, ask))
	if role.Purpose && !ask.scopeOnly() {
		b.WriteString(epicPurposeText(ask))
	}
	b.WriteString(epicDecidedText(run, ask))
	b.WriteString(epicScopeRoundText(run, ask))
	writeEpicFocus(&b, role.Focus)
	return epicPromptWithTail(run, role.ID, &b, ask, epicContextText(run, role.Context, ask))
}

// epicScopeRoundText is what a role asked a scope unit reads of the round
// before, as regression reads it of an epic: each asked scope unit's block
// as that round read it and as it reads now, and the findings on one that
// round resolved fixed, to confirm.
func epicScopeRoundText(run *epicReviewRun, ask *epicRoleAsk) string {
	scope := epicRoleAsk{kinds: ask.kinds}
	for _, u := range ask.units {
		if engine.EpicUnitKind(u.Key) == model.RoleUnitScope {
			scope.units = append(scope.units, u)
		}
	}
	text := epicScopeChangeText(run, scope.units)
	if fixed := scope.rows(run.carry.ScopeFixed); len(fixed) > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "\nFindings round %d resolved fixed on the asked scope units — confirm each fix holds:\n", run.round-1)
		for _, row := range fixed {
			loc, _ := row["location"].(string)
			finding, _ := row["finding"].(string)
			fmt.Fprintf(&b, "- %s: %s\n", loc, finding)
		}
		text += b.String()
	}
	return text
}

// epicRegressionInstructions frame the built-in regression role over a graph.
const epicRegressionInstructions = "You guard an epic graph against regressions from the round before. " +
	"An asked unit changed since that round or carried a finding that was resolved fixed. " +
	"Check that each change kept what the unit's acceptance, edges and whys needed and broke no neighbour, " +
	"and that each fixed finding listed is resolved in the current text. Report only what a change lost or broke; " +
	"finding nothing is a normal result.\n\n"

// epicRegressionPrompt renders the regression role's prompt: the asked units
// of its kinds, the findings the round before resolved fixed on them, and
// the full context.
func epicRegressionPrompt(run *epicReviewRun, ask *epicRoleAsk) epicReviewPrompt {
	var b strings.Builder
	b.WriteString(epicRegressionInstructions)
	b.WriteString(epicAskedText(run, ask) + epicDecidedText(run, ask))
	b.WriteString(epicChangedText(run, ask))
	if len(run.handoff) > 0 {
		b.WriteString("\nNamed by a changed unit — the epics its text hands work to; check each takes what it is handed, but do not review it:\n")
		for _, c := range run.handoff {
			b.WriteString("---\n" + c.Block)
		}
		b.WriteString("---\n")
	}
	fixed := ask.rows(run.carry.Fixed)
	if len(fixed) == 0 {
		fmt.Fprintf(&b, "\nNo finding was resolved fixed since round %d: the change is an edit, so check each changed unit against its neighbours.\n", run.round-1)
	} else {
		fmt.Fprintf(&b, "\nFindings round %d resolved fixed — confirm each fix holds:\n", run.round-1)
		for _, row := range fixed {
			loc, _ := row["location"].(string)
			finding, _ := row["finding"].(string)
			fmt.Fprintf(&b, "- %s: %s\n", loc, finding)
		}
	}
	return epicPromptWithTail(run, engine.RegressionRoleID, &b, ask, epicContextText(run, "", ask))
}

// epicPromptWithTail closes a role's prompt with the mechanized exclusion,
// the finding contract and the role's output path, then the context, and
// repeats the row keys and the path on the last line.
func epicPromptWithTail(run *epicReviewRun, role string, b *strings.Builder, ask *epicRoleAsk, context string) epicReviewPrompt {
	path := epicRoleOutputPath(run, role)
	if ask.kinds[model.RoleUnitEpics] || ask.kinds[model.RoleUnitGates] {
		// The mechanized classes are lint rules over epics and gates; a role
		// reading neither has nothing they would exclude.
		b.WriteString(mechanizedExclusionPrefix + strings.Join(engine.EpicErrorRules(), ", "))
	}
	b.WriteString(epicFindingContract(ask.kinds))
	fmt.Fprintf(b, "\nWrite this round's findings to: %s\nProduce findings for this round only, write them to that file, then stop.\n", path)
	b.WriteString(context)
	fmt.Fprintf(b, "\nReminder: write one NDJSON row per finding with severity, finding, location and evidence to %s, then stop.\n", path)
	return epicReviewPrompt{Role: role, Prompt: b.String(), PromptBytes: b.Len(), OutputPath: path}
}

// epicAskedText renders the round's header and the asked units' blocks, then,
// for each of the role's kinds no unit of which is asked, that its questions
// about that kind do not apply.
func epicAskedText(run *epicReviewRun, ask *epicRoleAsk) string {
	var b strings.Builder
	fmt.Fprintf(&b, "This is epic review round %d of %s.\n\nAsked units:\n", run.round, epicSliceLabel(run.key))
	have := map[string]bool{}
	for _, u := range ask.units {
		b.WriteString("---\n" + u.Block)
		have[engine.EpicUnitKind(u.Key)] = true
	}
	b.WriteString("---\n")
	var judged []string
	for _, k := range epicUnitKinds {
		if have[k.kind] {
			judged = append(judged, k.plural)
		}
	}
	for _, k := range epicUnitKinds {
		if ask.kinds[k.kind] && !have[k.kind] {
			fmt.Fprintf(&b, "\nNo %s unit is asked this round: every instruction or question about %s does not apply; judge the %s only.\n",
				k.noun, k.article, strings.Join(judged, " and "))
		}
	}
	return b.String()
}

// epicPurposeText is a purpose role's header: one line per milestone its
// asked units belong to that has a purpose, in the order the units name
// them, each once; with none it is empty.
func epicPurposeText(ask *epicRoleAsk) string {
	var b strings.Builder
	seen := map[string]bool{}
	for _, u := range ask.units {
		if u.Purpose == "" || seen[u.Milestone] {
			continue
		}
		if len(seen) == 0 {
			b.WriteString("\nMilestone purposes:\n")
		}
		seen[u.Milestone] = true
		b.WriteString("- " + u.Milestone + ": " + u.Purpose + "\n")
	}
	return b.String()
}

// epicDecidedText lists the earlier rounds' findings on a unit the role is
// asked that were accepted wontfix or duplicate, each with its location and
// the decision's evidence; with none it is empty.
func epicDecidedText(run *epicReviewRun, ask *epicRoleAsk) string {
	var decided []engine.EpicDecision
	for _, d := range run.carry.Decided {
		if loc, _ := d.Row["location"].(string); ask.holds(loc) {
			decided = append(decided, d)
		}
	}
	if len(decided) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\nAlready decided — do not raise these again unless the cause they name changed; \"this round\" in a decision's evidence means the round it names:\n")
	for _, d := range decided {
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
// regression role reads the change rather than reconstructing it. A unit
// whose only change is its description shows that description's line diff
// instead: the asked block above already is the block now.
func epicChangedText(run *epicReviewRun, ask *epicRoleAsk) string {
	var b strings.Builder
	for _, u := range ask.units {
		before, ok := run.carry.Before[u.Key]
		if !ok {
			continue
		}
		if b.Len() == 0 {
			fmt.Fprintf(&b, "\nChanged since round %d:\n", run.round-1)
		}
		if run.carry.Authored[u.Key] {
			fmt.Fprintf(&b, "--- %s, its description only, as a line diff (- removed, + added, … unchanged lines left out):\n%s", u.Key, engine.EpicDescriptionDiff(before, &u))
			continue
		}
		fmt.Fprintf(&b, "--- %s, as round %d read it:\n%s--- %s, as it reads now:\n%s", u.Key, run.round-1, before, u.Key, u.Block)
	}
	return b.String()
}

// epicContextText renders the context blocks a role is shown — every
// neighbour of its asked epics, only their dependencies under
// model.RoleContextDependencies, or the dependencies in full and the
// dependents in brief under model.RoleContextNeighboursBrief — then, of the
// units of its kinds it is not asked, each gate's block, read only, and the
// rest no block shows, by key.
func epicContextText(run *epicReviewRun, scope string, ask *epicRoleAsk) string {
	var b strings.Builder
	var blocks []string
	context := engine.EpicCarriedContext(run.g, ask.units)
	for _, c := range context {
		switch {
		case c.Dependency || scope == model.RoleContextNeighbours || scope == "":
			blocks = append(blocks, c.Block)
		case scope == model.RoleContextNeighboursBrief:
			blocks = append(blocks, c.Brief)
		}
	}
	if len(blocks) > 0 {
		b.WriteString("\nContext — the epics one edge away; read them, do not review them:\n")
		for _, block := range blocks {
			b.WriteString("---\n" + block)
		}
		b.WriteString("---\n")
	}
	carried := ask.carried(run)
	for _, u := range carried {
		if engine.EpicUnitKind(u.Key) == model.RoleUnitGates {
			fmt.Fprintf(&b, "\nCarried gate — unchanged since round %d; read it, do not review it:\n---\n%s---\n", run.round-1, u.Block)
		}
	}
	if ids := epicCarriedByID(carried, context); len(ids) > 0 {
		fmt.Fprintf(&b, "\nCarried — unchanged since round %d and not asked again: %s\n", run.round-1, strings.Join(ids, ", "))
	}
	for _, u := range ask.units {
		if engine.EpicUnitKind(u.Key) == model.RoleUnitScope {
			b.WriteString(engine.EpicScopeContext(run.g, run.states, u.Milestone))
		}
	}
	return b.String()
}

// epicCarriedByID returns the carried units no block shows: an epic adjacent
// to no asked unit, or a scope unit, each by its key only (D5). A carried
// gate is shown by its block instead.
func epicCarriedByID(carried []engine.EpicReviewUnit, context []engine.EpicContextBlock) []string {
	shown := map[string]bool{}
	for _, c := range context {
		shown[engine.EpicUnitPrefix+c.ID] = true
	}
	ids := []string{}
	for _, u := range carried {
		if !shown[u.Key] && engine.EpicUnitKind(u.Key) != model.RoleUnitGates {
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
// is the role's part file in the round directory; by hand it is the absolute
// <state dir>/inbox/<role>-r<N>.ndjson (spec/1.7.0.md §1), so it resolves the
// same from any cwd and two slices never name one file.
func epicRoleOutputPath(run *epicReviewRun, role string) string {
	if roundDir := os.Getenv(engine.EnvRoundDir); roundDir != "" {
		return engine.RoleFindingsPartPath(roundDir, role)
	}
	return engine.EpicReviewInboxPath(run.stateDir, role, run.round)
}

// epicReviewStateDirOut renders the slice's state directory relative to the
// project root, as every tp epic review payload prints it.
func epicReviewStateDirOut(run *epicReviewRun) string {
	if rel, err := filepath.Rel(filepath.Dir(run.tpDir), run.stateDir); err == nil {
		return rel
	}
	return run.stateDir
}

// epicReviewPayload assembles the emission from the run and the kept prompts;
// built is how many prompts the round built before --role narrowed them.
func epicReviewPayload(run *epicReviewRun, roles []model.Role, prompts []epicReviewPrompt, built int) epicReviewResult {
	units, carried := epicEmittedUnits(run, roles)
	context := engine.EpicCarriedContext(run.g, units)
	contextIDs := make([]string, 0, len(context))
	for _, c := range context {
		contextIDs = append(contextIDs, c.ID)
	}
	next := fmt.Sprintf("run each prompt, each role writing its output_path (an empty file when it finds nothing), then tp epic review %s --record-inbox",
		epicSliceLabel(run.key))
	switch {
	case len(units) == 0:
		next = fmt.Sprintf("no unit is asked this round: record it empty with tp epic review %s --record-inbox, which needs no file", epicSliceLabel(run.key))
	case built == 0:
		next = fmt.Sprintf("no role is asked a unit of its kinds this round: record it empty with tp epic review %s --record-inbox, which needs no file", epicSliceLabel(run.key))
	}
	return epicReviewResult{
		StateDir:          epicReviewStateDirOut(run),
		Round:             run.round,
		Units:             engine.EpicUnitKeys(units),
		Context:           contextIDs,
		Carried:           engine.EpicUnitKeys(carried),
		MechanicalChecks:  run.checks,
		MechanizedClasses: engine.EpicErrorRules(),
		Prompts:           prompts,
		NextAction:        next,
	}
}

// epicEmittedUnits splits the slice's units, in slice order, into those the
// round asks some role, the carry's asked units and the purpose-stale units
// a purpose role of their kind is asked, and the rest, carried.
func epicEmittedUnits(run *epicReviewRun, roles []model.Role) (asked, carried []engine.EpicReviewUnit) {
	want := map[string]bool{}
	for _, u := range run.slice.Units {
		want[u.Key] = true
	}
	for i := range roles {
		if roles[i].Purpose {
			ask := epicAskOf(run, roles[i].UnitKinds(), true, map[string]bool{})
			for _, u := range ask.units {
				want[u.Key] = true
			}
		}
	}
	for _, u := range run.all {
		if want[u.Key] {
			asked = append(asked, u)
		} else {
			carried = append(carried, u)
		}
	}
	return asked, carried
}
