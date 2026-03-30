package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

type showResult struct {
	model.Task
	Blocks      []string `json:"blocks"`
	IsReady     bool     `json:"is_ready"`
	SpecExcerpt string   `json:"spec_excerpt,omitempty"`
}

func newShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show task details",
		Args:  cobra.ExactArgs(1),
		RunE:  runShow,
	}
}

func runShow(_ *cobra.Command, args []string) error {
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

	task, _, err := model.FindTask(tf, args[0])
	if err != nil {
		output.Error(ExitState, err.Error())
		os.Exit(ExitState)
		return nil
	}

	// Compute blocks (reverse deps)
	blocks := make([]string, 0)
	for i := range tf.Tasks {
		for _, dep := range tf.Tasks[i].DependsOn {
			if dep == task.ID {
				blocks = append(blocks, tf.Tasks[i].ID)
			}
		}
	}

	// Compute is_ready
	done := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			done[tf.Tasks[i].ID] = true
		}
	}
	isReady := task.Status == model.StatusOpen
	for _, dep := range task.DependsOn {
		if !done[dep] {
			isReady = false
			break
		}
	}

	result := showResult{
		Task:    *task,
		Blocks:  blocks,
		IsReady: isReady,
	}

	return output.JSON(result)
}
