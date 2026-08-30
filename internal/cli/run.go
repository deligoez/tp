package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/output"
)

func newRunCmd() *cobra.Command {
	var statusMode bool

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

Output: {run_id, phase, stop_reason, units}`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if statusMode {
				return runRunStatus(args)
			}
			return runRun(cmd, args)
		},
	}

	cmd.Flags().BoolVar(&statusMode, "status", false, "report the current or last run instead of driving one")
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

	if err := output.JSON(map[string]any{
		"run_id":      result.RunID,
		"phase":       result.Phase,
		"stop_reason": result.StopReason,
		"units":       result.Units,
	}); err != nil {
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
