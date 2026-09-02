package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

// §5.1's refusals. Every one of them exits 2 and names the attempt a
// user-approved decision, then points at the supported route out: tp escalate
// writes a record the driver reads and stops the run on, so a unit that needs
// one of these decisions has somewhere to go that is not a workaround.

// escalationDecision maps a fenced field to the §5.2 decision name a unit
// should escalate under. Anything without its own name escalates as "other".
func escalationDecision(field string) string {
	switch field {
	case "review_max_rounds":
		return "raise-review-cap"
	case "audit_max_rounds":
		return "raise-audit-cap"
	case "audit_converge_on":
		// v0.37.0 §3: the field has its own name so that a run stopped over
		// it is nameable. Under `other` the reason survives only in the free
		// text of --evidence, which no driver can route on.
		return "audit-converge-on"
	}
	return "other"
}

// refuseUnattended reports a user-only decision attempted under TP_UNATTENDED
// and exits 2. what names the attempt in the operator's own words ("--skip-gate",
// "raising review_max_rounds above the resolved 5"); decision is the §5.2 name
// the hint tells the unit to escalate under.
func refuseUnattended(what, decision string) {
	output.Error(ExitUsage,
		fmt.Sprintf("%s is a user-approved decision and is refused under TP_UNATTENDED", what),
		fmt.Sprintf("escalate instead: tp escalate --decision %s --evidence <what you found>", decision))
	os.Exit(ExitUsage)
}

// refuseUnattendedCommandField reports an attempt to set runner or notify_cmd
// under TP_UNATTENDED and exits 2. These are fenced more strictly than the
// caps: they name commands the driver executes, so no value at any layer is
// acceptable from a unit.
func refuseUnattendedCommandField(field string) {
	output.Error(ExitUsage,
		fmt.Sprintf("%s names a command the driver executes and cannot be set under TP_UNATTENDED, at any layer", field),
		fmt.Sprintf("escalate instead: tp escalate --decision %s --evidence <what you found>", escalationDecision(field)))
	os.Exit(ExitUsage)
}

// fenceSink names which of §3's four write paths is refusing, because the
// unintended-case exits are not the same at all four and a message that offered
// all of them everywhere would send a unit at tp set to go and re-author a
// document that is not what refused.
type fenceSink int

const (
	// fenceSinkSet is tp set --workflow and its --project form: the write
	// names the value itself.
	fenceSinkSet fenceSink = iota
	// fenceSinkImport is tp import, the one sink whose input the unit can
	// re-author into a document that resolves the same value — which is why
	// §3's two documents are remedies here and nowhere else.
	fenceSinkImport
	// fenceSinkExtract is tp config --extract, which hoists every field the
	// task files share in one move and cannot leave one behind.
	fenceSinkExtract
)

// auditConvergeOnUnintendedExit returns the exits a unit that did NOT mean to
// change the value has at this sink, with resolved naming the value the write
// moves off so "carry the resolved value explicitly" is something a unit can
// act on rather than a pointer to a value it has to go and look up.
//
// The two import remedies are §3's own and are stated only at tp import. At the
// other two sinks the write IS the change: tp set names the literal, and
// --extract moves every common field at once, so at neither is there a form of
// the command that both lands and leaves the value alone. Offering the
// documents there would be as wrong as offering nothing.
func auditConvergeOnUnintendedExit(sink fenceSink, resolved string) string {
	switch sink {
	case fenceSinkImport:
		return fmt.Sprintf(
			`the document is the fix: omit its top-level "workflow" key to carry the current block forward, or write "audit_converge_on": %q into it`,
			resolved)
	case fenceSinkExtract:
		return fmt.Sprintf(
			`do not run this hoist: --extract moves every field the task files share in one write, so no form of it lands while audit_converge_on still resolves %s`,
			resolved)
	}
	// fenceSinkSet, which is also the zero value: tp set --workflow and its
	// --project form both name the literal, so the write is the change.
	return fmt.Sprintf(
		`do not make this write: it names the value itself, so no form of it lands while audit_converge_on still resolves %s`,
		resolved)
}

// refuseUnattendedAuditConvergeOn reports a write that would change the
// resolved audit_converge_on to blocking under TP_UNATTENDED and exits 2.
// what names the attempt in the operator's own words ("tp import"), sink
// selects the authoring exits, and resolved is the value the write moves off.
//
// §3 gives this field its own refusal rather than reusing
// refuseUnattendedCommandField: that message says the field "names a command
// the driver executes", which is false here and would mislead the one reader
// who most needs the truth — a unit deciding whether it has an authoring error
// it can fix itself or a decision it must escalate.
//
// The message names all three exits AND the condition selecting each, because
// it has two audiences and the wrong pairing is silent: a unit that reads only
// "refused" stops a run over something one edit fixes, and a unit that meant
// the relax and reads only the authoring exits writes the resolved value back,
// passes, and reverts an operator-approved blocking with no escalation and no
// record.
func refuseUnattendedAuditConvergeOn(what string, sink fenceSink, resolved string) {
	output.Error(ExitUsage,
		fmt.Sprintf("%s changes the resolved audit_converge_on from %s to blocking, which relaxes the audit gate and is a user-approved decision refused under TP_UNATTENDED",
			what, resolved),
		fmt.Sprintf("if you did not mean to change it, %s; if you did, escalate: tp escalate --decision %s --evidence <what you found>",
			auditConvergeOnUnintendedExit(sink, resolved),
			escalationDecision("audit_converge_on")))
	os.Exit(ExitUsage)
}

// auditConvergeOnLayer names the override layer a write lands in, so §3's
// change rule can resolve the write through §2's precedence instead of assuming
// the written layer is the one that wins.
type auditConvergeOnLayer int

const (
	auditConvergeOnTaskLayer auditConvergeOnLayer = iota
	auditConvergeOnProjectLayer
)

// fenceOverrideLayers returns the two override layers §2's precedence resolves
// through for the active pointer: the discovered task file's own workflow block
// and the project config's. Both are best-effort — an unreadable layer
// contributes no override, exactly as EffectiveWorkflowForTaskFile treats it —
// because a fence that aborted on a malformed file would turn a read error into
// a refusal at a sink that has not yet decided anything.
//
// It speaks for the active pointer alone, which is the whole of what the
// task-level write can change. The --project form does not read it: that write
// lands under every base, and the bases it is compared over come from the scan.
func fenceOverrideLayers() (task, project model.WorkflowOverride) {
	if path, err := engine.DiscoverTaskFile(".", flagFile); err == nil {
		if o, loadErr := engine.LoadTaskWorkflowOverride(path); loadErr == nil {
			task = o
		}
	}
	return task, engine.ProjectWorkflowOverride()
}

// fenceAuditConvergeOnSet applies §3's change rule to a tp set --workflow write
// of audit_converge_on, at whichever layer the command addresses.
//
// It is string-shaped and its call sites sit before the type dispatch, which is
// the point §3 makes about the existing numeric fence: fenceWorkflowWrite takes
// a float64 and every one of its call sites runs after the literal has been
// parsed as a number, so widening it for a string field ships a silent no-op.
//
// The comparison is resolved-before against resolved-after rather than the
// literal against the layer it was written to. That is what makes a --project
// write of blocking beneath a task override of all pass (§7 row 13).
//
// The two layers are evaluated over different populations, because they do not
// write the same thing. The task-level form writes the top layer of the one base
// it discovers, so that base is the whole of what it can change and it stays
// single-base. The --project form writes the layer every base resolves through,
// so it is evaluated per base over the bases the scan enumerates. Row 13's
// carve-out is per base rather than global — a base whose task override of all
// shields it passes on its own account, while the rest are still compared.
//
// The population is the scanned overrides and nothing else. Seeding it with the
// discovered override as well was measured refusing a tree in which nothing
// moves: two bases both carrying all and no .tp/local.json make discovery
// ambiguous, so the seed is the ZERO override — an entry matching no file, which
// resolves the default and therefore relaxes against any project write of
// blocking. .tp/local.json is git-ignored, so that is the state of every fresh
// clone, CI checkout and worktree of a repository holding more than one task
// file. The seed never carried a base the scan does not: the file discovery
// finds is a scanned task file too.
//
// The population deliberately does NOT carry --extract's unconditional empty
// override, and row 13 is the reason: measured, appending it refuses a
// --project write of blocking into a tree whose only base carries a task
// override of all, which is the write that row requires to pass. The single
// empty override below is the zero-file case alone, where no base shields
// anything and there is nothing else to compare. The cost is that a base with no
// task file at all, in a tree that has other task files, is outside this
// population — tp scans task files and has no enumeration of specs, so such a
// base has no shape this fence can see.
//
// A degraded read does not shrink the population, because the command refuses
// rather than comparing what it managed to read. engine.ScanProjectTaskFiles
// returns its partial list *and* the walk error, and filepath.WalkDir stops at
// the first error handed back to it, so every base sorting after an unreadable
// directory is simply absent — and a base the fence cannot see cannot refuse.
// Measured against the earlier warn-and-proceed form: a 0000 directory sorting
// before the task files landed the write at exit 0, and under --quiet
// output.Notice returned early, so the gap was neither fenced nor reported. A
// task file that will not parse is the other case and is still compared — it
// contributes an empty override, exactly as fenceOverrideLayers does with the
// same error — so the two degraded reads are not treated alike.
//
// The refusal names the base it speaks for. The loop is per base, so a message
// that named none would be byte-identical whether one base moved or fifty, and
// no second command recovers it — tp validate --project reports no deviation
// here, because the project layer does not set the field yet, which is the
// whole reason the write was refused.
func fenceAuditConvergeOnSet(what, value string, layer auditConvergeOnLayer) {
	if !engine.Unattended() {
		return
	}
	task, project := fenceOverrideLayers()
	if layer == auditConvergeOnTaskLayer {
		before := engine.ResolveWorkflowLayers(&task, &project).AuditConvergeOn
		task.AuditConvergeOn = &value
		after := engine.ResolveWorkflowLayers(&task, &project).AuditConvergeOn
		if engine.AuditConvergeOnRelaxes(before, after) {
			refuseUnattendedAuditConvergeOn(what, fenceSinkSet, before)
		}
		return
	}
	afterProject := project
	afterProject.AuditConvergeOn = &value
	files, scanErr := engine.ScanProjectTaskFiles(engine.ProjectRoot("."))
	if scanErr != nil {
		// ExitFile rather than this fence's own ExitUsage, because an
		// unreadable path is not a refused decision: there is no approval
		// that would make it listable and nothing for tp escalate to
		// record, so a unit sent to escalate here would stop a run over a
		// chmod. tp config --extract exits ExitFile on this identical error
		// from this identical call. output.Error writes on every stream
		// mode, --quiet included, which output.Notice does not.
		output.Error(ExitFile,
			fmt.Sprintf("project scan incomplete: %v; audit_converge_on cannot be compared over every base", scanErr),
			"make that path readable, or move it outside the project root, then re-run")
		os.Exit(ExitFile)
	}
	population := make([]model.WorkflowOverride, 0, len(files))
	bases := make([]string, 0, len(files))
	for _, f := range files {
		o, _ := engine.LoadTaskWorkflowOverride(f)
		population = append(population, o)
		bases = append(bases, filepath.Base(f))
	}
	if len(population) == 0 {
		population = append(population, model.WorkflowOverride{})
		bases = append(bases, "")
	}
	for i := range population {
		before := engine.ResolveWorkflowLayers(&population[i], &project).AuditConvergeOn
		after := engine.ResolveWorkflowLayers(&population[i], &afterProject).AuditConvergeOn
		if engine.AuditConvergeOnRelaxes(before, after) {
			attempt := what
			if bases[i] != "" {
				attempt = fmt.Sprintf("%s, resolved through %s,", what, bases[i])
			}
			refuseUnattendedAuditConvergeOn(attempt, fenceSinkSet, before)
			return
		}
	}
}

// fenceAuditConvergeOnImport applies §3's change rule to tp import, comparing
// what targetPath resolves today against what the incoming document's workflow
// block would make it resolve.
//
// It takes targetPath rather than calling fenceOverrideLayers because an import
// writes the file its own document's spec names, which is not necessarily the
// active pointer — resolvedWorkflowForFence and fenceOverrideLayers both read
// the active pointer, and reusing either here would compare the wrong file.
//
// incoming is the block that will be written, *after* tp import's preservation
// step has run, so a document omitting the top-level workflow key is compared
// as the carried-forward block it will actually become. That is what makes an
// import of an already-resolved blocking pass (§7 row 13) while the document
// that replaces the block and names neither literal is refused (row 13b).
func fenceAuditConvergeOnImport(targetPath string, incoming *model.WorkflowOverride) {
	if !engine.Unattended() {
		return
	}
	project := engine.ProjectWorkflowOverride()
	var existing model.WorkflowOverride
	if o, err := engine.LoadTaskWorkflowOverride(targetPath); err == nil {
		existing = o
	}
	before := engine.ResolveWorkflowLayers(&existing, &project).AuditConvergeOn
	after := engine.ResolveWorkflowLayers(incoming, &project).AuditConvergeOn
	if engine.AuditConvergeOnRelaxes(before, after) {
		refuseUnattendedAuditConvergeOn("tp import", fenceSinkImport, before)
	}
}

// fenceWorkflowWrite refuses a raise of a fenced cap field under
// TP_UNATTENDED, comparing the requested value against the currently resolved
// one. It is a no-op when the mode is off or the field is not fenced, so both
// tp set --workflow and its --project form can call it on every field they
// parse.
func fenceWorkflowWrite(field string, requested float64) {
	if !engine.Unattended() {
		return
	}
	if engine.FencedCommandField(field) {
		refuseUnattendedCommandField(field)
		return
	}
	if !engine.FencedCapField(field) {
		return
	}
	wf := resolvedWorkflowForFence()
	resolved, ok := engine.ResolvedCapValue(&wf, field)
	if !ok || !engine.UnattendedRaise(field, requested, resolved) {
		return
	}
	refuseUnattended(
		fmt.Sprintf("setting %s to %s above the resolved %s",
			field, formatCapValue(requested), formatCapValue(resolved)),
		escalationDecision(field))
}

// formatCapValue renders a cap for a message without a float's trailing zeros,
// so an integer field reads as "5" rather than "5.000000".
func formatCapValue(v float64) string {
	return fmt.Sprintf("%g", v)
}

// resolvedWorkflowForFence returns the workflow the fence compares against: the
// discovered task file's effective workflow, or the project config's alone when
// no task file is discoverable. Those two layers plus the built-in are the whole
// of section 7's precedence for workflow fields — tp reads no TP_<FIELD>
// environment variable and exposes no CLI flag for any of them — so the fence
// has no upper layer to ignore, rather than ignoring one.
func resolvedWorkflowForFence() model.Workflow {
	if path, err := engine.DiscoverTaskFile(".", flagFile); err == nil {
		return engine.EffectiveWorkflowForTaskFile(path)
	}
	empty := model.WorkflowOverride{}
	if wf, _, err := engine.ResolveEffectiveWorkflow(".", &empty); err == nil {
		return wf
	}
	return engine.ResolveWorkflowLayers(&empty, &empty)
}
