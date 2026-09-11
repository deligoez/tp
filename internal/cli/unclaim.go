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

func newUnclaimCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "unclaim <id> [id...]",
		Short: "Transition: wip → open (undo a claim)",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runUnclaim,
	}
}

// unclaimFailure is one id tp unclaim could not return to open. Hint names the
// command that does apply, so a batch caller need not re-derive it per id.
type unclaimFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
	Hint  string `json:"hint,omitempty"`
}

// unclaimResult is tp unclaim's payload, the same shape for one id or many.
// already_open is a no-op rather than a failure: the task is where the caller
// wanted it.
type unclaimResult struct {
	Unclaimed   []string         `json:"unclaimed"`
	AlreadyOpen []string         `json:"already_open"`
	Failed      []unclaimFailure `json:"failed"`
}

// runUnclaim is the only way back from wip short of closing the task. It exits
// 4 only when no id was unclaimed or already open, the rule tp claim's batch
// follows.
func runUnclaim(_ *cobra.Command, args []string) error {
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

		result := unclaimTasks(tf, args)

		// A run that unclaimed nothing leaves the file byte-identical, so a
		// no-op does not move updated_at.
		if len(result.Unclaimed) > 0 {
			tf.UpdatedAt = time.Now().UTC()
			if writeErr := model.WriteTaskFile(taskFilePath, tf); writeErr != nil {
				output.Error(ExitFile, writeErr.Error())
				os.Exit(ExitFile)
				return nil
			}
		}

		if jsonErr := output.JSON(result); jsonErr != nil {
			output.Error(ExitFile, jsonErr.Error())
		}

		if len(result.Unclaimed) == 0 && len(result.AlreadyOpen) == 0 {
			os.Exit(ExitState)
		}
		return nil
	})
}

// unclaimTasks moves every wip task named in ids back to open in tf, clearing
// what claiming set (started_at, duration_source), and sorts each id into the
// payload bucket it belongs to. It does not write the file.
func unclaimTasks(tf *model.TaskFile, ids []string) unclaimResult {
	result := unclaimResult{
		Unclaimed:   make([]string, 0, len(ids)),
		AlreadyOpen: make([]string, 0),
		Failed:      make([]unclaimFailure, 0),
	}
	for _, id := range ids {
		task, _, err := model.FindTask(tf, id)
		switch {
		case err != nil:
			result.Failed = append(result.Failed, unclaimFailure{ID: id, Error: err.Error()})
		case task.Status == model.StatusOpen:
			result.AlreadyOpen = append(result.AlreadyOpen, id)
		case task.Status == model.StatusWIP:
			task.Status = model.StatusOpen
			task.StartedAt = nil
			task.DurationSource = ""
			result.Unclaimed = append(result.Unclaimed, id)
		default:
			result.Failed = append(result.Failed, unclaimFailure{
				ID:    id,
				Error: fmt.Sprintf("cannot unclaim: task %s is %s (must be wip)", task.ID, task.Status),
				Hint:  backToOpenHint(task),
			})
		}
	}
	return result
}
