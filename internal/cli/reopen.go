package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

func newReopenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "reopen <id>",
		Short: "Transition: done → open",
		Args:  cobra.ExactArgs(1),
		RunE:  runReopen,
	}
}

// backToOpenHint names the command that returns task to open, which depends on
// where it is: a wip task is unclaimed, a done task is reopened. A hint that
// named tp reopen for a wip task sent the caller to a command that refuses it.
func backToOpenHint(task *model.Task) string {
	if task.Status == model.StatusWIP {
		return fmt.Sprintf("Use `tp unclaim %s` to return the wip task to open.", task.ID)
	}
	return fmt.Sprintf("Use `tp reopen %s` to return the done task to open.", task.ID)
}

func runReopen(_ *cobra.Command, args []string) error {
	taskFilePath, err := discoverWriteTarget()
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
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

		if !model.ValidTransition(task.Status, model.StatusOpen) {
			hint := make([]string, 0, 1)
			if task.Status == model.StatusWIP {
				hint = append(hint, backToOpenHint(task))
			}
			output.Error(ExitState, fmt.Sprintf("cannot reopen: task %s is %s (must be done)", task.ID, task.Status), hint...)
			os.Exit(ExitState)
			return nil
		}

		task.Status = model.StatusOpen
		task.StartedAt = nil
		task.ClosedAt = nil
		task.ClosedReason = nil
		task.GatePassedAt = nil
		task.CommitSHA = nil
		task.CommitSHAs = nil
		task.CommitFiles = nil
		task.CommitFilesTotal = 0
		task.DurationSource = ""
		task.GateSkippedReason = nil
		tf.UpdatedAt = time.Now().UTC()

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("reopened %s", task.ID))
		return output.JSON(map[string]string{"reopened": task.ID, "file": taskFileLabel(taskFilePath)})
	})
}
