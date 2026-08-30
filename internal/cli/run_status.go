package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

// The three values §3.5 gives run_state. A run is stopped once it recorded a
// stop reason; without one it is either a driver still working or a driver that
// died, and only the run lock tells those apart.
const (
	runStateStopped  = "stopped"
	runStateInFlight = "in_flight"
	runStateCrashed  = "crashed"
)

const noRunStateHint = "start a run with tp run; --status reports the current or last run of the resolved task file"

// runRunStatus implements `tp run --status` (§3.5): the current or last run's
// phase, its units done and accrual against each cap, the last unit's exit code
// and log path, its stop reason once it has ended, and run_state.
//
// It reads the run state file and takes no run lock, so it reports on a run
// that is still in flight instead of queueing behind it. That is the whole
// reason the run state is written at all: §3.5 makes the file observability
// rather than truth, and this is its one reader.
func runRunStatus(args []string) error {
	taskFilePath, specPath, _ := resolveCycle(args)

	st, err := engine.ReadRunState(".", taskFilePath)
	if err != nil {
		// A missing file is not a failure — it is the answer that no run has
		// been driven over this cycle — so §3.5 gives it exit 3 with a hint
		// naming the command that would produce one.
		if errors.Is(err, os.ErrNotExist) {
			output.Error(ExitFile,
				fmt.Sprintf("no run state for %s", filepath.Base(taskFilePath)), noRunStateHint)
		} else {
			output.Error(ExitFile, err.Error(), "delete "+engine.RunStatePath(".", taskFilePath)+" to clear a corrupt run state")
		}
		os.Exit(ExitFile)
		return nil
	}

	wf := engine.EffectiveWorkflowForTaskFile(taskFilePath)
	result := map[string]any{
		"run_id":      st.RunID,
		"started_at":  st.StartedAt,
		"phase":       st.Phase,
		"run_state":   runStateOf(st, taskFilePath),
		"stop_reason": st.StopReason,
		// The accrual, beside the cap that bounds it. units_done counts
		// ATTEMPTS, the same number run_max_units is compared against and the
		// same number totals.units carries (§3.4) — a retried unit is counted
		// once per attempt, so the three never disagree.
		"units_done":         st.Totals.Units,
		"wall_clock_seconds": st.Totals.WallClockSeconds,
		"spend_usd":          st.Totals.SpendUSD,
		"caps": map[string]any{
			"max_units":              wf.RunMaxUnits,
			"max_wall_clock_seconds": wf.RunMaxWallClockSeconds,
			"max_budget_usd":         wf.RunMaxBudgetUSD,
		},
	}
	// §7: under --compact the per-unit row goes, and the log path — the one
	// unbounded string in the payload, an absolute path per attempt — goes
	// with it. stop_reason and the cap totals above are what a driver or an
	// operator decides on, so they survive; the row is diagnosis, and
	// diagnosis is what --compact exists to defer to the full report.
	if !flagCompact {
		result["last_unit"] = lastRunUnit(st)
	}
	// §1/§3.5: the divergence signal travels on any audit-phase stop, so the
	// operator gets at the end of a run what v0.32.0's operator had to derive
	// by hand at round ten.
	if st.StopReason != nil && st.Phase == engine.PhaseAudit {
		addAuditSignals(result, specPath, wf.AuditCleanRounds)
	}
	return output.JSON(result)
}

// runStateOf classifies a run (§3.5). A recorded stop reason is conclusive;
// without one the run lock is the only evidence, because a driver that died
// left a file indistinguishable from one still being written.
func runStateOf(st *engine.RunState, taskFilePath string) string {
	if st.StopReason != nil {
		return runStateStopped
	}
	if engine.RunLockHeld(taskFilePath) {
		return runStateInFlight
	}
	return runStateCrashed
}

// lastRunUnit returns the last attempt row, or nil for a run that has not
// spawned anything yet. It is a pointer into the caller's state rather than a
// copy because the payload is serialized and discarded.
func lastRunUnit(st *engine.RunState) *engine.RunUnitRow {
	if len(st.Units) == 0 {
		return nil
	}
	return &st.Units[len(st.Units)-1]
}

// addAuditSignals writes §2.5's three fields onto the status payload from the
// same inputs `tp audit <spec> --status` computes them from, so an operator
// reading the end of a run reads the same numbers as one running the audit
// command by hand.
//
// Every failure omits the fields rather than failing the report: --status's job
// is to say what the run did, and a spec that has moved or an audit state that
// cannot be loaded must not cost the operator the run's phase and stop reason.
func addAuditSignals(result map[string]any, specPath string, cleanRounds int) {
	if specPath == "" {
		return
	}
	if _, err := os.Stat(specPath); err != nil {
		return
	}
	specHash, err := engine.SpecHash(specPath)
	if err != nil {
		return
	}
	st, err := engine.LoadReviewState(specPath)
	if err != nil && !engine.IsRebuildableStateIndex(err) {
		return
	}
	rounds := []engine.ReviewRound{}
	if st != nil && err == nil {
		rounds = st.AuditRounds
	}
	rolesHash, _ := engine.ComputeRolesHash(filepath.Dir(specPath), engine.PhaseAuditors)
	auditSignalFields(result, specPath, rounds, cleanRounds,
		engine.StateStale(rounds, specHash),
		engine.Converged(rounds, cleanRounds, specHash),
		rolesHash)
}
