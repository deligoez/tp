package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var removeForce bool

func newRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove <id>",
		Short: "Remove a task (open status only)",
		Args:  cobra.ExactArgs(1),
		RunE:  runRemove,
	}
	cmd.Flags().BoolVar(&removeForce, "force", false, "remove and clean up dependency references")
	return cmd
}

func runRemove(_ *cobra.Command, args []string) error {
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

		_, idx, err := model.FindTask(tf, args[0])
		if err != nil {
			output.Error(ExitState, err.Error())
			os.Exit(ExitState)
			return nil
		}

		task := &tf.Tasks[idx]
		if task.Status != model.StatusOpen {
			output.Error(ExitState, fmt.Sprintf("cannot remove: task %s is %s (must be open)", task.ID, task.Status), "Use `tp reopen` first to reset to open, then retry.")
			os.Exit(ExitState)
			return nil
		}

		// Check dependents
		var dependents []string
		for i := range tf.Tasks {
			for _, dep := range tf.Tasks[i].DependsOn {
				if dep == task.ID {
					dependents = append(dependents, tf.Tasks[i].ID)
				}
			}
		}

		if len(dependents) > 0 && !removeForce {
			output.Error(ExitState, fmt.Sprintf("cannot remove: %d tasks depend on %s (%v). Use --force to remove and clean up references", len(dependents), task.ID, dependents))
			os.Exit(ExitState)
			return nil
		}

		// Force: clean up deps
		if removeForce {
			for i := range tf.Tasks {
				newDeps := make([]string, 0)
				for _, dep := range tf.Tasks[i].DependsOn {
					if dep != args[0] {
						newDeps = append(newDeps, dep)
					}
				}
				tf.Tasks[i].DependsOn = newDeps
			}
		}

		// Remove the task
		tf.Tasks = append(tf.Tasks[:idx], tf.Tasks[idx+1:]...)

		if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
			output.Error(ExitFile, err.Error())
			os.Exit(ExitFile)
			return nil
		}

		output.Success(fmt.Sprintf("removed %s", args[0]))
		return output.JSON(map[string]string{"removed": args[0]})
	})
}
