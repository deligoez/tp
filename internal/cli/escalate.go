package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// escalateOutsideRunHint answers the one refusal an agent is likely to hit by
// accident — running the command by hand — by naming what it should do instead.
const escalateOutsideRunHint = "tp escalate records a decision a running unit cannot take; outside a run there is no unit to stop, so make the decision directly"

// escalateEvidenceHint says what the evidence is for. The record's only reader
// is a human deciding, so evidence that says nothing costs them the run.
const escalateEvidenceHint = "--evidence is what the operator reads before deciding: name the observation that forced the decision, not the decision"

func newEscalateCmd() *cobra.Command {
	var decision, evidence string
	var options []string

	cmd := &cobra.Command{
		Use:   "escalate",
		Short: "Record a decision only the operator can take, and stop the unit",
		Long: `Write this unit's escalation record and exit 2.

An escalation is a normal, expected outcome — not a crash. A unit that reaches a
user-only decision (--skip-gate, raising a review or audit cap, tp import
--force, relaxing audit_converge_on to blocking) records what it needs decided
instead of deciding it, and the run stops with stop_reason "escalation" so an
operator can answer.

The record is written to $TP_RUN_DIR/$TP_UNIT_SEQ-escalation.json — per unit, so
concurrent role siblings never clobber each other — and carries {decision,
unit_kind, unit_id, phase, evidence, options[], at}. The record, not the exit
code, is what the driver reads.

--decision is one of skip-gate, raise-review-cap, raise-audit-cap, import-force,
audit-converge-on or other. --option is repeatable and lists the ways forward
the unit saw.

Outside a run (TP_RUN_DIR unset) this is a usage error, so the command cannot be
used to fabricate a record.`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runEscalate(decision, evidence, options)
		},
	}

	cmd.Flags().StringVar(&decision, "decision", "", "the decision needed: "+strings.Join(engine.EscalationDecisions(), ", "))
	cmd.Flags().StringVar(&evidence, "evidence", "", "what the operator needs to know to decide")
	// StringArray, not StringSlice: an option is prose, and splitting it on
	// commas would turn one option into several.
	cmd.Flags().StringArrayVar(&options, "option", nil, "a way forward the unit saw (repeatable)")

	return cmd
}

// runEscalate implements `tp escalate` (§5.2).
//
// Every refusal here is exit 2 and writes nothing, which is the fence: the
// record is evidence that a unit under the driver stopped for a decision, so a
// command that could produce one outside a run — or with a decision outside the
// documented set, or with no evidence to read — would let a unit fabricate that
// evidence rather than report it.
//
// A successful escalation also exits 2. tp did not run the request the unit was
// spawned for, and the unit has stopped; what separates it from any other exit
// 2 is the record on disk, which is why the record is emitted on stdout too.
func runEscalate(decision, evidence string, options []string) error {
	runDir := os.Getenv(engine.EnvRunDir)
	if runDir == "" {
		return escalateRefusal(engine.EnvRunDir+" is not set: tp escalate runs only inside a unit tp run spawned", escalateOutsideRunHint)
	}
	unitSeq := os.Getenv(engine.EnvUnitSeq)
	if unitSeq == "" {
		return escalateRefusal(engine.EnvUnitSeq+" is not set, so the escalation record has no path", escalateOutsideRunHint)
	}

	if !engine.IsEscalationDecision(decision) {
		return escalateRefusal(
			fmt.Sprintf("invalid --decision: %q", decision),
			"--decision is one of "+strings.Join(engine.EscalationDecisions(), ", "))
	}
	if strings.TrimSpace(evidence) == "" {
		return escalateRefusal("--evidence is required and must not be blank", escalateEvidenceHint)
	}

	record := &engine.Escalation{
		Decision: decision,
		UnitKind: engine.UnitKind(os.Getenv(engine.EnvUnitKind)),
		UnitID:   os.Getenv(engine.EnvUnitID),
		Phase:    os.Getenv(engine.EnvPhase),
		Evidence: evidence,
		Options:  options,
	}
	path, err := engine.WriteEscalation(runDir, unitSeq, record)
	if err != nil {
		// The record is the signal, so a record that could not be written is
		// not an escalation at all: it is a file error, and the unit will be
		// judged by its predicate and exit code instead.
		output.Error(ExitFile, "cannot write the escalation record: "+err.Error(),
			"check that "+runDir+" exists and is writable")
		os.Exit(ExitFile)
	}

	if err := output.JSON(map[string]any{
		"escalation": path,
		"decision":   record.Decision,
		"unit_kind":  record.UnitKind,
		"unit_id":    record.UnitID,
		"phase":      record.Phase,
		"options":    record.Options,
	}); err != nil {
		return err
	}
	os.Exit(ExitUsage)
	return nil
}

// escalateRefusal emits one of the command's refusals and exits 2. It never
// returns — the return value exists only so every call site can be written as
// `return escalateRefusal(...)`, which keeps the refusals visibly terminal.
func escalateRefusal(msg, hint string) error {
	output.Error(ExitUsage, msg, hint)
	os.Exit(ExitUsage)
	return nil
}
