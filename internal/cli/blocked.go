package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

type blockedTask struct {
	model.Task
	WaitingOn []waitingDep `json:"waiting_on"`
}

type waitingDep struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func newBlockedCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "blocked",
		Short: "Tasks waiting on unsatisfied deps",
		RunE:  runBlocked,
	}
}

func runBlocked(_ *cobra.Command, _ []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	statusMap := make(map[string]string)
	for i := range tf.Tasks {
		statusMap[tf.Tasks[i].ID] = tf.Tasks[i].Status
	}

	blocked := make([]blockedTask, 0)
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusOpen {
			continue
		}
		var waiting []waitingDep
		for _, dep := range t.DependsOn {
			if statusMap[dep] != model.StatusDone {
				waiting = append(waiting, waitingDep{ID: dep, Status: statusMap[dep]})
			}
		}
		if len(waiting) > 0 {
			blocked = append(blocked, blockedTask{Task: *t, WaitingOn: waiting})
		}
	}

	return output.JSON(blocked)
}
