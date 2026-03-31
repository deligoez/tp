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

func runReopen(_ *cobra.Command, args []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
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
			output.Error(ExitState, fmt.Sprintf("cannot reopen: task %s is %s (must be done)", task.ID, task.Status))
			os.Exit(ExitState)
			return nil
		}

		task.Status = model.StatusOpen
		task.ClosedAt = nil
		task.ClosedReason = nil
		task.GatePassedAt = nil
		task.CommitSHA = nil
		tf.UpdatedAt = time.Now().UTC()

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("reopened %s", task.ID))
		return output.JSON(map[string]string{"reopened": task.ID})
	})
}
