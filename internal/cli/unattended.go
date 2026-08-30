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
