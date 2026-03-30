package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

type statusResult struct {
	Total           int `json:"total"`
	Open            int `json:"open"`
	WIP             int `json:"wip"`
	Done            int `json:"done"`
	Blocked         int `json:"blocked"`
	Ready           int `json:"ready"`
	ProgressPercent int `json:"progress_percent"`
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Summary: open/wip/done counts",
		RunE:  runStatus,
	}
}

func runStatus(_ *cobra.Command, _ []string) error {
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

	done := make(map[string]bool)
	result := statusResult{Total: len(tf.Tasks)}

	for i := range tf.Tasks {
		switch tf.Tasks[i].Status {
		case model.StatusOpen:
			result.Open++
		case model.StatusWIP:
			result.WIP++
		case model.StatusDone:
			result.Done++
			done[tf.Tasks[i].ID] = true
		}
	}

	// Compute blocked and ready
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
			result.Ready++
		} else {
			result.Blocked++
		}
	}

	if result.Total > 0 {
		result.ProgressPercent = result.Done * 100 / result.Total
	}

	return output.JSON(result)
}
