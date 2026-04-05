package cli

import (
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	nextPeek    bool
	nextMinimal bool
)

func newNextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Get or resume WIP task, or claim next ready (fallback)",
		Long: `Returns one task with full context. If a WIP task exists, resumes it (idempotent).
Otherwise claims the highest-priority ready task. Exit 4 when no tasks remain.
Output: {task, spec_excerpt, blocks, remaining, quality_gate}`,
		Example: `  tp next --json                  # get task with full context
  tp next --peek                  # preview without claiming`,
		RunE: runNext,
	}
	cmd.Flags().BoolVar(&nextPeek, "peek", false, "preview next ready without claiming")
	cmd.Flags().BoolVar(&nextMinimal, "minimal", false, "minimal output: only id + acceptance (always JSON)")
	return cmd
}

func runNext(_ *cobra.Command, _ []string) error {
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

	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)

	// If --peek, always show next ready (ignore WIP)
	if !nextPeek {
		// Check for WIP task first (resume)
		for i := range tf.Tasks {
			if tf.Tasks[i].Status == model.StatusWIP {
				return outputNextTask(tf, &tf.Tasks[i], specPath)
			}
		}
	}

	// Find ready tasks with priority ordering
	done := make(map[string]bool)
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			done[tf.Tasks[i].ID] = true
		}
	}

	dependentCount := make(map[string]int)
	for i := range tf.Tasks {
		for _, dep := range tf.Tasks[i].DependsOn {
			dependentCount[dep]++
		}
	}

	var ready []*model.Task
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
			ready = append(ready, &tf.Tasks[i])
		}
	}

	if len(ready) == 0 {
		// Check if all done or blocked
		allDone := true
		blocked := make([]string, 0)
		for i := range tf.Tasks {
			if tf.Tasks[i].Status != model.StatusDone {
				allDone = false
				if tf.Tasks[i].Status == model.StatusOpen {
					blocked = append(blocked, tf.Tasks[i].ID)
				}
			}
		}
		if allDone {
			_ = output.JSON(map[string]any{"done": true, "message": "All tasks complete"})
		} else {
			_ = output.JSON(map[string]any{
				"done":    false,
				"message": fmt.Sprintf("No ready tasks. %d tasks blocked.", len(blocked)),
				"blocked": blocked,
			})
		}
		os.Exit(ExitState)
		return nil
	}

	// Sort by priority
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

	task := ready[0]

	if !nextPeek {
		// Claim the task (need file lock for write)
		return engine.WithFileLock(taskFilePath, func() error {
			// Re-read to avoid stale data
			tf2, readErr := model.ReadTaskFile(taskFilePath)
			if readErr != nil {
				output.Error(ExitFile, readErr.Error())
				os.Exit(ExitFile)
				return nil
			}
			t, _, findErr := model.FindTask(tf2, task.ID)
			if findErr != nil {
				output.Error(ExitState, findErr.Error())
				os.Exit(ExitState)
				return nil
			}
			if t.Status != model.StatusOpen {
				// Already claimed by another agent between read and lock
				output.Error(ExitState, fmt.Sprintf("task %s already claimed", t.ID))
				os.Exit(ExitState)
				return nil
			}
			now := time.Now().UTC()
			t.Status = model.StatusWIP
			t.StartedAt = &now
			if writeErr := model.WriteTaskFile(taskFilePath, tf2); writeErr != nil {
				output.Error(ExitFile, writeErr.Error())
				os.Exit(ExitFile)
				return nil
			}
			return outputNextTask(tf2, t, specPath)
		})
	}

	return outputNextTask(tf, task, specPath)
}

func outputNextTask(tf *model.TaskFile, task *model.Task, specPath string) error {
	if nextMinimal {
		return output.JSON(map[string]any{
			"id":         task.ID,
			"acceptance": task.Acceptance,
		})
	}

	// Compute blocks
	blocks := make([]string, 0)
	for i := range tf.Tasks {
		for _, dep := range tf.Tasks[i].DependsOn {
			if dep == task.ID {
				blocks = append(blocks, tf.Tasks[i].ID)
			}
		}
	}

	// Compute remaining
	openCount, wipCount, doneCount := 0, 0, 0
	for i := range tf.Tasks {
		switch tf.Tasks[i].Status {
		case model.StatusOpen:
			openCount++
		case model.StatusWIP:
			wipCount++
		case model.StatusDone:
			doneCount++
		}
	}

	result := map[string]any{
		"task":         task,
		"blocks":       blocks,
		"remaining":    map[string]any{"total": len(tf.Tasks), "open": openCount, "wip": wipCount, "done": doneCount},
		"quality_gate": tf.Workflow.QualityGate,
	}

	if !flagCompact {
		result["spec_excerpt"] = engine.ExtractSpecExcerpt(specPath, task.SourceLines)
	}

	return output.JSON(result)
}
