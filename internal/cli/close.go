package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	closeStdin      bool
	closeReasonFile string
	closeSkipGate   string
)

func newCloseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "close <id> [reason]",
		Short: "Transition: wip → done (with verification)",
		Args:  cobra.RangeArgs(1, 2),
		RunE:  runClose,
	}
	cmd.Flags().BoolVar(&closeStdin, "stdin", false, "read reason from stdin")
	cmd.Flags().StringVar(&closeReasonFile, "reason-file", "", "read reason from file")
	cmd.Flags().StringVar(&closeSkipGate, "skip-gate", "", "skip gate execution, recording the reason on the closed task")
	return cmd
}

func runClose(cmd *cobra.Command, args []string) error {
	// --skip-gate usage check (§6.5)
	closeSkipGate = strings.TrimSpace(closeSkipGate)
	if cmd.Flags().Changed("skip-gate") && closeSkipGate == "" {
		output.Error(ExitUsage, "--skip-gate requires a non-empty reason")
		os.Exit(ExitUsage)
		return nil
	}
	// Determine reason source
	sources := 0
	if len(args) > 1 {
		sources++
	}
	if closeStdin {
		sources++
	}
	if closeReasonFile != "" {
		sources++
	}
	if sources > 1 {
		output.Error(ExitUsage, "multiple reason sources provided. Use exactly one: positional argument, --stdin, or --reason-file")
		os.Exit(ExitUsage)
		return nil
	}

	var reason string
	switch {
	case len(args) > 1:
		reason = args[1]
	case closeStdin:
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("read stdin: %v", err))
			os.Exit(ExitFile)
			return nil
		}
		reason = string(data)
	case closeReasonFile != "":
		data, err := os.ReadFile(closeReasonFile)
		if err != nil {
			output.Error(ExitFile, fmt.Sprintf("read reason file: %v", err))
			os.Exit(ExitFile)
			return nil
		}
		reason = string(data)
	default:
		output.Error(ExitUsage, "reason is required")
		os.Exit(ExitUsage)
		return nil
	}

	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	// Cheap checks, then a single gate run, both pre-flock (§6.1, §6.2)
	tfPre, preErr := model.ReadTaskFile(taskFilePath)
	if preErr != nil {
		output.Error(ExitFile, preErr.Error())
		os.Exit(ExitFile)
		return nil
	}
	preTask, _, preFindErr := model.FindTask(tfPre, args[0])
	if preFindErr != nil {
		output.Error(ExitState, preFindErr.Error())
		os.Exit(ExitState)
		return nil
	}
	if !model.ValidTransition(preTask.Status, model.StatusDone) {
		output.Error(ExitState, fmt.Sprintf("cannot close: task %s is %s (must be wip)", preTask.ID, preTask.Status), "Use `tp done` for implicit claim from open, or `tp claim` first.")
		os.Exit(ExitState)
		return nil
	}
	if verifyErr := engine.VerifyClosure(preTask.Acceptance, reason, false); verifyErr != nil {
		output.Error(ExitValidation, fmt.Sprintf("closure verification failed: %v", verifyErr), engine.ClosureHint(verifyErr, "Rewrite reason to address all acceptance criteria."))
		os.Exit(ExitValidation)
		return nil
	}
	gateRan := false
	if closeSkipGate == "" {
		gateRan = runQualityGatePreFlock(taskFilePath)
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, err := model.ReadTaskFile(taskFilePath)
		if err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		task, _, err := model.FindTask(tf, args[0])
		if err != nil {
			output.Error(ExitState, err.Error())
			os.Exit(ExitState)
			return nil
		}

		if !model.ValidTransition(task.Status, model.StatusDone) {
			output.Error(ExitState, fmt.Sprintf("cannot close: task %s is %s (must be wip)", task.ID, task.Status), "Use `tp done` for implicit claim from open, or `tp claim` first.")
			os.Exit(ExitState)
			return nil
		}

		// Run closure verification
		if err := engine.VerifyClosure(task.Acceptance, reason, false); err != nil {
			output.Error(ExitValidation, fmt.Sprintf("closure verification failed: %v", err), engine.ClosureHint(err, "Rewrite reason to address all acceptance criteria."))
			os.Exit(ExitValidation)
			return nil
		}

		now := time.Now().UTC()
		task.Status = model.StatusDone
		task.ClosedAt = &now
		task.ClosedReason = &reason
		switch {
		case closeSkipGate != "":
			sr := closeSkipGate
			task.GateSkippedReason = &sr
		case gateRan:
			task.GatePassedAt = &now
		}
		tf.UpdatedAt = now

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("closed %s", task.ID))
		return output.JSON(task)
	})
}
