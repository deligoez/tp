package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
		exitIfClaimOverlaps(tf, task)
	} else {
		exitIfNoTasks(tf, taskFilePath)
		task = selectBriefTarget(tf)
		if task == nil {
			return nil // selectBriefTarget emitted the {done, message} shape and exited.
		}
	}

	return printBrief(tf, task, taskFilePath, specPath, priorSet)
}

// exitIfClaimOverlaps refuses the brief of an open task whose touches overlap
// a claimed task's, exit 4 with the {done, message} shape, while that claim
// stands: tp brief hands out work under the rule tp run's pool uses
// (spec/backlog/worktree-dispatch.md §2.3). A claimed task reads its own.
func exitIfClaimOverlaps(tf *model.TaskFile, task *model.Task) {
	if msg := claimOverlapRefusal(tf, task); msg != "" {
		_ = output.JSON(map[string]any{"done": false, "message": msg})
		os.Exit(ExitState)
	}
}

// claimOverlapRefusal is why the open task is not handed out (tp brief) or
// claimed (tp claim): its touches overlap a claimed task's
// (engine.ClaimOverlap). "" when task is not open or overlaps no claim.
func claimOverlapRefusal(tf *model.TaskFile, task *model.Task) string {
	if task.Status != model.StatusOpen {
		return ""
	}
	held := engine.ClaimOverlap(tf, task)
	if held == nil {
		return ""
	}
	return fmt.Sprintf("task %q is not handed out while task %q is claimed: their touches overlap; brief it once %q is done or released",
		task.ID, held.ID, held.ID)
}

// printBrief builds and prints one task's brief under the task file's
// effective commit strategy and quality gate (§8.1, §8.2).
func printBrief(tf *model.TaskFile, task *model.Task, taskFilePath, specPath string, priorSet bool) error {
	effective := resolveEffectiveStrategy(taskFilePath)
	override := tf.Workflow
	engine.ClampWorkflowRanges(&override)
	project := engine.ProjectWorkflowOverride(filepath.Dir(taskFilePath))
	wf := engine.ResolveWorkflowLayers(&override, &project)

	b, err := engine.BuildBrief(tf, task, specPath, effective, wf.QualityGate, briefPrior, priorSet, flagCompact)
	if err != nil {
		output.Error(ExitValidation, err.Error())
		os.Exit(ExitValidation)
		return nil
	}

	failureRoot := setBriefClose(b, task, taskFilePath, effective, wf.QualityGate)

	// v0.35.0 §4.2: the cycle's last failure, when one is present, so the unit
	// about to start reads it before it walks into the same wall.
	b.LastFailure = engine.ReadLastFailure(failureRoot, taskFilePath)
	b.Marker = engine.TaskMarker(taskFilePath, task.ID)
	if b.Marker != "" && failureRoot == "." {
		if token := spawnClaimToken(taskFilePath, task.ID); token != "" {
			b.ClaimToken = token
			// A main-tree task closes where it was claimed, so its close
			// names no token: the token is for an agent's own worktree.
			if !engine.TouchesProjectConfig(task) {
				b.Close = engine.CloseRecipeTextFor(effective, wf.QualityGate, token, engine.RepoRelativePath(taskFilePath))
			}
		}
	}

	if output.IsJSON() {
		return output.JSON(b)
	}
	fmt.Print(b.Text())
	return nil
}

// setBriefClose gives b the close recipe for where task runs and returns the
// root the cycle's last failure lives under: the main tree's, which in a
// worktree is not the working directory. A unit in a worktree beside others
// commits on its branch and asks for the close, which gates the worktree; the
// driver gates again on the main branch (spec/backlog/parallel-implement.md
// §2.4). A task whose touches name .tp/config.json runs in the main tree
// wherever its brief is printed (spec/backlog/worktree-dispatch.md §2.1).
func setBriefClose(b *engine.Brief, task *model.Task, taskFilePath, effective, gate string) string {
	root := donePendingRoot(taskFilePath)
	switch {
	case engine.TouchesProjectConfig(task):
		main := root
		if main == "" {
			main = engine.CanonicalProjectRoot(filepath.Dir(taskFilePath))
		}
		b.Close = engine.MainTreeCloseRecipeText(effective, gate, main)
	case root != "":
		b.Close = engine.WorktreeCloseRecipeText(effective, gate, task.ID, task.Touches, engine.WorktreeRoot("."))
	}
	if root == "" {
		return "."
	}
	return root
}

// spawnClaimToken mints the claim token of a brief an Agent spawn will claim
// from: none inside a tp run unit, whose hook does not claim, and none in an
// agent's worktree whose task another tree holds, where the agent already has
// its token (spec/backlog/a-land-request-names-its-agent.md).
func spawnClaimToken(taskFilePath, id string) string {
	if os.Getenv(engine.EnvUnitKind) != "" || engine.ClaimHolderCopy(taskFilePath, id) != "" {
		return ""
	}
	return engine.NewClaimToken()
}

// selectBriefTarget picks the read-only brief target when no id is given (§9.1):
// the in-progress task if one exists, else the next ready task by tp next's
// ordering (dependent-count desc, estimate asc, id asc). It returns nil after
// emitting the {done, message} shape tp next uses and exiting ExitState when no
// task is available. It never claims or writes.
func selectBriefTarget(tf *model.TaskFile) *model.Task {
	// In-progress task first (resume), mirroring tp next; a task an agent
	// spawn holds is that agent's.
	if wip := engine.ResumableWIP(tf); wip != nil {
		return wip
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
		if tf.Tasks[i].Startable(done) && engine.ClaimOverlap(tf, &tf.Tasks[i]) == nil {
			ready = append(ready, &tf.Tasks[i])
		}
	}

	if len(ready) == 0 {
		// The same payload tp next gives: blocked, held and overlapping
		// a held task's touches, each named.
		exitNoReady(tf, done)
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
