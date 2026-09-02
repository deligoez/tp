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

// refuseUnattendedAuditConvergeOn reports a write that would change the
// resolved audit_converge_on to blocking under TP_UNATTENDED and exits 2.
// what names the attempt in the operator's own words ("tp import").
//
// §3 gives this field its own refusal rather than reusing
// refuseUnattendedCommandField: that message says the field "names a command
// the driver executes", which is false here and would mislead the one reader
// who most needs the truth — a unit deciding whether it has an authoring error
// it can fix itself or a decision it must escalate. The three exits and the
// condition selecting each are the refusal's own deliverable and are not stated
// here yet.
func refuseUnattendedAuditConvergeOn(what string) {
	output.Error(ExitUsage,
		fmt.Sprintf("%s changes the resolved audit_converge_on to blocking, which relaxes the audit gate and is a user-approved decision refused under TP_UNATTENDED", what),
		fmt.Sprintf("escalate instead: tp escalate --decision %s --evidence <what you found>",
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
func fenceAuditConvergeOnSet(what, value string, layer auditConvergeOnLayer) {
	if !engine.Unattended() {
		return
	}
	task, project := fenceOverrideLayers()
	before := engine.ResolveWorkflowLayers(&task, &project).AuditConvergeOn
	switch layer {
	case auditConvergeOnTaskLayer:
		task.AuditConvergeOn = &value
	case auditConvergeOnProjectLayer:
		project.AuditConvergeOn = &value
	}
	after := engine.ResolveWorkflowLayers(&task, &project).AuditConvergeOn
	if engine.AuditConvergeOnRelaxes(before, after) {
		refuseUnattendedAuditConvergeOn(what)
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
		refuseUnattendedAuditConvergeOn("tp import")
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
