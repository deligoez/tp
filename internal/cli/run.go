package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

func newRunCmd() *cobra.Command {
	var statusMode, dryRunMode bool

	cmd := &cobra.Command{
		Use:   "run [spec]",
		Short: "Drive the cycle unattended, one unit at a time",
		Long: `The unattended driver. It takes the same optional spec argument and --file flag
as tp resume and discovers the same way when neither is given, then executes one
iteration at a time: read the cycle state, stop if it is releasable, take
next_units, spawn a runner process per unit — units marked concurrent together,
every other kind alone — re-read the state from disk, check caps, loop.

A unit's result is whatever it wrote to disk. The driver reads a child's exit
code and nothing else it said, and tp resume remains the single authority on
whether the cycle is finished.

--status reports the current or last run instead of driving one: phase, units
done, the accrual against each cap, the last unit's exit code and log path,
stop_reason once the run has ended, and run_state (in_flight, crashed or
stopped). It takes no run lock, so it reports on a run still in flight, and it
exits 3 when no run state exists for the resolved task file. Under --compact the
stop reason and the cap totals survive and the last unit's row — its log path
with it — is stripped.

--dry-run reports the units the driver would execute next instead of driving
them: the same reading of the cycle the loop opens each iteration with, and the
same batch it would spawn, printed as tp resume's next_units. It spawns
nothing, writes no run state and takes no run lock, so it is safe to point at a
cycle another run is already driving.

A stop for any reason other than converged invokes the configured notify_cmd,
exec'd without a shell and split on whitespace, carrying TP_STOP_REASON,
TP_RUN_ID and — on an escalation — TP_ESCALATION_PATH. What it did is reported
under notify; its own exit code changes nothing about the run.

Output: {run_id, phase, stop_reason, units}
        notify: {cmd, exit_code, error} when a notify_cmd ran
        --dry-run: {phase, round, next_units}`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// The two read-only sub-modes are checked before the driving
			// one, --status first. Neither spawns, writes or locks
			// anything, so the combination is harmless rather than a usage
			// error — but the order is fixed here so that it is an answer
			// rather than a coin toss.
			if statusMode {
				return runRunStatus(args)
			}
			if dryRunMode {
				return runRunDryRun(args)
			}
			return runRun(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&statusMode, "status", false, "report the current or last run instead of driving one")
	cmd.Flags().BoolVar(&dryRunMode, "dry-run", false, "list the units the driver would execute next, without spawning any of them")
	return cmd
}

// runRun drives one run to its stop reason.
//
// The run-scoped lock is held for the whole run (§3.1.1) — a second tp run over
// the same task file is refused with exit 4 — and it is deliberately not the
// task-file write lock: that one stays the children's, taken and released by
// each tp write they make, so a child closes its own task while its driver
// runs.
func runRun(_ *cobra.Command, args []string) error {
	taskFilePath, specPath, _ := resolveCycle(args)

	opts := engine.DriverOptions{
		Root:     ".",
		TaskFile: taskFilePath,
		Spec:     specPath,
		Workflow: engine.EffectiveWorkflowForTaskFile(taskFilePath),
	}

	var result engine.DriverResult
	if err := engine.WithRunLock(taskFilePath, func() error {
		var runErr error
		result, runErr = engine.RunDriver(&opts)
		return runErr
	}); err != nil {
		return err
	}

	payload := map[string]any{
		"run_id":      result.RunID,
		"phase":       result.Phase,
		"stop_reason": result.StopReason,
		"units":       result.Units,
	}
	// §5.2: a configured notify_cmd reports what it did, beside the stop it
	// announced. The key is absent when no command ran, so the common case
	// costs a reader nothing, and it is a report rather than a result: the
	// exit code below is decided over stop_reason alone, so a notification
	// that failed never reads as a run that ended differently.
	if n := result.Notify; n != nil {
		payload["notify"] = notifyPayload(n)
	}
	if err := output.JSON(payload); err != nil {
		return err
	}
	// §3.4: the payload is emitted first and the exit code is decided over it,
	// so a caller that reads stdout and a caller that branches on the exit code
	// are told the same thing about the same run. Exiting before the write
	// would leave the non-converged stops — every stop that actually needs a
	// human — as the only ones with no report.
	if code := runExitCode(result.StopReason); code != ExitSuccess {
		os.Exit(code)
	}
	return nil
}

// notifyPayload renders one notify_cmd invocation (§5.2).
//
// exit_code and error are both always present, one of them null: the command
// either came back with a code or never ran, and a shape that changes with the
// outcome would make a reader test for a key before it can read a value.
func notifyPayload(n *engine.NotifyOutcome) map[string]any {
	row := map[string]any{"cmd": n.Cmd, "exit_code": nil, "error": nil}
	if n.ExitCode != nil {
		row["exit_code"] = *n.ExitCode
	}
	if n.Err != nil {
		row["error"] = n.Err.Error()
	}
	return row
}

// runExitCode is §3.4's exit-code contract: 0 on `converged`, 4 on every other
// stop reason. (A usage error raised before the loop starts exits 2 through
// dispatchError, and never reaches here.)
//
// It is a whitelist of the one reason that exits 0 rather than a list of the
// eight that exit 4, so a stop reason added later — or an empty one from a run
// that ended without naming its reason — is reported as needing a human instead
// of being silently accepted as convergence. That asymmetry is the point: every
// non-converged stop is a report to a human, never an acceptance, so the
// direction the default falls in is the whole safety property.
func runExitCode(reason engine.StopReason) int {
	if reason == engine.StopConverged {
		return ExitSuccess
	}
	return ExitState
}

// runRunDryRun implements `tp run --dry-run` (§3.5): the units the driver would
// execute next, printed instead of driven.
//
// It takes no run lock, spawns nothing and writes no run state, so it answers
// "what happens if I start this?" without changing the answer — including while
// another run holds the lock, which is exactly when an operator asks.
//
// The workflow is deliberately not resolved: the caps bound a run and the runner
// says what to spawn, and this mode does neither. Leaving it zero keeps the
// mode's promise structural rather than a rule to remember, since there is no
// resolved runner here for a later edit to accidentally spawn.
func runRunDryRun(args []string) error {
	taskFilePath, specPath, _ := resolveCycle(args)

	phase, round, units, err := engine.DryRunUnits(&engine.DriverOptions{
		Root:     ".",
		TaskFile: taskFilePath,
		Spec:     specPath,
	})
	if err != nil {
		return err
	}

	// §4.1: the entries are tp resume's next_units, key for key, so an
	// operator and a driver read one contract rather than two spellings of
	// it. The array is built with make so an empty listing serializes as []
	// and never as null.
	rows := make([]map[string]any, 0, len(units))
	for _, u := range units {
		rows = append(rows, map[string]any{
			"kind":          string(u.Kind),
			"id":            u.ID,
			"brief_command": u.BriefCommand,
		})
	}
	return output.JSON(map[string]any{
		"phase":      phase,
		"round":      round,
		"next_units": rows,
	})
}
