package cli

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"

	"github.com/deligoez/tp/internal/engine"
	"github.com/deligoez/tp/internal/model"
	"github.com/deligoez/tp/internal/output"
)

var briefPrior int

func newBriefCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "brief [id]",
		Short: "Print the unit brief for one task (read-only)",
		Long: `Print the five-section implementation brief for one unit (Identity, Scope,
Prior work, Your unit, How to close). Read-only: it claims nothing and mutates
nothing, so an orchestrator may produce a brief before deciding to spawn.

With no argument it targets the in-progress task, else the next ready task by the
same ordering tp next uses. With no task available, or for an unknown id, it
exits 4 with the {done, message} shape tp next uses.`,
		Args: cobra.MaximumNArgs(1),
		RunE: runBrief,
	}
	cmd.Flags().IntVar(&briefPrior, "prior", 0, "override the recency count of prior-work entries (range 0-20)")
	return cmd
}

func runBrief(cmd *cobra.Command, args []string) error {
	taskFilePath, err := engine.DiscoverTaskFile(".", flagFile)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	// --prior range check (§5.4): an out-of-range value is a usage error (exit 2).
	priorSet := cmd.Flags().Changed("prior")
	if err := engine.ValidatePriorCount(briefPrior, priorSet); err != nil {
		output.Error(ExitUsage, err.Error())
		os.Exit(ExitUsage)
		return nil
	}

	tf, err := model.ReadTaskFile(taskFilePath)
	if err != nil {
		output.Error(ExitFile, err.Error())
		os.Exit(ExitFile)
		return nil
	}

	specPath, _ := engine.ResolveSpecPath(taskFilePath, tf.Spec)

	// Select the target task. Read-only: nothing is claimed or written.
	var task *model.Task
	if len(args) == 1 {
		t, _, findErr := model.FindTask(tf, args[0])
		if findErr != nil {
			// §9.1: an unknown explicit id exits 4 (state) with the {done, message} shape.
			_ = output.JSON(map[string]any{"done": false, "message": fmt.Sprintf("task %q not found", args[0])})
			os.Exit(ExitState)
			return nil
		}
		task = t
	} else {
		task = selectBriefTarget(tf)
		if task == nil {
			return nil // selectBriefTarget emitted the {done, message} shape and exited.
		}
	}

	// Resolve the effective commit strategy (auto → hc/builtin) and the resolved
	// quality_gate command verbatim (§8.1, §8.2).
	effective := resolveEffectiveStrategy(taskFilePath)
	override := tf.Workflow
	engine.ClampWorkflowRanges(&override)
	wf := engine.ResolveWorkflowLayers(override, engine.ProjectWorkflowOverride("."))

	b, err := engine.BuildBrief(tf, task, specPath, effective, wf.QualityGate, briefPrior, priorSet, flagCompact)
	if err != nil {
		output.Error(ExitValidation, err.Error())
		os.Exit(ExitValidation)
		return nil
	}

	if output.IsJSON() {
		return output.JSON(b)
	}
	fmt.Print(b.Text())
	return nil
}

// selectBriefTarget picks the read-only brief target when no id is given (§9.1):
// the in-progress task if one exists, else the next ready task by tp next's
// ordering (dependent-count desc, estimate asc, id asc). It returns nil after
// emitting the {done, message} shape tp next uses and exiting ExitState when no
// task is available. It never claims or writes.
func selectBriefTarget(tf *model.TaskFile) *model.Task {
	// In-progress task first (resume), mirroring tp next.
	for i := range tf.Tasks {
		if tf.Tasks[i].Status == model.StatusWIP {
			return &tf.Tasks[i]
		}
	}

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

	return ready[0]
}
