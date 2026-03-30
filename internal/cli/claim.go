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

var claimAllReady bool

func newClaimCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "claim <id> [id...]",
		Short: "Transition: open → wip",
		Args:  cobra.MinimumNArgs(0),
		RunE:  runClaim,
	}
	cmd.Flags().BoolVar(&claimAllReady, "all-ready", false, "claim all ready tasks")
	return cmd
}

func runClaim(_ *cobra.Command, args []string) error {
	if !claimAllReady && len(args) == 0 {
		output.Error(ExitUsage, "task ID required (or use --all-ready)")
		os.Exit(ExitUsage)
		return nil
	}

	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	return engine.WithFileLock(taskFilePath, func() error {
		tf, readErr := model.ReadTaskFile(taskFilePath)
		if readErr != nil {
			output.Error(ExitFile, readErr.Error())
			os.Exit(ExitFile)
			return nil
		}

		// Determine which IDs to claim
		var ids []string
		if claimAllReady {
			done := make(map[string]bool)
			for i := range tf.Tasks {
				if tf.Tasks[i].Status == model.StatusDone {
					done[tf.Tasks[i].ID] = true
				}
			}
			for i := range tf.Tasks {
				if tf.Tasks[i].Status != model.StatusOpen {
					continue
				}
				allDone := true
				for _, dep := range tf.Tasks[i].DependsOn {
					if !done[dep] {
						allDone = false
						break
					}
				}
				if allDone {
					ids = append(ids, tf.Tasks[i].ID)
				}
			}
			if len(ids) == 0 {
				output.Error(ExitState, "no ready tasks to claim")
				os.Exit(ExitState)
				return nil
			}
		} else {
			ids = args
		}

		// Single claim: preserve original output format
		if len(ids) == 1 && !claimAllReady {
			return claimSingle(tf, taskFilePath, ids[0])
		}

		// Batch claim
		return claimBatch(tf, taskFilePath, ids)
	})
}

func claimSingle(tf *model.TaskFile, taskFilePath, id string) error {
	task, _, err := model.FindTask(tf, id)
	if err != nil {
		output.Error(ExitState, err.Error())
		os.Exit(ExitState)
		return nil
	}

	if !model.ValidTransition(task.Status, model.StatusWIP) {
		output.Error(ExitState, fmt.Sprintf("cannot claim: task %s is %s (must be open)", task.ID, task.Status))
		os.Exit(ExitState)
		return nil
	}

	// Check deps are done
	done := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			done[tf.Tasks[i].ID] = true
		}
	}
	for _, dep := range task.DependsOn {
		if !done[dep] {
			output.Error(ExitState, fmt.Sprintf("cannot claim: task %s is blocked by %s", task.ID, dep), fmt.Sprintf("Complete %s first, then retry.", dep))
			os.Exit(ExitState)
			return nil
		}
	}

	task.Status = model.StatusWIP
	tf.UpdatedAt = time.Now().UTC()

	if err := model.WriteTaskFile(taskFilePath, tf); err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	output.Success(fmt.Sprintf("claimed %s", task.ID))
	return output.JSON(task)
}

type claimFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

func claimBatch(tf *model.TaskFile, taskFilePath string, ids []string) error {
	done := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			done[tf.Tasks[i].ID] = true
		}
	}

	claimed := make([]string, 0, len(ids))
	var failures []claimFailure

	for _, id := range ids {
		task, _, err := model.FindTask(tf, id)
		if err != nil {
			failures = append(failures, claimFailure{ID: id, Error: err.Error()})
			continue
		}

		if !model.ValidTransition(task.Status, model.StatusWIP) {
			failures = append(failures, claimFailure{
				ID:    id,
				Error: fmt.Sprintf("cannot claim: task %s is %s (must be open)", task.ID, task.Status),
			})
			continue
		}

		blocked := false
		for _, dep := range task.DependsOn {
			if !done[dep] {
				failures = append(failures, claimFailure{
					ID:    id,
					Error: fmt.Sprintf("cannot claim: task %s is blocked by %s", task.ID, dep),
				})
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		task.Status = model.StatusWIP
		claimed = append(claimed, id)
	}

	tf.UpdatedAt = time.Now().UTC()

	if writeErr := model.WriteTaskFile(taskFilePath, tf); writeErr != nil {
		output.Error(ExitFile, writeErr.Error())
		os.Exit(ExitFile)
		return nil
	}

	result := map[string]any{
		"claimed": claimed,
		"failed":  failures,
	}

	if jsonErr := output.JSON(result); jsonErr != nil {
		output.Error(ExitFile, jsonErr.Error())
	}

	if len(claimed) == 0 {
		os.Exit(ExitState)
	}
	return nil
}
