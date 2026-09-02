package cli

import (
	"fmt"
	"os"

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
	// fenceSinkSet is tp set --workflow: the write names the value itself.
	// Its --project form does not use this sink — that layer is refused on
	// the value alone and carries its own message, because it has no single
	// resolved value to name.
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
	// fenceSinkSet, which is also the zero value: tp set --workflow names
	// the literal, so the write is the change.
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
// The message names the exits ITS OWN SINK has, and the condition selecting
// each — three at tp import, two at tp set (either layer) and at
// tp config --extract, since the two authoring exits are an import's and no
// other sink's. It names them because the refusal has two audiences and the
// wrong pairing is silent: a unit that reads only
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
// task-level write can change. The --project form does not call it at all: that
// write lands under every base, so no single base's layers decide it.
func fenceOverrideLayers() (task, project model.WorkflowOverride) {
	if path, err := engine.DiscoverTaskFile(".", flagFile); err == nil {
		if o, loadErr := engine.LoadTaskWorkflowOverride(path); loadErr == nil {
			task = o
		}
	}
	return task, engine.ProjectWorkflowOverride()
}

// refuseUnattendedAuditConvergeOnProject reports a tp set --workflow --project
// write of blocking under TP_UNATTENDED and exits 2.
//
// It is separate from refuseUnattendedAuditConvergeOn because that message
// names the resolved value the write moves off, and this sink has no single
// such value: the write lands under every base at once, and the bases it moves
// include ones tp cannot enumerate. A message saying "from all to blocking"
// here would report one base's resolution as if it were the tree's.
//
// The unintended-case exit is the phrase tp set's other layer already uses —
// the write names the value itself, so no form of it lands — carrying the scope
// the value rule actually has, which is every tree rather than this one.
func refuseUnattendedAuditConvergeOnProject(what string) {
	output.Error(ExitUsage,
		fmt.Sprintf("%s writes blocking into the layer every base resolves through, including bases tp cannot enumerate; blocking is the value that relaxes the audit gate, and setting it is a user-approved decision refused under TP_UNATTENDED",
			what),
		fmt.Sprintf("if you did not mean to change it, do not make this write: blocking at the project layer is refused whatever any single base resolves today; if you did, escalate: tp escalate --decision %s --evidence <what you found>",
			escalationDecision("audit_converge_on")))
	os.Exit(ExitUsage)
}

// fenceAuditConvergeOnSet applies §3's fence to a tp set --workflow write of
// audit_converge_on, at whichever layer the command addresses. The two layers
// get different rules, and that asymmetry is the point rather than an
// inconsistency:
//
//   - The task-level form writes one base's own top layer. That base is the
//     whole of what the write can change, so single-base reasoning is correct
//     there and the rule is §3's change rule, resolved through §2's precedence.
//
//   - The --project form writes the layer every base resolves through,
//     including bases tp cannot enumerate. A write of blocking is refused on
//     its VALUE, whatever any single base resolves today.
//
// The value rule replaced three attempts at a per-base change rule here, each
// falsified by a base the population could not reach: a --file naming a task
// file outside the project root, and any base under vendor/, node_modules/,
// .tp/, .git/ or a nested submodule, which engine.ScanProjectTaskFiles skips by
// design and without returning an error. The discriminating pair was the same
// base with the same content in two places — refused in spec/, exit 0 in
// vendor/. A fence that must enumerate its population cannot be correct over a
// population it cannot enumerate, so this one enumerates nothing.
//
// The change rule stands at every sink that is not this one, because the sinks
// have different shapes and not because one rule is better.
//
//   - tp set --workflow at the task layer: the change rule, for the reason
//     above — that base's own top layer is the whole of what the write changes.
//   - tp import: the change rule, because it carries the existing block
//     forward, so a value rule there would refuse every import a project makes
//     once it resolves blocking — the Workflow A step 6 deadlock §3 documents.
//   - tp config --extract: the change rule, evaluated over an unconditional
//     empty override, which is what makes it equivalent to a value rule in
//     effect; its scan is its own input rather than only the fence's.
//
// What makes a value rule free at THIS sink is that tp set names a value rather
// than carrying state forward: there is no document to re-author and nothing
// carried in. That is the whole of why the --project layer can take one and tp
// import cannot.
//
// A write of all never trips it at either layer, since all is the default and
// the only value that tightens the gate. That is also why no scan runs: a
// degraded scan used to exit 3 on a write of all, which can relax nothing.
//
// It is string-shaped and its call sites sit before the type dispatch, which is
// the point §3 makes about the existing numeric fence: fenceWorkflowWrite takes
// a float64 and every one of its call sites runs after the literal has been
// parsed as a number, so widening it for a string field ships a silent no-op.
func fenceAuditConvergeOnSet(what, value string, layer auditConvergeOnLayer) {
	if !engine.Unattended() {
		return
	}
	if layer == auditConvergeOnProjectLayer {
		if value == engine.AuditConvergeOnBlocking {
			refuseUnattendedAuditConvergeOnProject(what)
		}
		return
	}
	task, project := fenceOverrideLayers()
	before := engine.ResolveWorkflowLayers(&task, &project).AuditConvergeOn
	task.AuditConvergeOn = &value
	after := engine.ResolveWorkflowLayers(&task, &project).AuditConvergeOn
	if engine.AuditConvergeOnRelaxes(before, after) {
		refuseUnattendedAuditConvergeOn(what, fenceSinkSet, before)
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
