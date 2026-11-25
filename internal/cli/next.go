package cli

import (
	"fmt"
	"os"
	"path/filepath"
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
	nextBrief   bool
)

func newNextCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Get or resume WIP task, or claim next ready (fallback)",
		Long: `Returns one task with full context. If a WIP task exists, resumes it (idempotent).
Otherwise claims the highest-priority ready task. Exit 4 when no tasks remain.
--peek differs: it previews the next ready task and ignores WIP, so it can name
a different task than plain tp next, tp brief or tp resume.
Output: {task, spec_excerpt, blocks, remaining, quality_gate}`,
		Example: `  tp next --json                  # get task with full context
  tp next --peek                  # preview without claiming
  tp next --brief --json          # claim next task and return its brief`,
		RunE: runNext,
	}
	cmd.Flags().BoolVar(&nextPeek, "peek", false, "preview the next ready task without claiming; ignores WIP, unlike plain tp next")
	cmd.Flags().BoolVar(&nextMinimal, "minimal", false, "minimal output: only id + acceptance (always JSON)")
	cmd.Flags().BoolVar(&nextBrief, "brief", false, "claim the next ready task and return its implementation brief")
	addEpicFileFlag(cmd)
	return cmd
}

// withAgentHeld adds to a no-ready-task answer the tasks agent spawns hold,
// and the coordinator's step: their landing and the pool, not a resume.
func withAgentHeld(tf *model.TaskFile, out map[string]any) map[string]any {
	if held := engine.AgentHeld(tf); len(held) > 0 {
		out["agent_held"] = held
		out["hint"] = "agents hold these tasks: run tp land when one finishes, and tp pool --session <your session id> to refill the free slots"
	}
	return out
}

func runNext(_ *cobra.Command, _ []string) error {
	// §9.2: --brief and --minimal are mutually exclusive (one assembles context,
	// the other strips it); a usage error (exit 2), checked before any file work.
	if nextBrief && nextMinimal {
		output.Error(ExitUsage, "--brief and --minimal are mutually exclusive")
		os.Exit(ExitUsage)
		return nil
	}
	applyEpicFileFlag()

	// --peek never writes, so it resolves without the write notice.
	discover := discoverWriteTarget
	if nextPeek {
		discover = func() (string, error) { return engine.DiscoverTaskFile(".", flagFile) }
	}
	taskFilePath, err := discover()
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
		// Check for WIP task first (resume); a task an agent spawn holds is
		// that agent's.
		if wip := engine.ResumableWIP(tf); wip != nil {
			return outputNextTask(tf, wip, taskFilePath, specPath)
		}
	}
	exitIfNoTasks(tf, taskFilePath)
	exitIfNextBlocked(tf, taskFilePath)

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

	// A task whose touches overlap a claimed task's is passed over, as tp
	// brief passes over it (engine.ClaimOverlap).
	var ready []*model.Task
	for i := range tf.Tasks {
		if tf.Tasks[i].Startable(done) && engine.ClaimOverlap(tf, &tf.Tasks[i]) == nil {
			ready = append(ready, &tf.Tasks[i])
		}
	}

	if len(ready) == 0 {
		exitNoReady(tf, done)
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
			if t.Status != model.StatusOpen || t.Waiting != "" {
				// Claimed or held by another agent between read and lock
				output.Error(ExitState, fmt.Sprintf("task %s already claimed or held", t.ID))
				os.Exit(ExitState)
				return nil
			}
			now := time.Now().UTC()
			t.Status = model.StatusWIP
			t.StartedAt = &now
			t.DurationSource = model.DurationSourceClaimed
			if writeErr := model.WriteTaskFile(taskFilePath, tf2); writeErr != nil {
				output.Error(ExitFile, writeErr.Error())
				os.Exit(ExitFile)
				return nil
			}
			return outputNextTask(tf2, t, taskFilePath, specPath)
		})
	}

	return outputNextTask(tf, task, taskFilePath, specPath)
}

func outputNextTask(tf *model.TaskFile, task *model.Task, taskFilePath, specPath string) error {
	if nextBrief {
		return outputNextBrief(tf, task, specPath, taskFilePath)
	}

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
		"file":         taskFileLabel(taskFilePath),
	}

	if !flagCompact {
		result["spec_excerpt"] = engine.ExtractSpecExcerptForTask(specPath, task.SourceLines, task.SourceSections)
	}

	return output.JSON(result)
}

// outputNextBrief builds and emits the implementation brief for task, mirroring
// the tail of runBrief. The caller has already claimed the task (§9.2): tp next
// --brief takes ownership and delivers the brief in one call. Under JSON it emits
// the structured brief; otherwise the rendered text brief (tp brief's contract).
func outputNextBrief(tf *model.TaskFile, task *model.Task, specPath, taskFilePath string) error {
	effective := resolveEffectiveStrategy(taskFilePath)
	override := tf.Workflow
	engine.ClampWorkflowRanges(&override)
	project := engine.ProjectWorkflowOverride(filepath.Dir(taskFilePath))
	wf := engine.ResolveWorkflowLayers(&override, &project)
	b, err := engine.BuildBrief(tf, task, specPath, effective, wf.QualityGate, 0, false, flagCompact)
	if err != nil {
		output.Error(ExitValidation, err.Error())
		os.Exit(ExitValidation)
		return nil
	}
	b.LastFailure = engine.ReadLastFailure(".", taskFilePath)
	if output.IsJSON() {
		return output.JSON(b)
	}
	fmt.Print(b.Text())
	return nil
}

// nextOverlap is a task tp next passed over although it is ready: its
// touches overlap those of HeldBy, a task a spawn or session holds
// (engine.ClaimOverlap).
type nextOverlap struct {
	ID     string `json:"id"`
	HeldBy string `json:"held_by"`
}

// exitNoReady prints why a file has no ready task and exits 4, or, when
// nothing is open or wip, says every task is complete.
func exitNoReady(tf *model.TaskFile, done map[string]bool) {
	blocked, held, overlapping, allDone := notReadyIDs(tf, done)
	if allDone {
		_ = output.JSON(map[string]any{"done": true, "message": "All tasks complete"})
		os.Exit(ExitState)
	}
	out := map[string]any{
		"done": false,
		"message": fmt.Sprintf("No ready tasks. %d tasks blocked, %d held, %d overlapping a held task.",
			len(blocked), len(held), len(overlapping)),
		"blocked": blocked,
	}
	if len(held) > 0 {
		out["held"] = held
	}
	if len(overlapping) > 0 {
		out["overlapping"] = overlapping
	}
	_ = output.JSON(withAgentHeld(tf, out))
	os.Exit(ExitState)
}

// notReadyIDs splits the open tasks of a file with no ready task: blocked by
// a dependency, held by a gate that is not a task, or startable but
// overlapping a held task's touches. An unmet dependency outranks the hold,
// as in tp status. allDone is true when nothing is open or wip.
func notReadyIDs(tf *model.TaskFile, done map[string]bool) (blocked, held []string, overlapping []nextOverlap, allDone bool) {
	blocked = []string{}
	allDone = true
	for i := range tf.Tasks {
		t := &tf.Tasks[i]
		if t.Status == model.StatusDone {
			continue
		}
		allDone = false
		switch {
		case t.Status != model.StatusOpen:
		case !t.DepsDone(done):
			blocked = append(blocked, t.ID)
		case t.Held():
			held = append(held, t.ID)
		default:
			if holder := engine.ClaimOverlap(tf, t); holder != nil {
				overlapping = append(overlapping, nextOverlap{ID: t.ID, HeldBy: holder.ID})
			}
		}
	}
	return blocked, held, overlapping, allDone
}
