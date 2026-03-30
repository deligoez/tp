package cli

import (
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var (
	planFrom  string
	planLevel string
)

type planResult struct {
	Workflow       model.Workflow `json:"workflow"`
	ExecutionOrder any            `json:"execution_order"` // []planTask or []output.CompactTaskView
	Summary        planSummary   `json:"summary"`
}

type planTask struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Tags            []string `json:"tags,omitempty"`
	Acceptance      string   `json:"acceptance"`
	EstimateMinutes int      `json:"estimate_minutes"`
	DependsOn       []string `json:"depends_on"`
	SourceSections  []string `json:"source_sections,omitempty"`
	SourceLines     string   `json:"source_lines,omitempty"`
	SpecExcerpt     string   `json:"spec_excerpt,omitempty"`
}

type planSummary struct {
	Total             int `json:"total"`
	Remaining         int `json:"remaining"`
	Done              int `json:"done"`
	EstimatedMinutes  int `json:"estimated_minutes"`
	ParallelismLevels int `json:"parallelism_levels"`
}

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Full execution plan with context (THE primary command)",
		RunE:  runPlan,
	}
	cmd.Flags().StringVar(&planFrom, "from", "", "start from this task ID onward")
	cmd.Flags().StringVar(&planLevel, "level", "", "filter by parallelism levels (comma-separated: 0,1)")
	return cmd
}

func runPlan(_ *cobra.Command, _ []string) error {
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

	// Get sorted execution order
	sorted := engine.TopoSort(tf.Tasks)

	if len(sorted) == 0 {
		if err := output.JSON(map[string]any{"done": true, "message": "All tasks complete"}); err != nil {
			output.Error(ExitFile, err.Error())
		}
		os.Exit(ExitState)
		return nil
	}

	// Apply --from filter
	if planFrom != "" {
		idx := -1
		for i := range sorted {
			if sorted[i].ID == planFrom {
				idx = i
				break
			}
		}
		if idx >= 0 {
			sorted = sorted[idx:]
		}
		// If the from task is done, it's already excluded by TopoSort.
		// If not found, return all (no-op).
	}

	// Apply --level filter
	if planLevel != "" {
		levels := engine.ComputeParallelismLevels(tf.Tasks)
		allowedLevels := make(map[int]bool)
		for _, l := range strings.Split(planLevel, ",") {
			n, parseErr := strconv.Atoi(strings.TrimSpace(l))
			if parseErr == nil {
				allowedLevels[n] = true
			}
		}
		// Build set of task IDs at allowed levels
		allowedIDs := make(map[string]bool)
		for i := range levels {
			if allowedLevels[levels[i].Level] {
				for _, id := range levels[i].Tasks {
					allowedIDs[id] = true
				}
			}
		}
		var filtered []model.Task
		for i := range sorted {
			if allowedIDs[sorted[i].ID] {
				filtered = append(filtered, sorted[i])
			}
		}
		sorted = filtered
	}

	// Resolve spec path for excerpts
	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)

	// Build summary
	doneCount := 0
	totalEst := 0
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusDone {
			doneCount++
		}
	}
	for i := range sorted {
		totalEst += sorted[i].EstimateMinutes
	}
	parallelismLevels := len(engine.ComputeParallelismLevels(tf.Tasks))

	result := planResult{
		Workflow: tf.Workflow,
		Summary: planSummary{
			Total:             len(tf.Tasks),
			Remaining:         len(sorted),
			Done:              doneCount,
			EstimatedMinutes:  totalEst,
			ParallelismLevels: parallelismLevels,
		},
	}

	// Build execution order
	if flagCompact {
		compact := make([]output.CompactTaskView, len(sorted))
		for i := range sorted {
			compact[i] = output.CompactTask(&sorted[i])
		}
		result.ExecutionOrder = compact
	} else {
		tasks := make([]planTask, len(sorted))
		for i := range sorted {
			t := &sorted[i]
			tasks[i] = planTask{
				ID:              t.ID,
				Title:           t.Title,
				Description:     t.Description,
				Tags:            t.Tags,
				Acceptance:      t.Acceptance,
				EstimateMinutes: t.EstimateMinutes,
				DependsOn:       t.DependsOn,
				SourceSections:  t.SourceSections,
				SourceLines:     t.SourceLines,
				SpecExcerpt:     engine.ExtractSpecExcerpt(specPath, t.SourceLines),
			}
		}
		result.ExecutionOrder = tasks
	}

	return output.JSON(result)
}
