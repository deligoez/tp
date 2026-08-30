package cli

import (
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

	return output.JSON(map[string]any{
		"run_id":      result.RunID,
		"phase":       result.Phase,
		"stop_reason": result.StopReason,
		"units":       result.Units,
	})
}
