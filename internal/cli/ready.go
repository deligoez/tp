package cli

import (
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	readyFirst bool
	readyCount bool
	readyIDs   bool
)

func newReadyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ready",
		Short: "List tasks with all deps satisfied",
		RunE:  runReady,
	}
	cmd.Flags().BoolVar(&readyFirst, "first", false, "single highest-priority ready task")
	cmd.Flags().BoolVar(&readyCount, "count", false, "just the count")
	cmd.Flags().BoolVar(&readyIDs, "ids", false, "just IDs")
	return cmd
}

func runReady(_ *cobra.Command, _ []string) error {
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

	// Find done task IDs
	done := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			done[tf.Tasks[i].ID] = true
		}
	}

	// Find ready tasks: open with all deps done
	var ready []model.Task
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status != model.StatusOpen {
			continue
		}
		allDone := true
		for _, dep := range t.DependsOn {
			if !done[dep] {
				allDone = false
				break
			}
		}
		if allDone {
			ready = append(ready, *t)
		}
	}

	// Count dependents for priority
	dependentCount := make(map[string]int)
	for i := range tf.Tasks {
		for _, dep := range tf.Tasks[i].DependsOn {
			dependentCount[dep]++
		}
	}

	// Sort: dependents desc, estimate asc, alpha
	sort.Slice(ready, func(i, j int) bool {
		di := dependentCount[ready[i].ID]
		dj := dependentCount[ready[j].ID]
		if di != dj {
			return di > dj
		}
		if ready[i].EstimateMinutes != ready[j].EstimateMinutes {
			return ready[i].EstimateMinutes < ready[j].EstimateMinutes
		}
		return ready[i].ID < ready[j].ID
	})

	if readyCount {
		return output.JSON(len(ready))
	}

	if readyIDs {
		ids := make([]string, len(ready))
		for i := range ready {
			ids[i] = ready[i].ID
		}
		return output.JSON(ids)
	}

	if readyFirst {
		if len(ready) == 0 {
			output.Error(ExitState, "no ready tasks")
			os.Exit(ExitState)
			return nil
		}
		return output.JSON(ready[0])
	}

	return output.JSON(ready)
}
