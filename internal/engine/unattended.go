package engine

import (
	"os"

	"github.com/deligoez/tp/internal/model"
)

// §5.1's fence. tp run exports TP_UNATTENDED=1 to every child, and under it the
// decisions CLAUDE.md reserves for the user stop being available to the agent:
// skipping a quality gate, raising a round budget or a driver cap, forcing an
// import, and naming a command the driver itself executes.
//
// The postmortem behind it is that a filesystem cannot tell an agent-authored
// approval from a user-authored one, so a forged approval is treated as
// authoritative downstream. tp's equivalent is a unit that raises its own round
// budget and then declares convergence. This is a fence rather than a sandbox:
// it is enforced at tp's own CLI, and a unit that strips the variable from its
// environment is outside what tp can prevent. The guarantee is only that no
// unattended unit reaches those decisions through a supported route.

// UnattendedActive reports whether unattended mode is active for a given
// TP_UNATTENDED value: on when the variable is present, non-empty and not "0",
// off on unset, empty or "0" (§5.1). The active side is deliberately
// permissive — a harness that writes "true" fences its unit exactly as the
// driver's own "1" does.
func UnattendedActive(value string) bool {
	return value != "" && value != "0"
}

// Unattended reports whether this process is running as an unattended unit,
// reading TP_UNATTENDED from its own environment.
func Unattended() bool {
	return UnattendedActive(os.Getenv(EnvUnattended))
}

// fencedCapFields are the workflow fields §5.1 fences by value: under the
// variable a unit may lower them but never raise them, since a unit that can
// raise its own budget can manufacture convergence, and one that can raise
// run_max_units or run_max_wall_clock_seconds can run itself indefinitely.
//
// The mapped bool says whether 0 means *disabled* for that field rather than
// *lowest*. For those, 0 is the least restrictive value there is, so setting 0
// over a non-zero resolved value is a raise — while run_max_unit_retries' 0 is
// simply the fewest attempts and is always accepted.
var fencedCapFields = map[string]bool{
	"review_max_rounds":          true,
	"audit_max_rounds":           true,
	"run_max_budget_usd":         true,
	"run_max_unit_budget_usd":    true,
	"run_max_units":              false,
	"run_max_wall_clock_seconds": false,
	"run_max_unit_retries":       false,
}

// FencedCapField reports whether field is one §5.1 fences by value.
func FencedCapField(field string) bool {
	_, ok := fencedCapFields[field]
	return ok
}

// FencedCommandField reports whether field names a command the driver itself
// executes — runner and notify_cmd. §5.1 fences these more strictly than the
// caps: under the variable a unit cannot set them at all, at any layer and at
// any value.
func FencedCommandField(field string) bool {
	return field == "runner" || field == "notify_cmd"
}

// UnattendedRaise reports whether setting a fenced cap field to requested is a
// raise against the currently resolved value. An equal or lower value is not,
// since lowering a budget cannot manufacture convergence; 0 is treated as
// unbounded for the fields whose 0 means disabled.
func UnattendedRaise(field string, requested, resolved float64) bool {
	if fencedCapFields[field] {
		switch {
		case requested == 0 && resolved == 0:
			// 0 where 0 already resolves changes nothing.
			return false
		case requested == 0:
			return true
		case resolved == 0:
			// Anything is more restrictive than no cap at all.
			return false
		}
	}
	return requested > resolved
}

// ResolvedCapValue returns an already-resolved workflow's value for a fenced
// cap field, and whether field is one. Reading the number out of the resolved
// workflow is what keeps the comparison layer-agnostic: the fence consults no
// layer of its own, so it compares whatever the documented precedence resolved
// and cannot be fed a different number by a route that bypasses it. tp reads no
// TP_<FIELD> environment variable for any workflow field (section 7), so there
// is no env layer here to exclude — an earlier draft of this comment said there
// was, and a test written against it could not have discriminated.
func ResolvedCapValue(wf *model.Workflow, field string) (float64, bool) {
	switch field {
	case "review_max_rounds":
		return float64(wf.ReviewMaxRounds), true
	case "audit_max_rounds":
		return float64(wf.AuditMaxRounds), true
	case "run_max_units":
		return float64(wf.RunMaxUnits), true
	case "run_max_wall_clock_seconds":
		return float64(wf.RunMaxWallClockSeconds), true
	case "run_max_budget_usd":
		return wf.RunMaxBudgetUSD, true
	case "run_max_unit_budget_usd":
		return wf.RunMaxUnitBudgetUSD, true
	case "run_max_unit_retries":
		return float64(wf.RunMaxUnitRetries), true
	}
	return 0, false
}
